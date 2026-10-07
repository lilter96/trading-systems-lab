using System.Collections.Concurrent;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using SMC.Engine.Core.Models;
using SMC.Engine.Core.Repositories;

namespace SMC.Engine.Core.Analysis;

internal sealed class InstrumentTrackers
{
    public TimeframeAggregator LtfAgg { get; }
    public TimeframeAggregator HtfAgg { get; }
    public SwingDetector LtfDetector { get; }
    public SwingDetector HtfDetector { get; }

    public InstrumentTrackers(string instrument, int ltfMinutes, int htfMinutes, int ltfN, int htfN, ILogger? logger)
    {
        LtfAgg = new TimeframeAggregator(ltfMinutes);
        HtfAgg = new TimeframeAggregator(htfMinutes);
        LtfDetector = new SwingDetector(ltfN, $"{ltfMinutes}m", logger, instrument);
        HtfDetector = new SwingDetector(htfN, $"{htfMinutes}m", logger, instrument);
    }
}

public class MarketStructureService : IMarketStructureService
{
    private readonly IServiceProvider _serviceProvider;
    private readonly ILogger<MarketStructureService> _logger;

    private readonly ConcurrentDictionary<string, InstrumentTrackers> _instruments = new();

    private int _ltfMinutes = 15;
    private int _htfMinutes = 60;
    private int _ltfN = 3;
    private int _htfN = 5;
    private bool _initialized;
    private readonly object _initLock = new();

    public MarketStructureService(IServiceProvider serviceProvider, ILogger<MarketStructureService> logger)
    {
        _serviceProvider = serviceProvider;
        _logger = logger;
    }

    private async Task EnsureInitializedAsync()
    {
        if (_initialized) return;

        using var scope = _serviceProvider.CreateScope();
        var configRepo = scope.ServiceProvider.GetRequiredService<IStrategyConfigRepository>();

        try
        {
            var cfg = await configRepo.GetByNameAsync("swing_detector");
            if (cfg is not null)
            {
                var parts = cfg.Value.Split(',');
                if (parts.Length >= 4
                    && int.TryParse(parts[0], out var ltfM)
                    && int.TryParse(parts[1], out var htfM)
                    && int.TryParse(parts[2], out var ltfN)
                    && int.TryParse(parts[3], out var htfN))
                {
                    _ltfMinutes = ltfM;
                    _htfMinutes = htfM;
                    _ltfN = ltfN;
                    _htfN = htfN;
                    _logger.LogInformation(
                        "Swing detector config loaded: LTF={LtfMin}m N={LtfN} HTF={HtfMin}m N={HtfN}",
                        _ltfMinutes, _ltfN, _htfMinutes, _htfN);
                }
            }
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "Could not load swing_detector config, using defaults");
        }

        lock (_initLock)
        {
            _initialized = true;
        }
    }

    public async Task<IReadOnlyList<StructureBreak>> FeedBarAsync(string instrument, BarDataProto bar)
    {
        await EnsureInitializedAsync();

        var trackers = _instruments.GetOrAdd(instrument, key =>
            new InstrumentTrackers(key, _ltfMinutes, _htfMinutes, _ltfN, _htfN, _logger));

        var results = new List<StructureBreak>();

        var oneMinBar = new AggregatedBar
        {
            Timestamp = bar.Timestamp.ToUnixTimeMilliseconds(),
            Open = bar.Open,
            High = bar.High,
            Low = bar.Low,
            Close = bar.Close,
            Volume = bar.Volume,
        };

        if (trackers.LtfAgg.TryFeed(oneMinBar, out var ltfBar))
        {
            var ltfBreaks = trackers.LtfDetector.Feed(
                ltfBar.Timestamp, DateTimeOffset.FromUnixTimeMilliseconds(ltfBar.Timestamp),
                ltfBar.Open, ltfBar.High, ltfBar.Low, ltfBar.Close);
            results.AddRange(ltfBreaks);
        }

        if (trackers.HtfAgg.TryFeed(oneMinBar, out var htfBar))
        {
            var htfBreaks = trackers.HtfDetector.Feed(
                htfBar.Timestamp, DateTimeOffset.FromUnixTimeMilliseconds(htfBar.Timestamp),
                htfBar.Open, htfBar.High, htfBar.Low, htfBar.Close);
            results.AddRange(htfBreaks);
        }

        if (results.Count > 0)
        {
            _logger.LogInformation(
                "{Count} structure break(s) for {Instrument}: {Breaks}",
                results.Count, instrument,
                string.Join("; ", results.Select(b => $"{b.EventType} bias={b.ResultingBias} tf={b.Timeframe}")));
        }

        return results;
    }

    public StructureStateSnapshot GetState(string instrument, string timeframe)
    {
        if (!_instruments.TryGetValue(instrument, out var trackers))
        {
            return new StructureStateSnapshot(Bias.Undetermined, null, null, Array.Empty<SwingPoint>());
        }

        SwingDetector detector;
        if (trackers.LtfDetector.TimeframeName == timeframe)
            detector = trackers.LtfDetector;
        else if (trackers.HtfDetector.TimeframeName == timeframe)
            detector = trackers.HtfDetector;
        else
        {
            return new StructureStateSnapshot(Bias.Undetermined, null, null, Array.Empty<SwingPoint>());
        }

        return new StructureStateSnapshot(
            detector.CurrentBias,
            detector.ContinuationReference,
            detector.ReversalReference,
            detector.SwingPoints);
    }
}
