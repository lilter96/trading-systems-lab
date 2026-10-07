namespace SMC.Engine.Core.Models;

public readonly record struct SmtDivergenceInfo(
    string PrimaryInstrument,
    string SecondaryInstrument,
    string Timeframe,
    Bias Direction,
    StructureBreak PrimaryBOS,
    SmtSecondaryState SecondaryState,
    decimal? CorrelationAtEvent,
    DateTimeOffset TriggerBarTimestamp);
