using System.Collections.Concurrent;
using System.Text.Json;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using SMC.Engine.Core.Models;
using SMC.Engine.Core.Repositories;
using SMC.Engine.Core.Utils;

namespace SMC.Engine.Core.Analysis;

public class SmtDivergenceService : ISmtDivergenceService
{
    private readonly IMarketStructureService _structureService;
    private readonly ILogger<SmtDivergenceService> _logger;
    private readonly IServiceProvider _serviceProvider;

    private readonly ConcurrentDictionary<(string Instrument, string Timeframe), RingBuffer<decimal>> _closeBuffers = new();
    private readonly ConcurrentDictionary<(string Instrument, string Timeframe), RingBuffer<StructureBreak>> _breakBuffers = new();
    private readonly ConcurrentQueue<(CorrelationPairConfig Pair, StructureBreak PrimaryBOS)> _pendingBOS = new();

    internal List<CorrelationPairConfig> _pairs = new();
    private bool _initialized;
    private readonly object _initLock = new();

    public void SetTestConfiguration(List<CorrelationPairConfig> pairs)
    {
        _pairs = pairs;
        _initialized = true;
    }

    public SmtDivergenceService(
        IServiceProvider serviceProvider,
        IMarketStructureService structureService,
        ILogger<SmtDivergenceService> logger)
    {
        _serviceProvider = serviceProvider;
        _structureService = structureService;
        _logger = logger;
    }

    private void EnsureInitialized()
    {
        if (_initialized) return;
        lock (_initLock)
        {
            if (_initialized) return;
            using var scope = _serviceProvider.CreateScope();
            var configRepo = scope.ServiceProvider.GetRequiredService<IStrategyConfigRepository>();
            try
            {
                var cfg = configRepo.GetByNameAsync("smt_divergence").GetAwaiter().GetResult();
                if (cfg is not null)
                {
                    var parsed = JsonSerializer.Deserialize<SmtDivergenceConfig>(cfg.Value,
                        new JsonSerializerOptions { PropertyNameCaseInsensitive = true });
                    if (parsed?.Pairs is { Count: > 0 })
                    {
                        _pairs = parsed.Pairs;
                        _logger.LogInformation("Loaded {Count} SMT divergence pair(s)", _pairs.Count);
                    }
                }
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "Failed to load smt_divergence config, no pairs configured");
            }
            _initialized = true;
        }
    }

    public IReadOnlyList<SmtDivergenceInfo> ProcessBar(
        string instrument, BarDataProto bar, IReadOnlyList<StructureBreak> breaks)
    {
        EnsureInitialized();

        // Store close for correlation
        int windowBars = GetWindowBars(instrument, bar.Timeframe);
        var closeKey = (instrument, bar.Timeframe);
        var closeBuf = _closeBuffers.GetOrAdd(closeKey, _ => new RingBuffer<decimal>(windowBars));
        closeBuf.Add(bar.Close);

        // Store all breaks for this instrument/timeframe
        if (breaks.Count > 0)
        {
            var breakKey = (instrument, bar.Timeframe);
            var breakBuf = _breakBuffers.GetOrAdd(breakKey, _ => new RingBuffer<StructureBreak>(100));
            foreach (var b in breaks)
                breakBuf.Add(b);
        }

        var results = new List<SmtDivergenceInfo>();

        foreach (var bos in breaks)
        {
            if (bos.EventType != StructureEventType.BOS) continue;

            foreach (var pair in _pairs)
            {
                if (!string.Equals(pair.Primary, instrument, StringComparison.OrdinalIgnoreCase)
                    || !string.Equals(pair.Timeframe, bos.Timeframe, StringComparison.OrdinalIgnoreCase))
                    continue;

                var divergence = CheckPair(pair, bos);
                if (divergence.HasValue)
                    results.Add(divergence.Value);
                else
                {
                    // No divergence yet — queue as pending for re-check when
                    // the secondary's break buffer gets updated later
                    _pendingBOS.Enqueue((pair, bos));
                }
            }
        }

        // Re-check pending BOS events in case a secondary CHoCH arrived
        results.AddRange(CheckPendingBOS());

        return results;
    }

    private SmtDivergenceInfo? CheckPair(CorrelationPairConfig pair, StructureBreak primaryBos)
    {
        var secondaryKey = (pair.Secondary, pair.Timeframe);
        if (!_breakBuffers.TryGetValue(secondaryKey, out var secondaryBreakBuf))
            return null;

        int minuteSize = MinuteFromTimeframe(pair.Timeframe);
        long windowMs = pair.K * minuteSize * 60_000L;
        var triggerTs = primaryBos.TriggerBarTimestamp;
        var windowStart = triggerTs.AddMilliseconds(-windowMs);
        var windowEnd = triggerTs.AddMilliseconds(windowMs);

        bool sameDirectionBOS = false;
        bool oppositeCHoCH = false;

        foreach (var sb in secondaryBreakBuf.Snapshot())
        {
            if (sb.TriggerBarTimestamp < windowStart) continue;
            if (sb.TriggerBarTimestamp > windowEnd) continue;

            if (sb.EventType == StructureEventType.BOS && sb.ResultingBias == primaryBos.ResultingBias)
                sameDirectionBOS = true;
            if (sb.EventType == StructureEventType.CHoCH && sb.ResultingBias != primaryBos.ResultingBias)
                oppositeCHoCH = true;
        }

        if (sameDirectionBOS) return null;
        if (!oppositeCHoCH) return null;

        decimal? correlation = ComputeCorrelation(pair);
        if (correlation.HasValue && Math.Abs(correlation.Value) < (decimal)pair.CorrelationThreshold)
        {
            if (string.Equals(pair.BelowThresholdBehavior, "Suppress", StringComparison.OrdinalIgnoreCase))
                return null;
        }

        _logger.LogInformation(
            "SMT divergence: {P}/{S} dir={Dir} state={State} corr={Corr}",
            pair.Primary, pair.Secondary, primaryBos.ResultingBias,
            SmtSecondaryState.AttemptedAndFailed, correlation);

        return new SmtDivergenceInfo(
            pair.Primary,
            pair.Secondary,
            pair.Timeframe,
            primaryBos.ResultingBias,
            primaryBos,
            SmtSecondaryState.AttemptedAndFailed,
            correlation,
            triggerTs);
    }

    private int GetWindowBars(string instrument, string timeframe)
    {
        foreach (var pair in _pairs)
        {
            if (string.Equals(pair.Primary, instrument, StringComparison.OrdinalIgnoreCase)
                && string.Equals(pair.Timeframe, timeframe, StringComparison.OrdinalIgnoreCase))
                return Math.Max(pair.CorrelationWindowBars, 10);
        }
        return 100;
    }

    private decimal? ComputeCorrelation(CorrelationPairConfig pair)
    {
        var primaryKey = (pair.Primary, pair.Timeframe);
        var secondaryKey = (pair.Secondary, pair.Timeframe);

        if (!_closeBuffers.TryGetValue(primaryKey, out var primaryBuf) ||
            !_closeBuffers.TryGetValue(secondaryKey, out var secondaryBuf))
            return null;

        var primaryCloses = primaryBuf.Snapshot();
        var secondaryCloses = secondaryBuf.Snapshot();

        int count = Math.Min(primaryCloses.Count, secondaryCloses.Count);
        if (count < 3) return null;

        var sliceA = primaryCloses.Skip(primaryCloses.Count - count).Take(count).ToArray();
        var sliceB = secondaryCloses.Skip(secondaryCloses.Count - count).Take(count).ToArray();

        var returnsA = ReturnsCalculator.ComputeLogReturns(sliceA);
        var returnsB = ReturnsCalculator.ComputeLogReturns(sliceB);

        return RollingCorrelation.Compute(returnsA, returnsB);
    }

    private readonly HashSet<string> _resolvedBOS = new();

    private IReadOnlyList<SmtDivergenceInfo> CheckPendingBOS()
    {
        var results = new List<SmtDivergenceInfo>();
        var remaining = new ConcurrentQueue<(CorrelationPairConfig Pair, StructureBreak PrimaryBOS)>();

        foreach (var (pair, bos) in _pendingBOS)
        {
            string key = BosKey(pair, bos);
            if (_resolvedBOS.Contains(key))
                continue;

            var divergence = CheckPair(pair, bos);
            if (divergence.HasValue)
            {
                results.Add(divergence.Value);
                _resolvedBOS.Add(key);
            }
            else
            {
                remaining.Enqueue((pair, bos));
            }
        }

        _pendingBOS.Clear();
        foreach (var item in remaining)
            _pendingBOS.Enqueue(item);

        return results;
    }

    private static string BosKey(CorrelationPairConfig pair, StructureBreak bos) =>
        $"{pair.Primary}:{pair.Secondary}:{pair.Timeframe}:{bos.TriggerBarTimestamp.Ticks}:{bos.ResultingBias}";

    private static int MinuteFromTimeframe(string tf)
    {
        if (tf.EndsWith("m") && int.TryParse(tf.TrimEnd('m'), out var m)) return m;
        if (tf.EndsWith("H") && int.TryParse(tf.TrimEnd('H'), out var h)) return h * 60;
        if (tf.EndsWith("h") && int.TryParse(tf.TrimEnd('h'), out var hl)) return hl * 60;
        return 15;
    }
}
