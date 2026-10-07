namespace SMC.Engine.Core.Entities;

public class TradingSession
{
    public Guid Id { get; set; }
    public string StrategyName { get; set; } = string.Empty;
    public DateTimeOffset StartedAt { get; set; } = DateTimeOffset.UtcNow;
    public DateTimeOffset? EndedAt { get; set; }
    public decimal InitialBalance { get; set; }
    public decimal CurrentEquity { get; set; }
    public string Status { get; set; } = "active";
    public DateTimeOffset CreatedAt { get; set; } = DateTimeOffset.UtcNow;

    public ICollection<Order> Orders { get; set; } = new List<Order>();
    public ICollection<Position> Positions { get; set; } = new List<Position>();
    public ICollection<AuditLogEntry> AuditLogs { get; set; } = new List<AuditLogEntry>();
}
