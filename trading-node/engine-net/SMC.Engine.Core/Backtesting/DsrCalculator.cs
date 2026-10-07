namespace SMC.Engine.Core.Backtesting;

public static class DsrCalculator
{
    public static double NormalCdf(double x)
    {
        if (x < -38.0) return 0.0;
        if (x > 38.0) return 1.0;

        double absX = Math.Abs(x);
        double t = 1.0 / (1.0 + 0.2316419 * absX);

        double[] b = [0.319381530, -0.356563782, 1.781477937, -1.821255978, 1.330274429];

        double poly = 0.0;
        double term = t;
        for (int i = 0; i < 5; i++)
        {
            poly += b[i] * term;
            term *= t;
        }

        double pdf = Math.Exp(-absX * absX / 2.0) / Math.Sqrt(2.0 * Math.PI);
        double cdf = 1.0 - pdf * poly;

        return x >= 0.0 ? cdf : 1.0 - cdf;
    }

    public static double NormalInverseCdf(double p)
    {
        if (p <= 0.0) return double.NegativeInfinity;
        if (p >= 1.0) return double.PositiveInfinity;

        double[] a =
        [
            -3.969683028665376e+01, 2.209460984245205e+02,
            -2.759285104469687e+02, 1.383577518672690e+02,
            -3.066479806614716e+01, 2.506628277459239e+00
        ];

        double[] b =
        [
            -5.447609879822406e+01, 1.615858368580409e+02,
            -1.556989798598866e+02, 6.680131188771972e+01,
            -1.328068155288572e+01
        ];

        double[] c =
        [
            -7.784894002430293e-03, -3.223964580411365e-01,
            -2.400758277161838e+00, -2.549732539343734e+00,
            4.374664141464968e+00, 2.938163982698783e+00
        ];

        double[] d =
        [
            7.784695709041462e-03, 3.224671290700398e-01,
            2.445134137142996e+00, 3.754408661907416e+00
        ];

        double x, r;

        if (p < 0.02425)
        {
            double q = Math.Sqrt(-2.0 * Math.Log(p));
            x = (((((c[0] * q + c[1]) * q + c[2]) * q + c[3]) * q + c[4]) * q + c[5]) /
                ((((d[0] * q + d[1]) * q + d[2]) * q + d[3]) * q + 1.0);
        }
        else if (p > 1.0 - 0.02425)
        {
            double q = Math.Sqrt(-2.0 * Math.Log(1.0 - p));
            x = -(((((c[0] * q + c[1]) * q + c[2]) * q + c[3]) * q + c[4]) * q + c[5]) /
                ((((d[0] * q + d[1]) * q + d[2]) * q + d[3]) * q + 1.0);
        }
        else
        {
            double q = p - 0.5;
            r = q * q;
            x = (((((a[0] * r + a[1]) * r + a[2]) * r + a[3]) * r + a[4]) * r + a[5]) * q /
                (((((b[0] * r + b[1]) * r + b[2]) * r + b[3]) * r + b[4]) * r + 1.0);
        }

        return x;
    }

    private static double NormalPdf(double x)
    {
        return Math.Exp(-x * x / 2.0) / Math.Sqrt(2.0 * Math.PI);
    }

    public static double ComputeDeflatedSharpeRatio(
        double dailySharpeRatio,
        int numObservations,
        int numTrials)
    {
        if (numObservations <= 1 || numTrials <= 0)
            return 0.0;

        double T = numObservations;
        double M = numTrials;

        double z = dailySharpeRatio * Math.Sqrt(T);
        double logPhi = Math.Log(NormalCdf(z));
        double dsr = Math.Exp(M * logPhi);

        if (double.IsNaN(dsr) || double.IsInfinity(dsr))
            return 0.0;

        return Math.Clamp(dsr, 0.0, 1.0);
    }

    public static double ComputeDeflatedSharpeRatioAdvanced(
        double dailySharpeRatio,
        int numObservations,
        int numTrials,
        double skewness = 0.0,
        double excessKurtosis = 0.0)
    {
        if (numObservations <= 1 || numTrials <= 0)
            return 0.0;

        double adjustedSR = dailySharpeRatio;

        if (Math.Abs(skewness) > 1e-12 || Math.Abs(excessKurtosis) > 1e-12)
        {
            double zStar = NormalInverseCdf(1.0 - 1.0 / numTrials);
            double cf = 1.0
                + skewness * zStar / 6.0
                + excessKurtosis * zStar * zStar / 24.0
                - skewness * skewness * zStar * zStar * zStar / 36.0;
            adjustedSR *= cf;
        }

        return ComputeDeflatedSharpeRatio(adjustedSR, numObservations, numTrials);
    }
}
