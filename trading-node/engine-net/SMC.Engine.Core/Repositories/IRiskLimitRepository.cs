using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public interface IRiskLimitRepository
{
    Task<RiskLimit?> GetBySessionIdAsync(Guid? sessionId, CancellationToken ct = default);
    Task AddOrUpdateAsync(RiskLimit limit, CancellationToken ct = default);
}
