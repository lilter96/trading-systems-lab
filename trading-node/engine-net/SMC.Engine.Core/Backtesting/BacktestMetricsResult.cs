namespace SMC.Engine.Core.Backtesting;

public sealed class BacktestMetricsResult
{
    public decimal TotalReturn { get; init; }
    public decimal AnnualizedReturn { get; init; }
    public decimal SharpeRatio { get; init; }
    public decimal SortinoRatio { get; init; }
    public decimal MaxDrawdown { get; init; }
    public decimal ProfitFactor { get; init; }
    public decimal WinRate { get; init; }
    public decimal AvgWin { get; init; }
    public decimal AvgLoss { get; init; }
    public decimal Expectancy { get; init; }
    public int TotalTrades { get; init; }
}
