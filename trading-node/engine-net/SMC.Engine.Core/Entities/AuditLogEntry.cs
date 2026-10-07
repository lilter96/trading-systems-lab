namespace SMC.Engine.Core.Entities;

public class AuditLogEntry
{
    public long Id { get; set; }
    public Guid SessionId { get; set; }
    public string EventType { get; set; } = string.Empty;
    public Guid? CorrelationId { get; set; }
    public string Payload { get; set; } = "{}";
    public DateTimeOffset CreatedAt { get; set; } = DateTimeOffset.UtcNow;

    public TradingSession Session { get; set; } = null!;
}
