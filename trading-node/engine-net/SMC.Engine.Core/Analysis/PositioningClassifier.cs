using SMC.Engine.Core.Models;

namespace SMC.Engine.Core.Analysis;

public static class PositioningClassifier
{
    public static PositioningType Classify(decimal deltaPrice, decimal deltaOi, decimal noiseBand)
    {
        if (Math.Abs(deltaPrice) <= noiseBand)
            return PositioningType.Ambiguous;

        if (deltaOi == 0m)
            return PositioningType.Ambiguous;

        if (deltaOi > 0m)
            return deltaPrice > 0m ? PositioningType.FreshLongs : PositioningType.FreshShorts;

        return deltaPrice > 0m ? PositioningType.ShortCovering : PositioningType.LongCovering;
    }
}
