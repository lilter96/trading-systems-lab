using System.Collections.Concurrent;
using System.Globalization;
using System.Text.Json;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using SMC.Engine.Core.Models;
using SMC.Engine.Core.Repositories;
using SMC.Engine.Core.Utils;

namespace SMC.Engine.Core.Analysis;

public class TriggerService : ITriggerService
{
    private readonly IServiceProvider _serviceProvider;
    private readonly ILogger<TriggerService> _logger;

    private TriggerConfig _config = new();
    private bool _initialized;
    private readonly object _initLock = new();

    private readonly ConcurrentDictionary<(string Instrument, string Timeframe), TriggerState> _states = new();

    public void SetTestConfiguration(TriggerConfig config)
    {
        _config = config;
        _initialized = true;
    }

    public TriggerService(IServiceProvider serviceProvider, ILogger<TriggerService> logger)
    {
        _serviceProvider = serviceProvider;
        _logger = logger;
    }

    private sealed class TriggerState
    {
        public RingBuffer<decimal> Closes { get; }
        public RingBuffer<decimal> Deltas { get; }
        public decimal? PrevOi { get; set; }
        public int BarsSeen { get; set; }

        public TriggerState(int deltaWindowBars)
        {
            Closes = new(deltaWindowBars + 1);
            Deltas = new(deltaWindowBars);
        }
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
                var cfg = configRepo.GetByNameAsync("trigger").GetAwaiter().GetResult();
                if (cfg is not null)
                {
                    var parsed = JsonSerializer.Deserialize<TriggerConfig>(cfg.Value,
                        new JsonSerializerOptions { PropertyNameCaseInsensitive = true });
                    if (parsed is not null)
                        _config = parsed;
                }
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "Failed to load trigger config, using defaults");
            }
            _initialized = true;
        }
    }

    public IReadOnlyList<TriggerEvaluation> Evaluate(
        string instrument,
        BarDataProto bar,
        IReadOnlyList<POI> activePois,
        IReadOnlyList<StructureBreak> breaks,
        IReadOnlyList<SmtDivergenceInfo> divergences)
    {
        EnsureInitialized();
        var state = _states.GetOrAdd((instrument, bar.Timeframe), _ => new TriggerState(_config.DeltaWindowBars));

        state.Closes.Add(bar.Close);
        state.Deltas.Add(bar.BuySellVolumeDelta);
        state.BarsSeen++;

        decimal deltaOi = bar.OpenInterest - (state.PrevOi ?? bar.OpenInterest);
        state.PrevOi = bar.OpenInterest;

        var results = new List<TriggerEvaluation>();

        foreach (var poi in activePois)
        {
            if (!IsZoneEntered(poi, bar)) continue;

            var poiQuality = ComputePoiQuality(poi);
            var absorption = ComputeAbsorptionScore(state);
            var oiScore = ComputeOiScore(poi, deltaOi, state);
            var smtScore = ComputeSmtScore(poi, divergences);

            decimal smcSetup = Clamp(
                (decimal)_config.PoiWeight * poiQuality
                + (decimal)_config.DeltaWeight * absorption
                + (decimal)_config.OiWeight * oiScore
                + (decimal)_config.SmtWeight * smtScore,
                -1m, 1m);

            decimal sentimentScore = GetLiveSentiment(instrument);
            decimal confluence = Clamp(
                (decimal)_config.TechnicalWeight * smcSetup
                + (decimal)_config.SentimentWeight * sentimentScore,
                -1m, 1m);

            var positioning = ClassifyPositioning(state, deltaOi);

            results.Add(new TriggerEvaluation
            {
                Zone = poi,
                Positioning = positioning,
                AbsorptionScore = absorption,
                OiScore = oiScore,
                SmtScore = smtScore,
                PoiQualityScore = poiQuality,
                SmcSetupScore = smcSetup,
                ConfluenceScore = confluence,
            });
        }

        return results;
    }

    private static bool IsZoneEntered(POI poi, BarDataProto bar)
    {
        if (poi.Status == POIStatus.Invalidated)
            return false;

        return poi.Type switch
        {
            POIType.FairValueGap => poi.Bias switch
            {
                MarketBias.Bullish => bar.Low < poi.UpperBound,
                MarketBias.Bearish => bar.High > poi.LowerBound,
                _ => false,
            },
            POIType.OrderBlock => bar.Low <= poi.UpperBound && bar.High >= poi.LowerBound,
            _ => false,
        };
    }

    private decimal ComputePoiQuality(POI poi)
    {
        int tfMinutes = ParseTimeframeMinutes(poi.Timeframe);
        decimal tfWeight = tfMinutes >= _config.HtfMinutesThreshold
            ? (decimal)_config.TimeframeWeightHtf
            : (decimal)_config.TimeframeWeightLtf;

        decimal statusWeight = poi.Status == POIStatus.Active
            ? (decimal)_config.StatusWeightActive
            : (decimal)_config.StatusWeightPartial;

        decimal typeWeight = poi.Type == POIType.OrderBlock
            ? (decimal)_config.TypeWeightOb
            : (decimal)_config.TypeWeightFvg;

        return tfWeight * statusWeight * typeWeight;
    }

    private static int ParseTimeframeMinutes(string tf)
    {
        if (tf.EndsWith("m") && int.TryParse(tf.TrimEnd('m'), out var m))
            return m;
        if (tf.EndsWith("h") && int.TryParse(tf.TrimEnd('h'), out var h))
            return h * 60;
        return 15;
    }

    private decimal ComputeAbsorptionScore(TriggerState state)
    {
        if (state.BarsSeen < _config.DeltaWindowBars + 1)
            return 0m;

        var closes = state.Closes.Snapshot();
        var deltas = state.Deltas.Snapshot();

        if (closes.Count < 2 || deltas.Count == 0)
            return 0m;

        decimal deltaPrice = closes[^1] - closes[0];
        decimal cumDelta = deltas.Sum();

        if (deltaPrice > 0m && cumDelta < 0m)
            return 1.0m;
        if (deltaPrice < 0m && cumDelta > 0m)
            return -1.0m;

        return 0m;
    }

    private decimal ComputeOiScore(POI poi, decimal deltaOi, TriggerState state)
    {
        if (state.BarsSeen < 2)
            return 0m;

        var closes = state.Closes.Snapshot();
        decimal deltaPrice = closes.Count >= 2
            ? closes[^1] - closes[^2]
            : 0m;

        var positioning = PositioningClassifier.Classify(deltaPrice, deltaOi, (decimal)_config.OiNoiseBand);

        return positioning switch
        {
            PositioningType.FreshLongs => poi.Bias == MarketBias.Bullish ? 1.0m : -1.0m,
            PositioningType.FreshShorts => poi.Bias == MarketBias.Bearish ? 1.0m : -1.0m,
            PositioningType.LongCovering => 0.3m,
            PositioningType.ShortCovering => 0.3m,
            PositioningType.Ambiguous => 0m,
            _ => 0m,
        };
    }

    private static PositioningType ClassifyPositioning(TriggerState state, decimal deltaOi)
    {
        if (state.BarsSeen < 2)
            return PositioningType.Ambiguous;

        var closes = state.Closes.Snapshot();
        decimal deltaPrice = closes.Count >= 2
            ? closes[^1] - closes[^2]
            : 0m;

        return PositioningClassifier.Classify(deltaPrice, deltaOi, 0.0001m);
    }

    private static decimal ComputeSmtScore(POI poi, IReadOnlyList<SmtDivergenceInfo> divergences)
    {
        foreach (var d in divergences)
        {
            if (d.PrimaryInstrument != poi.Instrument && d.SecondaryInstrument != poi.Instrument)
                continue;

            bool divergenceBullish = d.Direction == Bias.Bullish;
            bool poiBullish = poi.Bias == MarketBias.Bullish;

            return divergenceBullish == poiBullish ? 1.0m : -1.0m;
        }

        return 0m;
    }

    private decimal GetLiveSentiment(string instrument)
    {
        try
        {
            using var scope = _serviceProvider.CreateScope();
            var store = scope.ServiceProvider.GetService<ISentimentStore>();
            if (store is null)
                return (decimal)_config.SentimentScoreOverride;

            var score = store.GetSentimentScore(instrument);
            if (!score.HasValue)
                return (decimal)_config.SentimentScoreOverride;

            return Clamp(score.Value, -1m, 1m);
        }
        catch
        {
            return (decimal)_config.SentimentScoreOverride;
        }
    }

    private static decimal Clamp(decimal value, decimal min, decimal max)
        => Math.Max(min, Math.Min(max, value));
}
