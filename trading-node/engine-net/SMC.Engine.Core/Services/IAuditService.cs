using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Services;

public interface IAuditService
{
    Task<long> RecordAsync(Guid sessionId, string eventType, Guid? correlationId, object payload, CancellationToken ct = default);
}
