using NetMQ;
using NetMQ.Sockets;
using System.Runtime.InteropServices;
using System.Text.Json;
using System.Text;

Console.WriteLine("Hello, World!");


using var subSocket = new SubscriberSocket();
subSocket.Connect("tcp://127.0.0.1:5555");
subSocket.Subscribe("");

var capacity = 1024;
while (true)
{
    byte[] bytes = subSocket.ReceiveFrameBytes();

    if (bytes != null && bytes.Length == 32)
    {
        TickMessage tick = MemoryMarshal.Read<TickMessage>(bytes);

        string symbolString;
        unsafe
        {
            symbolString = Encoding.UTF8.GetString(tick.Symbol, 8).TrimEnd('\0');
        }

        var jsonFriendlyTick = new
        {
            Timestamp = tick.Timestamp,
            Symbol = symbolString,
            Price = tick.Price,
            Volume = tick.Volume
        };

        Console.WriteLine(JsonSerializer.Serialize(jsonFriendlyTick));
    }
}


[StructLayout(LayoutKind.Sequential, Pack = 1)]
public struct TickMessage
{
    public ulong Timestamp;
    public unsafe fixed byte Symbol[8];
    public double Price;
    public double Volume;
}

public unsafe class UnmanagedRingBuffer
{
    private readonly int _capacity;
    private readonly int _mask;
    private TickMessage* _buffer;
    private bool _disposed;

    public UnmanagedRingBuffer(int capacity = 1024)
    {
        if (capacity <= 0 || (capacity & (capacity - 1)) != 0)
        {
            throw new ArgumentException("Capacity must be a power of 2.");
        }

        _capacity = capacity;
        
        _mask = capacity - 1; 

        _buffer = (TickMessage*)NativeMemory.AllocZeroed((nuint)capacity, (nuint)sizeof(TickMessage));
    }

    public void Push(in TickMessage tick)
    {
        
    }
}