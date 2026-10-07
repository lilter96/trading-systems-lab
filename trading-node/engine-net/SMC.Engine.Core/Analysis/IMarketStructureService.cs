using SMC.Engine.Core.Models;

namespace SMC.Engine.Core.Analysis;

public interface IMarketStructureService
{
    Task<IReadOnlyList<StructureBreak>> FeedBarAsync(string instrument, BarDataProto bar);
    StructureStateSnapshot GetState(string instrument, string timeframe);
}

public readonly record struct BarDataProto(
    DateTimeOffset Timestamp,
    string Timeframe,
    decimal Open,
    decimal High,
    decimal Low,
    decimal Close,
    decimal Volume,
    decimal BuySellVolumeDelta = 0m,
    decimal OpenInterest = 0m,
    string Instrument = "");
