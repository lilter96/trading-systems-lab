namespace SMC.Engine.Core.Backtesting;

public static class BacktestMetricsCalculator
{
    private const double AnnualizationFactor = 252.0;
    private const int TradingDaysPerYear = 252;

    public static BacktestMetricsResult Compute(
        decimal startingCapital,
        IReadOnlyList<BacktestPosition> completedTrades)
    {
        var validTrades = completedTrades
            .Where(t => t.Status == PositionStatus.Closed && t.ClosedAt.HasValue)
            .ToList();

        if (validTrades.Count == 0)
        {
            var snapshots = BuildDailyEquity(startingCapital, []);
            return new BacktestMetricsResult
            {
                TotalReturn = 0m,
                AnnualizedReturn = 0m,
                SharpeRatio = 0m,
                SortinoRatio = 0m,
                MaxDrawdown = 0m,
                ProfitFactor = 0m,
                WinRate = 0m,
                AvgWin = 0m,
                AvgLoss = 0m,
                Expectancy = 0m,
                TotalTrades = 0,
            };
        }

        var dailyEquity = BuildDailyEquity(startingCapital, validTrades);
        var dailyReturns = dailyEquity
            .Select(e => (double)e.ReturnPercentage)
            .ToArray();

        double totalReturn = 0.0;
        double annualizedReturn = 0.0;
        double sharpe = 0.0;
        double sortino = 0.0;
        double maxDD = 0.0;

        if (dailyReturns.Length > 0)
        {
            totalReturn = (double)(dailyEquity[^1].TotalEquity - startingCapital) / (double)startingCapital;
            double years = dailyReturns.Length / (double)TradingDaysPerYear;
            annualizedReturn = years > 0
                ? Math.Pow(1.0 + totalReturn, 1.0 / years) - 1.0
                : 0.0;

            double mean = dailyReturns.Average();
            double variance = dailyReturns.Length > 1
                ? dailyReturns.Sum(r => (r - mean) * (r - mean)) / (dailyReturns.Length - 1)
                : 0.0;

            double std = Math.Sqrt(variance);
            if (std > 1e-15)
            {
                sharpe = mean / std * Math.Sqrt(AnnualizationFactor);

                double downsideVariance = dailyReturns.Length > 1
                    ? dailyReturns.Where(r => r < 0).Sum(r => (r - mean) * (r - mean)) / (dailyReturns.Length - 1)
                    : 0.0;
                double downsideStd = Math.Sqrt(downsideVariance);
                sortino = downsideStd > 1e-15
                    ? mean / downsideStd * Math.Sqrt(AnnualizationFactor)
                    : 0.0;
            }

            maxDD = ComputeMaxDrawdown(dailyEquity);
        }

        int wins = validTrades.Count(t => t.RealizedPnl > 0);
        int losses = validTrades.Count(t => t.RealizedPnl < 0);
        int even = validTrades.Count(t => t.RealizedPnl == 0);

        decimal avgWin = wins > 0 ? validTrades.Where(t => t.RealizedPnl > 0).Average(t => t.RealizedPnl) : 0m;
        decimal avgLoss = losses > 0 ? validTrades.Where(t => t.RealizedPnl < 0).Average(t => t.RealizedPnl) : 0m;

        decimal totalGrossProfit = validTrades.Where(t => t.RealizedPnl > 0).Sum(t => t.RealizedPnl);
        decimal totalGrossLoss = validTrades.Where(t => t.RealizedPnl < 0).Sum(t => Math.Abs(t.RealizedPnl));
        decimal profitFactor = totalGrossLoss > 0 ? totalGrossProfit / totalGrossLoss : totalGrossProfit > 0 ? 999999m : 0m;

        decimal expectancy = validTrades.Count > 0
            ? validTrades.Average(t => t.RealizedPnl)
            : 0m;

        decimal winRate = validTrades.Count > 0
            ? (decimal)wins / validTrades.Count
            : 0m;

        return new BacktestMetricsResult
        {
            TotalReturn = (decimal)totalReturn,
            AnnualizedReturn = (decimal)annualizedReturn,
            SharpeRatio = (decimal)sharpe,
            SortinoRatio = (decimal)sortino,
            MaxDrawdown = (decimal)maxDD,
            ProfitFactor = profitFactor,
            WinRate = winRate,
            AvgWin = avgWin,
            AvgLoss = avgLoss,
            Expectancy = expectancy,
            TotalTrades = validTrades.Count,
        };
    }

    public static IReadOnlyList<DailyEquitySnapshot> BuildDailyEquity(
        decimal startingCapital,
        IReadOnlyList<BacktestPosition> trades)
    {
        if (trades.Count == 0)
        {
            return new List<DailyEquitySnapshot>();
        }

        var orderedTrades = trades
            .Where(t => t.ClosedAt.HasValue)
            .OrderBy(t => t.ClosedAt!.Value)
            .ToList();

        if (orderedTrades.Count == 0)
            return new List<DailyEquitySnapshot>();

        var startDate = orderedTrades.Min(t => t.OpenedAt.Date);
        var endDate = orderedTrades.Max(t => t.ClosedAt!.Value.Date);

        var dailyPnL = new Dictionary<DateOnly, decimal>();
        foreach (var trade in orderedTrades)
        {
            var closeDate = DateOnly.FromDateTime(trade.ClosedAt!.Value.Date);
            dailyPnL.TryGetValue(closeDate, out var existing);
            dailyPnL[closeDate] = existing + trade.RealizedPnl;
        }

        var result = new List<DailyEquitySnapshot>();
        decimal equity = startingCapital;

        for (var d = DateOnly.FromDateTime(startDate); d <= DateOnly.FromDateTime(endDate); d = d.AddDays(1))
        {
            decimal prevEquity = equity;
            if (dailyPnL.TryGetValue(d, out var dayPnL))
                equity += dayPnL;

            decimal returnPct = prevEquity != 0
                ? (equity - prevEquity) / prevEquity
                : 0m;

            result.Add(new DailyEquitySnapshot(
                d.ToDateTime(TimeOnly.MinValue),
                equity,
                returnPct));
        }

        return result;
    }

    private static double ComputeMaxDrawdown(IReadOnlyList<DailyEquitySnapshot> equity)
    {
        if (equity.Count == 0) return 0.0;

        double peak = (double)equity[0].TotalEquity;
        double maxDd = 0.0;

        foreach (var snap in equity)
        {
            double eq = (double)snap.TotalEquity;
            if (eq > peak)
                peak = eq;
            double dd = peak > 0 ? (peak - eq) / peak : 0.0;
            if (dd > maxDd)
                maxDd = dd;
        }

        return maxDd;
    }
}
