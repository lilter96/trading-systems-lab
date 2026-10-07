using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public interface ISessionRepository
{
    Task<TradingSession?> GetByIdAsync(Guid id, CancellationToken ct = default);
    Task<List<TradingSession>> GetActiveAsync(CancellationToken ct = default);
    Task AddAsync(TradingSession session, CancellationToken ct = default);
    Task UpdateAsync(TradingSession session, CancellationToken ct = default);
}
