using Microsoft.Extensions.Logging.Abstractions;
using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Models;

namespace SMC.Engine.Tests;

public class SwingStructureTrackerTests
{
    private static readonly DateTimeOffset Epoch = DateTimeOffset.UnixEpoch;

    private static SwingDetector MakeDetector(int n = 2, string tf = "15m")
        => new(n, tf, NullLogger.Instance);

    private static List<StructureBreak> Feed(SwingDetector d, long i, decimal h, decimal l, decimal c)
        => d.Feed(i, Epoch + TimeSpan.FromMinutes(i), 0, h, l, c).ToList();

    // ======== Swing Detection ========

    [Fact]
    public void DetectSwingHigh_N2()
    {
        var d = MakeDetector(n: 2);
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 112, 109, 110);
        Feed(d, 2, 120, 112, 115);   // SH candidate
        Feed(d, 3, 115, 110, 113);
        var bs = Feed(d, 4, 113, 110, 114); // confirms bar 2

        Assert.Single(d.SwingPoints);
        var sp = d.SwingPoints[0];
        Assert.Equal(SwingType.High, sp.Type);
        Assert.Equal(2, sp.BarIndex);
        Assert.Equal(120m, sp.Price);
    }

    [Fact]
    public void DetectSwingLow_N2()
    {
        var d = MakeDetector(n: 2);
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 112, 109, 110);
        Feed(d, 2, 108, 100, 105);   // SL candidate
        Feed(d, 3, 110, 105, 107);
        var bs = Feed(d, 4, 111, 107, 110); // confirms bar 2

        Assert.Single(d.SwingPoints);
        var sp = d.SwingPoints[0];
        Assert.Equal(SwingType.Low, sp.Type);
        Assert.Equal(2, sp.BarIndex);
        Assert.Equal(100m, sp.Price);
    }

    [Fact]
    public void DetectSwingHigh_N3()
    {
        var d = MakeDetector(n: 3);
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 112, 109, 110);
        Feed(d, 2, 108, 103, 107);
        Feed(d, 3, 120, 112, 115);   // SH candidate
        Feed(d, 4, 115, 110, 113);
        Feed(d, 5, 117, 111, 116);
        Assert.Empty(d.SwingPoints);         // not yet
        var bs = Feed(d, 6, 113, 110, 114);  // confirms bar 3

        Assert.Single(d.SwingPoints);
        var sp = d.SwingPoints[0];
        Assert.Equal(3, sp.BarIndex);
        Assert.Equal(120m, sp.Price);
    }

    [Fact]
    public void EqualHigh_NotASwing()
    {
        var d = MakeDetector(n: 2);
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 115, 110, 112);
        Feed(d, 2, 120, 112, 115);   // H=120
        Feed(d, 3, 117, 113, 116);
        var bs = Feed(d, 4, 120, 114, 118); // H=120 (equal to bar 2)

        Assert.Empty(d.SwingPoints);
    }

    [Fact]
    public void SwingWindow_ExcludesCandidateItself()
    {
        var d = MakeDetector(n: 2);
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 115, 110, 112);
        Feed(d, 2, 120, 112, 115);   // candidate
        Feed(d, 3, 117, 113, 116);
        // At bar 4 (candidate=2), window [0..4] exc 2:
        // H[0]=110, H[1]=115, H[3]=117, H[4]=120 — 120 >= 120 → NOT a swing
        var bs = Feed(d, 4, 120, 114, 118);
        Assert.Empty(d.SwingPoints);
    }

    [Fact]
    public void SwingTie_NotConfirmed()
    {
        var d = MakeDetector(n: 2);
        // Bar 2 has H=120, bar 1 also has H=120 — equal is not a break
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 120, 110, 115);   // H=120 (equals bar 2)
        Feed(d, 2, 120, 112, 116);   // H=120 (equals bar 1)
        Feed(d, 3, 117, 113, 116);
        var bs = Feed(d, 4, 115, 110, 114);

        Assert.Empty(d.SwingPoints);
    }

    // ======== Confirmation Lag ========

    [Fact]
    public void ContinuationLag_Nbars()
    {
        var d = MakeDetector(n: 2);
        Feed(d, 0, 110, 108, 109);
        Assert.Empty(d.SwingPoints);
        Feed(d, 1, 112, 109, 110);
        Assert.Empty(d.SwingPoints);
        Feed(d, 2, 120, 112, 115);   // candidate
        Assert.Empty(d.SwingPoints);  // not yet
        Feed(d, 3, 115, 110, 113);
        Assert.Empty(d.SwingPoints);  // still not
        var bs = Feed(d, 4, 113, 110, 114); // N=2 bars after → confirmed
        Assert.Single(d.SwingPoints);
    }

    // ======== Bias Establishment (Rule 1) ========

    [Fact]
    public void InitialBiasEstablished_NotEmittedAsStructureBreak()
    {
        var d = MakeDetector(n: 2);
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 112, 109, 110);
        Feed(d, 2, 120, 112, 115);   // SH#1
        Feed(d, 3, 115, 105, 112);   // SL#1
        Feed(d, 4, 113, 110, 114);   // confirms SH#1
        Assert.Equal(Bias.Undetermined, d.CurrentBias);

        var bs5 = Feed(d, 5, 114, 108, 113);  // confirms SL#1
        Assert.Equal(Bias.Undetermined, d.CurrentBias);
        Assert.Empty(bs5);

        // close=121 > lastHigh(120) → bias Bullish, but no StructureBreak
        var bs6 = Feed(d, 6, 125, 115, 121);
        Assert.Equal(Bias.Bullish, d.CurrentBias);
        Assert.Empty(bs6);
    }

    // ======== BOS ========

    [Fact]
    public void DetectBOS_Bullish()
    {
        var d = MakeDetector(n: 2);

        // Phase 1: build uptrend → establish bias
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 112, 109, 110);
        Feed(d, 2, 120, 112, 115);   // SH#1
        Feed(d, 3, 115, 105, 112);   // SL#1
        Feed(d, 4, 113, 110, 114);   // confirms SH#1
        Feed(d, 5, 114, 108, 113);   // confirms SL#1
        Feed(d, 6, 125, 115, 121);   // bias=Bullish (no break)

        // Phase 2: BOS — close > ContRef(=SH#1=120)
        var bs7 = Feed(d, 7, 122, 116, 123);
        var bos = bs7.Single(b => b.EventType == StructureEventType.BOS);
        Assert.Equal(Bias.Bullish, bos.ResultingBias);
        Assert.Equal(123m, bos.BreakClosePrice);
    }

    [Fact]
    public void DetectBOS_Bearish()
    {
        var d = MakeDetector(n: 2);

        // Build downtrend
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 112, 109, 110);
        Feed(d, 2, 108, 100, 105);   // SL#1
        Feed(d, 3, 115, 105, 107);   // SH#1
        Feed(d, 4, 111, 107, 110);   // confirms SL#1
        Feed(d, 5, 110, 104, 108);   // confirms SH#1
        Feed(d, 6, 112, 98, 99);     // bias=Bearish (99<100, no break)

        // BOS — close < ContRef(=SL#1=100)
        var bs7 = Feed(d, 7, 108, 94, 96);
        var bos = bs7.Single(b => b.EventType == StructureEventType.BOS);
        Assert.Equal(Bias.Bearish, bos.ResultingBias);
        Assert.Equal(96m, bos.BreakClosePrice);
    }

    // ======== CHoCH ========

    [Fact]
    public void DetectCHoCH_BearishReversal()
    {
        var d = MakeDetector(n: 2);

        // Uptrend: SH#1(2,120), SL#1(3,105) → bias Bullish → BOS → SH#2(8,128) → SH#3(13,140)
        // ContRef=SH#3(13,140), RevRef=SL#1(3,105)
        // Bar 14: close=100 < RevRef=105 → CHoCH Bearish
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 112, 109, 110);
        Feed(d, 2, 120, 112, 115);
        Feed(d, 3, 115, 105, 112);
        Feed(d, 4, 113, 110, 114);
        Feed(d, 5, 114, 108, 113);
        Feed(d, 6, 125, 115, 121);    // bias Bullish
        Feed(d, 7, 122, 116, 123);    // BOS
        Feed(d, 8, 128, 115, 125);
        Feed(d, 9, 124, 118, 122);
        Feed(d, 10, 126, 119, 124);
        Feed(d, 11, 130, 120, 125);
        Feed(d, 12, 124, 115, 118);
        Feed(d, 13, 140, 115, 135);   // SH#3 (ContRef=140)

        // Bar 14: close=100 < RevRef=105 → CHoCH Bearish
        var bs14 = Feed(d, 14, 135, 120, 100);
        var choch = bs14.Single(b => b.EventType == StructureEventType.CHoCH);
        Assert.Equal(Bias.Bearish, choch.ResultingBias);
        Assert.Equal(Bias.Bearish, d.CurrentBias);
    }

    [Fact]
    public void DetectCHoCH_BullishReversal()
    {
        var d = MakeDetector(n: 2);

        // Downtrend → Bullish CHoCH
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 112, 109, 110);
        Feed(d, 2, 108, 100, 105);   // SL#1
        Feed(d, 3, 115, 105, 107);   // SH#1
        Feed(d, 4, 111, 107, 110);   // confirms SL#1
        Feed(d, 5, 110, 104, 108);   // confirms SH#1
        Feed(d, 6, 112, 98, 99);     // bias Bearish
        Feed(d, 7, 108, 94, 96);     // BOS
        Feed(d, 8, 105, 92, 95);     // SL#2
        Feed(d, 9, 108, 96, 100);
        Feed(d, 10, 106, 93, 98);
        Feed(d, 11, 104, 90, 94);    // SL#3 (ContRef=90)
        Feed(d, 12, 107, 95, 102);
        Feed(d, 13, 105, 91, 100);

        // Bar 14: close=115 > RevRef(=SH#1=107) → CHoCH Bullish
        var bs14 = Feed(d, 14, 118, 108, 115);
        var choch = bs14.Single(b => b.EventType == StructureEventType.CHoCH);
        Assert.Equal(Bias.Bullish, choch.ResultingBias);
        Assert.Equal(Bias.Bullish, d.CurrentBias);
    }

    // ======== BOS After CHoCH ========

    [Fact]
    public void BOS_AfterCHoCH_UsesNewBias()
    {
        var d = MakeDetector(n: 2);

        // Phase 1: Uptrend → Bearish CHoCH at bar 14
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 112, 109, 110);
        Feed(d, 2, 120, 112, 115);
        Feed(d, 3, 115, 105, 112);
        Feed(d, 4, 113, 110, 114);
        Feed(d, 5, 114, 108, 113);
        Feed(d, 6, 125, 115, 121);    // bias Bullish
        Feed(d, 7, 122, 116, 123);    // BOS
        Feed(d, 8, 128, 115, 125);
        Feed(d, 9, 124, 118, 122);
        Feed(d, 10, 126, 119, 124);
        Feed(d, 11, 130, 120, 125);
        Feed(d, 12, 124, 115, 118);
        Feed(d, 13, 140, 115, 135);   // SH#3 (ContRef=140, RevRef=105)
        Feed(d, 14, 135, 120, 100);   // CHoCH Bearish (close=100 < 105)

        Assert.Equal(Bias.Bearish, d.CurrentBias);
        Assert.Null(d.ContinuationReference);

        // Phase 2: Create a swing low to fill ContRef
        // SL#4 candidate at bar 16: L=108, window [14..18] exc 16
        // L[14]=120, L[15]=?, L[17]=?, L[18]=? must all be > 108
        Feed(d, 15, 130, 115, 112);
        Feed(d, 16, 120, 108, 110);   // SL#4 candidate (L=108)
        Feed(d, 17, 125, 110, 116);
        // Bar 18 = i=18, candidate=16. L[16]=108.
        // Window [14..18] exc 16: L[14]=120, L[15]=115, L[17]=110, L[18]=?
        // Need L[18] > 108
        var bs18 = Feed(d, 18, 122, 112, 118); // L=112 > 108 → SL#4 confirmed!
        // SL#4(16,108) fills ContRef for Bearish
        Assert.NotNull(d.ContinuationReference);
        Assert.Equal(108m, d.ContinuationReference.Value.Price);

        // Phase 3: BOS — close < ContRef(=108)
        var bs19 = Feed(d, 19, 118, 100, 99);
        var bos = bs19.Single(b => b.EventType == StructureEventType.BOS);
        Assert.Equal(Bias.Bearish, bos.ResultingBias);
        // Verify NOT a CHoCH
        Assert.DoesNotContain(bs19, b => b.EventType == StructureEventType.CHoCH);
    }

    [Fact]
    public void ContinuationReference_PendingAfterCHoCH_BlocksFurtherBOS()
    {
        var d = MakeDetector(n: 2);

        // Uptrend → CHoCH at bar 14
        Feed(d, 0, 110, 108, 109);
        Feed(d, 1, 112, 109, 110);
        Feed(d, 2, 120, 112, 115);
        Feed(d, 3, 115, 105, 112);
        Feed(d, 4, 113, 110, 114);
        Feed(d, 5, 114, 108, 113);
        Feed(d, 6, 125, 115, 121);
        Feed(d, 7, 122, 116, 123);
        Feed(d, 8, 128, 115, 125);
        Feed(d, 9, 124, 118, 122);
        Feed(d, 10, 126, 119, 124);
        Feed(d, 11, 130, 120, 125);
        Feed(d, 12, 124, 115, 118);
        Feed(d, 13, 140, 115, 135);
        Feed(d, 14, 135, 120, 100);   // CHoCH Bearish

        Assert.Equal(Bias.Bearish, d.CurrentBias);
        Assert.Null(d.ContinuationReference);

        // Bars 15-17: price moves lower but no SL confirmed yet.
        // ContRef is null → no BOS fires even though close < previous levels.
        var bs15 = Feed(d, 15, 130, 115, 112);
        Assert.Empty(bs15);
        var bs16 = Feed(d, 16, 125, 110, 114);
        Assert.Empty(bs16);
        var bs17 = Feed(d, 17, 122, 108, 113);
        Assert.Empty(bs17);

        // Create a clear SL: bar 18 L=100, window [16..20] exc 18
        // L[16]=110, L[17]=108, L[19]=?, L[20]=? must all be > 100
        Feed(d, 18, 120, 100, 115);    // SL candidate
        Feed(d, 19, 118, 106, 112);    // L=106 > 100
        // Bar 20 = i=20, candidate=18. L[18]=100.
        // Window [16..20] exc 18: L[16]=110, L[17]=108, L[19]=106, L[20]=? must be > 100
        var bs20 = Feed(d, 20, 122, 110, 118); // L=110 > 100 → SL confirmed!
        Assert.NotNull(d.ContinuationReference);
        Assert.Equal(100m, d.ContinuationReference.Value.Price);

        // Now BOS can fire: close < 100
        var bs21 = Feed(d, 21, 115, 92, 95);
        var bos = bs21.Single(b => b.EventType == StructureEventType.BOS);
        Assert.Equal(Bias.Bearish, bos.ResultingBias);
    }

    // ======== Flat Market ========

    [Fact]
    public void FlatMarket_NoFalseBreaks()
    {
        var d = MakeDetector(n: 2);
        for (int i = 0; i < 20; i++)
        {
            var bs = Feed(d, i, 101, 99, 100);
            Assert.Empty(bs);
        }
        Assert.Empty(d.SwingPoints);
        Assert.Equal(Bias.Undetermined, d.CurrentBias);
    }

    // ======== Multi-Timeframe ========

    [Fact]
    public void TimeframeAggregator_AccumulatesBars()
    {
        var agg = new TimeframeAggregator(3);
        var completed = new List<AggregatedBar>();

        for (int i = 0; i < 10; i++)
        {
            var bar = new AggregatedBar
            {
                Timestamp = i * 60_000L,
                Open = 100 + i,
                High = 102 + i,
                Low = 99 + i,
                Close = 101 + i,
                Volume = 1,
            };
            if (agg.TryFeed(bar, out var outBar))
                completed.Add(outBar);
        }

        Assert.Equal(3, completed.Count);
        Assert.Equal(0, completed[0].Timestamp);
        Assert.Equal(100m, completed[0].Open);
        Assert.Equal(104m, completed[0].High);
        Assert.Equal(99m, completed[0].Low);
        Assert.Equal(103m, completed[0].Close);
    }

    [Fact]
    public void MultiTimeframe_BiasFromHTF()
    {
        var ltfAgg = new TimeframeAggregator(15);
        var ltfDetector = new SwingDetector(2, "15m", NullLogger.Instance);

        for (int i = 0; i < 300; i++)
        {
            long ts = i * 60_000L;
            double trend = i * 0.1;
            double cycle = Math.Sin(i * 0.1) * 5;
            double h = 100 + trend + cycle + 2;
            double l = 100 + trend + cycle - 2;
            double c = (h + l) / 2;

            var bar = new AggregatedBar
            {
                Timestamp = ts,
                Open = (decimal)(c - 0.5),
                High = (decimal)h,
                Low = (decimal)l,
                Close = (decimal)c,
                Volume = 1,
            };
            if (ltfAgg.TryFeed(bar, out var ltfBar))
            {
                ltfDetector.Feed(
                    ltfBar.Timestamp,
                    DateTimeOffset.FromUnixTimeMilliseconds(ltfBar.Timestamp),
                    ltfBar.Open, ltfBar.High, ltfBar.Low, ltfBar.Close);
            }
        }

        Assert.NotEmpty(ltfDetector.SwingPoints);
        Assert.True(ltfDetector.SwingPoints.Count >= 2,
            $"Expected >=2 swings, got {ltfDetector.SwingPoints.Count}");
    }
}
