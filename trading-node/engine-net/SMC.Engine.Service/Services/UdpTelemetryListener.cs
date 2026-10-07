using System.Net.Sockets;
using System.Text;
using System.Text.Json;

namespace SMC.Engine.Service.Services;

public sealed class UdpTelemetryListener : BackgroundService
{
    private readonly DashboardStateService _stateService;
    private readonly ILogger<UdpTelemetryListener> _logger;

    public UdpTelemetryListener(DashboardStateService stateService, ILogger<UdpTelemetryListener> logger)
    {
        _stateService = stateService;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        using var udp = new UdpClient(5555);
        _logger.LogInformation("UDP telemetry listener started on port 5555");

        while (!stoppingToken.IsCancellationRequested)
        {
            try
            {
                var result = await udp.ReceiveAsync(stoppingToken);
                var json = Encoding.UTF8.GetString(result.Buffer);
                var telemetry = JsonSerializer.Deserialize<TelemetryPayload>(json);
                if (telemetry is not null)
                {
                    _stateService.RecordHeartbeat(telemetry.sender);
                    _stateService.UpdateTelemetry(
                        telemetry.ticks_per_second,
                        telemetry.total_ticks,
                        telemetry.last_price);
                }
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "UDP telemetry parse error");
            }
        }
    }
}

public record TelemetryPayload
{
    public string sender { get; init; } = "";
    public long timestamp { get; init; }
    public string status { get; init; } = "";
    public long ticks_per_second { get; init; }
    public long total_ticks { get; init; }
    public double last_price { get; init; }
}
