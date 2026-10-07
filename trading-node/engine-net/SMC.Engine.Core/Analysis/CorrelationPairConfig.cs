namespace SMC.Engine.Core.Analysis;

public sealed class CorrelationPairConfig
{
    public string Primary { get; set; } = string.Empty;
    public string Secondary { get; set; } = string.Empty;
    public string Timeframe { get; set; } = string.Empty;
    public int K { get; set; } = 2;
    public double CorrelationThreshold { get; set; } = 0.7;
    public string BelowThresholdBehavior { get; set; } = "Suppress";
    public int CorrelationWindowBars { get; set; } = 2880;
}

public sealed class SmtDivergenceConfig
{
    public List<CorrelationPairConfig> Pairs { get; set; } = new();
}
