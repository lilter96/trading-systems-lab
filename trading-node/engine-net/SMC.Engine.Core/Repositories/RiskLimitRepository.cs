using Microsoft.EntityFrameworkCore;
using SMC.Engine.Core.Data;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public class RiskLimitRepository : IRiskLimitRepository
{
    private readonly EngineDbContext _db;

    public RiskLimitRepository(EngineDbContext db)
    {
        _db = db;
    }

    public Task<RiskLimit?> GetBySessionIdAsync(Guid? sessionId, CancellationToken ct = default)
        => _db.RiskLimits.FirstOrDefaultAsync(x => x.SessionId == sessionId, ct);

    public Task AddOrUpdateAsync(RiskLimit limit, CancellationToken ct = default)
    {
        var existing = _db.RiskLimits.Local.FirstOrDefault(x => x.Id == limit.Id)
                      ?? _db.RiskLimits.Find(limit.Id);
        if (existing is not null)
        {
            _db.Entry(existing).CurrentValues.SetValues(limit);
        }
        else
        {
            _db.RiskLimits.Add(limit);
        }
        return Task.CompletedTask;
    }
}
