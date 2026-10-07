using SMC.Engine.Core.Models;

namespace SMC.Engine.Core.Analysis;

public interface ISmtDivergenceService
{
    IReadOnlyList<SmtDivergenceInfo> ProcessBar(
        string instrument, BarDataProto bar, IReadOnlyList<StructureBreak> breaks);
}
