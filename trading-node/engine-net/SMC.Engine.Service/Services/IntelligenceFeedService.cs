using System.Collections.Concurrent;
using System.Globalization;
using Google.Protobuf.WellKnownTypes;
using Grpc.Core;
using Intelligence;
using SMC.Engine.Core.Analysis;

namespace SMC.Engine.Service.Services;

public sealed class IntelligenceFeedService : IntelligenceFeed.IntelligenceFeedBase
{
    private readonly ILogger<IntelligenceFeedService> _logger;
    private readonly IOsintScoreStore _scoreStore;

    public IntelligenceFeedService(
        ILogger<IntelligenceFeedService> logger,
        IOsintScoreStore scoreStore)
    {
        _logger = logger;
        _scoreStore = scoreStore;
    }

    public override async Task<Empty> StreamOsintScores(
        IAsyncStreamReader<OsintScore> requestStream,
        ServerCallContext context)
    {
        _logger.LogInformation("IntelligenceFeed client connected");

        await foreach (var score in requestStream.ReadAllAsync(context.CancellationToken))
        {
            _scoreStore.Update(score.Instrument, score);
            _logger.LogDebug(
                "OsintScore received: instrument={Instrument} macro={Macro} sentiment={Sentiment} volume={Volume} regime={Regime}",
                score.Instrument,
                score.MacroComponent,
                score.SentimentComponent,
                score.SentimentVolume,
                score.VolatilityRegime);
        }

        _logger.LogInformation("IntelligenceFeed client disconnected");
        return new Empty();
    }
}

public interface IOsintScoreStore
{
    void Update(string instrument, OsintScore score);
    OsintScore? Get(string instrument);
}

public sealed class InMemorySentimentStore : IOsintScoreStore, ISentimentStore
{
    private readonly ConcurrentDictionary<string, OsintScore> _scores = new(StringComparer.Ordinal);

    public void Update(string instrument, OsintScore score)
    {
        _scores[instrument] = score;
    }

    public OsintScore? Get(string instrument)
    {
        return _scores.TryGetValue(instrument, out var score) ? score : null;
    }

    decimal? ISentimentStore.GetSentimentScore(string instrument)
    {
        if (_scores.TryGetValue(instrument, out var score)
            && decimal.TryParse(score.SentimentComponent, NumberStyles.Any, CultureInfo.InvariantCulture, out var parsed))
        {
            return parsed;
        }
        return null;
    }
}
