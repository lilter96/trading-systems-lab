namespace SMC.Engine.Core.Analysis;

public readonly struct AggregatedBar
{
    public long Timestamp { get; init; }
    public decimal Open { get; init; }
    public decimal High { get; init; }
    public decimal Low { get; init; }
    public decimal Close { get; init; }
    public decimal Volume { get; init; }
}

public class TimeframeAggregator
{
    private readonly long _targetMs;
    private long _currentBarTime;
    private decimal _open, _high, _low, _close, _volume;
    private bool _hasData;

    public TimeframeAggregator(int targetMinutes)
    {
        _targetMs = targetMinutes * 60_000L;
    }

    public void Reset()
    {
        _hasData = false;
    }

    public bool TryFeed(AggregatedBar oneMinBar, out AggregatedBar completed)
    {
        long aligned = (oneMinBar.Timestamp / _targetMs) * _targetMs;

        if (!_hasData)
        {
            _currentBarTime = aligned;
            _open = oneMinBar.Open;
            _high = oneMinBar.High;
            _low = oneMinBar.Low;
            _close = oneMinBar.Close;
            _volume = oneMinBar.Volume;
            _hasData = true;
            completed = default;
            return false;
        }

        if (aligned == _currentBarTime)
        {
            if (oneMinBar.High > _high) _high = oneMinBar.High;
            if (oneMinBar.Low < _low) _low = oneMinBar.Low;
            _close = oneMinBar.Close;
            _volume += oneMinBar.Volume;
            completed = default;
            return false;
        }

        completed = new AggregatedBar
        {
            Timestamp = _currentBarTime,
            Open = _open,
            High = _high,
            Low = _low,
            Close = _close,
            Volume = _volume,
        };

        _currentBarTime = aligned;
        _open = oneMinBar.Open;
        _high = oneMinBar.High;
        _low = oneMinBar.Low;
        _close = oneMinBar.Close;
        _volume = oneMinBar.Volume;
        return true;
    }

    public bool Flush(out AggregatedBar completed)
    {
        if (!_hasData)
        {
            completed = default;
            return false;
        }
        completed = new AggregatedBar
        {
            Timestamp = _currentBarTime,
            Open = _open,
            High = _high,
            Low = _low,
            Close = _close,
            Volume = _volume,
        };
        _hasData = false;
        return true;
    }
}
