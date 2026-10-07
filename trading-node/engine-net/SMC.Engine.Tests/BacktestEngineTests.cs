using System.Text.Json;
using Microsoft.Extensions.Logging.Abstractions;
using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Backtesting;
using SMC.Engine.Core.Feeds;
using SMC.Engine.Core.Models;

namespace SMC.Engine.Tests;

public class BacktestEngineTests
{
    private static readonly DateTimeOffset Epoch = DateTimeOffset.UnixEpoch;

    private static readonly PoiZoneConfig TestPoiConfig = new()
    {
        MinGapAtrMultiple = 0.1,
        DisplacementAtrMultiple = 1.5,
        ObPromotionWindowBars = 4,
        AtrPeriod = 14,
    };

    private static readonly TriggerConfig TestTriggerConfig = new()
    {
        PoiWeight = 0.25, DeltaWeight = 0.25, OiWeight = 0.25, SmtWeight = 0.25,
        TechnicalWeight = 1.0, SentimentWeight = 0.0, DeltaWindowBars = 4,
        OiNoiseBand = 0.0001, SentimentScoreOverride = 0.0,
    };

    private sealed class MockStructureService : IMarketStructureService
    {
        public Task<IReadOnlyList<StructureBreak>> FeedBarAsync(string instrument, BarDataProto bar)
            => Task.FromResult<IReadOnlyList<StructureBreak>>(Array.Empty<StructureBreak>());

        public StructureStateSnapshot GetState(string instrument, string timeframe)
            => throw new NotSupportedException();
    }

    private sealed class CollectingEventSink : IEngineEventSink
    {
        public List<string> Events { get; } = new();

        public Task HandleStructureBreakAsync(string instrument, string timeframe, StructureBreak @break)
        {
            Events.Add($"STRUCTURE|{instrument}|{timeframe}|{@break.EventType}|{@break.ResultingBias}");
            return Task.CompletedTask;
        }

        public Task HandlePoiLifecycleAsync(string instrument, string timeframe, POI poi)
        {
            Events.Add($"POI|{instrument}|{timeframe}|{poi.Type}|{poi.Bias}|{poi.Status}|{poi.LowerBound}|{poi.UpperBound}");
            return Task.CompletedTask;
        }

        public Task HandleTriggerAsync(string instrument, string timeframe, TriggerEvaluation evaluation)
        {
            Events.Add($"TRIGGER|{instrument}|{timeframe}|{evaluation.Zone.Type}|{evaluation.Zone.Bias}|{evaluation.ConfluenceScore}|{evaluation.AbsorptionScore}|{evaluation.OiScore}|{evaluation.SmtScore}");
            return Task.CompletedTask;
        }
    }

    // ======== Helpers ========

    private static BacktestSession CreateSession(IEngineEventSink sink)
    {
        var structure = new MockStructureService();

        var poi = new PoiZoneService(null!, NullLogger<PoiZoneService>.Instance);
        poi.SetTestConfiguration(TestPoiConfig);

        var smt = new SmtDivergenceService(null!, new MockStructureService(), NullLogger<SmtDivergenceService>.Instance);
        smt.SetTestConfiguration(new List<CorrelationPairConfig>());

        var trigger = new TriggerService(null!, NullLogger<TriggerService>.Instance);
        trigger.SetTestConfiguration(TestTriggerConfig);

        return new BacktestSession(structure, poi, smt, trigger, sink);
    }

    private static List<BarDataProto> GenerateTestBars(int count)
    {
        var bars = new List<BarDataProto>(count);
        for (int i = 0; i < count; i++)
        {
            double phase = i * 0.1;
            decimal price = 100m + (decimal)Math.Sin(phase) * 5m;
            decimal high = price + 1m + (decimal)Math.Abs(Math.Sin(phase + 0.5)) * 2m;
            decimal low = price - 1m - (decimal)Math.Abs(Math.Sin(phase - 0.5)) * 2m;
            decimal delta = (decimal)Math.Sin(i * 0.2) * 10m;
            decimal oi = 1000m + (decimal)Math.Sin(i * 0.05) * 200m;

            bars.Add(new BarDataProto(
                Epoch + TimeSpan.FromMinutes(i),
                "15m",
                price, high, low, price,
                1000m, delta, oi,
                "BTC-USDT"));
        }
        return bars;
    }

    private static BarDataProto MakeBar(long tsMin, decimal close, string instrument)
        => new(
            Epoch + TimeSpan.FromMinutes(tsMin),
            "15m",
            close, close + 1m, close - 1m, close,
            1000m, 0m, 1000m,
            instrument);

    private static BarDataProto MakeBar(long tsMin, decimal open, decimal high, decimal low, decimal close,
        string instrument, decimal delta = 0, decimal oi = 1000, string timeframe = "15m")
        => new(
            Epoch + TimeSpan.FromMinutes(tsMin),
            timeframe, open, high, low, close,
            1000m, delta, oi,
            instrument);

    // ======== Stream Merger Tests ========

    [Fact]
    public async Task Backtest_StrictChronology()
    {
        var feedA = new InMemoryFeed(new[]
        {
            MakeBar(1, 100m, "BTC-USDT"),
            MakeBar(3, 101m, "BTC-USDT"),
            MakeBar(5, 102m, "BTC-USDT"),
        });
        var feedB = new InMemoryFeed(new[]
        {
            MakeBar(2, 200m, "ETH-USDT"),
            MakeBar(4, 201m, "ETH-USDT"),
            MakeBar(6, 202m, "ETH-USDT"),
        });

        var merged = new List<BarDataProto>();
        await foreach (var bar in ChronologicalStreamMerger.MergeAsync(new IMarketDataFeed[] { feedA, feedB }))
            merged.Add(bar);

        Assert.Equal(6, merged.Count);
        Assert.Equal("BTC-USDT", merged[0].Instrument);
        Assert.Equal(100m, merged[0].Close);
        Assert.Equal("ETH-USDT", merged[1].Instrument);
        Assert.Equal(200m, merged[1].Close);
        Assert.Equal("BTC-USDT", merged[2].Instrument);
        Assert.Equal("ETH-USDT", merged[3].Instrument);
        Assert.Equal("BTC-USDT", merged[4].Instrument);
        Assert.Equal("ETH-USDT", merged[5].Instrument);

        for (int i = 1; i < merged.Count; i++)
            Assert.True(merged[i].Timestamp > merged[i - 1].Timestamp,
                $"Bar {i} timestamp is not after bar {i - 1}");
    }

    [Fact]
    public async Task Backtest_Chronology_SingleFeed()
    {
        var feed = new InMemoryFeed(new[] { MakeBar(1, 100m, "BTC"), MakeBar(2, 101m, "BTC") });
        var merged = new List<BarDataProto>();
        await foreach (var bar in ChronologicalStreamMerger.MergeAsync(new IMarketDataFeed[] { feed }))
            merged.Add(bar);
        Assert.Equal(2, merged.Count);
        Assert.Equal(100m, merged[0].Close);
        Assert.Equal(101m, merged[1].Close);
    }

    [Fact]
    public async Task Backtest_Chronology_EmptyFeeds()
    {
        var feedA = new InMemoryFeed(Array.Empty<BarDataProto>());
        var feedB = new InMemoryFeed(Array.Empty<BarDataProto>());
        var merged = new List<BarDataProto>();
        await foreach (var bar in ChronologicalStreamMerger.MergeAsync(new IMarketDataFeed[] { feedA, feedB }))
            merged.Add(bar);
        Assert.Empty(merged);
    }

    [Fact]
    public async Task Backtest_Chronology_UnequalLengths()
    {
        var feedA = new InMemoryFeed(new[] { MakeBar(1, 100m, "BTC"), MakeBar(3, 101m, "BTC") });
        var feedB = new InMemoryFeed(new[] { MakeBar(2, 200m, "ETH") });
        var merged = new List<BarDataProto>();
        await foreach (var bar in ChronologicalStreamMerger.MergeAsync(new IMarketDataFeed[] { feedA, feedB }))
            merged.Add(bar);
        Assert.Equal(3, merged.Count);
        Assert.Equal("BTC", merged[0].Instrument);
        Assert.Equal(1, merged[0].Timestamp.Minute);
        Assert.Equal("ETH", merged[1].Instrument);
        Assert.Equal(2, merged[1].Timestamp.Minute);
        Assert.Equal("BTC", merged[2].Instrument);
        Assert.Equal(3, merged[2].Timestamp.Minute);
    }

    // ======== Feed Tests ========

    [Fact]
    public async Task Backtest_JsonLinesFeed_RoundTrip()
    {
        var bars = new[]
        {
            new BarDataProto(Epoch, "15m", 100m, 101m, 99m, 100.5m, 1000m, 5m, 1000m, "BTC-USDT"),
            new BarDataProto(Epoch + TimeSpan.FromMinutes(15), "15m", 101m, 102m, 100m, 101.5m, 1000m, -3m, 1005m, "BTC-USDT"),
        };

        var path = Path.GetTempFileName();
        try
        {
            await File.WriteAllLinesAsync(path, bars.Select(b => JsonSerializer.Serialize(b)));

            var feed = new JsonLinesFeed(path);
            var read = new List<BarDataProto>();
            await foreach (var bar in feed.StreamBarsAsync())
                read.Add(bar);

            Assert.Equal(2, read.Count);
            Assert.Equal(bars[0], read[0]);
            Assert.Equal(bars[1], read[1]);
        }
        finally
        {
            if (File.Exists(path)) File.Delete(path);
        }
    }

    // ======== Pipeline Tests ========

    [Fact]
    public async Task Backtest_DeterministicReproducibility()
    {
        var bars = GenerateTestBars(200);

        var sink1 = new CollectingEventSink();
        var sink2 = new CollectingEventSink();
        var session1 = CreateSession(sink1);
        var session2 = CreateSession(sink2);

        foreach (var bar in bars)
        {
            await session1.ProcessBarAsync(bar);
            await session2.ProcessBarAsync(bar);
        }

        Assert.Equal(sink1.Events.Count, sink2.Events.Count);
        for (int i = 0; i < sink1.Events.Count; i++)
            Assert.Equal(sink1.Events[i], sink2.Events[i]);
    }

    [Fact]
    public async Task Backtest_RealServices_ProducesPoiAndTriggerEvents()
    {
        var poi = new PoiZoneService(null!, NullLogger<PoiZoneService>.Instance);
        poi.SetTestConfiguration(TestPoiConfig);

        var trigger = new TriggerService(null!, NullLogger<TriggerService>.Instance);
        trigger.SetTestConfiguration(TestTriggerConfig);

        var smt = new SmtDivergenceService(null!, new MockStructureService(), NullLogger<SmtDivergenceService>.Instance);
        smt.SetTestConfiguration(new List<CorrelationPairConfig>());

        var sink = new CollectingEventSink();
        var session = new BacktestSession(new MockStructureService(), poi, smt, trigger, sink);

        static BarDataProto Stable(int i) => new(Epoch + TimeSpan.FromMinutes(i), "15m",
            100.5m, 101m, 100m, 100.5m, 1000m, 0m, 1000m, "BTC-USDT");

        // Seed 14 ATR bars
        for (int i = 0; i < 14; i++)
            await session.ProcessBarAsync(Stable(i));

        // Bar 14: rising
        await session.ProcessBarAsync(MakeBar(14, 100.5m, 101.5m, 100m, 101m, "BTC-USDT"));
        // Bar 15: rising
        await session.ProcessBarAsync(MakeBar(15, 101m, 102m, 100.5m, 101.5m, "BTC-USDT"));
        // Bar 16: bullish gap → FVG detection [high[14]=101.5, low[16]=108]
        await session.ProcessBarAsync(MakeBar(16, 108m, 110m, 108m, 109m, "BTC-USDT", delta: 10m, oi: 1000m));

        // Bar 17: enters zone — low=107 < UpperBound=108 → trigger fires
        await session.ProcessBarAsync(MakeBar(17, 108m, 111m, 107m, 110m, "BTC-USDT", delta: -5m, oi: 1010m));

        var poiEvents = sink.Events.Where(e => e.StartsWith("POI|")).ToList();
        var triggerEvents = sink.Events.Where(e => e.StartsWith("TRIGGER|")).ToList();

        Assert.NotEmpty(poiEvents);
        Assert.Contains(poiEvents, e => e.Contains("FairValueGap|Bullish|Active"));
        Assert.NotEmpty(triggerEvents);
    }

    [Fact]
    public async Task Backtest_IntegratedMergeAndProcess()
    {
        var feedA = new InMemoryFeed(new[]
        {
            MakeBar(1, 100m, "BTC-USDT"),
            MakeBar(3, 102m, "BTC-USDT"),
            MakeBar(5, 104m, "BTC-USDT"),
        });
        var feedB = new InMemoryFeed(new[]
        {
            MakeBar(2, 200m, "ETH-USDT"),
            MakeBar(4, 202m, "ETH-USDT"),
            MakeBar(6, 204m, "ETH-USDT"),
        });

        var sink = new CollectingEventSink();
        var session = CreateSession(sink);

        var merged = ChronologicalStreamMerger.MergeAsync(new IMarketDataFeed[] { feedA, feedB });
        int processed = 0;
        await foreach (var bar in merged)
        {
            await session.ProcessBarAsync(bar);
            processed++;
        }

        // All 6 merged bars processed in chronological order without exception
        Assert.Equal(6, processed);
    }

    // ======== Stability Tests ========

    [Fact]
    public async Task Backtest_MemoryStability()
    {
        var bars = GenerateTestBars(10_000);

        var sink = new CollectingEventSink();
        var session = CreateSession(sink);

        foreach (var bar in bars)
            await session.ProcessBarAsync(bar);
    }

    [Fact(Skip = "Manual performance/allocation benchmark")]
    public async Task Backtest_MemoryBenchmark()
    {
        var bars = GenerateTestBars(100_000);

        // Force a full GC before measuring
        GC.Collect(2, GCCollectionMode.Forced, blocking: true);
        GC.WaitForPendingFinalizers();
        long beforeBytes = GC.GetTotalMemory(true);

        var sink = new CollectingEventSink();
        var session = CreateSession(sink);

        foreach (var bar in bars)
            await session.ProcessBarAsync(bar);

        long afterBytes = GC.GetTotalMemory(true);
        long deltaBytes = afterBytes - beforeBytes;

        Console.WriteLine($"[MEMORY] Processed 100,000 bars. Heap delta: {deltaBytes:N0} bytes ({deltaBytes / 100_000.0:N2} bytes/bar)");

        // After warmup the per-bar allocation should be small (buffer-fill only)
        Assert.True(deltaBytes / 100_000 < 500,
            $"Per-bar allocation ({deltaBytes / 100_000} bytes/bar) exceeds 500-byte threshold. " +
            "Investigate hot-path allocations (string interning, boxing, LINQ in loop).");
    }
}
