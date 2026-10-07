namespace SMC.Engine.Core.Models;

public sealed class TriggerEvaluation
{
    public required POI Zone { get; init; }
    public required PositioningType Positioning { get; init; }
    public required decimal AbsorptionScore { get; init; }
    public required decimal OiScore { get; init; }
    public required decimal SmtScore { get; init; }
    public required decimal PoiQualityScore { get; init; }
    public required decimal SmcSetupScore { get; init; }
    public required decimal ConfluenceScore { get; init; }
}
