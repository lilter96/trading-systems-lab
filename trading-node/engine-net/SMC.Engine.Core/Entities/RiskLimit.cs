namespace SMC.Engine.Core.Entities;

public class RiskLimit
{
    public Guid Id { get; set; }
    public Guid? SessionId { get; set; }
    public decimal DailyLossLimit { get; set; }
    public int MaxOpenPositions { get; set; }
    public bool GlobalHalt { get; set; }
    public decimal? MaxPositionSize { get; set; }
    public decimal? MaxLeverage { get; set; }
    public DateTimeOffset UpdatedAt { get; set; } = DateTimeOffset.UtcNow;

    public TradingSession? Session { get; set; }
}
