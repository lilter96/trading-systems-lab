namespace SMC.Engine.Core.Entities;

public class SystemState
{
    public Guid Id { get; set; }
    public string SystemId { get; set; } = "primary_instance";
    public string State { get; set; } = "ACTIVE";
    public bool TradingEnabled { get; set; } = true;
    public DateTimeOffset UpdatedAt { get; set; } = DateTimeOffset.UtcNow;
}
