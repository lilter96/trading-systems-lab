namespace SMC.Engine.Core.Entities;

public class StrategyConfig
{
    public Guid Id { get; set; }
    public string Name { get; set; } = string.Empty;
    public string Value { get; set; } = "{}";
    public string? Description { get; set; }
    public DateTimeOffset UpdatedAt { get; set; } = DateTimeOffset.UtcNow;
}
