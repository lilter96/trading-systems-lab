namespace SMC.Engine.Core.Backtesting;

public enum SlTpResolutionMode
{
    ConservativeFirst,
    TargetFirst,
    Chronological
}

public sealed class BrokerConfig
{
    public decimal InitialBalance { get; set; } = 10000.0m;
    public decimal MakerFeeRate { get; set; } = 0.0002m;
    public decimal TakerFeeRate { get; set; } = 0.0005m;
    public decimal SlippageTicks { get; set; } = 0.5m;
    public decimal TickSize { get; set; } = 0.5m;
    public SlTpResolutionMode AmbiguityMode { get; set; } = SlTpResolutionMode.ConservativeFirst;
}
