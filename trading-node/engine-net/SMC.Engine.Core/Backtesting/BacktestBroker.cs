using SMC.Engine.Core.Analysis;

namespace SMC.Engine.Core.Backtesting;

public sealed class BacktestBroker
{
    private readonly BrokerConfig _config;
    private readonly List<BacktestPosition> _positions = new();

    public decimal CurrentBalance { get; private set; }
    public IReadOnlyList<BacktestPosition> Positions => _positions;

    public decimal GetCurrentEquity() => CurrentBalance;

    public BacktestBroker(BrokerConfig config)
    {
        _config = config;
        CurrentBalance = config.InitialBalance;
    }

    public void ExecuteMarketEntry(
        string instrument,
        PositionSide side,
        decimal quantity,
        decimal targetPrice,
        decimal slPrice,
        decimal tpPrice,
        DateTimeOffset timestamp)
    {
        decimal slippageAmount = _config.SlippageTicks * _config.TickSize;
        decimal executedPrice = side == PositionSide.Long
            ? targetPrice + slippageAmount
            : targetPrice - slippageAmount;

        decimal volume = quantity * executedPrice;
        decimal entryFee = volume * _config.TakerFeeRate;

        var position = new BacktestPosition
        {
            Instrument = instrument,
            Side = side,
            Quantity = quantity,
            EntryPrice = executedPrice,
            SlPrice = slPrice,
            TpPrice = tpPrice,
            EntryFeePaid = entryFee,
            OpenedAt = timestamp,
        };

        CurrentBalance -= entryFee;
        _positions.Add(position);
    }

    public void ProcessBar(BarDataProto bar)
    {
        var openPositions = _positions
            .Where(p => p.Status == PositionStatus.Open && p.Instrument == bar.Instrument)
            .ToList();

        foreach (var pos in openPositions)
        {
            bool slTouched, tpTouched;

            if (pos.Side == PositionSide.Long)
            {
                slTouched = bar.Low <= pos.SlPrice;
                tpTouched = bar.High >= pos.TpPrice;
            }
            else
            {
                slTouched = bar.High >= pos.SlPrice;
                tpTouched = bar.Low <= pos.TpPrice;
            }

            if (slTouched && tpTouched)
            {
                ResolveDoubleTouch(pos, bar);
            }
            else if (slTouched)
            {
                CloseAtStopLoss(pos, bar.Timestamp);
            }
            else if (tpTouched)
            {
                CloseAtTakeProfit(pos, bar.Timestamp);
            }
        }
    }

    private void ResolveDoubleTouch(BacktestPosition pos, BarDataProto bar)
    {
        switch (_config.AmbiguityMode)
        {
            case SlTpResolutionMode.ConservativeFirst:
            case SlTpResolutionMode.Chronological:
                CloseAtStopLoss(pos, bar.Timestamp);
                break;
            case SlTpResolutionMode.TargetFirst:
                CloseAtTakeProfit(pos, bar.Timestamp);
                break;
        }
    }

    private void CloseAtStopLoss(BacktestPosition pos, DateTimeOffset timestamp)
    {
        decimal slippageAmount = _config.SlippageTicks * _config.TickSize;
        decimal executedPrice = pos.Side == PositionSide.Long
            ? pos.SlPrice - slippageAmount
            : pos.SlPrice + slippageAmount;

        ClosePosition(pos, executedPrice, _config.TakerFeeRate, timestamp);
    }

    private void CloseAtTakeProfit(BacktestPosition pos, DateTimeOffset timestamp)
    {
        ClosePosition(pos, pos.TpPrice, _config.MakerFeeRate, timestamp);
    }

    private void ClosePosition(BacktestPosition pos, decimal executedPrice, decimal feeRate, DateTimeOffset timestamp)
    {
        decimal volume = pos.Quantity * executedPrice;
        decimal exitFee = volume * feeRate;

        decimal pnl = pos.Side == PositionSide.Long
            ? (executedPrice - pos.EntryPrice) * pos.Quantity
            : (pos.EntryPrice - executedPrice) * pos.Quantity;

        pos.Status = PositionStatus.Closed;
        pos.ExitPrice = executedPrice;
        pos.ExitFeePaid = exitFee;
        pos.RealizedPnl = pnl;
        pos.ClosedAt = timestamp;

        CurrentBalance += pnl - exitFee;
    }
}
