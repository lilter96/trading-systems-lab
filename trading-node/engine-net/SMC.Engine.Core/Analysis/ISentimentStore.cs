namespace SMC.Engine.Core.Analysis;

public interface ISentimentStore
{
    decimal? GetSentimentScore(string instrument);
}
