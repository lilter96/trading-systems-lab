using System.Runtime.CompilerServices;
using System.Text.Json;
using SMC.Engine.Core.Analysis;
using SMC.Engine.Core.Feeds;

namespace SMC.Engine.Core.Backtesting;

public sealed class JsonLinesFeed : IMarketDataFeed
{
    private readonly string _filePath;

    public JsonLinesFeed(string filePath)
    {
        _filePath = filePath;
    }

    public async IAsyncEnumerable<BarDataProto> StreamBarsAsync(
        [EnumeratorCancellation] CancellationToken ct = default)
    {
        using var reader = new StreamReader(_filePath);
        while (await reader.ReadLineAsync(ct) is { } line)
        {
            if (string.IsNullOrWhiteSpace(line))
                continue;

            var bar = JsonSerializer.Deserialize<BarDataProto?>(line);
            if (bar.HasValue)
                yield return bar.Value;
        }
    }
}
