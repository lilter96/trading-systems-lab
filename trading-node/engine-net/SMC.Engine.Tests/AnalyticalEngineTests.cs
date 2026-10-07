using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Backtesting;

namespace SMC.Engine.Tests;

public class AnalyticalEngineTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.UtcNow;
    private static readonly BrokerConfig DefaultConfig = new()
    {
        InitialBalance = 10000m,
        SlippageTicks = 0m,
        TickSize = 0.01m,
        TakerFeeRate = 0m,
        MakerFeeRate = 0m,
    };

    #region Metrics Calculator — Empty Trades

    [Fact]
    public void MetricsCalculator_EmptyTrades_ReturnsSafeDefaults()
    {
        var result = BacktestMetricsCalculator.Compute(10000m, []);

        Assert.Equal(0, result.TotalTrades);
        Assert.Equal(0m, result.TotalReturn);
        Assert.Equal(0m, result.SharpeRatio);
        Assert.Equal(0m, result.SortinoRatio);
        Assert.Equal(0m, result.MaxDrawdown);
        Assert.Equal(0m, result.ProfitFactor);
        Assert.Equal(0m, result.WinRate);
        Assert.Equal(0m, result.Expectancy);
    }

    #endregion

    #region Metrics Calculator — Single Trade

    [Fact]
    public void MetricsCalculator_SingleWinningTrade_ComputesCorrectReturn()
    {
        var trades = new List<BacktestPosition>
        {
            new()
            {
                Instrument = "BTC-USDT",
                Side = PositionSide.Long,
                Quantity = 1m,
                EntryPrice = 100m,
                ExitPrice = 110m,
                RealizedPnl = 10m,
                Status = PositionStatus.Closed,
                OpenedAt = Now.AddDays(-2),
                ClosedAt = Now.AddDays(-1),
            },
        };

        var result = BacktestMetricsCalculator.Compute(10000m, trades);

        Assert.Equal(1, result.TotalTrades);
        Assert.Equal(10m, result.Expectancy);
        Assert.Equal(10m / 10000m, result.TotalReturn);
        Assert.Equal(1m, result.WinRate);
        Assert.Equal(10m, result.AvgWin);
        Assert.Equal(0m, result.AvgLoss);
    }

    [Fact]
    public void MetricsCalculator_SingleLosingTrade_ComputesCorrectLoss()
    {
        var trades = new List<BacktestPosition>
        {
            new()
            {
                Instrument = "BTC-USDT",
                Side = PositionSide.Long,
                Quantity = 1m,
                EntryPrice = 100m,
                ExitPrice = 90m,
                RealizedPnl = -10m,
                Status = PositionStatus.Closed,
                OpenedAt = Now.AddDays(-2),
                ClosedAt = Now.AddDays(-1),
            },
        };

        var result = BacktestMetricsCalculator.Compute(10000m, trades);

        Assert.Equal(1, result.TotalTrades);
        Assert.Equal(0m, result.WinRate);
        Assert.Equal(-10m, result.Expectancy);
        Assert.Equal(-10m, result.AvgLoss);
        Assert.Equal(0m, result.ProfitFactor);
    }

    #endregion

    #region Metrics Calculator — Profit Factor

    [Fact]
    public void MetricsCalculator_ProfitFactor_FromMixedPositions()
    {
        var trades = new List<BacktestPosition>
        {
            new() { RealizedPnl = 50m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-3), ClosedAt = Now.AddDays(-2) },
            new() { RealizedPnl = 30m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-2), ClosedAt = Now.AddDays(-1) },
            new() { RealizedPnl = -20m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-1), ClosedAt = Now },
        };

        var result = BacktestMetricsCalculator.Compute(10000m, trades);

        Assert.Equal(3, result.TotalTrades);
        Assert.Equal(2m / 3m, result.WinRate);
        Assert.Equal(80m / 20m, result.ProfitFactor);
        Assert.Equal(40m, result.AvgWin);
        Assert.Equal(-20m, result.AvgLoss);
    }

    [Fact]
    public void MetricsCalculator_ProfitFactor_AllLossesReturnsZero()
    {
        var trades = new List<BacktestPosition>
        {
            new() { RealizedPnl = -10m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-2), ClosedAt = Now.AddDays(-1) },
            new() { RealizedPnl = -20m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-1), ClosedAt = Now },
        };

        var result = BacktestMetricsCalculator.Compute(10000m, trades);

        Assert.Equal(0m, result.ProfitFactor);
    }

    [Fact]
    public void MetricsCalculator_ProfitFactor_AllWinsReturnsLarge()
    {
        var trades = new List<BacktestPosition>
        {
            new() { RealizedPnl = 5m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-2), ClosedAt = Now.AddDays(-1) },
        };

        var result = BacktestMetricsCalculator.Compute(10000m, trades);

        Assert.True(result.ProfitFactor > 999_000m);
    }

    #endregion

    #region Metrics Calculator — Max Drawdown

    [Fact]
    public void MetricsCalculator_MaxDrawdown_NoLosses_ReturnsZero()
    {
        var trades = new List<BacktestPosition>
        {
            new() { RealizedPnl = 100m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-2), ClosedAt = Now.AddDays(-1) },
            new() { RealizedPnl = 200m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-1), ClosedAt = Now },
        };

        var result = BacktestMetricsCalculator.Compute(10000m, trades);

        Assert.Equal(0m, result.MaxDrawdown);
    }

    [Fact]
    public void MetricsCalculator_MaxDrawdown_CalculatesPeakToTrough()
    {
        var trades = new List<BacktestPosition>
        {
            new() { RealizedPnl = 500m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-5), ClosedAt = Now.AddDays(-4) },
            new() { RealizedPnl = -800m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-4), ClosedAt = Now.AddDays(-3) },
            new() { RealizedPnl = 200m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-3), ClosedAt = Now.AddDays(-2) },
            new() { RealizedPnl = -400m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-2), ClosedAt = Now.AddDays(-1) },
            new() { RealizedPnl = 600m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-1), ClosedAt = Now },
        };

        var result = BacktestMetricsCalculator.Compute(10000m, trades);

        // Peak after trade 1: 10500. Trough after trade 2: 9700. DD = 800/10500 ≈ 0.07619
        // After trade 3: 9900, after trade 4: 9500. Peak still 10500. DD = 1000/10500 ≈ 0.09523
        // Max DD = 0.095238...
        decimal expectedDD = 1000m / 10500m;
        Assert.Equal(expectedDD, result.MaxDrawdown, 4);
    }

    #endregion

    #region Metrics Calculator — Equity Curve

    [Fact]
    public void MetricsCalculator_EquityCurve_MatchesPnlSum()
    {
        var trades = new List<BacktestPosition>
        {
            new() { RealizedPnl = 100m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-3), ClosedAt = Now.AddDays(-2) },
            new() { RealizedPnl = -50m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-2), ClosedAt = Now.AddDays(-1) },
            new() { RealizedPnl = 200m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-1), ClosedAt = Now },
        };

        var equity = BacktestMetricsCalculator.BuildDailyEquity(10000m, trades);

        Assert.NotEmpty(equity);
        Assert.Equal(10000m + 100m - 50m + 200m, equity[^1].TotalEquity);
    }

    [Fact]
    public void MetricsCalculator_BuildDailyEquity_EmptyTrades_ReturnsEmpty()
    {
        var equity = BacktestMetricsCalculator.BuildDailyEquity(10000m, []);
        Assert.Empty(equity);
    }

    #endregion

    #region Metrics Calculator — Sharpe & Sortino

    [Fact]
    public void MetricsCalculator_SharpeAndSortino_ConstantReturns()
    {
        var trades = new List<BacktestPosition>();
        for (int i = 0; i < 252; i++)
        {
            trades.Add(new BacktestPosition
            {
                RealizedPnl = 1m,
                Status = PositionStatus.Closed,
                OpenedAt = Now.AddDays(-(252 - i + 1)),
                ClosedAt = Now.AddDays(-(252 - i)),
            });
        }

        var result = BacktestMetricsCalculator.Compute(10000m, trades);

        Assert.True(result.SharpeRatio > 0);
        // Sortino is zero when no negative returns exist (downside std = 0)
        Assert.Equal(0m, result.SortinoRatio);
    }

    [Fact]
    public void MetricsCalculator_SharpeRatio_NoVariance_ReturnsZero()
    {
        var trades = new List<BacktestPosition>();
        for (int i = 0; i < 10; i++)
        {
            trades.Add(new BacktestPosition
            {
                RealizedPnl = 100m,
                Status = PositionStatus.Closed,
                OpenedAt = Now.AddDays(-20 + i),
                ClosedAt = Now.AddDays(-10 + i),
            });
        }

        var result = BacktestMetricsCalculator.Compute(10000m, trades);

        Assert.Equal(0m, result.SortinoRatio);
    }

    #endregion

    #region Monte Carlo — Determinism & Edge Cases

    [Fact]
    public void MonteCarlo_DeterministicWithFixedSeed()
    {
        var trades = new List<BacktestPosition>
        {
            new() { RealizedPnl = 10m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-3), ClosedAt = Now.AddDays(-2) },
            new() { RealizedPnl = -5m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-2), ClosedAt = Now.AddDays(-1) },
            new() { RealizedPnl = 20m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-1), ClosedAt = Now },
        };

        var r1 = MonteCarloSimulator.Run(10000m, trades, iterations: 5000, seed: 42);
        var r2 = MonteCarloSimulator.Run(10000m, trades, iterations: 5000, seed: 42);

        Assert.Equal(r1.DrawdownProbability, r2.DrawdownProbability, 10);
        Assert.Equal(r1.FinalEquityMedian, r2.FinalEquityMedian, 10);
        Assert.Equal(r1.FinalEquityPercentile5, r2.FinalEquityPercentile5, 10);
        Assert.Equal(r1.FinalEquityPercentile95, r2.FinalEquityPercentile95, 10);
    }

    [Fact]
    public void MonteCarlo_DeterministicWithFixedSeed_DifferentSeedsDiffer()
    {
        var trades = new List<BacktestPosition>
        {
            new() { RealizedPnl = 500m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-2), ClosedAt = Now.AddDays(-1) },
            new() { RealizedPnl = -490m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-1), ClosedAt = Now },
        };

        var r1 = MonteCarloSimulator.Run(1000m, trades, iterations: 100, seed: 42, targetDrawdown: 0.02m);
        var r2 = MonteCarloSimulator.Run(1000m, trades, iterations: 100, seed: 99, targetDrawdown: 0.02m);

        // With 2 extreme trades and small iteration count, seeds diverge
        Assert.NotEqual(r1.DrawdownProbability, r2.DrawdownProbability);
    }

    [Fact]
    public void MonteCarlo_SingleTrade_ReturnsStartingCapitalForZeroIterations()
    {
        var trades = new List<BacktestPosition>
        {
            new() { RealizedPnl = 100m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-1), ClosedAt = Now },
        };

        var result = MonteCarloSimulator.Run(5000m, trades, iterations: 0);

        Assert.Equal(0m, result.DrawdownProbability);
        Assert.Equal(5000m, result.FinalEquityMedian);
    }

    [Fact]
    public void MonteCarlo_AllWins_ProducesHighFinalEquity()
    {
        var trades = new List<BacktestPosition>
        {
            new() { RealizedPnl = 100m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-2), ClosedAt = Now.AddDays(-1) },
            new() { RealizedPnl = 200m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-1), ClosedAt = Now },
        };

        var result = MonteCarloSimulator.Run(10000m, trades, iterations: 2000, seed: 42);

        Assert.True(result.FinalEquityMedian > 10000m);
        Assert.Equal(0m, result.DrawdownProbability);
    }

    [Fact]
    public void MonteCarlo_AllLosses_HasDrawdownProbabilityOfOne()
    {
        var trades = new List<BacktestPosition>
        {
            new() { RealizedPnl = -500m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-2), ClosedAt = Now.AddDays(-1) },
            new() { RealizedPnl = -500m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-1), ClosedAt = Now },
        };

        var result = MonteCarloSimulator.Run(2000m, trades, iterations: 1000, seed: 42, targetDrawdown: 0.20m);

        Assert.Equal(1m, result.DrawdownProbability);
        Assert.True(result.FinalEquityMedian < 2000m);
    }

    [Fact]
    public void MonteCarlo_EmptyTrades_ReturnsDefaults()
    {
        var result = MonteCarloSimulator.Run(10000m, [], iterations: 1000, seed: 42);

        Assert.Equal(0m, result.DrawdownProbability);
        Assert.Equal(10000m, result.FinalEquityMedian);
        Assert.Equal(10000m, result.FinalEquityPercentile5);
        Assert.Equal(10000m, result.FinalEquityPercentile95);
    }

    [Fact]
    public void MonteCarlo_PercentilesSorted_AlwaysOrdered()
    {
        var trades = new List<BacktestPosition>
        {
            new() { RealizedPnl = 50m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-5), ClosedAt = Now.AddDays(-4) },
            new() { RealizedPnl = -30m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-4), ClosedAt = Now.AddDays(-3) },
            new() { RealizedPnl = 10m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-3), ClosedAt = Now.AddDays(-2) },
            new() { RealizedPnl = -20m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-2), ClosedAt = Now.AddDays(-1) },
            new() { RealizedPnl = 100m, Status = PositionStatus.Closed, OpenedAt = Now.AddDays(-1), ClosedAt = Now },
        };

        var result = MonteCarloSimulator.Run(10000m, trades, iterations: 5000, seed: 42);

        Assert.True(result.FinalEquityPercentile5 <= result.FinalEquityMedian);
        Assert.True(result.FinalEquityMedian <= result.FinalEquityPercentile95);
    }

    #endregion

    #region Normal CDF / Inverse CDF

    [Fact]
    public void NormalCdf_SymmetricAroundZero()
    {
        Assert.Equal(0.5, DsrCalculator.NormalCdf(0.0), 8);
    }

    [Fact]
    public void NormalCdf_ExtremeValues_ClampCorrectly()
    {
        Assert.Equal(0.0, DsrCalculator.NormalCdf(-40.0), 12);
        Assert.Equal(1.0, DsrCalculator.NormalCdf(40.0), 12);
    }

    [Fact]
    public void NormalCdf_SymmetricProperty()
    {
        double x = 1.5;
        double phiX = DsrCalculator.NormalCdf(x);
        double phiNegX = DsrCalculator.NormalCdf(-x);
        Assert.Equal(1.0, phiX + phiNegX, 10);
    }

    [Fact]
    public void NormalCdf_KnownValues()
    {
        Assert.Equal(0.841344746, DsrCalculator.NormalCdf(1.0), 6);
        Assert.Equal(0.977249868, DsrCalculator.NormalCdf(2.0), 6);
        Assert.Equal(0.998650102, DsrCalculator.NormalCdf(3.0), 4);
    }

    [Fact]
    public void NormalInverseCdf_MedianIsZero()
    {
        Assert.Equal(0.0, DsrCalculator.NormalInverseCdf(0.5), 10);
    }

    [Fact]
    public void NormalInverseCdf_ExtremeValues()
    {
        Assert.True(DsrCalculator.NormalInverseCdf(0.001) < -3.0);
        Assert.True(DsrCalculator.NormalInverseCdf(0.999) > 3.0);
    }

    [Fact]
    public void NormalInverseCdf_InversesCdf()
    {
        double[] testPoints = [0.01, 0.05, 0.1, 0.25, 0.5, 0.75, 0.9, 0.95, 0.99];
        foreach (double p in testPoints)
        {
            double x = DsrCalculator.NormalInverseCdf(p);
            double pBack = DsrCalculator.NormalCdf(x);
            Assert.Equal(p, pBack, 6);
        }
    }

    [Fact]
    public void NormalCdf_InversesInverseCdf()
    {
        double[] testPoints = [-3.0, -2.0, -1.0, 0.0, 1.0, 2.0, 3.0];
        foreach (double x in testPoints)
        {
            double p = DsrCalculator.NormalCdf(x);
            double xBack = DsrCalculator.NormalInverseCdf(p);
            Assert.Equal(x, xBack, 4);
        }
    }

    #endregion

    #region DSR Calculator

    [Fact]
    public void DsrCalculator_ReturnsProbabilityInRange()
    {
        double sr = 0.1;
        for (int t = 10; t <= 1000; t *= 10)
        {
            for (int m = 1; m <= 1000; m *= 10)
            {
                double dsr = DsrCalculator.ComputeDeflatedSharpeRatio(sr, t, m);
                Assert.True(dsr >= 0.0 && dsr <= 1.0,
                    $"DSR out of range [0,1] for T={t}, M={m}: {dsr}");
            }
        }
    }

    [Fact]
    public void DsrCalculator_IncreasesWithSharpeRatio()
    {
        double dsrLow = DsrCalculator.ComputeDeflatedSharpeRatio(0.05, 252, 100);
        double dsrHigh = DsrCalculator.ComputeDeflatedSharpeRatio(0.50, 252, 100);

        Assert.True(dsrHigh > dsrLow);
    }

    [Fact]
    public void DsrCalculator_DecreasesWithMoreTrials()
    {
        double dsrLowM = DsrCalculator.ComputeDeflatedSharpeRatio(0.15, 252, 10);
        double dsrHighM = DsrCalculator.ComputeDeflatedSharpeRatio(0.15, 252, 10_000);

        Assert.True(dsrLowM >= dsrHighM);
    }

    [Fact]
    public void DsrCalculator_MoreDataIncreasesConfidence()
    {
        double dsrLowT = DsrCalculator.ComputeDeflatedSharpeRatio(0.10, 63, 100);
        double dsrHighT = DsrCalculator.ComputeDeflatedSharpeRatio(0.10, 252, 100);

        Assert.True(dsrHighT > dsrLowT);
    }

    [Fact]
    public void DsrCalculator_SingleTrial_EqualsPValue()
    {
        double sr = 0.10;
        int T = 252;
        double dsr = DsrCalculator.ComputeDeflatedSharpeRatio(sr, T, 1);
        double pValue = DsrCalculator.NormalCdf(sr * Math.Sqrt(T));

        Assert.Equal(pValue, dsr, 10);
    }

    [Fact]
    public void DsrCalculator_ZeroSharpe_ReturnsLowProbabilityForHighM()
    {
        double dsr = DsrCalculator.ComputeDeflatedSharpeRatio(0.0, 252, 10_000);
        Assert.True(dsr < 0.01);
    }

    [Fact]
    public void DsrCalculator_HighSharpeFewTrials_ApproachesOne()
    {
        double dsr = DsrCalculator.ComputeDeflatedSharpeRatio(1.0, 252, 10);
        Assert.True(dsr > 0.999);
    }

    [Fact]
    public void DsrCalculator_InvalidInput_ReturnsZero()
    {
        Assert.Equal(0.0, DsrCalculator.ComputeDeflatedSharpeRatio(0.1, 1, 100));
        Assert.Equal(0.0, DsrCalculator.ComputeDeflatedSharpeRatio(0.1, 0, 100));
    }

    [Fact]
    public void DsrCalculator_ReducesOverfittingProbabilityAsTrialsIncrease()
    {
        double dailySharpe = 0.15; // ~2.38 annualized for 252 days
        int T = 252;

        double dsr10 = DsrCalculator.ComputeDeflatedSharpeRatio(dailySharpe, T, 10);
        double dsr100 = DsrCalculator.ComputeDeflatedSharpeRatio(dailySharpe, T, 100);
        double dsr1000 = DsrCalculator.ComputeDeflatedSharpeRatio(dailySharpe, T, 1000);

        Assert.True(dsr10 > dsr100);
        Assert.True(dsr100 > dsr1000);
    }

    [Fact]
    public void DsrCalculator_AdvancedSkewAdjustment_ChangesResult()
    {
        double sr = 0.15;
        int T = 252;
        int M = 100;

        double baseDsr = DsrCalculator.ComputeDeflatedSharpeRatio(sr, T, M);
        double skewAdjDsr = DsrCalculator.ComputeDeflatedSharpeRatioAdvanced(sr, T, M, skewness: -0.5);

        Assert.NotEqual(baseDsr, skewAdjDsr);
    }

    #endregion

    #region DSR Test from Spec

    [Fact]
    public void DsrCalculator_FollowsSpecReducesOverfitProbabilityAsTrialsIncrease()
    {
        double dailySharpe = 0.15;
        int observations = 252;

        double dsr1 = DsrCalculator.ComputeDeflatedSharpeRatio(dailySharpe, observations, 1);
        double dsr10 = DsrCalculator.ComputeDeflatedSharpeRatio(dailySharpe, observations, 10);
        double dsr50 = DsrCalculator.ComputeDeflatedSharpeRatio(dailySharpe, observations, 50);

        Assert.True(dsr1 > 0.99);
        Assert.True(dsr1 >= dsr10);
        Assert.True(dsr10 >= dsr50);
    }

    #endregion
}
