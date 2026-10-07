using Microsoft.Extensions.Logging;
using SMC.Engine.Core.Models;
using SMC.Engine.Core.Utils;

namespace SMC.Engine.Core.Analysis;

public sealed class StructureStateSnapshot
{
    public Bias Bias { get; }
    public SwingPoint? ContinuationReference { get; }
    public SwingPoint? ReversalReference { get; }
    public IReadOnlyList<SwingPoint> SwingPoints { get; }

    internal StructureStateSnapshot(Bias bias, SwingPoint? contRef, SwingPoint? revRef, IReadOnlyList<SwingPoint> swings)
    {
        Bias = bias;
        ContinuationReference = contRef;
        ReversalReference = revRef;
        SwingPoints = swings;
    }
}

public class SwingDetector
{
    private readonly int _n;
    private readonly string _timeframe;
    private readonly string _instrument;
    private readonly ILogger? _logger;

    private readonly List<decimal> _highs = new();
    private readonly List<decimal> _lows = new();
    private readonly List<decimal> _closes = new();
    private readonly List<DateTimeOffset> _timestamps = new();

    private readonly List<SwingPoint> _swingPoints = new();
    private readonly HashSet<long> _brokenSwingIndices = new();
    private readonly RingBuffer<StructureBreak> _recentBreaks;

    private Bias _bias = Bias.Undetermined;
    private SwingPoint? _continuationReference;
    private SwingPoint? _reversalReference;

    public Bias CurrentBias => _bias;
    public IReadOnlyList<SwingPoint> SwingPoints => _swingPoints;
    public SwingPoint? ContinuationReference => _continuationReference;
    public SwingPoint? ReversalReference => _reversalReference;

    public SwingDetector(int n, string timeframe, ILogger? logger = null, string instrument = "")
    {
        _n = n;
        _timeframe = timeframe;
        _instrument = instrument;
        _logger = logger;
        _recentBreaks = new RingBuffer<StructureBreak>(100);
    }

    public IReadOnlyList<StructureBreak> RecentBreaks => _recentBreaks.Snapshot();
    public string TimeframeName => _timeframe;

    public IReadOnlyList<StructureBreak> Feed(long barIndex, DateTimeOffset barTimestamp, decimal open, decimal high, decimal low, decimal close)
    {
        _highs.Add(high);
        _lows.Add(low);
        _closes.Add(close);
        _timestamps.Add(barTimestamp);

        int i = _highs.Count - 1;
        var breaks = new List<StructureBreak>();

        int candidate = i - _n;
        if (candidate >= 0 && candidate - _n >= 0)
        {
            CheckSwing(candidate, i, breaks);
        }

        CheckForBreaks(i, breaks);

        return breaks;
    }

    private void CheckSwing(int candidate, int currentIdx, List<StructureBreak> breaks)
    {
        int start = Math.Max(0, candidate - _n);
        int end = Math.Min(_highs.Count - 1, candidate + _n);

        bool isHigh = true;
        bool isLow = true;
        decimal high = _highs[candidate];
        decimal low = _lows[candidate];

        for (int j = start; j <= end; j++)
        {
            if (j == candidate) continue;
            if (_highs[j] >= high) isHigh = false;
            if (_lows[j] <= low) isLow = false;
        }

        if (isHigh)
        {
            var swing = new SwingPoint(
                candidate,
                _timestamps[candidate],
                _timestamps[currentIdx],
                SwingType.High,
                high,
                _instrument,
                _timeframe);
            _swingPoints.Add(swing);
            OnSwingConfirmed(swing);
            _logger?.LogDebug(
                "Swing HIGH confirmed at bar {BarIdx} price={Price} timeframe={Tf}",
                candidate, swing.Price, _timeframe);
        }

        if (isLow)
        {
            var swing = new SwingPoint(
                candidate,
                _timestamps[candidate],
                _timestamps[currentIdx],
                SwingType.Low,
                low,
                _instrument,
                _timeframe);
            _swingPoints.Add(swing);
            OnSwingConfirmed(swing);
            _logger?.LogDebug(
                "Swing LOW confirmed at bar {BarIdx} price={Price} timeframe={Tf}",
                candidate, swing.Price, _timeframe);
        }
    }

    private void OnSwingConfirmed(SwingPoint p)
    {
        if (_bias == Bias.Undetermined) return;

        bool isReversalType = (_bias == Bias.Bullish && p.Type == SwingType.Low)
                           || (_bias == Bias.Bearish && p.Type == SwingType.High);

        bool isContinuationType = (_bias == Bias.Bullish && p.Type == SwingType.High)
                               || (_bias == Bias.Bearish && p.Type == SwingType.Low);

        if (isReversalType)
        {
            _reversalReference = p;
        }
        else if (isContinuationType)
        {
            if (_continuationReference == null)
            {
                _continuationReference = p;
            }
            else if ((_bias == Bias.Bullish && p.Price > _continuationReference.Value.Price)
                  || (_bias == Bias.Bearish && p.Price < _continuationReference.Value.Price))
            {
                _continuationReference = p;
            }
        }
    }

    private void CheckForBreaks(int currentIdx, List<StructureBreak> breaks)
    {
        if (_bias == Bias.Undetermined)
        {
            TryEstablishBias(currentIdx);
            return;
        }

        decimal close = _closes[currentIdx];

        if (_bias == Bias.Bullish)
        {
            if (_continuationReference.HasValue
                && close > _continuationReference.Value.Price
                && !_brokenSwingIndices.Contains(_continuationReference.Value.BarIndex))
            {
                _brokenSwingIndices.Add(_continuationReference.Value.BarIndex);
                var bos = new StructureBreak(
                    StructureEventType.BOS,
                    _continuationReference.Value,
                    close,
                    _timestamps[currentIdx],
                    _instrument,
                    _timeframe,
                    Bias.Bullish);
                _recentBreaks.Add(bos);
                breaks.Add(bos);
            }
            else if (_reversalReference.HasValue
                && close < _reversalReference.Value.Price
                && !_brokenSwingIndices.Contains(_reversalReference.Value.BarIndex))
            {
                _brokenSwingIndices.Add(_reversalReference.Value.BarIndex);
                var chochRef = _reversalReference.Value;

                _reversalReference = _continuationReference;
                _bias = Bias.Bearish;
                _continuationReference = null;

                var choch = new StructureBreak(
                    StructureEventType.CHoCH,
                    chochRef,
                    close,
                    _timestamps[currentIdx],
                    _instrument,
                    _timeframe,
                    Bias.Bearish);
                _recentBreaks.Add(choch);
                breaks.Add(choch);
            }
        }
        else
        {
            if (_continuationReference.HasValue
                && close < _continuationReference.Value.Price
                && !_brokenSwingIndices.Contains(_continuationReference.Value.BarIndex))
            {
                _brokenSwingIndices.Add(_continuationReference.Value.BarIndex);
                var bos = new StructureBreak(
                    StructureEventType.BOS,
                    _continuationReference.Value,
                    close,
                    _timestamps[currentIdx],
                    _instrument,
                    _timeframe,
                    Bias.Bearish);
                _recentBreaks.Add(bos);
                breaks.Add(bos);
            }
            else if (_reversalReference.HasValue
                && close > _reversalReference.Value.Price
                && !_brokenSwingIndices.Contains(_reversalReference.Value.BarIndex))
            {
                _brokenSwingIndices.Add(_reversalReference.Value.BarIndex);
                var chochRef = _reversalReference.Value;

                _reversalReference = _continuationReference;
                _bias = Bias.Bullish;
                _continuationReference = null;

                var choch = new StructureBreak(
                    StructureEventType.CHoCH,
                    chochRef,
                    close,
                    _timestamps[currentIdx],
                    _instrument,
                    _timeframe,
                    Bias.Bullish);
                _recentBreaks.Add(choch);
                breaks.Add(choch);
            }
        }
    }

    private void TryEstablishBias(int currentIdx)
    {
        var lastHigh = LastSwingOfType(SwingType.High);
        var lastLow = LastSwingOfType(SwingType.Low);
        if (lastHigh == null || lastLow == null) return;

        decimal close = _closes[currentIdx];

        if (close > lastHigh.Value.Price)
        {
            _bias = Bias.Bullish;
            _continuationReference = lastHigh;
            _reversalReference = lastLow;
            _logger?.LogInformation(
                "Bias established Bullish: close={Close} > high={High} tf={Tf}",
                close, lastHigh.Value.Price, _timeframe);
        }
        else if (close < lastLow.Value.Price)
        {
            _bias = Bias.Bearish;
            _continuationReference = lastLow;
            _reversalReference = lastHigh;
            _logger?.LogInformation(
                "Bias established Bearish: close={Close} < low={Low} tf={Tf}",
                close, lastLow.Value.Price, _timeframe);
        }
    }

    private SwingPoint? LastSwingOfType(SwingType type)
    {
        for (int i = _swingPoints.Count - 1; i >= 0; i--)
        {
            if (_swingPoints[i].Type == type)
                return _swingPoints[i];
        }
        return null;
    }
}
