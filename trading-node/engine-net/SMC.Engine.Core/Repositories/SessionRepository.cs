using Microsoft.EntityFrameworkCore;
using SMC.Engine.Core.Data;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public class SessionRepository : ISessionRepository
{
    private readonly EngineDbContext _db;

    public SessionRepository(EngineDbContext db)
    {
        _db = db;
    }

    public Task<TradingSession?> GetByIdAsync(Guid id, CancellationToken ct = default)
        => _db.TradingSessions.FirstOrDefaultAsync(x => x.Id == id, ct);

    public Task<List<TradingSession>> GetActiveAsync(CancellationToken ct = default)
        => _db.TradingSessions.Where(x => x.Status == "active").ToListAsync(ct);

    public Task AddAsync(TradingSession session, CancellationToken ct = default)
    {
        _db.TradingSessions.Add(session);
        return Task.CompletedTask;
    }

    public Task UpdateAsync(TradingSession session, CancellationToken ct = default)
    {
        _db.TradingSessions.Update(session);
        return Task.CompletedTask;
    }
}
