using Microsoft.Extensions.Logging.Abstractions;
using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Models;
using SMC.Engine.Core.Utils;

namespace SMC.Engine.Tests;

public class SmtDivergenceEngineTests
{
    private static readonly DateTimeOffset Epoch = DateTimeOffset.UnixEpoch;
    private static readonly CorrelationPairConfig TestPair = new()
    {
        Primary = "BTC-USDT",
        Secondary = "ETH-USDT",
        Timeframe = "15m",
        K = 2,
        CorrelationThreshold = 0.7,
        BelowThresholdBehavior = "Suppress",
        CorrelationWindowBars = 100,
    };

    private static SwingDetector MakeDetector(int n = 2, string tf = "15m", string instrument = "")
        => new(n, tf, NullLogger.Instance, instrument);

    private static List<StructureBreak> Feed(SwingDetector d, long i, decimal h, decimal l, decimal c)
        => d.Feed(i, Epoch + TimeSpan.FromMinutes(i), 0, h, l, c).ToList();

    // ======== Foundation Tests ========

    [Fact]
    public void RingBuffer_BasicOperations()
    {
        var buf = new RingBuffer<int>(3);
        Assert.Equal(0, buf.Count);
        Assert.Equal(3, buf.Capacity);

        buf.Add(10);
        Assert.Equal(1, buf.Count);
        Assert.Equal(new[] { 10 }, buf.Snapshot());

        buf.Add(20);
        buf.Add(30);
        Assert.Equal(3, buf.Count);
        Assert.Equal(new[] { 10, 20, 30 }, buf.Snapshot());

        buf.Add(40); // wraps: 10 is evicted
        Assert.Equal(3, buf.Count);
        Assert.Equal(new[] { 20, 30, 40 }, buf.Snapshot());
    }

    [Fact]
    public void RingBuffer_Clear()
    {
        var buf = new RingBuffer<int>(5);
        buf.Add(1);
        buf.Add(2);
        buf.Clear();
        Assert.Equal(0, buf.Count);
        Assert.Empty(buf.Snapshot());
    }

    [Fact]
    public void ReturnsCalculator_LogReturns()
    {
        var closes = new decimal[] { 100, 110, 121 };
        var returns = ReturnsCalculator.ComputeLogReturns(closes);
        Assert.Equal(2, returns.Count);

        // ln(110/100) ≈ 0.09531; ln(121/110) ≈ 0.09531
        Assert.Equal(0.0953101798m, Math.Round(returns[0], 10));
        Assert.Equal(0.0953101798m, Math.Round(returns[1], 10));
    }

    [Fact]
    public void ReturnsCalculator_LessThanTwoCloses_ReturnsEmpty()
    {
        Assert.Empty(ReturnsCalculator.ComputeLogReturns(new decimal[] { 100 }));
        Assert.Empty(ReturnsCalculator.ComputeLogReturns(Array.Empty<decimal>()));
    }

    [Fact]
    public void RollingCorrelation_PerfectPositive()
    {
        var a = new decimal[] { 0.01m, 0.02m, 0.03m, 0.04m, 0.05m };
        var b = new decimal[] { 0.02m, 0.04m, 0.06m, 0.08m, 0.10m };
        var r = RollingCorrelation.Compute(a, b);
        Assert.NotNull(r);
        Assert.Equal(1.0m, Math.Round(r!.Value, 10));
    }

    [Fact]
    public void RollingCorrelation_PerfectNegative()
    {
        var a = new decimal[] { 0.01m, 0.02m, 0.03m, 0.04m, 0.05m };
        var b = new decimal[] { -0.01m, -0.02m, -0.03m, -0.04m, -0.05m };
        var r = RollingCorrelation.Compute(a, b);
        Assert.NotNull(r);
        Assert.Equal(-1.0m, Math.Round(r!.Value, 10));
    }

    [Fact]
    public void RollingCorrelation_ZeroVariance_ReturnsNull()
    {
        var a = new decimal[] { 0.01m, 0.01m, 0.01m };
        var b = new decimal[] { 0.02m, 0.03m, 0.04m };
        var r = RollingCorrelation.Compute(a, b);
        Assert.Null(r);
    }

    [Fact]
    public void RollingCorrelation_MismatchedLengths_ReturnsNull()
    {
        var r = RollingCorrelation.Compute(new decimal[] { 0.01m, 0.02m }, new decimal[] { 0.03m });
        Assert.Null(r);
    }

    [Fact]
    public void RollingCorrelation_LessThanTwoSamples_ReturnsNull()
    {
        var r = RollingCorrelation.Compute(new decimal[] { 0.01m }, new decimal[] { 0.02m });
        Assert.Null(r);
    }

    // ======== SmtDivergenceService integration ========

    private static SmtDivergenceService MakeService()
    {
        return new SmtDivergenceService(null!, null!, NullLogger<SmtDivergenceService>.Instance);
    }

    private static BarDataProto MakeBar(long timestampMinutes, decimal close)
        => new(DateTimeOffset.UnixEpoch + TimeSpan.FromMinutes(timestampMinutes), "15m", 0, 0, 0, close, 0);

    /// <summary>
    /// Feed a SwingDetector bars 0..5 that establish SH#1(2,120) and SL#1(3,105).
    /// Returns the detector ready for bars 6+.
    /// </summary>
    private static void FeedSwings0to5(SwingDetector d)
    {
        // Bar 0: 110/108/109
        // Bar 1: 112/109/110
        // Bar 2: 120/112/115  → SH#1 candidate
        // Bar 3: 115/105/112  → SL#1 candidate
        // Bar 4: 113/110/114  → confirms SH#1(2,120)
        // Bar 5: 114/108/113  → confirms SL#1(3,105)
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 112, 109, 110);
        Feed(d, 2, 120, 112, 115);
        Feed(d, 3, 115, 105, 112);
        Feed(d, 4, 113, 110, 114);
        // bar 5 is a dummy — we call Feed(d, 5, ...) later via the caller
    }

    [Fact]
    public void DivergenceDetected_PrimaryBreaksSecondaryFails_NoDivergenceWhenBothBOS()
    {
        var svc = MakeService();
        svc.SetTestConfiguration(new List<CorrelationPairConfig>
        {
            new()
            {
                Primary = "BTC-USDT",
                Secondary = "ETH-USDT",
                Timeframe = "15m",
                K = 5,
                CorrelationThreshold = 0.01,
                BelowThresholdBehavior = "Suppress",
                CorrelationWindowBars = 50,
            }
        });

        var p = new SwingDetector(2, "15m", instrument: "BTC-USDT");
        var s = new SwingDetector(2, "15m", instrument: "ETH-USDT");

        // Bars 0-5: build swings (SH#1 at 120, SL#1 at 105)
        foreach (var d in new[] { p, s })
        {
            Feed(d, 0, 110, 108, 109);
            Feed(d, 1, 112, 109, 110);
            Feed(d, 2, 120, 112, 115);
            Feed(d, 3, 115, 105, 112);
            Feed(d, 4, 113, 110, 114);
            Feed(d, 5, 114, 108, 113);
        }

        // Bar 6: bias Bullish for both (close=121 > SH#1=120)
        var bar6 = MakeBar(6, 121);
        svc.ProcessBar("ETH-USDT", bar6, Feed(s, 6, 125, 115, 121));
        svc.ProcessBar("BTC-USDT", bar6, Feed(p, 6, 125, 115, 121));

        Assert.Equal(Bias.Bullish, p.CurrentBias);
        Assert.Equal(Bias.Bullish, s.CurrentBias);

        // Bar 7: BOTH get BOS (close > ContRef=120)
        var pBreaks7 = Feed(p, 7, 122, 116, 123);
        var sBreaks7 = Feed(s, 7, 122, 116, 123);

        Assert.Contains(pBreaks7, b => b.EventType == StructureEventType.BOS && b.ResultingBias == Bias.Bullish);
        Assert.Contains(sBreaks7, b => b.EventType == StructureEventType.BOS && b.ResultingBias == Bias.Bullish);

        var bar7 = MakeBar(7, 123);
        svc.ProcessBar("BTC-USDT", bar7, pBreaks7);
        var secBreaks = svc.ProcessBar("ETH-USDT", bar7, sBreaks7);

        // Same-direction BOS for secondary → no divergence
        Assert.Empty(secBreaks);

        // Even after pending BOS re-check, same-direction BOS still blocks
        var bar8 = MakeBar(8, 125);
        var pBreaks8 = svc.ProcessBar("BTC-USDT", bar8, Feed(p, 8, 130, 125, 125));
        Assert.Empty(pBreaks8);
    }

    [Fact]
    public void DivergenceDetected_ChochPrecedesBos_DetectedImmediately()
    {
        // When secondary's CHoCH is in the break buffer BEFORE the primary's BOS
        // is processed, divergence is detected in a single ProcessBar call.
        var svc = MakeService();
        svc.SetTestConfiguration(new List<CorrelationPairConfig>
        {
            new()
            {
                Primary = "BTC-USDT",
                Secondary = "ETH-USDT",
                Timeframe = "15m",
                K = 5,
                CorrelationThreshold = 0.01,
                BelowThresholdBehavior = "Suppress",
                CorrelationWindowBars = 50,
            }
        });

        var p = new SwingDetector(2, "15m", instrument: "BTC-USDT");
        var s = new SwingDetector(2, "15m", instrument: "ETH-USDT");

        // Bars 0-5: swings
        foreach (var d in new[] { p, s })
        {
            Feed(d, 0, 110, 108, 109);
            Feed(d, 1, 112, 109, 110);
            Feed(d, 2, 120, 112, 115);
            Feed(d, 3, 115, 105, 112);
            Feed(d, 4, 113, 110, 114);
            Feed(d, 5, 114, 108, 113);
        }

        // Bar 6: bias Bullish
        svc.ProcessBar("ETH-USDT", MakeBar(6, 121), Feed(s, 6, 125, 115, 121));
        svc.ProcessBar("BTC-USDT", MakeBar(6, 121), Feed(p, 6, 125, 115, 121));

        // Bar 7: secondary close=119 < ContRef=120 → no BOS (stays Bullish)
        //          primary   close=119 < 120 → no BOS yet
        var sBreaks7 = Feed(s, 7, 120, 115, 119);
        var pBreaks7 = Feed(p, 7, 120, 115, 119);
        Assert.DoesNotContain(sBreaks7, b => b.EventType == StructureEventType.BOS);
        Assert.DoesNotContain(pBreaks7, b => b.EventType == StructureEventType.BOS);
        svc.ProcessBar("ETH-USDT", MakeBar(7, 119), sBreaks7);
        svc.ProcessBar("BTC-USDT", MakeBar(7, 119), pBreaks7);

        // Bar 8: secondary CHoCH (close=102 < RevRef=105) → CHoCH Bearish
        var sBreaks8 = Feed(s, 8, 120, 100, 102);
        Assert.Contains(sBreaks8, b => b.EventType == StructureEventType.CHoCH);
        svc.ProcessBar("ETH-USDT", MakeBar(8, 102), sBreaks8);

        // Now secondary has CHoCH in break buffer.
        // Process primary bar 8: close=125 > ContRef=120 → BOS
        var pBreaks8 = Feed(p, 8, 125, 115, 125);
        Assert.Contains(pBreaks8, b => b.EventType == StructureEventType.BOS);
        var results = svc.ProcessBar("BTC-USDT", MakeBar(8, 125), pBreaks8);

        var div = Assert.Single(results);
        Assert.Equal("ETH-USDT", div.SecondaryInstrument);
        Assert.Equal(Bias.Bullish, div.Direction);
        Assert.Equal(SmtSecondaryState.AttemptedAndFailed, div.SecondaryState);
    }

    [Fact]
    public void DivergenceDetected_BosPrecedesChoch_PendingCatchesIt()
    {
        // Primary BOS at bar 7, secondary CHoCH at bar 8.
        // Pending BOS queue re-checks when secondary's CHoCH triggers a ProcessBar call.
        var svc = MakeService();
        svc.SetTestConfiguration(new List<CorrelationPairConfig>
        {
            new()
            {
                Primary = "BTC-USDT",
                Secondary = "ETH-USDT",
                Timeframe = "15m",
                K = 5,
                CorrelationThreshold = 0.01,
                BelowThresholdBehavior = "Suppress",
                CorrelationWindowBars = 50,
            }
        });

        var p = new SwingDetector(2, "15m", instrument: "BTC-USDT");
        var s = new SwingDetector(2, "15m", instrument: "ETH-USDT");

        // Bars 0-5: swings
        foreach (var d in new[] { p, s })
        {
            Feed(d, 0, 110, 108, 109);
            Feed(d, 1, 112, 109, 110);
            Feed(d, 2, 120, 112, 115);
            Feed(d, 3, 115, 105, 112);
            Feed(d, 4, 113, 110, 114);
            Feed(d, 5, 114, 108, 113);
        }

        // Bar 6: bias Bullish
        svc.ProcessBar("BTC-USDT", MakeBar(6, 121), Feed(p, 6, 125, 115, 121));
        svc.ProcessBar("ETH-USDT", MakeBar(6, 121), Feed(s, 6, 125, 115, 121));

        // Bar 7: primary close=123 > ContRef=120 → BOS Bullish
        //        secondary close=119 (< 120) → no BOS (stays Bullish trend, hasn't broken yet)
        var pBreaks7 = Feed(p, 7, 122, 116, 123);
        Assert.Contains(pBreaks7, b => b.EventType == StructureEventType.BOS);

        var sBreaks7 = Feed(s, 7, 120, 115, 119);
        Assert.DoesNotContain(sBreaks7, b => b.EventType == StructureEventType.BOS);

        // Process primary BOS — divergence NOT detected yet (no secondary CHoCH)
        var results = svc.ProcessBar("BTC-USDT", MakeBar(7, 123), pBreaks7);
        Assert.Empty(results);
        svc.ProcessBar("ETH-USDT", MakeBar(7, 119), sBreaks7);

        // Bar 8: secondary CHoCH (close=102 < RevRef=105)
        var sBreaks8 = Feed(s, 8, 120, 100, 102);
        Assert.Contains(sBreaks8, b => b.EventType == StructureEventType.CHoCH);

        // ProcessBar for secondary should trigger pending BOS re-check and find divergence
        results = svc.ProcessBar("ETH-USDT", MakeBar(8, 102), sBreaks8);

        var div = Assert.Single(results);
        Assert.Equal("BTC-USDT", div.PrimaryInstrument);
        Assert.Equal("ETH-USDT", div.SecondaryInstrument);
        Assert.Equal(Bias.Bullish, div.Direction);
        Assert.Equal(SmtSecondaryState.AttemptedAndFailed, div.SecondaryState);
    }
}
