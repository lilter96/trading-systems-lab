using SMC.Engine.Core.Models;

namespace SMC.Engine.Core.Analysis;

public interface IPoiZoneService
{
    IReadOnlyList<POI> ProcessBar(string instrument, BarDataProto bar, IReadOnlyList<StructureBreak> breaks);
    IReadOnlyList<POI> GetActivePOIs(string instrument, string timeframe);
    IReadOnlyList<POI> GetAllActivePOIs(string instrument);
}
