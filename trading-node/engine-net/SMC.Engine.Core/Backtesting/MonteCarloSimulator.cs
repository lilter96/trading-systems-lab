namespace SMC.Engine.Core.Backtesting;

public static class MonteCarloSimulator
{
    public static MonteCarloResult Run(
        decimal startingCapital,
        IReadOnlyList<BacktestPosition> completedTrades,
        int iterations = 10_000,
        decimal targetDrawdown = 0.20m,
        int? seed = null)
    {
        var validTrades = completedTrades
            .Where(t => t.Status == PositionStatus.Closed && t.ClosedAt.HasValue)
            .Select(t => t.RealizedPnl)
            .ToList();

        if (validTrades.Count == 0 || iterations <= 0)
        {
            return new MonteCarloResult
            {
                DrawdownProbability = 0m,
                FinalEquityPercentile5 = startingCapital,
                FinalEquityMedian = startingCapital,
                FinalEquityPercentile95 = startingCapital,
            };
        }

        var rng = seed.HasValue ? new Random(seed.Value) : Random.Shared;

        double[] finalEquities = new double[iterations];
        int drawdownCount = 0;

        for (int i = 0; i < iterations; i++)
        {
            double equity = (double)startingCapital;
            double peak = equity;
            bool exceededDrawdown = false;

            for (int j = 0; j < validTrades.Count; j++)
            {
                int idx = rng.Next(validTrades.Count);
                equity += (double)validTrades[idx];

                if (equity > peak)
                    peak = equity;

                double dd = peak > 0 ? (peak - equity) / peak : 0.0;
                if (dd >= (double)targetDrawdown)
                    exceededDrawdown = true;
            }

            finalEquities[i] = equity;
            if (exceededDrawdown)
                drawdownCount++;
        }

        Array.Sort(finalEquities);

        int p5Idx = (int)(0.05 * iterations);
        int p95Idx = (int)(0.95 * iterations);
        int medIdx = iterations / 2;

        return new MonteCarloResult
        {
            DrawdownProbability = (decimal)drawdownCount / iterations,
            FinalEquityPercentile5 = (decimal)finalEquities[p5Idx],
            FinalEquityMedian = (decimal)finalEquities[medIdx],
            FinalEquityPercentile95 = (decimal)finalEquities[p95Idx],
        };
    }
}
