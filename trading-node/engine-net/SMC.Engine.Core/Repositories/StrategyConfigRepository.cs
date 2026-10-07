using Microsoft.EntityFrameworkCore;
using SMC.Engine.Core.Data;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public class StrategyConfigRepository : IStrategyConfigRepository
{
    private readonly EngineDbContext _db;

    public StrategyConfigRepository(EngineDbContext db)
    {
        _db = db;
    }

    public Task<List<StrategyConfig>> GetAllAsync(CancellationToken ct = default)
        => _db.StrategyConfigs.ToListAsync(ct);

    public Task<StrategyConfig?> GetByNameAsync(string name, CancellationToken ct = default)
        => _db.StrategyConfigs.FirstOrDefaultAsync(x => x.Name == name, ct);

    public Task AddOrUpdateAsync(StrategyConfig config, CancellationToken ct = default)
    {
        var existing = _db.StrategyConfigs.Local.FirstOrDefault(x => x.Name == config.Name)
                      ?? _db.StrategyConfigs.FirstOrDefault(x => x.Name == config.Name);
        if (existing is not null)
        {
            existing.Value = config.Value;
            existing.Description = config.Description;
            existing.UpdatedAt = DateTimeOffset.UtcNow;
        }
        else
        {
            _db.StrategyConfigs.Add(config);
        }
        return Task.CompletedTask;
    }
}
