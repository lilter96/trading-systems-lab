using SMC.Engine.Core.Models;

namespace SMC.Engine.Core.Backtesting;

public interface IEngineEventSink
{
    Task HandleStructureBreakAsync(string instrument, string timeframe, StructureBreak @break);
    Task HandlePoiLifecycleAsync(string instrument, string timeframe, POI poi);
    Task HandleTriggerAsync(string instrument, string timeframe, TriggerEvaluation evaluation);
}
