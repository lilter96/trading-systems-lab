package main

import (
	"context"
	"math"
	"runtime"
	"sync/atomic"
	"time"

	"trading-node/gateway-go/clickhouse"
	"trading-node/gateway-go/okxclient"
)

type priceLevel struct {
	price   float64
	buyVol  float64
	sellVol float64
}

type BarAggregator struct {
	instrument string
	interval   time.Duration
	tickSize   float64
	flushAfter time.Duration

	barTime   int64
	open      float64
	high      float64
	low       float64
	close     float64
	volume    float64
	buyVol    float64
	sellVol   float64
	oiChange  float64
	tickCount int64
	footprint map[int64]*priceLevel
	hasTrade  bool

	tickSink     chan<- okxclient.Tick
	barSink      chan<- clickhouse.BarRecord
	engineBarCh  chan<- BarData
}

func NewBarAggregator(instrument string, interval time.Duration, tickSize float64, tickSink chan<- okxclient.Tick, barSink chan<- clickhouse.BarRecord, engineBarCh chan<- BarData) *BarAggregator {
	return &BarAggregator{
		instrument:   instrument,
		interval:     interval,
		tickSize:     tickSize,
		flushAfter:   interval / 2,
		footprint:    make(map[int64]*priceLevel),
		tickSink:     tickSink,
		barSink:      barSink,
		engineBarCh:  engineBarCh,
	}
}

func (a *BarAggregator) Feed(tick *okxclient.Tick) {
	if a.tickSink != nil {
		select {
		case a.tickSink <- *tick:
		default:
		}
	}

	if !a.hasTrade {
		a.open = tick.Price
		a.high = tick.Price
		a.low = tick.Price
		a.barTime = (tick.ExchangeTime / int64(a.interval.Milliseconds())) * int64(a.interval.Milliseconds())
		a.hasTrade = true
	}

	if tick.Price > a.high {
		a.high = tick.Price
	}
	if tick.Price < a.low {
		a.low = tick.Price
	}
	a.close = tick.Price

	a.volume += tick.Quantity

	switch tick.Side {
	case 'b':
		a.buyVol += tick.Quantity
	case 's':
		a.sellVol += tick.Quantity
	default:
		a.buyVol += tick.Quantity
	}

	a.tickCount++

	key := int64(math.Round(tick.Price / a.tickSize))
	lvl, ok := a.footprint[key]
	if !ok {
		lvl = &priceLevel{price: math.Round(tick.Price/a.tickSize) * a.tickSize}
		a.footprint[key] = lvl
	}
	switch tick.Side {
	case 'b':
		lvl.buyVol += tick.Quantity
	case 's':
		lvl.sellVol += tick.Quantity
	default:
		lvl.buyVol += tick.Quantity
	}
}

func (a *BarAggregator) SealBar() BarData {
	bar := BarData{
		Instrument: a.instrument,
		BarTime:    a.barTime,
		Open:       a.open,
		High:       a.high,
		Low:        a.low,
		Close:      a.close,
		Volume:     a.volume,
		BuyVolume:  a.buyVol,
		SellVolume: a.sellVol,
		OIChange:   a.oiChange,
		TickCount:  a.tickCount,
		Complete:   true,
	}

	if len(a.footprint) > 0 {
		keys := sortedKeys(a.footprint)
		bar.Footprint = make([]FootprintBucket, len(keys))
		for i, k := range keys {
			lvl := a.footprint[k]
			bar.Footprint[i] = FootprintBucket{
				PriceLevel: lvl.price,
				BuyVolume:  lvl.buyVol,
				SellVolume: lvl.sellVol,
			}
		}
	}

	a.resetBar()
	return bar
}

func (a *BarAggregator) resetBar() {
	a.open = 0
	a.high = 0
	a.low = 0
	a.close = 0
	a.volume = 0
	a.buyVol = 0
	a.sellVol = 0
	a.oiChange = 0
	a.tickCount = 0
	a.footprint = make(map[int64]*priceLevel)
	a.hasTrade = false
}

func (a *BarAggregator) publishBar(bar BarData) {
	if a.barSink != nil {
		rec := barToRecord(bar)
		select {
		case a.barSink <- rec:
		default:
		}
	}

	if a.engineBarCh != nil {
		select {
		case a.engineBarCh <- bar:
		default:
		}
	}
}

func (a *BarAggregator) Run(ctx context.Context, queue *okxclient.SPSCTickQueue, historyBuffer *okxclient.TickRingBuffer, tickCount *int64) {
	var tick okxclient.Tick

	for {
		select {
		case <-ctx.Done():
			if a.hasTrade {
				bar := a.SealBar()
				a.publishBar(bar)
			}
			return
		default:
		}

		if queue.Pop(&tick) {
			barTime := (tick.ExchangeTime / int64(a.interval.Milliseconds())) * int64(a.interval.Milliseconds())

			if a.hasTrade && barTime != a.barTime {
				bar := a.SealBar()
				bar.BarTime = a.barTime
				a.publishBar(bar)
			}

			if !a.hasTrade {
				a.barTime = barTime
			}

			a.Feed(&tick)

			historyBuffer.Push(&tick)
			select {
			case tickStreamCh <- tick:
			default:
			}
			atomic.AddInt64(tickCount, 1)

			continue
		}

		if a.hasTrade {
			now := time.Now().UnixMilli()
			deadline := a.barTime + int64(a.interval.Milliseconds()) + int64(a.flushAfter.Milliseconds())
			if now >= deadline {
				bar := a.SealBar()
				bar.Complete = false
				bar.BarTime = a.barTime
				a.publishBar(bar)
			}
		}

		runtime.Gosched()
	}
}

func barToRecord(bar BarData) clickhouse.BarRecord {
	fp := make([]clickhouse.FootprintRow, len(bar.Footprint))
	for i, b := range bar.Footprint {
		fp[i] = clickhouse.FootprintRow{
			Price:      b.PriceLevel,
			BuyVolume:  b.BuyVolume,
			SellVolume: b.SellVolume,
		}
	}
	return clickhouse.BarRecord{
		Timestamp:          time.UnixMilli(bar.BarTime),
		Timeframe:          "1m",
		Symbol:             bar.Instrument,
		Open:               bar.Open,
		High:               bar.High,
		Low:                bar.Low,
		Close:              bar.Close,
		Volume:             bar.Volume,
		BuyVolume:          bar.BuyVolume,
		SellVolume:         bar.SellVolume,
		BuySellVolumeDelta: bar.BuyVolume - bar.SellVolume,
		Vwap:               0,
		OpenInterest:       bar.OIChange,
		TickCount:          bar.TickCount,
		Complete:           bar.Complete,
		Footprint:          fp,
	}
}
