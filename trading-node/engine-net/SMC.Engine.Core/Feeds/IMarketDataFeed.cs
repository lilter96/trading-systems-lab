using SMC.Engine.Core.Analysis;

namespace SMC.Engine.Core.Feeds;

public interface IMarketDataFeed
{
    IAsyncEnumerable<BarDataProto> StreamBarsAsync(CancellationToken cancellationToken = default);
}
