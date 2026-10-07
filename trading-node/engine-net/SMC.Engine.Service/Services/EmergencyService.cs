using Common;
using Grpc.Core;
using SMC.Engine.Core.Repositories;
using SMC.Engine.Core.Services;

namespace SMC.Engine.Service.Services;

public class EmergencyServiceImpl : EmergencyService.EmergencyServiceBase
{
    private readonly ISystemStateRepository _repo;
    private readonly IAuditService _auditService;
    private readonly ISessionRepository _sessionRepo;
    private readonly ILogger<EmergencyServiceImpl> _logger;

    public EmergencyServiceImpl(
        ISystemStateRepository repo,
        IAuditService auditService,
        ISessionRepository sessionRepo,
        ILogger<EmergencyServiceImpl> logger)
    {
        _repo = repo;
        _auditService = auditService;
        _sessionRepo = sessionRepo;
        _logger = logger;
    }

    public override async Task<HaltResponse> HaltSystem(HaltRequest request, ServerCallContext context)
    {
        _logger.LogCritical(
            "EMERGENCY HALT received: reason={Reason} triggered_by={TriggeredBy} timestamp={Timestamp}",
            request.Reason, request.TriggeredBy, request.Timestamp);

        try
        {
            await _repo.SetHaltedAsync(context.CancellationToken);
            _logger.LogInformation("System state updated to HALTED in database.");

            await WriteHaltAuditLogAsync(request, context.CancellationToken);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Failed to update system state to HALTED.");
            return new HaltResponse
            {
                Success = false,
                ErrorMessage = ex.Message,
            };
        }

        return new HaltResponse { Success = true };
    }

    private async Task WriteHaltAuditLogAsync(HaltRequest request, CancellationToken ct)
    {
        try
        {
            var sessions = await _sessionRepo.GetActiveAsync(ct);
            var sessionId = sessions.FirstOrDefault()?.Id ?? Guid.Empty;

            var payload = new
            {
                reason = request.Reason,
                triggered_by = request.TriggeredBy,
                issued_by = request.TriggeredBy,
                timestamp = request.Timestamp,
                source = "EmergencyService"
            };

            await _auditService.RecordAsync(
                sessionId,
                "SYSTEM_HALT",
                correlationId: null,
                payload,
                ct);

            _logger.LogInformation(
                "Audit log entry recorded: event=SYSTEM_HALT issued_by={TriggeredBy} session={SessionId}",
                request.TriggeredBy, sessionId);
        }
        catch (Exception ex)
        {
            _logger.LogWarning(ex, "Failed to write halt audit log (halt already executed).");
        }
    }
}
