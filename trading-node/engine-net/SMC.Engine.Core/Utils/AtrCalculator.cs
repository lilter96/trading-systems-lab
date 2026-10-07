namespace SMC.Engine.Core.Utils;

public static class AtrCalculator
{
    public static decimal ComputeAtr(IReadOnlyList<decimal> trueRanges)
    {
        if (trueRanges.Count < 1) return 0;
        decimal sum = 0;
        for (int i = 0; i < trueRanges.Count; i++)
            sum += trueRanges[i];
        return sum / trueRanges.Count;
    }
}
