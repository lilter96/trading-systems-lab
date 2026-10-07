namespace SMC.Engine.Core.Utils;

public sealed class RingBuffer<T>
{
    private readonly T[] _buffer;
    private int _head;
    private int _count;

    public int Capacity => _buffer.Length;
    public int Count => _count;

    public RingBuffer(int capacity)
    {
        if (capacity < 1) throw new ArgumentOutOfRangeException(nameof(capacity), "Capacity must be >= 1");
        _buffer = new T[capacity];
    }

    public void Add(T item)
    {
        _buffer[_head] = item;
        _head = (_head + 1) % _buffer.Length;
        if (_count < _buffer.Length) _count++;
    }

    public void Clear()
    {
        _head = 0;
        _count = 0;
    }

    public IReadOnlyList<T> Snapshot()
    {
        var result = new T[_count];
        int start = _count < _buffer.Length ? 0 : _head;
        for (int i = 0; i < _count; i++)
            result[i] = _buffer[(start + i) % _buffer.Length];
        return result;
    }
}
