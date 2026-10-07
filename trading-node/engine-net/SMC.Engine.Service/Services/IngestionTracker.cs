namespace SMC.Engine.Service.Services;

public sealed class IngestionTracker : IDisposable
{
    private readonly DashboardStateService _stateService;
    private long _processedCount;
    private readonly Timer _timer;

    public IngestionTracker(DashboardStateService stateService)
    {
        _stateService = stateService;
        _timer = new Timer(CalculateRate, null, TimeSpan.Zero, TimeSpan.FromSeconds(1));
    }

    public void IncrementProcessed() => Interlocked.Increment(ref _processedCount);

    private void CalculateRate(object? state)
    {
        long currentCount = Interlocked.Exchange(ref _processedCount, 0);
        _stateService.UpdateIngestionRate(currentCount);
    }

    public void Dispose()
    {
        _timer.Dispose();
    }
}
