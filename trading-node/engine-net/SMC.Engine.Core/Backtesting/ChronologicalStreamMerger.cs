using System.Runtime.CompilerServices;
using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Feeds;

namespace SMC.Engine.Core.Backtesting;

public static class ChronologicalStreamMerger
{
    public static async IAsyncEnumerable<BarDataProto> MergeAsync(
        IReadOnlyList<IMarketDataFeed> feeds,
        [EnumeratorCancellation] CancellationToken ct = default)
    {
        var enumerators = new List<IAsyncEnumerator<BarDataProto>>();

        try
        {
            foreach (var feed in feeds)
            {
                var enumerator = feed.StreamBarsAsync(ct).GetAsyncEnumerator(ct);
                enumerators.Add(enumerator);
            }

            var queue = new PriorityQueue<(BarDataProto Bar, IAsyncEnumerator<BarDataProto> Enumerator), (long Ticks, string Instrument)>();

            foreach (var enumerator in enumerators)
            {
                if (await enumerator.MoveNextAsync())
                {
                    var bar = enumerator.Current;
                    queue.Enqueue((bar, enumerator), (bar.Timestamp.Ticks, bar.Instrument));
                }
            }

            while (queue.Count > 0)
            {
                var (currentBar, enumerator) = queue.Dequeue();
                yield return currentBar;

                if (await enumerator.MoveNextAsync())
                {
                    var nextBar = enumerator.Current;
                    queue.Enqueue((nextBar, enumerator), (nextBar.Timestamp.Ticks, nextBar.Instrument));
                }
            }
        }
        finally
        {
            foreach (var enumerator in enumerators)
            {
                await enumerator.DisposeAsync();
            }
        }
    }
}
