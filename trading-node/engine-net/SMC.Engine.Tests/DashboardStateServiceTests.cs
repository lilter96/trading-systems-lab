using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Models;
using SMC.Engine.Service.Services;

namespace SMC.Engine.Tests;

public sealed class DashboardStateServiceTests
{
    #region Event dispatch

    [Fact]
    public void OnStateChanged_FiresTelemetry_OnUpdateTelemetry()
    {
        var svc = Create();
        var kinds = CaptureKinds(svc);
        svc.UpdateTelemetry(100, 1000, 50000.0);
        Assert.Contains(StateChangeKind.Telemetry, kinds);
    }

    [Fact]
    public void OnStateChanged_FiresHeartbeat_OnRecordHeartbeat()
    {
        var svc = Create();
        var kinds = CaptureKinds(svc);
        svc.RecordHeartbeat("test");
        Assert.Contains(StateChangeKind.Heartbeat, kinds);
    }

    [Fact]
    public void OnStateChanged_FiresCandles_OnRecordBarReceived()
    {
        var svc = Create();
        var kinds = CaptureKinds(svc);
        svc.RecordBarReceived(MakeBar());
        Assert.Contains(StateChangeKind.Candles, kinds);
    }

    [Fact]
    public void OnStateChanged_FiresPois_OnSetActivePois()
    {
        var svc = Create();
        var kinds = CaptureKinds(svc);
        svc.SetActivePois(new[] { MakePoi() });
        Assert.Contains(StateChangeKind.Pois, kinds);
    }

    [Fact]
    public void OnStateChanged_FiresSelection_OnToggleSelectedPoi()
    {
        var svc = Create();
        var kinds = CaptureKinds(svc);
        svc.ToggleSelectedPoi(MakePoi());
        Assert.Contains(StateChangeKind.Selection, kinds);
    }

    [Fact]
    public void OnStateChanged_FiresAll_OnUpdateSystemStatus()
    {
        var svc = Create();
        var kinds = CaptureKinds(svc);
        svc.UpdateSystemStatus("ERROR");
        Assert.Contains(StateChangeKind.All, kinds);
    }

    [Fact]
    public void PoiSelectionChanged_FiresOnToggle()
    {
        var svc = Create();
        var fired = false;
        svc.PoiSelectionChanged += () => fired = true;
        svc.ToggleSelectedPoi(MakePoi());
        Assert.True(fired);
    }

    [Fact]
    public void PoiSelectionChanged_FiresOnClear()
    {
        var svc = Create();
        svc.ToggleSelectedPoi(MakePoi());
        var fired = false;
        svc.PoiSelectionChanged += () => fired = true;
        svc.ClearSelectedPois();
        Assert.True(fired);
    }

    [Fact]
    public void PoiSelectionChanged_FiresOnSelectAllFiltered()
    {
        var svc = Create();
        var poi = MakePoi();
        svc.SetActivePois(new[] { poi });
        var fired = false;
        svc.PoiSelectionChanged += () => fired = true;
        svc.SelectAllFiltered(new[] { poi });
        Assert.True(fired);
    }

    [Fact]
    public void PoiSelectionChanged_FiresOnInvertSelection()
    {
        var svc = Create();
        var poi = MakePoi();
        svc.SetActivePois(new[] { poi });
        var fired = false;
        svc.PoiSelectionChanged += () => fired = true;
        svc.InvertSelection(new[] { poi });
        Assert.True(fired);
    }

    [Fact]
    public void PoiSelectionChanged_NotFiredOnSetActivePois()
    {
        var svc = Create();
        svc.ToggleSelectedPoi(MakePoi());
        var fired = false;
        svc.PoiSelectionChanged += () => fired = true;
        svc.SetActivePois(new[] { MakePoi() });
        Assert.False(fired);
    }

    #endregion

    #region POI selection

    [Fact]
    public void ToggleSelectedPoi_Adds()
    {
        var svc = Create();
        var poi = MakePoi();
        svc.ToggleSelectedPoi(poi);
        Assert.Contains(poi, svc.GetSelectedPois());
    }

    [Fact]
    public void ToggleSelectedPoi_Removes()
    {
        var svc = Create();
        var poi = MakePoi();
        svc.ToggleSelectedPoi(poi);
        svc.ToggleSelectedPoi(poi);
        Assert.DoesNotContain(poi, svc.GetSelectedPois());
    }

    [Fact]
    public void ClearSelectedPois_RemovesAll()
    {
        var svc = Create();
        svc.ToggleSelectedPoi(MakePoi(upper: 50000m, lower: 49000m));
        svc.ToggleSelectedPoi(MakePoi(upper: 60000m, lower: 59000m));
        svc.ClearSelectedPois();
        Assert.Empty(svc.GetSelectedPois());
    }

    [Fact]
    public void ClearSelectedPois_NoOp_WhenEmpty()
    {
        var svc = Create();
        var kinds = new List<StateChangeKind>();
        svc.OnStateChanged += k => kinds.Add(k);
        svc.ClearSelectedPois();
        Assert.Empty(kinds);
    }

    [Fact]
    public void SelectAllFiltered_AddsAll()
    {
        var svc = Create();
        var poi1 = MakePoi(upper: 50000m, lower: 49000m);
        var poi2 = MakePoi(upper: 60000m, lower: 59000m);
        svc.SetActivePois(new[] { poi1, poi2 });
        svc.SelectAllFiltered(new[] { poi1, poi2 });
        Assert.Equal(2, svc.GetSelectedPois().Count);
    }

    [Fact]
    public void SelectAllFiltered_OnlyAddsFiltered()
    {
        var svc = Create();
        var poi1 = MakePoi(upper: 50000m, lower: 49000m);
        var poi2 = MakePoi(upper: 60000m, lower: 59000m);
        svc.SetActivePois(new[] { poi1, poi2 });
        svc.SelectAllFiltered(new[] { poi1 });
        Assert.Contains(poi1, svc.GetSelectedPois());
        Assert.DoesNotContain(poi2, svc.GetSelectedPois());
    }

    [Fact]
    public void InvertSelection_TogglesFiltered()
    {
        var svc = Create();
        var poi1 = MakePoi(upper: 50000m, lower: 49000m);
        var poi2 = MakePoi(upper: 60000m, lower: 59000m);
        svc.ToggleSelectedPoi(poi1);
        svc.InvertSelection(new[] { poi1, poi2 });
        Assert.DoesNotContain(poi1, svc.GetSelectedPois());
        Assert.Contains(poi2, svc.GetSelectedPois());
    }

    [Fact]
    public void InvertSelection_UnaffectedNotTouched()
    {
        var svc = Create();
        var poi1 = MakePoi(upper: 50000m, lower: 49000m);
        var poi2 = MakePoi(upper: 60000m, lower: 59000m);
        var poi3 = MakePoi(upper: 70000m, lower: 69000m);
        svc.SetActivePois(new[] { poi1, poi2, poi3 });
        svc.ToggleSelectedPoi(poi1);
        svc.InvertSelection(new[] { poi1, poi2 });
        Assert.DoesNotContain(poi1, svc.GetSelectedPois());
        Assert.Contains(poi2, svc.GetSelectedPois());
        Assert.DoesNotContain(poi3, svc.GetSelectedPois());
    }

    [Fact]
    public void GetSelectedPois_ReturnsCopy()
    {
        var svc = Create();
        var poi = MakePoi();
        svc.ToggleSelectedPoi(poi);
        var copy = svc.GetSelectedPois();
        svc.ClearSelectedPois();
        Assert.Contains(poi, copy);
        Assert.Empty(svc.GetSelectedPois());
    }

    #endregion

    #region POI lifecycle

    [Fact]
    public void SetActivePois_PreservesSelected()
    {
        var svc = Create();
        var poi = MakePoi(upper: 50000m, lower: 49000m);
        svc.ToggleSelectedPoi(poi);
        svc.SetActivePois(new[] { poi });
        Assert.Contains(poi, svc.GetActivePois());
        Assert.Contains(poi, svc.GetSelectedPois());
    }

    [Fact]
    public void SetActivePois_RemovesStaleSelected()
    {
        var svc = Create();
        var poi1 = MakePoi(upper: 50000m, lower: 49000m);
        var poi2 = MakePoi(upper: 60000m, lower: 59000m);
        svc.ToggleSelectedPoi(poi1);
        svc.ToggleSelectedPoi(poi2);
        svc.SetActivePois(new[] { poi1 });
        Assert.Contains(poi1, svc.GetSelectedPois());
        Assert.DoesNotContain(poi2, svc.GetSelectedPois());
    }

    [Fact]
    public void SetActivePois_Empty_RemovesAll()
    {
        var svc = Create();
        svc.SetActivePois(new[] { MakePoi() });
        Assert.NotEmpty(svc.GetActivePois());
        svc.SetActivePois(Array.Empty<POI>());
        Assert.Empty(svc.GetActivePois());
    }

    [Fact]
    public void SetActivePois_StableKey_NoDuplicates()
    {
        var svc = Create();
        var formedAt = new DateTimeOffset(2026, 7, 19, 12, 0, 0, TimeSpan.Zero);
        var poi = MakePoi(upper: 50000m, lower: 49000m, formedAt: formedAt);
        var poiDup = MakePoi(upper: 50000m, lower: 49000m, formedAt: formedAt);
        svc.SetActivePois(new[] { poi, poiDup });
        Assert.Single(svc.GetActivePois());
    }

    [Fact]
    public void GetActivePois_ReturnsCopy()
    {
        var svc = Create();
        var poi = MakePoi();
        svc.SetActivePois(new[] { poi });
        var copy = svc.GetActivePois();
        svc.SetActivePois(Array.Empty<POI>());
        Assert.NotEmpty(copy);
        Assert.Empty(svc.GetActivePois());
    }

    #endregion

    #region Candle caching

    [Fact]
    public void GetCandles_Empty_ReturnsEmpty()
    {
        var svc = Create();
        Assert.Empty(svc.GetCandles("5m"));
    }

    [Fact]
    public void GetCandles_AggregatesCorrectly()
    {
        var svc = Create();
        svc.LoadHistoricalBars(MakeBars(5));
        var candles = svc.GetCandles("5m");
        Assert.Single(candles);
        Assert.Equal(100, candles[0].Open);
        Assert.Equal(109, candles[0].High);
        Assert.Equal(95, candles[0].Low);
        Assert.Equal(106, candles[0].Close);
    }

    [Fact]
    public void GetCandles_CacheHit_ReturnsSameInstance()
    {
        var svc = Create();
        svc.LoadHistoricalBars(MakeBars(5));
        Assert.Same(svc.GetCandles("5m"), svc.GetCandles("5m"));
    }

    [Fact]
    public void GetCandles_CacheInvalidated_OnRecordBarReceived()
    {
        var svc = Create();
        svc.LoadHistoricalBars(MakeBars(5));
        var first = svc.GetCandles("5m");
        svc.RecordBarReceived(MakeBar(addMinutes: 5));
        Assert.NotSame(first, svc.GetCandles("5m"));
    }

    [Fact]
    public void GetCandles_CacheInvalidated_OnLoadHistoricalBars()
    {
        var svc = Create();
        svc.LoadHistoricalBars(MakeBars(5));
        var first = svc.GetCandles("5m");
        svc.LoadHistoricalBars(new List<BarRecord>
        {
            new(DateTimeOffset.UtcNow.AddMinutes(5), 110, 115, 105, 112, 1000)
        });
        Assert.NotSame(first, svc.GetCandles("5m"));
    }

    [Fact]
    public void GetCandles_MultipleTimeframes_Independent()
    {
        var svc = Create();
        svc.LoadHistoricalBars(MakeBars(10));
        Assert.Equal(2, svc.GetCandles("5m").Count);
    }

    [Fact]
    public void GetCandles_UnknownTimeframe_DefaultsTo60s()
    {
        var svc = Create();
        svc.LoadHistoricalBars(MakeBars(5));
        Assert.Equal(5, svc.GetCandles("unknown").Count);
    }

    [Fact]
    public void LoadHistoricalBars_Empty_DoesNotThrow()
    {
        var svc = Create();
        svc.LoadHistoricalBars(Array.Empty<BarRecord>());
        Assert.Empty(svc.GetCandles("1m"));
    }

    [Fact]
    public void RecordBarReceived_ReplacesDuplicate()
    {
        var svc = Create();
        var ts = DateTimeOffset.UtcNow;
        svc.RecordBarReceived(new BarDataProto(ts, "1m", 100m, 110m, 90m, 105m, 1000m));
        svc.RecordBarReceived(new BarDataProto(ts, "1m", 101m, 111m, 91m, 106m, 1001m));
        var candles = svc.GetCandles("1m");
        Assert.Single(candles);
        Assert.Equal(101, candles[0].Open);
    }

    [Fact]
    public void MaxBarsRespected_RecordBarReceived()
    {
        var svc = Create();
        var baseTime = DateTimeOffset.UtcNow.AddDays(-1);
        for (int i = 0; i < 15000; i++)
            svc.RecordBarReceived(new BarDataProto(baseTime.AddMinutes(i), "1m", 100m, 110m, 90m, 105m, 1000m));
        Assert.True(svc.GetCandles("1m").Count <= 10000);
    }

    [Fact]
    public void MaxBarsRespected_LoadHistoricalBars()
    {
        var svc = Create();
        var baseTime = DateTimeOffset.UtcNow.AddDays(-1);
        var bars = new List<BarRecord>();
        for (int i = 0; i < 15000; i++)
            bars.Add(new BarRecord(baseTime.AddMinutes(i), 100, 110, 90, 105, 1000));
        svc.LoadHistoricalBars(bars);
        Assert.True(svc.GetCandles("1m").Count <= 10000);
    }

    #endregion

    #region Telemetry and status

    [Fact]
    public void UpdateTelemetry_SetsProperties()
    {
        var svc = Create();
        svc.UpdateTelemetry(100, 5000, 52000.0);
        Assert.Equal(100, svc.TicksPerSecond);
        Assert.Equal(5000, svc.TotalTicks);
        Assert.Equal(52000.0, svc.LastPrice);
        Assert.True(svc.HasTelemetry);
    }

    [Fact]
    public void UpdateTelemetry_ActivatesSystem()
    {
        var svc = Create();
        Assert.Equal("INITIALIZING", svc.SystemStatus);
        svc.UpdateTelemetry(100, 1000, 50000.0);
        Assert.Equal("ACTIVE", svc.SystemStatus);
    }

    [Fact]
    public void RecordBarReceived_UpdatesTimestampsAndActivates()
    {
        var svc = Create();
        Assert.Null(svc.FirstBarTime);
        Assert.Equal("INITIALIZING", svc.SystemStatus);
        svc.RecordBarReceived(MakeBar());
        Assert.NotNull(svc.FirstBarTime);
        Assert.NotNull(svc.LastBarTime);
        Assert.Equal("ACTIVE", svc.SystemStatus);
    }

    [Fact]
    public void UpdateIngestionRate_SetsRate()
    {
        var svc = Create();
        svc.UpdateIngestionRate(42.5);
        Assert.Equal(42.5, svc.IngestionRate);
    }

    [Fact]
    public void RecordHeartbeat_Stores()
    {
        var svc = Create();
        svc.RecordHeartbeat("gateway-go");
        Assert.Contains("gateway-go", svc.GetHeartbeats().Keys);
    }

    [Fact]
    public void UpdateSystemStatus_SetsStatus()
    {
        var svc = Create();
        svc.UpdateSystemStatus("ERROR");
        Assert.Equal("ERROR", svc.SystemStatus);
    }

    [Fact]
    public void SetOperatorContext_SetsProperties()
    {
        var svc = Create();
        svc.SetOperatorContext("op1", "session-1");
        Assert.Equal("op1", svc.CurrentOperator);
        Assert.Equal("session-1", svc.CurrentSessionId);
    }

    #endregion

    #region Tick accumulation

    [Fact]
    public void RecordTick_AccumulatesIntraBar()
    {
        var svc = Create();
        svc.RecordTick(new TickPayload { price = 50000.0, exchange_time = 1700000000 });
        svc.RecordTick(new TickPayload { price = 50100.0, exchange_time = 1700000030 });
        var candles = svc.GetCandles("1m");
        Assert.NotEmpty(candles);
        Assert.Equal(50000, candles[0].Open);
        Assert.Equal(50100, candles[0].Close);
    }

    #endregion

    #region Thread safety

    [Fact]
    public void ToggleSelectedPoi_ThreadSafe()
    {
        var svc = Create();
        var poi = MakePoi();
        Parallel.For(0, 99, _ => svc.ToggleSelectedPoi(poi));
        Assert.Single(svc.GetSelectedPois());
    }

    #endregion

    #region Helpers

    private static DashboardStateService Create() => new();

    private static List<StateChangeKind> CaptureKinds(DashboardStateService svc)
    {
        var kinds = new List<StateChangeKind>();
        svc.OnStateChanged += k => kinds.Add(k);
        return kinds;
    }

    private static POI MakePoi(
        POIType type = POIType.FairValueGap,
        MarketBias bias = MarketBias.Bullish,
        string instrument = "BTC-USDT",
        string timeframe = "5m",
        decimal upper = 50000m,
        decimal lower = 49000m,
        POIStatus status = POIStatus.Active,
        DateTimeOffset? formedAt = null)
    {
        return new POI
        {
            Type = type,
            Bias = bias,
            Instrument = instrument,
            Timeframe = timeframe,
            UpperBound = upper,
            LowerBound = lower,
            Status = status,
            FormedAt = formedAt ?? DateTimeOffset.UtcNow,
        };
    }

    private static BarDataProto MakeBar(DateTimeOffset? ts = null, int addMinutes = 0)
    {
        var t = (ts ?? new DateTimeOffset(2026, 7, 19, 12, 0, 0, TimeSpan.Zero)).AddMinutes(addMinutes);
        return new BarDataProto(t, "1m", 100m, 110m, 90m, 105m, 1000m);
    }

    private static List<BarRecord> MakeBars(int count)
    {
        var bars = new List<BarRecord>(count);
        var baseTime = new DateTimeOffset(2026, 7, 19, 12, 0, 0, TimeSpan.Zero);
        for (int i = 0; i < count; i++)
            bars.Add(new BarRecord(baseTime.AddMinutes(i), 100 + i, 105 + i, 95 + i, 102 + i, 1000));
        return bars;
    }

    #endregion
}
