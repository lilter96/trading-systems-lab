namespace SMC.Engine.Core.Backtesting;

public readonly record struct DailyEquitySnapshot(
    DateTime Date,
    decimal TotalEquity,
    decimal ReturnPercentage);
