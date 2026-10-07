using System.Text.Json.Serialization;

namespace SMC.Engine.Core.Analysis;

public sealed class PoiZoneConfig
{
    [JsonPropertyName("minGapAtrMultiple")]
    public double MinGapAtrMultiple { get; set; } = 0.1;

    [JsonPropertyName("displacementAtrMultiple")]
    public double DisplacementAtrMultiple { get; set; } = 1.5;

    [JsonPropertyName("obPromotionWindowBars")]
    public int ObPromotionWindowBars { get; set; } = 4;

    [JsonPropertyName("atrPeriod")]
    public int AtrPeriod { get; set; } = 14;
}
