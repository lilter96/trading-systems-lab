using System.Globalization;
using System.Text.Json;
using SMC.Engine.Core.Analysis;

namespace SMC.Engine.Service.Services;

public sealed class HistoricalDataLoader : BackgroundService
{
    private readonly DashboardStateService _state;
    private readonly ILogger<HistoricalDataLoader> _logger;
    private readonly HttpClient _http;
    private readonly IMarketStructureService _structureService;
    private readonly ISmtDivergenceService _smtService;
    private readonly IPoiZoneService _poiZoneService;

    private static readonly string[] HigherTfs = { "3m", "5m", "15m", "30m", "1h", "4h", "1d" };
    private static readonly Dictionary<string, int> TfBarCount = new()
    {
        ["3m"] = 3, ["5m"] = 5, ["15m"] = 15, ["30m"] = 30, ["1h"] = 60, ["4h"] = 240, ["1d"] = 1440,
    };

    public HistoricalDataLoader(
        DashboardStateService state,
        ILogger<HistoricalDataLoader> logger,
        HttpClient http,
        IMarketStructureService structureService,
        ISmtDivergenceService smtService,
        IPoiZoneService poiZoneService)
    {
        _state = state;
        _logger = logger;
        _http = http;
        _structureService = structureService;
        _smtService = smtService;
        _poiZoneService = poiZoneService;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        await Task.Delay(2000, stoppingToken);

        try
        {
            var query = "SELECT timestamp, open, high, low, close, volume FROM trading.aggregated_bars WHERE timeframe = '1m' AND symbol = 'BTC-USDT' AND complete = 1 ORDER BY timestamp DESC LIMIT 10000 FORMAT JSONEachRow";
            var content = new StringContent(query);
            var response = await _http.PostAsync("http://localhost:8123/", content, stoppingToken);
            response.EnsureSuccessStatusCode();
            var json = await response.Content.ReadAsStringAsync(stoppingToken);

            var records = new List<BarRecord>(10000);
            foreach (var line in json.Split('\n', StringSplitOptions.RemoveEmptyEntries))
            {
                using var doc = JsonDocument.Parse(line);
                var row = doc.RootElement;
                records.Add(new BarRecord(
                    DateTimeOffset.Parse(row.GetProperty("timestamp").GetString()!, null, DateTimeStyles.AssumeUniversal),
                    row.GetProperty("open").GetDouble(),
                    row.GetProperty("high").GetDouble(),
                    row.GetProperty("low").GetDouble(),
                    row.GetProperty("close").GetDouble(),
                    row.GetProperty("volume").GetDouble()));
            }
            records.Reverse();

            if (records.Count == 0) return;
            _logger.LogInformation("Loaded {Count} historical 1m bars, replaying through SMC pipeline...", records.Count);

            var oneMinBars = new List<BarDataProto>(records.Count);
            foreach (var r in records)
            {
                var barProto = new BarDataProto(
                    r.Timestamp, "1m",
                    (decimal)r.Open, (decimal)r.High, (decimal)r.Low,
                    (decimal)r.Close, (decimal)r.Volume,
                    Instrument: "BTC-USDT");

                var breaks = await _structureService.FeedBarAsync("BTC-USDT", barProto);
                _smtService.ProcessBar("BTC-USDT", barProto, breaks);
                _poiZoneService.ProcessBar("BTC-USDT", barProto, breaks);
                oneMinBars.Add(barProto);
            }

            var htLastTs = await ReplayHigherTimeframes(oneMinBars);

            _state.LastReplayedOneMinBarTs = oneMinBars[^1].Timestamp;
            foreach (var kv in htLastTs)
                _state.LastReplayedHtBarTss[kv.Key] = kv.Value;

            _state.LoadHistoricalBars(records);

            var allPois = _poiZoneService.GetAllActivePOIs("BTC-USDT");
            _state.SetActivePois(allPois);
            _logger.LogInformation("Historical SMC replay complete, {Count} active POIs", allPois.Count);
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "ClickHouse historical load failed (OK if ClickHouse is not yet populated)");
        }
    }

    private async Task<Dictionary<string, DateTimeOffset>> ReplayHigherTimeframes(List<BarDataProto> oneMinBars)
    {
        var lastTs = new Dictionary<string, DateTimeOffset>();
        foreach (var tf in HigherTfs)
        {
            var barsNeeded = TfBarCount[tf];
            var candleCount = oneMinBars.Count / barsNeeded;
            if (candleCount == 0) continue;

            _logger.LogDebug("Replaying {Count} {Tf} candles through SMC pipeline", candleCount, tf);

            for (int i = 0; i < candleCount; i++)
            {
                var startIdx = i * barsNeeded;
                var group = oneMinBars.GetRange(startIdx, barsNeeded);
                var aggBar = new BarDataProto(
                    group[^1].Timestamp, tf,
                    group[0].Open,
                    group.Max(b => b.High),
                    group.Min(b => b.Low),
                    group[^1].Close,
                    group.Sum(b => b.Volume),
                    Instrument: "BTC-USDT");

                var breaks = await _structureService.FeedBarAsync("BTC-USDT", aggBar);
                _poiZoneService.ProcessBar("BTC-USDT", aggBar, breaks);

                lastTs[tf] = aggBar.Timestamp;
            }
        }
        return lastTs;
    }
}
