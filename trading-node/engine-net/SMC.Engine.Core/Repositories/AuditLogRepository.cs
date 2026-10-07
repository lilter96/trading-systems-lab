using Microsoft.EntityFrameworkCore;
using SMC.Engine.Core.Data;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public class AuditLogRepository : IAuditLogRepository
{
    private readonly EngineDbContext _db;

    public AuditLogRepository(EngineDbContext db)
    {
        _db = db;
    }

    public Task<List<AuditLogEntry>> GetBySessionIdAsync(Guid sessionId, int limit = 100, CancellationToken ct = default)
        => _db.AuditLogEntries
            .Where(x => x.SessionId == sessionId)
            .OrderByDescending(x => x.Id)
            .Take(limit)
            .ToListAsync(ct);

    public async Task<long> InsertAsync(AuditLogEntry entry, CancellationToken ct = default)
    {
        _db.AuditLogEntries.Add(entry);
        await _db.SaveChangesAsync(ct);
        return entry.Id;
    }
}
