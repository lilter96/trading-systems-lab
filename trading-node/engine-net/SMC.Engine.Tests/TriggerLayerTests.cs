using Microsoft.Extensions.Logging.Abstractions;
using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Models;

namespace SMC.Engine.Tests;

public class TriggerLayerTests
{
    private static readonly DateTimeOffset Epoch = DateTimeOffset.UnixEpoch;

    private static readonly PoiZoneConfig TestPoiConfig = new()
    {
        MinGapAtrMultiple = 0.1,
        DisplacementAtrMultiple = 1.5,
        ObPromotionWindowBars = 4,
        AtrPeriod = 14,
    };

    private static readonly TriggerConfig DefaultTriggerConfig = new()
    {
        PoiWeight = 0.25,
        DeltaWeight = 0.25,
        OiWeight = 0.25,
        SmtWeight = 0.25,
        TechnicalWeight = 1.0,
        SentimentWeight = 0.0,
        DeltaWindowBars = 4,
        OiNoiseBand = 0.0001,
        SentimentScoreOverride = 0.0,
    };

    private static BarDataProto MakeBar(long tsMin, decimal open, decimal high, decimal low, decimal close,
        decimal delta = 0, decimal oi = 0)
        => new(Epoch + TimeSpan.FromMinutes(tsMin), "15m", open, high, low, close, 0, delta, oi);

    private static PoiZoneService MakePoiService()
    {
        var svc = new PoiZoneService(null!, NullLogger<PoiZoneService>.Instance);
        svc.SetTestConfiguration(TestPoiConfig);
        return svc;
    }

    private static TriggerService MakeTriggerService(TriggerConfig? config = null)
    {
        var svc = new TriggerService(null!, NullLogger<TriggerService>.Instance);
        svc.SetTestConfiguration(config ?? DefaultTriggerConfig);
        return svc;
    }

    private static void SeedAtr(PoiZoneService svc, int count = 14)
    {
        for (int i = 0; i < count; i++)
            svc.ProcessBar("BTC-USDT", MakeBar(i, 100.5m, 101m, 100m, 100.5m), Array.Empty<StructureBreak>());
    }

    private static (PoiZoneService poiService, List<POI> activePois) CreateBullishFvg()
    {
        var svc = MakePoiService();
        SeedAtr(svc);

        svc.ProcessBar("BTC-USDT", MakeBar(14, 100.5m, 101m, 100m, 100.5m), Array.Empty<StructureBreak>());
        svc.ProcessBar("BTC-USDT", MakeBar(15, 101m, 101.5m, 100.5m, 101m), Array.Empty<StructureBreak>());
        svc.ProcessBar("BTC-USDT", MakeBar(16, 108m, 110m, 108m, 109m), Array.Empty<StructureBreak>());

        var pois = svc.GetActivePOIs("BTC-USDT", "15m").ToList();
        return (svc, pois);
    }

    // ======== Absorption/Delta Tests ========

    [Fact]
    public void AbsorptionDetected_DeltaDivergesFromPrice()
    {
        var (_, activePois) = CreateBullishFvg();
        var trigger = MakeTriggerService();
        Assert.NotEmpty(activePois);

        // 5 bars with rising close but falling cumulative delta
        // Bars 17-20: don't enter zone (low >= 108)
        trigger.Evaluate("BTC-USDT", MakeBar(17, 110m, 111m, 108.5m, 110.5m, 10m, 1000m), activePois, [], []);
        trigger.Evaluate("BTC-USDT", MakeBar(18, 110.5m, 112m, 109m, 111m, 2m, 1010m), activePois, [], []);
        trigger.Evaluate("BTC-USDT", MakeBar(19, 111m, 113m, 109m, 111.5m, -5m, 1020m), activePois, [], []);
        trigger.Evaluate("BTC-USDT", MakeBar(20, 111.5m, 113m, 108.5m, 112m, -3m, 1030m), activePois, [], []);

        // Bar 21: enters zone (low=107 < UpperBound=108), absorption detectable
        var results = trigger.Evaluate("BTC-USDT", MakeBar(21, 112m, 114m, 107m, 113m, -8m, 1040m), activePois, [], []);

        var eval = Assert.Single(results);
        Assert.Equal(1.0m, eval.AbsorptionScore);
    }

    [Fact]
    public void NoAbsorption_DeltaConfirmsPrice()
    {
        var (_, activePois) = CreateBullishFvg();
        var trigger = MakeTriggerService();
        Assert.NotEmpty(activePois);

        // 5 bars with rising close and rising cumulative delta
        trigger.Evaluate("BTC-USDT", MakeBar(17, 110m, 111m, 108.5m, 110.5m, 1m, 1000m), activePois, [], []);
        trigger.Evaluate("BTC-USDT", MakeBar(18, 110.5m, 112m, 109m, 111m, 2m, 1010m), activePois, [], []);
        trigger.Evaluate("BTC-USDT", MakeBar(19, 111m, 113m, 109m, 111.5m, 3m, 1020m), activePois, [], []);
        trigger.Evaluate("BTC-USDT", MakeBar(20, 111.5m, 113m, 108.5m, 112m, 4m, 1030m), activePois, [], []);

        var results = trigger.Evaluate("BTC-USDT", MakeBar(21, 112m, 114m, 107m, 113m, 5m, 1040m), activePois, [], []);

        var eval = Assert.Single(results);
        Assert.Equal(0m, eval.AbsorptionScore);
    }

    // ======== Weighting Tests ========

    [Fact]
    public void ConfluenceScore_SubComponentWeighting()
    {
        var (_, activePois) = CreateBullishFvg();
        var triggerDefault = MakeTriggerService();

        var configHighPoi = new TriggerConfig
        {
            PoiWeight = 0.5,
            DeltaWeight = 0.25,
            OiWeight = 0.25,
            SmtWeight = 0.25,
            TechnicalWeight = 1.0,
            SentimentWeight = 0.0,
            DeltaWindowBars = 4,
            OiNoiseBand = 0.0001,
            SentimentScoreOverride = 0.0,
        };
        var triggerHighPoi = MakeTriggerService(configHighPoi);

        // Bars: enter zone on 5th bar
        triggerDefault.Evaluate("BTC-USDT", MakeBar(17, 110m, 111m, 108.5m, 110.5m, 10m, 1000m), activePois, [], []);
        triggerDefault.Evaluate("BTC-USDT", MakeBar(18, 110.5m, 112m, 109m, 111m, 2m, 1010m), activePois, [], []);
        triggerDefault.Evaluate("BTC-USDT", MakeBar(19, 111m, 113m, 109m, 111.5m, -5m, 1020m), activePois, [], []);
        triggerDefault.Evaluate("BTC-USDT", MakeBar(20, 111.5m, 113m, 108.5m, 112m, -3m, 1030m), activePois, [], []);

        triggerHighPoi.Evaluate("BTC-USDT", MakeBar(17, 110m, 111m, 108.5m, 110.5m, 10m, 1000m), activePois, [], []);
        triggerHighPoi.Evaluate("BTC-USDT", MakeBar(18, 110.5m, 112m, 109m, 111m, 2m, 1010m), activePois, [], []);
        triggerHighPoi.Evaluate("BTC-USDT", MakeBar(19, 111m, 113m, 109m, 111.5m, -5m, 1020m), activePois, [], []);
        triggerHighPoi.Evaluate("BTC-USDT", MakeBar(20, 111.5m, 113m, 108.5m, 112m, -3m, 1030m), activePois, [], []);

        var bar21 = MakeBar(21, 112m, 114m, 107m, 113m, -8m, 1040m);
        var resultDefault = Assert.Single(triggerDefault.Evaluate("BTC-USDT", bar21, activePois, [], []));
        var resultHighPoi = Assert.Single(triggerHighPoi.Evaluate("BTC-USDT", bar21, activePois, [], []));

        // Higher PoiWeight should produce higher SmcSetupScore
        Assert.True(resultHighPoi.SmcSetupScore > resultDefault.SmcSetupScore);
    }

    [Fact]
    public void ConfluenceScore_AblatedComponent_StillComputesCleanly()
    {
        var (_, activePois) = CreateBullishFvg();

        var zeroPoiConfig = new TriggerConfig
        {
            PoiWeight = 0.0, DeltaWeight = 0.25, OiWeight = 0.25, SmtWeight = 0.25,
            TechnicalWeight = 1.0, SentimentWeight = 0.0, DeltaWindowBars = 4,
            OiNoiseBand = 0.0001, SentimentScoreOverride = 0.0,
        };
        var zeroDeltaConfig = new TriggerConfig
        {
            PoiWeight = 0.25, DeltaWeight = 0.0, OiWeight = 0.25, SmtWeight = 0.25,
            TechnicalWeight = 1.0, SentimentWeight = 0.0, DeltaWindowBars = 4,
            OiNoiseBand = 0.0001, SentimentScoreOverride = 0.0,
        };

        var triggerPoi = MakeTriggerService(zeroPoiConfig);
        var triggerDelta = MakeTriggerService(zeroDeltaConfig);

        var bars = new[]
        {
            MakeBar(17, 110m, 111m, 108.5m, 110.5m, 10m, 1000m),
            MakeBar(18, 110.5m, 112m, 109m, 111m, 2m, 1010m),
            MakeBar(19, 111m, 113m, 109m, 111.5m, -5m, 1020m),
            MakeBar(20, 111.5m, 113m, 108.5m, 112m, -3m, 1030m),
            MakeBar(21, 112m, 114m, 107m, 113m, -8m, 1040m),
        };

        IReadOnlyList<TriggerEvaluation>? evalPoi = null;
        IReadOnlyList<TriggerEvaluation>? evalDelta = null;

        for (int i = 0; i < bars.Length; i++)
        {
            var r1 = triggerPoi.Evaluate("BTC-USDT", bars[i], activePois, [], []);
            var r2 = triggerDelta.Evaluate("BTC-USDT", bars[i], activePois, [], []);
            if (r1.Count > 0) evalPoi = r1;
            if (r2.Count > 0) evalDelta = r2;
        }

        Assert.NotNull(evalPoi);
        Assert.NotNull(evalDelta);
        var e1 = Assert.Single(evalPoi);
        var e2 = Assert.Single(evalDelta);

        Assert.InRange(e1.SmcSetupScore, -1m, 1m);
        Assert.InRange(e2.SmcSetupScore, -1m, 1m);
        Assert.InRange(e1.ConfluenceScore, -1m, 1m);
        Assert.InRange(e2.ConfluenceScore, -1m, 1m);
    }

    [Fact]
    public void ConfluenceScore_UsesDecimalThroughout()
    {
        var (_, activePois) = CreateBullishFvg();

        var config = new TriggerConfig
        {
            PoiWeight = 0.25, DeltaWeight = 0.25, OiWeight = 0.25, SmtWeight = 0.25,
            TechnicalWeight = 1.0, SentimentWeight = 0.0, DeltaWindowBars = 4,
            OiNoiseBand = 0.0001, SentimentScoreOverride = 0.0,
        };
        var svc = MakeTriggerService(config);

        // Repeating decimal values to expose any double precision artifacts
        var b1 = MakeBar(17, 100m / 3m, 110m / 3m, 100m / 3m, 100m / 3m, 10m / 3m, 1000m / 3m);
        var b2 = MakeBar(18, 100m / 3m, 110m / 3m, 100m / 3m, 110m / 3m, 10m / 3m, 1000m / 3m);
        var b3 = MakeBar(19, 110m / 3m, 120m / 3m, 110m / 3m, 120m / 3m, -5m / 3m, 1000m / 3m);
        var b4 = MakeBar(20, 120m / 3m, 130m / 3m, 120m / 3m, 130m / 3m, -5m / 3m, 1000m / 3m);
        var b5 = MakeBar(21, 130m / 3m, 140m / 3m, 107m, 140m / 3m, -8m / 3m, 1000m / 3m);

        svc.Evaluate("BTC-USDT", b1, activePois, [], []);
        svc.Evaluate("BTC-USDT", b2, activePois, [], []);
        svc.Evaluate("BTC-USDT", b3, activePois, [], []);
        svc.Evaluate("BTC-USDT", b4, activePois, [], []);

        var results = svc.Evaluate("BTC-USDT", b5, activePois, [], []);
        var eval = Assert.Single(results);

        // All score fields should be valid decimal values within [-1, 1] or [0, 1]
        Assert.InRange(eval.PoiQualityScore, 0m, 1m);
        Assert.InRange(eval.AbsorptionScore, -1m, 1m);
        Assert.InRange(eval.OiScore, -1m, 1m);
        Assert.InRange(eval.SmtScore, -1m, 1m);
        Assert.InRange(eval.SmcSetupScore, -1m, 1m);
        Assert.InRange(eval.ConfluenceScore, -1m, 1m);

        // Verify no silent truncation or zero due to overflow
        Assert.NotEqual(0m, eval.PoiQualityScore);
    }

    // ======== Positioning Classifier ========

    [Theory]
    [InlineData(10, 5, 0.0001, PositioningType.FreshLongs)]
    [InlineData(10, -5, 0.0001, PositioningType.ShortCovering)]
    [InlineData(-10, 5, 0.0001, PositioningType.FreshShorts)]
    [InlineData(-10, -5, 0.0001, PositioningType.LongCovering)]
    [InlineData(3, 5, 10, PositioningType.Ambiguous)]    // |ΔPrice| ≤ noiseBand
    [InlineData(10, 0, 0.0001, PositioningType.Ambiguous)] // ΔOI == 0
    public void PositioningClassifier_TruthTable(decimal deltaPrice, decimal deltaOi, double noiseBand, PositioningType expected)
    {
        var result = PositioningClassifier.Classify(deltaPrice, deltaOi, (decimal)noiseBand);
        Assert.Equal(expected, result);
    }
}
