namespace SMC.Engine.Core.Backtesting;

public enum PositionSide { Long, Short }
public enum PositionStatus { Open, Closed }

public sealed class BacktestPosition
{
    public Guid Id { get; init; } = Guid.NewGuid();
    public string Instrument { get; init; } = string.Empty;
    public PositionSide Side { get; init; }
    public decimal Quantity { get; init; }
    public decimal EntryPrice { get; init; }
    public decimal? ExitPrice { get; set; }
    public decimal SlPrice { get; init; }
    public decimal TpPrice { get; init; }
    public PositionStatus Status { get; set; } = PositionStatus.Open;

    public decimal EntryFeePaid { get; set; }
    public decimal ExitFeePaid { get; set; }
    public decimal RealizedPnl { get; set; }
    public decimal NetRealizedPnl => RealizedPnl - EntryFeePaid - ExitFeePaid;

    public DateTimeOffset OpenedAt { get; init; }
    public DateTimeOffset? ClosedAt { get; set; }
}
