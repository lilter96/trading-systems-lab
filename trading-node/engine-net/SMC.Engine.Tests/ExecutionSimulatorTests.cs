using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Backtesting;

namespace SMC.Engine.Tests;

public class ExecutionSimulatorTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.UtcNow;

    [Fact]
    public void ExecuteMarketEntry_AppliesSlippageAndTakerFees()
    {
        var config = new BrokerConfig
        {
            InitialBalance = 10000m,
            SlippageTicks = 0.5m,
            TickSize = 0.5m,
            TakerFeeRate = 0.001m,
        };
        var broker = new BacktestBroker(config);

        broker.ExecuteMarketEntry("BTC-USDT", PositionSide.Long, 10m, 9000m, 8900m, 9200m, Now);

        var pos = Assert.Single(broker.Positions);

        Assert.Equal(9000.25m, pos.EntryPrice);
        Assert.Equal(90.0025m, pos.EntryFeePaid);
        Assert.Equal(10000m - 90.0025m, broker.CurrentBalance);
    }

    [Fact]
    public void Matcher_StopsOutWithSlippageAndTakerFees()
    {
        var config = new BrokerConfig
        {
            SlippageTicks = 1.0m,
            TickSize = 0.1m,
            TakerFeeRate = 0.001m,
        };
        var broker = new BacktestBroker(config);
        broker.ExecuteMarketEntry("BTC-USDT", PositionSide.Long, 1m, 1000m, 950m, 1100m, Now);

        var bar = new BarDataProto(
            Now, "1h", 980m, 990m, 940m, 985m, 1000m,
            Instrument: "BTC-USDT");

        broker.ProcessBar(bar);

        var pos = Assert.Single(broker.Positions);
        Assert.Equal(PositionStatus.Closed, pos.Status);
        Assert.Equal(949.9m, pos.ExitPrice);
        Assert.True(pos.RealizedPnl < 0);
    }

    [Fact]
    public void SameBarResolution_EnforcesConservativeFirst()
    {
        var config = new BrokerConfig
        {
            AmbiguityMode = SlTpResolutionMode.ConservativeFirst,
            SlippageTicks = 0m,
        };
        var broker = new BacktestBroker(config);
        broker.ExecuteMarketEntry("BTC", PositionSide.Long, 1m, 100m, 90m, 110m, Now);

        var bar = new BarDataProto(
            Now, "1h", 100m, 115m, 85m, 105m, 1000m,
            Instrument: "BTC");

        broker.ProcessBar(bar);

        var pos = Assert.Single(broker.Positions);
        Assert.Equal(PositionStatus.Closed, pos.Status);
        Assert.True(pos.RealizedPnl < 0);
        Assert.Equal(-10m, pos.RealizedPnl);
    }

    [Fact]
    public void SameBarResolution_EnforcesTargetFirst()
    {
        var config = new BrokerConfig
        {
            AmbiguityMode = SlTpResolutionMode.TargetFirst,
            SlippageTicks = 0m,
        };
        var broker = new BacktestBroker(config);
        broker.ExecuteMarketEntry("BTC", PositionSide.Long, 1m, 100m, 90m, 110m, Now);

        var bar = new BarDataProto(
            Now, "1h", 100m, 115m, 85m, 105m, 1000m,
            Instrument: "BTC");

        broker.ProcessBar(bar);

        var pos = Assert.Single(broker.Positions);
        Assert.Equal(PositionStatus.Closed, pos.Status);
        Assert.True(pos.RealizedPnl > 0);
        Assert.Equal(10m, pos.RealizedPnl);
    }
}
