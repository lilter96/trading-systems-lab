namespace SMC.Engine.Core.Backtesting;

public sealed class MonteCarloResult
{
    public decimal DrawdownProbability { get; init; }
    public decimal FinalEquityPercentile5 { get; init; }
    public decimal FinalEquityMedian { get; init; }
    public decimal FinalEquityPercentile95 { get; init; }
}
