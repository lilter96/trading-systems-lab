using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public interface IPositionRepository
{
    Task<Position?> GetByIdAsync(Guid id, CancellationToken ct = default);
    Task<Position?> GetByAssetAsync(string asset, CancellationToken ct = default);
    Task<List<Position>> GetOpenBySessionIdAsync(Guid sessionId, CancellationToken ct = default);
    Task AddAsync(Position position, CancellationToken ct = default);
    Task UpdateAsync(Position position, CancellationToken ct = default);
}
