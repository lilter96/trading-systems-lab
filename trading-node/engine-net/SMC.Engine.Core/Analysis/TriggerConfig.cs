using System.Text.Json.Serialization;

namespace SMC.Engine.Core.Analysis;

public sealed class TriggerConfig
{
    [JsonPropertyName("poiWeight")]
    public double PoiWeight { get; set; } = 0.25;

    [JsonPropertyName("deltaWeight")]
    public double DeltaWeight { get; set; } = 0.25;

    [JsonPropertyName("oiWeight")]
    public double OiWeight { get; set; } = 0.25;

    [JsonPropertyName("smtWeight")]
    public double SmtWeight { get; set; } = 0.25;

    [JsonPropertyName("technicalWeight")]
    public double TechnicalWeight { get; set; } = 1.0;

    [JsonPropertyName("sentimentWeight")]
    public double SentimentWeight { get; set; } = 0.0;

    [JsonPropertyName("deltaWindowBars")]
    public int DeltaWindowBars { get; set; } = 4;

    [JsonPropertyName("oiNoiseBand")]
    public double OiNoiseBand { get; set; } = 0.0001;

    [JsonPropertyName("htfMinutesThreshold")]
    public int HtfMinutesThreshold { get; set; } = 60;

    [JsonPropertyName("timeframeWeightHtf")]
    public double TimeframeWeightHtf { get; set; } = 1.0;

    [JsonPropertyName("timeframeWeightLtf")]
    public double TimeframeWeightLtf { get; set; } = 0.5;

    [JsonPropertyName("statusWeightActive")]
    public double StatusWeightActive { get; set; } = 1.0;

    [JsonPropertyName("statusWeightPartial")]
    public double StatusWeightPartial { get; set; } = 0.5;

    [JsonPropertyName("typeWeightOb")]
    public double TypeWeightOb { get; set; } = 1.0;

    [JsonPropertyName("typeWeightFvg")]
    public double TypeWeightFvg { get; set; } = 0.7;

    [JsonPropertyName("sentimentScoreOverride")]
    public double SentimentScoreOverride { get; set; } = 0.0;
}
