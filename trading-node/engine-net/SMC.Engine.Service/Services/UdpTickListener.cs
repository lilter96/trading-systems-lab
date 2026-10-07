using System.Net.Sockets;
using System.Text;
using System.Text.Json;

namespace SMC.Engine.Service.Services;

public sealed class UdpTickListener : BackgroundService
{
    private readonly DashboardStateService _stateService;
    private readonly ILogger<UdpTickListener> _logger;

    public UdpTickListener(DashboardStateService stateService, ILogger<UdpTickListener> logger)
    {
        _stateService = stateService;
        _logger = logger;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        using var udp = new UdpClient(5556);
        _logger.LogInformation("UDP tick listener started on port 5556");

        while (!stoppingToken.IsCancellationRequested)
        {
            try
            {
                var result = await udp.ReceiveAsync(stoppingToken);
                var json = Encoding.UTF8.GetString(result.Buffer);
                var tick = JsonSerializer.Deserialize<TickPayload>(json);
                if (tick is not null)
                {
                    _stateService.RecordTick(tick);
                }
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch (Exception ex)
            {
                _logger.LogWarning(ex, "UDP tick parse error");
            }
        }
    }
}

public record TickPayload
{
    public double price { get; init; }
    public double quantity { get; init; }
    public string side { get; init; } = "";
    public long exchange_time { get; init; }
    public long receipt_time { get; init; }
    public string instrument { get; init; } = "";
}
