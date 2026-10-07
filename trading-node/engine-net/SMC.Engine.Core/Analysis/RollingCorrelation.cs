namespace SMC.Engine.Core.Analysis;

public static class RollingCorrelation
{
    public static decimal? Compute(IReadOnlyList<decimal> returnsA, IReadOnlyList<decimal> returnsB)
    {
        if (returnsA.Count != returnsB.Count || returnsA.Count < 2)
            return null;

        int n = returnsA.Count;

        double sumX = 0, sumY = 0, sumXy = 0, sumX2 = 0, sumY2 = 0;
        for (int i = 0; i < n; i++)
        {
            double x = (double)returnsA[i];
            double y = (double)returnsB[i];
            sumX += x;
            sumY += y;
            sumXy += x * y;
            sumX2 += x * x;
            sumY2 += y * y;
        }

        double num = n * sumXy - sumX * sumY;
        double denomX = n * sumX2 - sumX * sumX;
        double denomY = n * sumY2 - sumY * sumY;

        if (denomX <= 1e-15 || denomY <= 1e-15)
            return null;

        double r = num / Math.Sqrt(denomX * denomY);
        r = Math.Clamp(r, -1.0, 1.0);

        return (decimal)r;
    }
}
