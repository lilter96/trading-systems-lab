using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public interface IAuditLogRepository
{
    Task<List<AuditLogEntry>> GetBySessionIdAsync(Guid sessionId, int limit = 100, CancellationToken ct = default);
    Task<long> InsertAsync(AuditLogEntry entry, CancellationToken ct = default);
}
