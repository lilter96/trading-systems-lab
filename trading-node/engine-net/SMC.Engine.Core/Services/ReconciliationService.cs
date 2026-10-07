using Microsoft.Extensions.Logging;
using SMC.Engine.Core.Data;
using SMC.Engine.Core.Entities;
using SMC.Engine.Core.GatewayClient;
using SMC.Engine.Core.Repositories;
using Trading;

namespace SMC.Engine.Core.Services;

public interface IReconciliationService
{
    Task ReconcileAsync(CancellationToken cancellationToken = default);
}

public class ReconciliationService : IReconciliationService
{
    private readonly IExchangeStateClient _exchangeStateClient;
    private readonly IOrderRepository _orderRepository;
    private readonly IPositionRepository _positionRepository;
    private readonly ISessionRepository _sessionRepository;
    private readonly EngineDbContext _db;
    private readonly ILogger<ReconciliationService> _logger;

    public ReconciliationService(
        IExchangeStateClient exchangeStateClient,
        IOrderRepository orderRepository,
        IPositionRepository positionRepository,
        ISessionRepository sessionRepository,
        EngineDbContext db,
        ILogger<ReconciliationService> logger)
    {
        _exchangeStateClient = exchangeStateClient;
        _orderRepository = orderRepository;
        _positionRepository = positionRepository;
        _sessionRepository = sessionRepository;
        _db = db;
        _logger = logger;
    }

    public async Task ReconcileAsync(CancellationToken cancellationToken = default)
    {
        _logger.LogInformation("Starting three-way reconciliation...");

        var exchangeState = await _exchangeStateClient.GetExchangeStateAsync(new ExchangeStateRequest());

        var session = await GetOrCreateReconciliationSession(cancellationToken);

        int positionsCorrected = 0;
        int ordersCorrected = 0;

        // Reconcile positions
        foreach (var exPos in exchangeState.Positions)
        {
            var dbPos = await _positionRepository.GetByAssetAsync(exPos.InstId, cancellationToken);

            if (dbPos is null)
            {
                _logger.LogWarning("Position {InstId} exists on exchange but not in DB — inserting", exPos.InstId);
                var pos = ParsePosition(exPos, session.Id);
                await _positionRepository.AddAsync(pos, cancellationToken);
                positionsCorrected++;
                continue;
            }

            if (dbPos.CurrentSize.ToString() != exPos.Pos ||
                (exPos.AvgPx != "" && dbPos.EntryPrice.ToString() != exPos.AvgPx))
            {
                _logger.LogWarning(
                    "Position {InstId} mismatch: exchange={ExSize}@{ExPx} db={DbSize}@{DbPx} — updating",
                    exPos.InstId, exPos.Pos, exPos.AvgPx, dbPos.CurrentSize, dbPos.EntryPrice);

                decimal.TryParse(exPos.Pos, out var newSize);
                dbPos.CurrentSize = newSize;
                if (decimal.TryParse(exPos.AvgPx, out var newAvgPx))
                    dbPos.EntryPrice = newAvgPx;
                await _positionRepository.UpdateAsync(dbPos, cancellationToken);
                positionsCorrected++;
            }
        }

        // Reconcile orders
        foreach (var exOrd in exchangeState.Orders)
        {
            var dbOrd = await _orderRepository.GetByExchangeIdAsync(exOrd.OrdId, cancellationToken);

            if (dbOrd is null)
            {
                _logger.LogWarning("Order {OrdId} exists on exchange but not in DB — inserting", exOrd.OrdId);
                var order = ParseOrder(exOrd, session.Id);
                await _orderRepository.AddAsync(order, cancellationToken);
                ordersCorrected++;
                continue;
            }

            if (dbOrd.Status != exOrd.State)
            {
                _logger.LogWarning(
                    "Order {OrdId} status mismatch: exchange={ExState} db={DbState} — updating",
                    exOrd.OrdId, exOrd.State, dbOrd.Status);

                dbOrd.Status = exOrd.State;
                dbOrd.UpdatedAt = DateTimeOffset.UtcNow;
                await _orderRepository.UpdateAsync(dbOrd, cancellationToken);
                ordersCorrected++;
            }
        }

        // Close DB positions not on exchange
        var dbOrders = await _orderRepository.GetAllOpenAsync(cancellationToken);
        var exchangeOrderIds = new HashSet<string>(exchangeState.Orders.Select(o => o.OrdId));
        foreach (var dbOrd in dbOrders)
        {
            if (!string.IsNullOrEmpty(dbOrd.ExchangeOrderId) && !exchangeOrderIds.Contains(dbOrd.ExchangeOrderId))
            {
                _logger.LogWarning("Order {OrdId} in DB but not on exchange — cancelling", dbOrd.ExchangeOrderId);
                dbOrd.Status = "cancelled";
                dbOrd.UpdatedAt = DateTimeOffset.UtcNow;
                await _orderRepository.UpdateAsync(dbOrd, cancellationToken);
                ordersCorrected++;
            }
        }

        await _db.SaveChangesAsync(cancellationToken);

        _logger.LogInformation(
            "Reconciliation complete — positions corrected={Positions} orders corrected={Orders}",
            positionsCorrected, ordersCorrected);
    }

    private async Task<TradingSession> GetOrCreateReconciliationSession(CancellationToken ct)
    {
        var active = await _sessionRepository.GetActiveAsync(ct);
        if (active.Count > 0)
            return active[0];

        var session = new TradingSession
        {
            Id = Guid.NewGuid(),
            StrategyName = "Reconciliation",
            StartedAt = DateTimeOffset.UtcNow,
            Status = "active",
            InitialBalance = 0,
            CurrentEquity = 0,
        };
        await _sessionRepository.AddAsync(session, ct);
        await _db.SaveChangesAsync(ct);
        _logger.LogInformation("Created reconciliation session {SessionId}", session.Id);
        return session;
    }

    private static Position ParsePosition(ExchangePosition exPos, Guid sessionId)
    {
        decimal.TryParse(exPos.Pos, out var size);
        decimal.TryParse(exPos.AvgPx, out var avgPx);

        return new Position
        {
            Id = Guid.NewGuid(),
            SessionId = sessionId,
            Asset = exPos.InstId,
            Direction = exPos.PosSide == "long" ? "buy" : "sell",
            EntryPrice = avgPx,
            CurrentSize = size,
            Status = "open",
            OpenedAt = DateTimeOffset.UtcNow,
        };
    }

    private static Order ParseOrder(ExchangeOrder exOrd, Guid sessionId)
    {
        decimal.TryParse(exOrd.Sz, out var size);
        decimal.TryParse(exOrd.Px, out var price);

        return new Order
        {
            Id = Guid.NewGuid(),
            SessionId = sessionId,
            ExchangeOrderId = exOrd.OrdId,
            Asset = exOrd.InstId,
            Direction = exOrd.Side == "buy" ? "buy" : "sell",
            Size = size,
            Status = exOrd.State,
            OrderType = MapOrderType(exOrd.OrdType),
            LimitPrice = price,
            CreatedAt = DateTimeOffset.UtcNow,
            UpdatedAt = DateTimeOffset.UtcNow,
        };
    }

    private static string MapOrderType(string ordType)
    {
        return ordType switch
        {
            "limit" => "limit",
            "market" => "market",
            "post_only" => "limit",
            "fok" => "limit",
            "ioc" => "limit",
            "optimal_limit_ioc" => "limit",
            _ => "limit",
        };
    }
}
