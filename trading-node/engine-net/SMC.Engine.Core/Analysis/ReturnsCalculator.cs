namespace SMC.Engine.Core.Analysis;

public static class ReturnsCalculator
{
    public static IReadOnlyList<decimal> ComputeLogReturns(IReadOnlyList<decimal> closes)
    {
        if (closes.Count < 2) return Array.Empty<decimal>();

        var returns = new decimal[closes.Count - 1];
        for (int i = 0; i < returns.Length; i++)
        {
            decimal ratio = closes[i + 1] / closes[i];
            returns[i] = (decimal)Math.Log((double)ratio);
        }
        return returns;
    }
}
