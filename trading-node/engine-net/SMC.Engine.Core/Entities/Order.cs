namespace SMC.Engine.Core.Entities;

public class Order
{
    public Guid Id { get; set; }
    public Guid SessionId { get; set; }
    public string? ExchangeOrderId { get; set; }
    public string Asset { get; set; } = string.Empty;
    public string Direction { get; set; } = string.Empty;
    public decimal Size { get; set; }
    public string Status { get; set; } = string.Empty;
    public decimal? ExecutionPrice { get; set; }
    public string OrderType { get; set; } = string.Empty;
    public decimal? LimitPrice { get; set; }
    public decimal? StopPrice { get; set; }
    public DateTimeOffset CreatedAt { get; set; } = DateTimeOffset.UtcNow;
    public DateTimeOffset UpdatedAt { get; set; } = DateTimeOffset.UtcNow;
    public DateTimeOffset? FilledAt { get; set; }
    public string? RejectedReason { get; set; }

    public TradingSession Session { get; set; } = null!;
}
