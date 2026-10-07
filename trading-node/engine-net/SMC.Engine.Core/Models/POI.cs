namespace SMC.Engine.Core.Models;

public enum POIType { OrderBlock, FairValueGap }

public enum POIStatus { Active, PartiallyMitigated, Invalidated }

public enum MarketBias { Bullish, Bearish }

public sealed class POI : IEquatable<POI>
{
    public POIType Type { get; init; }
    public MarketBias Bias { get; init; }
    public string Instrument { get; init; } = string.Empty;
    public string Timeframe { get; init; } = string.Empty;
    public decimal UpperBound { get; init; }
    public decimal LowerBound { get; init; }
    public decimal MeanThreshold { get; init; }
    public POIStatus Status { get; set; }
    public DateTimeOffset FormedAt { get; init; }
    public StructureBreak? ConfirmingBreak { get; internal set; }

    public override bool Equals(object? obj) => Equals(obj as POI);

    public bool Equals(POI? other) =>
        other is not null &&
        Type == other.Type &&
        Bias == other.Bias &&
        Instrument == other.Instrument &&
        Timeframe == other.Timeframe &&
        UpperBound == other.UpperBound &&
        LowerBound == other.LowerBound &&
        FormedAt == other.FormedAt;

    public override int GetHashCode() =>
        HashCode.Combine(Type, Bias, Instrument, Timeframe, UpperBound, LowerBound, FormedAt);
}
