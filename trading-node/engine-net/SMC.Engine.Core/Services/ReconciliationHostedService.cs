using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

namespace SMC.Engine.Core.Services;

public class ReconciliationHostedService : IHostedService
{
    private readonly IServiceProvider _serviceProvider;
    private readonly ILogger<ReconciliationHostedService> _logger;

    public ReconciliationHostedService(IServiceProvider serviceProvider, ILogger<ReconciliationHostedService> logger)
    {
        _serviceProvider = serviceProvider;
        _logger = logger;
    }

    public async Task StartAsync(CancellationToken cancellationToken)
    {
        _logger.LogInformation("Running reconciliation before service start...");

        using var scope = _serviceProvider.CreateScope();
        var reconciliationService = scope.ServiceProvider.GetRequiredService<IReconciliationService>();

        const int maxRetries = 5;
        for (int i = 1; i <= maxRetries; i++)
        {
            try
            {
                await reconciliationService.ReconcileAsync(cancellationToken);
                _logger.LogInformation("Reconciliation complete — service starting");
                return;
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "Reconciliation attempt {Attempt}/{MaxRetries} failed, retrying in 5s...", i, maxRetries);
                if (i == maxRetries)
                {
                    _logger.LogError(ex, "Reconciliation failed after {MaxRetries} attempts, continuing without reconciliation", maxRetries);
                    return;
                }
                await Task.Delay(TimeSpan.FromSeconds(5), cancellationToken);
            }
        }
    }

    public Task StopAsync(CancellationToken cancellationToken)
    {
        return Task.CompletedTask;
    }
}
