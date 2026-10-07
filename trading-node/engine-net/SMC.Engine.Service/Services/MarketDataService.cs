using System.Globalization;
using Grpc.Core;
using Marketdata;
using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Models;

namespace SMC.Engine.Service.Services;

public class MarketDataService : MarketDataFeed.MarketDataFeedBase
{
    private readonly ILogger<MarketDataService> _logger;
    private readonly IMarketStructureService _structureService;
    private readonly ISmtDivergenceService _smtService;
    private readonly IPoiZoneService _poiZoneService;
    private readonly ITriggerService _triggerService;
    private readonly DashboardStateService _dashboard;
    private readonly IngestionTracker _tracker;

    private static readonly string[] HigherTimeframes = { "3m", "5m", "15m", "30m", "1h", "4h", "1d" };
    private static readonly Dictionary<string, int> TfBarCount = new()
    {
        ["3m"] = 3, ["5m"] = 5, ["15m"] = 15, ["30m"] = 30, ["1h"] = 60, ["4h"] = 240, ["1d"] = 1440,
    };
    private readonly List<BarDataProto> _oneMinBars = new();

    public MarketDataService(
        ILogger<MarketDataService> logger,
        IMarketStructureService structureService,
        ISmtDivergenceService smtService,
        IPoiZoneService poiZoneService,
        ITriggerService triggerService,
        DashboardStateService dashboard,
        IngestionTracker tracker)
    {
        _logger = logger;
        _structureService = structureService;
        _smtService = smtService;
        _poiZoneService = poiZoneService;
        _triggerService = triggerService;
        _dashboard = dashboard;
        _tracker = tracker;
    }

    public override async Task StreamBars(
        IAsyncStreamReader<BarData> requestStream,
        IServerStreamWriter<Common.CommandAck> responseStream,
        ServerCallContext context)
    {
        _logger.LogInformation("MarketDataFeed client connected");

        await foreach (var bar in requestStream.ReadAllAsync(context.CancellationToken))
        {
            _logger.LogInformation(
                "Bar received: timeframe={Timeframe} ts={Timestamp} O={Open} H={High} L={Low} C={Close} V={Volume}",
                bar.Timeframe, bar.Timestamp, bar.Open, bar.High, bar.Low, bar.Close, bar.Volume);

            var barTs = DateTimeOffset.FromUnixTimeMilliseconds(bar.Timestamp);
            var barProto = new BarDataProto(
                barTs,
                bar.Timeframe,
                decimal.Parse(bar.Open, CultureInfo.InvariantCulture),
                decimal.Parse(bar.High, CultureInfo.InvariantCulture),
                decimal.Parse(bar.Low, CultureInfo.InvariantCulture),
                decimal.Parse(bar.Close, CultureInfo.InvariantCulture),
                decimal.Parse(bar.Volume, CultureInfo.InvariantCulture),
                Instrument: "BTC-USDT");

            if (_dashboard.LastReplayedOneMinBarTs.HasValue && barTs <= _dashboard.LastReplayedOneMinBarTs.Value)
            {
                _logger.LogDebug("Skipping already-replayed 1m bar {Ts}", barTs);
                await responseStream.WriteAsync(new Common.CommandAck
                {
                    CommandId = $"ack-{bar.Timestamp}",
                    Status = "skipped",
                    Message = $"bar {bar.Timestamp} already replayed",
                    Timestamp = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()
                });
                continue;
            }

            var breaks = await _structureService.FeedBarAsync("BTC-USDT", barProto);

            foreach (var sb in breaks)
            {
                _logger.LogInformation(
                    "Structure break: {EventType} bias={Bias} tf={Tf} price={Price}",
                    sb.EventType, sb.ResultingBias, sb.Timeframe, sb.BreakClosePrice);
            }

            var divergences = _smtService.ProcessBar("BTC-USDT", barProto, breaks);
            foreach (var d in divergences)
            {
                _logger.LogInformation(
                    "SMT divergence: {Primary}/{Secondary} dir={Dir} state={State} corr={Corr}",
                    d.PrimaryInstrument, d.SecondaryInstrument,
                    d.Direction, d.SecondaryState, d.CorrelationAtEvent);
            }

            var zones = _poiZoneService.ProcessBar("BTC-USDT", barProto, breaks);
            foreach (var z in zones)
            {
                _logger.LogInformation(
                    "POI {Type} {Bias} status={Status} zone=[{Lo},{Hi}] tf={Tf}",
                    z.Type, z.Bias, z.Status, z.LowerBound, z.UpperBound, z.Timeframe);
            }

            _oneMinBars.Add(barProto);
            if (_oneMinBars.Count > 15000)
                _oneMinBars.RemoveRange(0, _oneMinBars.Count - 10000);
            await ScanHigherTimeframes("BTC-USDT");

            var activePois = _poiZoneService.GetActivePOIs("BTC-USDT", barProto.Timeframe);
            var evaluations = _triggerService.Evaluate("BTC-USDT", barProto, activePois, breaks, divergences);
            foreach (var e in evaluations)
            {
                _logger.LogInformation(
                    "Trigger evaluated: zone=[{Lo},{Hi}] pos={Pos} abs={Abs} oi={Oi} smt={Smt} poiQ={PoiQ} smc={Smc} conf={Conf}",
                    e.Zone.LowerBound, e.Zone.UpperBound, e.Positioning,
                    e.AbsorptionScore, e.OiScore, e.SmtScore,
                    e.PoiQualityScore, e.SmcSetupScore, e.ConfluenceScore);
            }

            _dashboard.RecordHeartbeat("gateway-go");
            _dashboard.RecordHeartbeat("engine-net");
            _dashboard.RecordBarReceived(barProto);
            _tracker.IncrementProcessed();
            _dashboard.SetActivePois(_poiZoneService.GetAllActivePOIs("BTC-USDT"));

            var ack = new Common.CommandAck
            {
                CommandId = $"ack-{bar.Timestamp}",
                Status = "received",
                Message = $"bar {bar.Timestamp} processed",
                Timestamp = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()
            };

            await responseStream.WriteAsync(ack);
        }

        _logger.LogInformation("MarketDataFeed client disconnected");
    }

    private async Task ScanHigherTimeframes(string instrument)
    {
        foreach (var tf in HigherTimeframes)
        {
            var barsNeeded = TfBarCount[tf];
            var currentCandleCount = _oneMinBars.Count / barsNeeded;
            if (currentCandleCount == 0) continue;

            var startIdx = (currentCandleCount - 1) * barsNeeded;
            var group = _oneMinBars.GetRange(startIdx, barsNeeded);

            var aggBar = new BarDataProto(
                group[^1].Timestamp,
                tf,
                group[0].Open,
                group.Max(b => b.High),
                group.Min(b => b.Low),
                group[^1].Close,
                group.Sum(b => b.Volume),
                Instrument: instrument);

            if (_dashboard.LastReplayedHtBarTss.TryGetValue(tf, out var lastHtTs) && aggBar.Timestamp <= lastHtTs)
                continue;

            var htBreaks = await _structureService.FeedBarAsync(instrument, aggBar);
            var htZones = _poiZoneService.ProcessBar(instrument, aggBar, htBreaks);

            foreach (var z in htZones)
            {
                _logger.LogInformation(
                    "HT POI: {Type} {Bias} tf={Tf} zone=[{Lo},{Hi}]",
                    z.Type, z.Bias, z.Timeframe, z.LowerBound, z.UpperBound);
            }
        }
    }
}
