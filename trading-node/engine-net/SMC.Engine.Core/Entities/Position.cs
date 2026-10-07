namespace SMC.Engine.Core.Entities;

public class Position
{
    public Guid Id { get; set; }
    public Guid SessionId { get; set; }
    public string Asset { get; set; } = string.Empty;
    public string Direction { get; set; } = string.Empty;
    public decimal EntryPrice { get; set; }
    public decimal CurrentSize { get; set; }
    public decimal? StopLoss { get; set; }
    public decimal? TakeProfit { get; set; }
    public string Status { get; set; } = "open";
    public DateTimeOffset OpenedAt { get; set; } = DateTimeOffset.UtcNow;
    public DateTimeOffset? ClosedAt { get; set; }
    public decimal? RealizedPnl { get; set; }

    public TradingSession Session { get; set; } = null!;
}
