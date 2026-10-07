using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Models;

namespace SMC.Engine.Core.Backtesting;

public sealed class BacktestSession
{
    private readonly IMarketStructureService _structureService;
    private readonly IPoiZoneService _poiZoneService;
    private readonly ISmtDivergenceService _smtService;
    private readonly ITriggerService _triggerService;
    private readonly IEngineEventSink _eventSink;
    private readonly BacktestBroker? _broker;

    public BacktestSession(
        IMarketStructureService structureService,
        IPoiZoneService poiZoneService,
        ISmtDivergenceService smtService,
        ITriggerService triggerService,
        IEngineEventSink eventSink,
        BacktestBroker? broker = null)
    {
        _structureService = structureService;
        _poiZoneService = poiZoneService;
        _smtService = smtService;
        _triggerService = triggerService;
        _eventSink = eventSink;
        _broker = broker;
    }

    public async Task ProcessBarAsync(BarDataProto bar)
    {
        _broker?.ProcessBar(bar);

        var breaks = await _structureService.FeedBarAsync(bar.Instrument, bar);
        foreach (var b in breaks)
            await _eventSink.HandleStructureBreakAsync(bar.Instrument, bar.Timeframe, b);

        var divergences = _smtService.ProcessBar(bar.Instrument, bar, breaks);

        var zones = _poiZoneService.ProcessBar(bar.Instrument, bar, breaks);
        foreach (var z in zones)
            await _eventSink.HandlePoiLifecycleAsync(bar.Instrument, bar.Timeframe, z);

        var activePois = _poiZoneService.GetActivePOIs(bar.Instrument, bar.Timeframe);
        var triggers = _triggerService.Evaluate(bar.Instrument, bar, activePois, breaks, divergences);
        foreach (var t in triggers)
            await _eventSink.HandleTriggerAsync(bar.Instrument, bar.Timeframe, t);
    }
}
