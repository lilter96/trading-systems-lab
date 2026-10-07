using SMC.Engine.Core.Models;

namespace SMC.Engine.Core.Analysis;

public interface ITriggerService
{
    IReadOnlyList<TriggerEvaluation> Evaluate(
        string instrument,
        BarDataProto bar,
        IReadOnlyList<POI> activePois,
        IReadOnlyList<StructureBreak> breaks,
        IReadOnlyList<SmtDivergenceInfo> divergences);
}
