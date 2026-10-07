namespace SMC.Engine.Core.Models;

public readonly record struct StructureBreak(
    StructureEventType EventType,
    SwingPoint ReferenceSwing,
    decimal BreakClosePrice,
    DateTimeOffset TriggerBarTimestamp,
    string Instrument,
    string Timeframe,
    Bias ResultingBias);
