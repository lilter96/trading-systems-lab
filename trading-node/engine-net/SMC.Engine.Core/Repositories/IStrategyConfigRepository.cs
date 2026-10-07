using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public interface IStrategyConfigRepository
{
    Task<List<StrategyConfig>> GetAllAsync(CancellationToken ct = default);
    Task<StrategyConfig?> GetByNameAsync(string name, CancellationToken ct = default);
    Task AddOrUpdateAsync(StrategyConfig config, CancellationToken ct = default);
}
