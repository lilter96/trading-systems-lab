using System.Collections.Concurrent;
using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Models;

namespace SMC.Engine.Service.Services;

public enum StateChangeKind
{
    Candles,
    Pois,
    Selection,
    Telemetry,
    Heartbeat,
    All,
}

public sealed class DashboardStateService
{
    public event Action<StateChangeKind>? OnStateChanged;
    public event Action? PoiSelectionChanged;

    public string SystemStatus { get; private set; } = "INITIALIZING";
    public double IngestionRate { get; private set; }
    public long TicksPerSecond { get; private set; }
    public long TotalTicks { get; private set; }
    public double LastPrice { get; private set; }
    public bool HasTelemetry => TotalTicks > 0;
    public string? CurrentOperator { get; private set; }
    public string? CurrentSessionId { get; private set; }

    public DateTimeOffset StartupTime { get; } = DateTimeOffset.UtcNow;
    public DateTimeOffset? FirstBarTime { get; private set; }
    public DateTimeOffset? LastBarTime { get; private set; }
    public bool HasReceivedData => FirstBarTime.HasValue;

    private readonly ConcurrentDictionary<string, DateTimeOffset> _heartbeats = new();
    private readonly ConcurrentDictionary<POI, byte> _activePois = new();
    private readonly HashSet<POI> _selectedPois = new();
    private readonly object _selectionLock = new();
    private const int MaxBars = 10000;
    public DateTimeOffset? LastReplayedOneMinBarTs { get; set; }
    public Dictionary<string, DateTimeOffset> LastReplayedHtBarTss { get; } = new();

    private readonly List<BarRecord> _oneMinBars = new();
    private readonly object _barLock = new();

    private readonly Dictionary<string, CachedCandleSet> _candleCache = new();
    private readonly object _cacheLock = new();

    private double? _currentTickOpen;
    private double _currentTickHigh;
    private double _currentTickLow;
    private double _currentTickClose;
    private long _currentTickMinKey;
    private readonly object _tickLock = new();

    public static readonly Dictionary<string, int> TimeframeSeconds = new()
    {
        ["1m"] = 60,
        ["3m"] = 180,
        ["5m"] = 300,
        ["15m"] = 900,
        ["30m"] = 1800,
        ["1h"] = 3600,
        ["4h"] = 14400,
        ["1d"] = 86400,
        ["1w"] = 604800,
        ["1mo"] = 2592000,
    };

    public IReadOnlyList<TickCandle> GetCandles(string timeframe)
    {
        var interval = TimeframeSeconds.GetValueOrDefault(timeframe, 60);

        lock (_cacheLock)
        {
            if (_candleCache.TryGetValue(timeframe, out var cached) && cached.IsValid(_oneMinBars.Count))
                return cached.Candles;
        }

        lock (_barLock)
        {
            var bars = _oneMinBars.ToList();

            BarRecord? virtualBar = null;
            lock (_tickLock)
            {
                if (_currentTickMinKey > 0)
                {
                    virtualBar = new BarRecord(
                        DateTimeOffset.FromUnixTimeSeconds(_currentTickMinKey),
                        _currentTickOpen ?? _currentTickClose,
                        _currentTickHigh,
                        _currentTickLow,
                        _currentTickClose,
                        0);
                }
            }

            if (virtualBar is not null && (bars.Count == 0 || bars[^1].Timestamp < virtualBar.Timestamp))
                bars.Add(virtualBar);
            else if (virtualBar is not null && bars.Count > 0 && bars[^1].Timestamp == virtualBar.Timestamp)
                bars[^1] = MergeBars(bars[^1], virtualBar);

            if (bars.Count == 0) return new List<TickCandle>();

            var candles = AggregateCandles(bars, TimeframeSeconds.GetValueOrDefault(timeframe, 60));

            lock (_cacheLock)
                _candleCache[timeframe] = new CachedCandleSet(candles, _oneMinBars.Count);

            return candles;
        }
    }

    private static List<TickCandle> AggregateCandles(List<BarRecord> bars, int intervalSeconds)
    {
        var candles = new List<TickCandle>();
        TickCandle? current = null;

        foreach (var bar in bars)
        {
            var key = (bar.Timestamp.ToUnixTimeSeconds() / intervalSeconds) * intervalSeconds;

            if (current is null || current.Timestamp.ToUnixTimeSeconds() < key)
            {
                if (current is not null) candles.Add(current);
                current = new TickCandle(
                    DateTimeOffset.FromUnixTimeSeconds(key),
                    bar.Open, bar.High, bar.Low, bar.Close);
            }
            else
            {
                var c = current;
                current = c with
                {
                    High = Math.Max(c.High, bar.High),
                    Low = Math.Min(c.Low, bar.Low),
                    Close = bar.Close,
                };
            }
        }

        if (current is not null) candles.Add(current);
        return candles;
    }

    public void LoadHistoricalBars(IEnumerable<BarRecord> bars)
    {
        lock (_barLock)
        {
            var existing = new HashSet<long>(_oneMinBars.Select(b => b.Timestamp.ToUnixTimeSeconds()));
            foreach (var bar in bars)
            {
                if (existing.Add(bar.Timestamp.ToUnixTimeSeconds()))
                {
                    _oneMinBars.Add(bar);
                }
            }
            _oneMinBars.Sort((a, b) => a.Timestamp.CompareTo(b.Timestamp));
            if (_oneMinBars.Count > MaxBars)
                _oneMinBars.RemoveRange(0, _oneMinBars.Count - MaxBars);
        }

        InvalidateCandleCache();
        NotifyStateChanged(StateChangeKind.Candles);
    }

    public void RecordBarReceived(BarDataProto bar)
    {
        var now = DateTimeOffset.UtcNow;
        FirstBarTime ??= now;
        LastBarTime = now;

        var rec = new BarRecord(
            bar.Timestamp,
            (double)bar.Open,
            (double)bar.High,
            (double)bar.Low,
            (double)bar.Close,
            (double)bar.Volume);

        lock (_barLock)
        {
            if (_oneMinBars.Count > 0 && _oneMinBars[^1].Timestamp == rec.Timestamp)
                _oneMinBars[^1] = rec;
            else
                _oneMinBars.Add(rec);

            if (_oneMinBars.Count > MaxBars)
                _oneMinBars.RemoveRange(0, _oneMinBars.Count - MaxBars);
        }

        InvalidateCandleCache();

        if (SystemStatus == "INITIALIZING") SystemStatus = "ACTIVE";
        NotifyStateChanged(StateChangeKind.Candles);
    }

    public void RecordTick(TickPayload tick)
    {
        var minKey = (tick.exchange_time / 60000) * 60;

        lock (_tickLock)
        {
            if (_currentTickMinKey != minKey)
            {
                _currentTickMinKey = minKey;
                _currentTickOpen = tick.price;
                _currentTickHigh = tick.price;
                _currentTickLow = tick.price;
                _currentTickClose = tick.price;
            }
            else
            {
                _currentTickHigh = Math.Max(_currentTickHigh, tick.price);
                _currentTickLow = Math.Min(_currentTickLow, tick.price);
                _currentTickClose = tick.price;
            }
        }
    }

    public void UpdateSystemStatus(string status)
    {
        SystemStatus = status;
        NotifyStateChanged(StateChangeKind.All);
    }

    public void SetOperatorContext(string? operatorName, string? sessionId)
    {
        CurrentOperator = operatorName;
        CurrentSessionId = sessionId;
    }

    public void UpdateTelemetry(long ticksPerSecond, long totalTicks, double lastPrice)
    {
        TicksPerSecond = ticksPerSecond;
        TotalTicks = totalTicks;
        LastPrice = lastPrice;
        if (SystemStatus == "INITIALIZING") SystemStatus = "ACTIVE";
        NotifyStateChanged(StateChangeKind.Telemetry);
    }

    public void UpdateIngestionRate(double rate)
    {
        IngestionRate = rate;
        NotifyStateChanged(StateChangeKind.Telemetry);
    }

    public void RecordHeartbeat(string componentName)
    {
        _heartbeats[componentName] = DateTimeOffset.UtcNow;
        NotifyStateChanged(StateChangeKind.Heartbeat);
    }

    public void SetActivePois(IEnumerable<POI> pois)
    {
        var newSet = new HashSet<POI>(pois);

        _activePois.Clear();
        foreach (var poi in newSet)
            _activePois.TryAdd(poi, 0);

        lock (_selectionLock)
            _selectedPois.IntersectWith(newSet);

        NotifyStateChanged(StateChangeKind.Pois);
    }

    public IReadOnlyDictionary<string, DateTimeOffset> GetHeartbeats() => _heartbeats;

    public IReadOnlyCollection<POI> GetActivePois() => _activePois.Keys.ToList();

    public IReadOnlySet<POI> GetSelectedPois()
    {
        lock (_selectionLock)
            return new HashSet<POI>(_selectedPois);
    }

    public void ToggleSelectedPoi(POI poi)
    {
        lock (_selectionLock)
        {
            if (!_selectedPois.Remove(poi))
                _selectedPois.Add(poi);
        }
        NotifyPoiSelectionChanged();
        NotifyStateChanged(StateChangeKind.Selection);
    }

    public void ClearSelectedPois()
    {
        lock (_selectionLock)
        {
            if (_selectedPois.Count == 0) return;
            _selectedPois.Clear();
        }
        NotifyPoiSelectionChanged();
        NotifyStateChanged(StateChangeKind.Selection);
    }

    public void SelectAllFiltered(IEnumerable<POI> filtered)
    {
        lock (_selectionLock)
        {
            foreach (var poi in filtered)
                _selectedPois.Add(poi);
        }
        NotifyPoiSelectionChanged();
        NotifyStateChanged(StateChangeKind.Selection);
    }

    public void InvertSelection(IEnumerable<POI> filtered)
    {
        lock (_selectionLock)
        {
            foreach (var poi in filtered)
            {
                if (!_selectedPois.Remove(poi))
                    _selectedPois.Add(poi);
            }
        }
        NotifyPoiSelectionChanged();
        NotifyStateChanged(StateChangeKind.Selection);
    }

    private void NotifyStateChanged(StateChangeKind kind = StateChangeKind.All) =>
        OnStateChanged?.Invoke(kind);

    private void NotifyPoiSelectionChanged() =>
        PoiSelectionChanged?.Invoke();

    private void InvalidateCandleCache()
    {
        lock (_cacheLock)
            _candleCache.Clear();
    }

    private static BarRecord MergeBars(BarRecord existing, BarRecord update)
    {
        return existing with
        {
            High = Math.Max(existing.High, update.High),
            Low = Math.Min(existing.Low, update.Low),
            Close = update.Close,
        };
    }

    private sealed record CachedCandleSet(IReadOnlyList<TickCandle> Candles, int BarCount)
    {
        public bool IsValid(int currentBarCount) => BarCount == currentBarCount;
    }
}

public record BarRecord(DateTimeOffset Timestamp, double Open, double High, double Low, double Close, double Volume);
public record TickCandle(DateTimeOffset Timestamp, double Open, double High, double Low, double Close);
