using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public interface IOrderRepository
{
    Task<Order?> GetByIdAsync(Guid id, CancellationToken ct = default);
    Task<Order?> GetByExchangeIdAsync(string exchangeOrderId, CancellationToken ct = default);
    Task<List<Order>> GetBySessionIdAsync(Guid sessionId, CancellationToken ct = default);
    Task<List<Order>> GetAllOpenAsync(CancellationToken ct = default);
    Task AddAsync(Order order, CancellationToken ct = default);
    Task UpdateAsync(Order order, CancellationToken ct = default);
}
