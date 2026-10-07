using Microsoft.Extensions.Logging.Abstractions;
using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Models;

namespace SMC.Engine.Tests;

public class PoiZoneEngineTests
{
    private static readonly DateTimeOffset Epoch = DateTimeOffset.UnixEpoch;

    private static PoiZoneService MakeService()
    {
        var svc = new PoiZoneService(null!, NullLogger<PoiZoneService>.Instance);
        svc.SetTestConfiguration(new PoiZoneConfig
        {
            MinGapAtrMultiple = 0.1,
            DisplacementAtrMultiple = 1.5,
            ObPromotionWindowBars = 4,
            AtrPeriod = 14,
        });
        return svc;
    }

    private static BarDataProto MakeBar(long tsMin, decimal open, decimal high, decimal low, decimal close)
        => new(Epoch + TimeSpan.FromMinutes(tsMin), "15m", open, high, low, close, 0);

    private static StructureBreak MakeBullishBreak(long tsMin)
        => new(StructureEventType.BOS,
            new SwingPoint(0, Epoch, Epoch, SwingType.High, 100m, "BTC-USDT", "15m"),
            105m,
            Epoch + TimeSpan.FromMinutes(tsMin),
            "BTC-USDT",
            "15m",
            Bias.Bullish);

    private static void SeedAtr(PoiZoneService svc, string instrument = "BTC-USDT", int count = 14)
    {
        for (int i = 0; i < count; i++)
            svc.ProcessBar(instrument, MakeBar(i, 100.5m, 101m, 100m, 100.5m), Array.Empty<StructureBreak>());
    }

    // ======== FVG Tests ========

    [Fact]
    public void FVG_Detected_BullishGap()
    {
        var svc = MakeService();
        SeedAtr(svc);

        // Build 3-candle bullish gap: low[16]=108 > high[14]=100.5
        // Bars 14-15 avoid creating spurious gaps by staying within seed range
        svc.ProcessBar("BTC-USDT", MakeBar(14, 100.5m, 101m, 100m, 100.5m), Array.Empty<StructureBreak>());
        svc.ProcessBar("BTC-USDT", MakeBar(15, 101m, 101.5m, 100.5m, 101m), Array.Empty<StructureBreak>());
        var results = svc.ProcessBar("BTC-USDT", MakeBar(16, 108m, 110m, 108m, 109m), Array.Empty<StructureBreak>());

        var fvg = Assert.Single(results);
        Assert.Equal(POIType.FairValueGap, fvg.Type);
        Assert.Equal(MarketBias.Bullish, fvg.Bias);
        Assert.Equal(101m, fvg.LowerBound);
        Assert.Equal(108m, fvg.UpperBound);
        Assert.Equal(POIStatus.Active, fvg.Status);
        Assert.Equal("BTC-USDT", fvg.Instrument);
        Assert.Equal("15m", fvg.Timeframe);
    }

    [Fact]
    public void FVG_Detected_BearishGap()
    {
        var svc = MakeService();
        SeedAtr(svc);

        // Build 3-candle bearish gap: high[16]=95 < low[14]=100.5
        svc.ProcessBar("BTC-USDT", MakeBar(14, 100.5m, 101m, 100m, 100.5m), Array.Empty<StructureBreak>());
        svc.ProcessBar("BTC-USDT", MakeBar(15, 101m, 101.5m, 100.5m, 101m), Array.Empty<StructureBreak>());
        var results = svc.ProcessBar("BTC-USDT", MakeBar(16, 94.5m, 95m, 94m, 94.5m), Array.Empty<StructureBreak>());

        var fvg = Assert.Single(results);
        Assert.Equal(POIType.FairValueGap, fvg.Type);
        Assert.Equal(MarketBias.Bearish, fvg.Bias);
        Assert.Equal(95m, fvg.LowerBound);
        Assert.Equal(100m, fvg.UpperBound);
        Assert.Equal(POIStatus.Active, fvg.Status);
    }

    [Fact]
    public void FVG_Rejected_BelowAtrThreshold()
    {
        var svc = MakeService();
        SeedAtr(svc);
        // ATR ≈ 1.0, so min gap = 0.1

        // Build tiny bullish gap: low[16]=100.08, high[14]=100, gap=0.08 < 0.1
        svc.ProcessBar("BTC-USDT", MakeBar(14, 100m, 100.5m, 99.5m, 100m), Array.Empty<StructureBreak>());
        svc.ProcessBar("BTC-USDT", MakeBar(15, 100.5m, 101m, 100m, 100.5m), Array.Empty<StructureBreak>());
        var results = svc.ProcessBar("BTC-USDT", MakeBar(16, 101m, 103m, 100.08m, 101m), Array.Empty<StructureBreak>());

        Assert.Empty(results);
    }

    [Fact]
    public void FVG_Invalidated_TradedThrough()
    {
        var svc = MakeService();
        SeedAtr(svc);

        // Create Bullish FVG at bar 16: low=108 > high[14]=101 → gap=[101,108]
        svc.ProcessBar("BTC-USDT", MakeBar(14, 100.5m, 101m, 100m, 100.5m), Array.Empty<StructureBreak>());
        svc.ProcessBar("BTC-USDT", MakeBar(15, 101m, 101.5m, 100.5m, 101m), Array.Empty<StructureBreak>());
        svc.ProcessBar("BTC-USDT", MakeBar(16, 108m, 110m, 108m, 109m), Array.Empty<StructureBreak>());

        // Bar 17: low=99 < LowerBound=101 → traded through → Invalidated
        var results = svc.ProcessBar("BTC-USDT", MakeBar(17, 100m, 102m, 99m, 100m), Array.Empty<StructureBreak>());

        var fvg = Assert.Single(results, p => p.Type == POIType.FairValueGap && p.Status == POIStatus.Invalidated);
        Assert.NotNull(fvg);
    }

    // ======== OB Tests ========

    [Fact]
    public void OB_Candidate_RequiresDisplacement()
    {
        var svc = MakeService();
        SeedAtr(svc);

        // Bar 14: bearish candle
        svc.ProcessBar("BTC-USDT", MakeBar(14, 101m, 102m, 99m, 100m), Array.Empty<StructureBreak>());

        // Bar 15: small bullish range (range=1.0) < displacement threshold (1.5)
        var results = svc.ProcessBar("BTC-USDT", MakeBar(15, 100.5m, 101m, 100m, 100.5m), Array.Empty<StructureBreak>());

        Assert.DoesNotContain(results, p => p.Type == POIType.OrderBlock);
    }

    [Fact]
    public void OB_PromotedOnSameDirectionBreak()
    {
        var svc = MakeService();
        SeedAtr(svc);

        // Bar 14: bearish candle (close < open) — this becomes the OB zone [99, 102]
        svc.ProcessBar("BTC-USDT", MakeBar(14, 101m, 102m, 99m, 100m), Array.Empty<StructureBreak>());

        // Bar 15: bullish displacement (range=6 > 1.5, close > open)
        svc.ProcessBar("BTC-USDT", MakeBar(15, 100.5m, 106m, 100m, 105m), Array.Empty<StructureBreak>());

        // Bar 16: neutral, no breaks
        svc.ProcessBar("BTC-USDT", MakeBar(16, 103m, 104m, 102m, 103.5m), Array.Empty<StructureBreak>());

        // Bar 17: bullish break arrives → promote
        var results = svc.ProcessBar(
            "BTC-USDT",
            MakeBar(17, 103m, 105m, 102m, 104m),
            new[] { MakeBullishBreak(17) });

        var ob = Assert.Single(results, p => p.Type == POIType.OrderBlock);
        Assert.Equal(POIStatus.Active, ob.Status);
        Assert.Equal(MarketBias.Bullish, ob.Bias);
        Assert.NotNull(ob.ConfirmingBreak);
        Assert.Equal(99m, ob.LowerBound);
        Assert.Equal(102m, ob.UpperBound);
        Assert.Equal(100.5m, ob.MeanThreshold);
        Assert.Equal("BTC-USDT", ob.Instrument);
        Assert.Equal("15m", ob.Timeframe);
    }

    [Fact]
    public void OB_DiscardedIfNoBreakWithinWindow()
    {
        var svc = MakeService();
        SeedAtr(svc);

        // Bar 14: bearish candle
        svc.ProcessBar("BTC-USDT", MakeBar(14, 101m, 102m, 99m, 100m), Array.Empty<StructureBreak>());

        // Bar 15: bullish displacement
        svc.ProcessBar("BTC-USDT", MakeBar(15, 100.5m, 106m, 100m, 105m), Array.Empty<StructureBreak>());

        // Bars 16-20: five neutral bars without breaks (exceeds 4-bar promotion window)
        for (int i = 16; i <= 20; i++)
            svc.ProcessBar("BTC-USDT", MakeBar(i, 103m, 104m, 102m, 103.5m), Array.Empty<StructureBreak>());

        // Bar 21: bullish break arrives, but candidate should already be expired
        var results = svc.ProcessBar(
            "BTC-USDT",
            MakeBar(21, 103m, 105m, 102m, 104m),
            new[] { MakeBullishBreak(21) });

        Assert.DoesNotContain(results, p => p.Type == POIType.OrderBlock);
    }

    [Fact]
    public void MeanThreshold_Invalidation_BullishOB()
    {
        var svc = MakeService();
        SeedAtr(svc);

        // Create Bullish OB zone=[99, 102], MeanThreshold=100.5
        svc.ProcessBar("BTC-USDT", MakeBar(14, 101m, 102m, 99m, 100m), Array.Empty<StructureBreak>());
        svc.ProcessBar("BTC-USDT", MakeBar(15, 100.5m, 106m, 100m, 105m), Array.Empty<StructureBreak>());

        // Promote OB at bar 16
        svc.ProcessBar(
            "BTC-USDT",
            MakeBar(16, 103m, 104m, 102m, 103.5m),
            new[] { MakeBullishBreak(16) });

        // Bar 17: close=100 < MeanThreshold=100.5 → Invalidated
        var results = svc.ProcessBar(
            "BTC-USDT",
            MakeBar(17, 101m, 102m, 99m, 100m),
            Array.Empty<StructureBreak>());

        var ob = Assert.Single(results, p => p.Type == POIType.OrderBlock && p.Status == POIStatus.Invalidated);
        Assert.Equal(MarketBias.Bullish, ob.Bias);
    }
}
