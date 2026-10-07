using Microsoft.EntityFrameworkCore;
using SMC.Engine.Core.Data;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public class PositionRepository : IPositionRepository
{
    private readonly EngineDbContext _db;

    public PositionRepository(EngineDbContext db)
    {
        _db = db;
    }

    public Task<Position?> GetByIdAsync(Guid id, CancellationToken ct = default)
        => _db.Positions.FirstOrDefaultAsync(x => x.Id == id, ct);

    public Task<Position?> GetByAssetAsync(string asset, CancellationToken ct = default)
        => _db.Positions.FirstOrDefaultAsync(x => x.Asset == asset && x.Status == "open", ct);

    public Task<List<Position>> GetOpenBySessionIdAsync(Guid sessionId, CancellationToken ct = default)
        => _db.Positions.Where(x => x.SessionId == sessionId && x.Status == "open").ToListAsync(ct);

    public Task AddAsync(Position position, CancellationToken ct = default)
    {
        _db.Positions.Add(position);
        return Task.CompletedTask;
    }

    public Task UpdateAsync(Position position, CancellationToken ct = default)
    {
        _db.Positions.Update(position);
        return Task.CompletedTask;
    }
}
