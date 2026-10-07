using System.Text.Json;
using SMC.Engine.Core.Entities;
using SMC.Engine.Core.Repositories;

namespace SMC.Engine.Core.Services;

public class AuditService : IAuditService
{
    private readonly IAuditLogRepository _repo;

    public AuditService(IAuditLogRepository repo)
    {
        _repo = repo;
    }

    public async Task<long> RecordAsync(Guid sessionId, string eventType, Guid? correlationId, object payload, CancellationToken ct = default)
    {
        var entry = new AuditLogEntry
        {
            SessionId = sessionId,
            EventType = eventType,
            CorrelationId = correlationId,
            Payload = JsonSerializer.Serialize(payload),
            CreatedAt = DateTimeOffset.UtcNow,
        };
        return await _repo.InsertAsync(entry, ct);
    }
}
