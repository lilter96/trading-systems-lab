using Microsoft.EntityFrameworkCore;
using SMC.Engine.Core.Data;
using SMC.Engine.Core.Entities;

namespace SMC.Engine.Core.Repositories;

public class OrderRepository : IOrderRepository
{
    private readonly EngineDbContext _db;

    public OrderRepository(EngineDbContext db)
    {
        _db = db;
    }

    public Task<Order?> GetByIdAsync(Guid id, CancellationToken ct = default)
        => _db.Orders.FirstOrDefaultAsync(x => x.Id == id, ct);

    public Task<Order?> GetByExchangeIdAsync(string exchangeOrderId, CancellationToken ct = default)
        => _db.Orders.FirstOrDefaultAsync(x => x.ExchangeOrderId == exchangeOrderId, ct);

    public Task<List<Order>> GetBySessionIdAsync(Guid sessionId, CancellationToken ct = default)
        => _db.Orders.Where(x => x.SessionId == sessionId).ToListAsync(ct);

    public Task<List<Order>> GetAllOpenAsync(CancellationToken ct = default)
        => _db.Orders.Where(x => x.Status == "live" || x.Status == "partially_filled").ToListAsync(ct);

    public Task AddAsync(Order order, CancellationToken ct = default)
    {
        _db.Orders.Add(order);
        return Task.CompletedTask;
    }

    public Task UpdateAsync(Order order, CancellationToken ct = default)
    {
        _db.Orders.Update(order);
        return Task.CompletedTask;
    }
}
