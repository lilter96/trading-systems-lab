using System.Runtime.CompilerServices;
using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Feeds;

namespace SMC.Engine.Core.Backtesting;

public sealed class InMemoryFeed : IMarketDataFeed
{
    private readonly IReadOnlyList<BarDataProto> _bars;

    public InMemoryFeed(IReadOnlyList<BarDataProto> bars)
    {
        _bars = bars;
    }

    public async IAsyncEnumerable<BarDataProto> StreamBarsAsync(
        [EnumeratorCancellation] CancellationToken ct = default)
    {
        foreach (var bar in _bars)
        {
            if (ct.IsCancellationRequested)
                yield break;
            yield return bar;
        }
    }
}
