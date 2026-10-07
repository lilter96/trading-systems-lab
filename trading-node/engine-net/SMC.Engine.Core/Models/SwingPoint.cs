namespace SMC.Engine.Core.Models;

public readonly record struct SwingPoint(
    long BarIndex,
    DateTimeOffset BarTimestamp,
    DateTimeOffset ConfirmedAtTimestamp,
    SwingType Type,
    decimal Price,
    string Instrument,
    string Timeframe);
