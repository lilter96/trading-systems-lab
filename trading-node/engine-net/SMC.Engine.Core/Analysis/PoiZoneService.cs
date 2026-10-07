using System.Collections.Concurrent;
using System.Text.Json;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using SMC.Engine.Core.Models;
using SMC.Engine.Core.Repositories;
using SMC.Engine.Core.Utils;

namespace SMC.Engine.Core.Analysis;

public class PoiZoneService : IPoiZoneService
{
    private readonly IServiceProvider _serviceProvider;
    private readonly ILogger<PoiZoneService> _logger;

    private PoiZoneConfig _config = new();
    private bool _initialized;
    private readonly object _initLock = new();

    private readonly ConcurrentDictionary<(string Instrument, string Timeframe), PoiState> _states = new();

    public void SetTestConfiguration(PoiZoneConfig config)
    {
        _config = config;
        _initialized = true;
    }

    public PoiZoneService(IServiceProvider serviceProvider, ILogger<PoiZoneService> logger)
    {
        _serviceProvider = serviceProvider;
        _logger = logger;
    }

    private sealed class PoiState
    {
        public RingBuffer<decimal> Highs { get; }
        public RingBuffer<decimal> Lows { get; }
        public RingBuffer<decimal> Closes { get; }
        public RingBuffer<decimal> Opens { get; }
        public RingBuffer<decimal> TrueRanges { get; }
        public List<OBCandidate> Candidates { get; } = new();
        public List<POI> ActivePois { get; } = new();

        public PoiState(int capacity, int atrPeriod)
        {
            Highs = new(capacity);
            Lows = new(capacity);
            Closes = new(capacity);
            Opens = new(capacity);
            TrueRanges = new(atrPeriod);
        }
    }

    private sealed class OBCandidate
    {
        public decimal LowerBound { get; init; }
        public decimal UpperBound { get; init; }
        public MarketBias Bias { get; init; }
        public int DisplacementBarIndex { get; init; }
        public DateTimeOffset FormedAt { get; init; }
        public decimal MeanThreshold => (LowerBound + UpperBound) / 2;
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
                var cfg = configRepo.GetByNameAsync("poi_zone").GetAwaiter().GetResult();
                if (cfg is not null)
                {
                    var parsed = JsonSerializer.Deserialize<PoiZoneConfig>(cfg.Value,
                        new JsonSerializerOptions { PropertyNameCaseInsensitive = true });
                    if (parsed is not null)
                    {
                        _config = parsed;
                        _logger.LogInformation("Loaded POI zone config: ATR period={Atr}, minGap={MinGap}, disp={Disp}, window={Win}",
                            _config.AtrPeriod, _config.MinGapAtrMultiple, _config.DisplacementAtrMultiple, _config.ObPromotionWindowBars);
                    }
                }
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "Failed to load poi_zone config, using defaults");
            }
            _initialized = true;
        }
    }

    public IReadOnlyList<POI> GetActivePOIs(string instrument, string timeframe)
    {
        if (_states.TryGetValue((instrument, timeframe), out var state))
            return state.ActivePois.ToList().AsReadOnly();
        return Array.Empty<POI>();
    }

    public IReadOnlyList<POI> GetAllActivePOIs(string instrument)
    {
        return _states
            .Where(s => s.Key.Instrument == instrument)
            .SelectMany(s => s.Value.ActivePois)
            .ToList()
            .AsReadOnly();
    }

    public IReadOnlyList<POI> ProcessBar(
        string instrument, BarDataProto bar, IReadOnlyList<StructureBreak> breaks)
    {
        EnsureInitialized();
        var state = _states.GetOrAdd((instrument, bar.Timeframe), _ =>
            new PoiState(Math.Max(_config.AtrPeriod + 5, 20), _config.AtrPeriod));

        var prevCloses = state.Closes.Snapshot();
        decimal? prevClose = prevCloses.Count > 0 ? prevCloses[^1] : null;

        state.Highs.Add(bar.High);
        state.Lows.Add(bar.Low);
        state.Closes.Add(bar.Close);
        state.Opens.Add(bar.Open);

        decimal tr = prevClose.HasValue
            ? Math.Max(bar.High - bar.Low,
                Math.Max(Math.Abs(bar.High - prevClose.Value), Math.Abs(bar.Low - prevClose.Value)))
            : bar.High - bar.Low;
        state.TrueRanges.Add(tr);

        int barCount = state.Highs.Count;
        var results = new List<POI>();
        var freshPois = new HashSet<POI>();

        var atrSnapshot = state.TrueRanges.Snapshot();
        decimal? atr = atrSnapshot.Count >= _config.AtrPeriod
            ? AtrCalculator.ComputeAtr(atrSnapshot)
            : null;

        if (barCount >= 3)
        {
            var fvg = DetectFvg(state, atr, bar, instrument);
            if (fvg is not null)
            {
                state.ActivePois.Add(fvg);
                freshPois.Add(fvg);
                results.Add(fvg);
            }
        }

        if (atr.HasValue && barCount >= 2)
        {
            var candidate = DetectObDisplacement(state, atr.Value, bar);
            if (candidate is not null)
            {
                state.Candidates.Add(candidate);
                _logger.LogDebug(
                    "OB candidate created: inst={Inst} tf={Tf} bias={Bias} zone=[{Lo},{Hi}] idx={Idx}",
                    instrument, bar.Timeframe, candidate.Bias, candidate.LowerBound, candidate.UpperBound,
                    candidate.DisplacementBarIndex);
            }
        }

        foreach (var breakEvent in breaks)
        {
            PromoteCandidates(state, breakEvent, results, freshPois);
        }

        int threshold = barCount - 1 - _config.ObPromotionWindowBars;
        int expired = state.Candidates.RemoveAll(c => c.DisplacementBarIndex <= threshold);
        if (expired > 0)
        {
            _logger.LogDebug("Expired {Count} OB candidate(s) for {Inst}/{Tf}", expired, instrument, bar.Timeframe);
        }

        CheckInvalidation(state, bar, results, freshPois);

        return results;
    }

    private POI? DetectFvg(PoiState state, decimal? atr, BarDataProto bar, string instrument)
    {
        var highs = state.Highs.Snapshot();
        var lows = state.Lows.Snapshot();
        int count = highs.Count;

        decimal lowC3 = lows[count - 1];
        decimal highC1 = highs[count - 3];

        if (lowC3 > highC1)
        {
            decimal gapSize = lowC3 - highC1;
            if (atr.HasValue && gapSize < (decimal)_config.MinGapAtrMultiple * atr.Value)
                return null;

            return new POI
            {
                Type = POIType.FairValueGap,
                Bias = MarketBias.Bullish,
                Instrument = instrument,
                Timeframe = bar.Timeframe,
                LowerBound = highC1,
                UpperBound = lowC3,
                MeanThreshold = (highC1 + lowC3) / 2,
                Status = POIStatus.Active,
                FormedAt = bar.Timestamp,
            };
        }

        decimal lowC1 = lows[count - 3];
        decimal highC3 = highs[count - 1];

        if (highC3 < lowC1)
        {
            decimal gapSize = lowC1 - highC3;
            if (atr.HasValue && gapSize < (decimal)_config.MinGapAtrMultiple * atr.Value)
                return null;

            return new POI
            {
                Type = POIType.FairValueGap,
                Bias = MarketBias.Bearish,
                Instrument = instrument,
                Timeframe = bar.Timeframe,
                LowerBound = highC3,
                UpperBound = lowC1,
                MeanThreshold = (highC3 + lowC1) / 2,
                Status = POIStatus.Active,
                FormedAt = bar.Timestamp,
            };
        }

        return null;
    }

    private OBCandidate? DetectObDisplacement(PoiState state, decimal atr, BarDataProto bar)
    {
        decimal range = bar.High - bar.Low;
        if (range <= (decimal)_config.DisplacementAtrMultiple * atr)
            return null;

        bool displacementBullish = bar.Close > bar.Open;

        var opens = state.Opens.Snapshot();
        var closes = state.Closes.Snapshot();
        int count = opens.Count;

        int prevIdx = count - 2;
        if (prevIdx < 0) return null;

        bool prevBearish = closes[prevIdx] < opens[prevIdx];
        bool prevBullish = closes[prevIdx] > opens[prevIdx];

        if (displacementBullish && !prevBearish)
            return null;

        if (!displacementBullish && !prevBullish)
            return null;

        var highs = state.Highs.Snapshot();
        var lows = state.Lows.Snapshot();

        return new OBCandidate
        {
            LowerBound = lows[prevIdx],
            UpperBound = highs[prevIdx],
            Bias = displacementBullish ? MarketBias.Bullish : MarketBias.Bearish,
            DisplacementBarIndex = count - 1,
            FormedAt = bar.Timestamp,
        };
    }

    private void PromoteCandidates(PoiState state, StructureBreak breakEvent, List<POI> results, HashSet<POI> freshPois)
    {
        if (breakEvent.ResultingBias == Bias.Undetermined) return;

        MarketBias breakBias = breakEvent.ResultingBias == Bias.Bullish
            ? MarketBias.Bullish
            : MarketBias.Bearish;

        for (int i = state.Candidates.Count - 1; i >= 0; i--)
        {
            var c = state.Candidates[i];
            if (c.Bias != breakBias) continue;

            var poi = new POI
            {
                Type = POIType.OrderBlock,
                Bias = c.Bias,
                Instrument = breakEvent.Instrument,
                Timeframe = breakEvent.Timeframe,
                UpperBound = c.UpperBound,
                LowerBound = c.LowerBound,
                MeanThreshold = (c.LowerBound + c.UpperBound) / 2,
                Status = POIStatus.Active,
                FormedAt = c.FormedAt,
            };

            poi.ConfirmingBreak = breakEvent;
            state.ActivePois.Add(poi);
            freshPois.Add(poi);
            results.Add(poi);
            state.Candidates.RemoveAt(i);

            _logger.LogInformation(
                "OB promoted: inst={Inst} tf={Tf} bias={Bias} zone=[{Lo},{Hi}] break={Ev}",
                breakEvent.Instrument, breakEvent.Timeframe, c.Bias, c.LowerBound, c.UpperBound,
                breakEvent.EventType);
        }
    }

    private void CheckInvalidation(PoiState state, BarDataProto bar, List<POI> results, HashSet<POI> freshPois)
    {
        var partiallyMitigated = new List<POI>();

        foreach (var poi in state.ActivePois)
        {
            if (freshPois.Contains(poi)) continue;
            if (poi.Status != POIStatus.Active) continue;

            bool hit = poi.Type switch
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

            if (hit)
            {
                poi.Status = POIStatus.PartiallyMitigated;
                partiallyMitigated.Add(poi);
            }
        }

        var invalidated = new List<POI>();

        foreach (var poi in state.ActivePois)
        {
            if (freshPois.Contains(poi)) continue;
            if (poi.Status != POIStatus.PartiallyMitigated) continue;

            bool done = poi.Type switch
            {
                POIType.FairValueGap => poi.Bias switch
                {
                    MarketBias.Bullish => bar.Low <= poi.LowerBound,
                    MarketBias.Bearish => bar.High >= poi.UpperBound,
                    _ => false,
                },
                POIType.OrderBlock => poi.Bias switch
                {
                    MarketBias.Bullish => bar.Close < poi.MeanThreshold,
                    MarketBias.Bearish => bar.Close > poi.MeanThreshold,
                    _ => false,
                },
                _ => false,
            };

            if (done)
            {
                poi.Status = POIStatus.Invalidated;
                invalidated.Add(poi);
            }
        }

        foreach (var poi in invalidated)
            results.Add(poi);

        foreach (var poi in partiallyMitigated)
        {
            if (!invalidated.Contains(poi))
                results.Add(poi);
        }
    }
}
