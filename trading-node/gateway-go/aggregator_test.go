package main

import (
	"testing"
	"time"

	"trading-node/gateway-go/okxclient"
)

func makeTick(price, qty float64, side byte, ts int64) okxclient.Tick {
	var tick okxclient.Tick
	copy(tick.InstID[:], "BTC-USDT")
	copy(tick.Channel[:], "trades")
	tick.Price = price
	tick.Quantity = qty
	tick.Side = side
	tick.ExchangeTime = ts
	tick.ReceiptTime = time.Now().UnixNano()
	return tick
}

func TestAggregatorBasicOHLCV(t *testing.T) {
	agg := NewBarAggregator("BTC-USDT", time.Minute, 0.1, nil, nil, nil)

	baseTime := int64(60000) // 00:01:00.000
	ticks := []okxclient.Tick{
		makeTick(100.0, 1.0, 'b', baseTime+1000),   // Open
		makeTick(102.0, 2.0, 's', baseTime+2000),   // High
		makeTick(98.0, 1.5, 'b', baseTime+3000),    // Low
		makeTick(101.0, 3.0, 's', baseTime+4000),   // Close
	}

	for i := range ticks {
		agg.Feed(&ticks[i])
	}

	bar := agg.SealBar()

	if bar.Open != 100.0 {
		t.Errorf("Open = %v, want 100.0", bar.Open)
	}
	if bar.High != 102.0 {
		t.Errorf("High = %v, want 102.0", bar.High)
	}
	if bar.Low != 98.0 {
		t.Errorf("Low = %v, want 98.0", bar.Low)
	}
	if bar.Close != 101.0 {
		t.Errorf("Close = %v, want 101.0", bar.Close)
	}
	if bar.Volume != 7.5 {
		t.Errorf("Volume = %v, want 7.5", bar.Volume)
	}
	if bar.TickCount != 4 {
		t.Errorf("TickCount = %v, want 4", bar.TickCount)
	}
	if !bar.Complete {
		t.Error("Complete should be true")
	}
}

func TestAggregatorBuySellVolume(t *testing.T) {
	agg := NewBarAggregator("BTC-USDT", time.Minute, 0.1, nil, nil, nil)

	baseTime := int64(60000)
	ticks := []okxclient.Tick{
		makeTick(100.0, 2.0, 'b', baseTime+1000),
		makeTick(101.0, 3.0, 's', baseTime+2000),
		makeTick(100.0, 1.0, 'b', baseTime+3000),
		makeTick(102.0, 4.0, 's', baseTime+4000),
	}

	for i := range ticks {
		agg.Feed(&ticks[i])
	}

	bar := agg.SealBar()

	if bar.BuyVolume != 3.0 {
		t.Errorf("BuyVolume = %v, want 3.0", bar.BuyVolume)
	}
	if bar.SellVolume != 7.0 {
		t.Errorf("SellVolume = %v, want 7.0", bar.SellVolume)
	}
	if bar.Volume != 10.0 {
		t.Errorf("Volume = %v, want 10.0", bar.Volume)
	}
	if bar.BuyVolume+bar.SellVolume != bar.Volume {
		t.Errorf("BuyVolume + SellVolume = %v, want %v", bar.BuyVolume+bar.SellVolume, bar.Volume)
	}
}

func TestAggregatorFootprint(t *testing.T) {
	agg := NewBarAggregator("BTC-USDT", time.Minute, 0.1, nil, nil, nil)

	baseTime := int64(60000)
	ticks := []okxclient.Tick{
		makeTick(100.0, 2.0, 'b', baseTime+1000),
		makeTick(100.0, 3.0, 's', baseTime+2000),
		makeTick(101.0, 1.5, 'b', baseTime+3000),
		makeTick(102.0, 4.0, 's', baseTime+4000),
		makeTick(101.0, 2.5, 's', baseTime+5000),
	}

	for i := range ticks {
		agg.Feed(&ticks[i])
	}

	bar := agg.SealBar()

	if len(bar.Footprint) != 3 {
		t.Fatalf("Footprint has %d buckets, want 3", len(bar.Footprint))
	}

	// 100.0: buyVol=2.0, sellVol=3.0
	if bar.Footprint[0].PriceLevel != 100.0 {
		t.Errorf("Footprint[0].PriceLevel = %v, want 100.0", bar.Footprint[0].PriceLevel)
	}
	if bar.Footprint[0].BuyVolume != 2.0 {
		t.Errorf("Footprint[0].BuyVolume = %v, want 2.0", bar.Footprint[0].BuyVolume)
	}
	if bar.Footprint[0].SellVolume != 3.0 {
		t.Errorf("Footprint[0].SellVolume = %v, want 3.0", bar.Footprint[0].SellVolume)
	}

	// 101.0: buyVol=1.5, sellVol=2.5
	if bar.Footprint[1].PriceLevel != 101.0 {
		t.Errorf("Footprint[1].PriceLevel = %v, want 101.0", bar.Footprint[1].PriceLevel)
	}
	if bar.Footprint[1].BuyVolume != 1.5 {
		t.Errorf("Footprint[1].BuyVolume = %v, want 1.5", bar.Footprint[1].BuyVolume)
	}
	if bar.Footprint[1].SellVolume != 2.5 {
		t.Errorf("Footprint[1].SellVolume = %v, want 2.5", bar.Footprint[1].SellVolume)
	}

	// 102.0: buyVol=0, sellVol=4.0
	if bar.Footprint[2].PriceLevel != 102.0 {
		t.Errorf("Footprint[2].PriceLevel = %v, want 102.0", bar.Footprint[2].PriceLevel)
	}
	if bar.Footprint[2].BuyVolume != 0 {
		t.Errorf("Footprint[2].BuyVolume = %v, want 0", bar.Footprint[2].BuyVolume)
	}
	if bar.Footprint[2].SellVolume != 4.0 {
		t.Errorf("Footprint[2].SellVolume = %v, want 4.0", bar.Footprint[2].SellVolume)
	}

	// sum of footprint equals total volume
	fpSum := bar.SumFootprintVolume()
	if fpSum != bar.Volume {
		t.Errorf("Footprint sum = %v, want %v", fpSum, bar.Volume)
	}
}

func TestAggregatorBarBoundary(t *testing.T) {
	agg := NewBarAggregator("BTC-USDT", time.Minute, 0.1, nil, nil, nil)

	firstBar := int64(0)
	secondBar := int64(60000)

	// bar 1 ticks
	agg.Feed(&okxclient.Tick{Price: 100, Quantity: 1, ExchangeTime: firstBar + 1000})
	agg.Feed(&okxclient.Tick{Price: 101, Quantity: 2, ExchangeTime: firstBar + 2000})

	bar1 := agg.SealBar()
	if bar1.Open != 100.0 {
		t.Errorf("Bar1 Open = %v, want 100.0", bar1.Open)
	}
	if bar1.Close != 101.0 {
		t.Errorf("Bar1 Close = %v, want 101.0", bar1.Close)
	}
	if bar1.High != 101.0 {
		t.Errorf("Bar1 High = %v, want 101.0", bar1.High)
	}
	if bar1.Low != 100.0 {
		t.Errorf("Bar1 Low = %v, want 100.0", bar1.Low)
	}
	if bar1.Volume != 3.0 {
		t.Errorf("Bar1 Volume = %v, want 3.0", bar1.Volume)
	}
	if bar1.TickCount != 2 {
		t.Errorf("Bar1 TickCount = %v, want 2", bar1.TickCount)
	}
	if bar1.BarTime != firstBar {
		t.Errorf("Bar1 BarTime = %v, want %v", bar1.BarTime, firstBar)
	}

	// bar 2 ticks
	agg.Feed(&okxclient.Tick{Price: 102, Quantity: 1, ExchangeTime: secondBar + 1000})
	agg.Feed(&okxclient.Tick{Price: 103, Quantity: 2, ExchangeTime: secondBar + 2000})

	bar2 := agg.SealBar()
	if bar2.Open != 102.0 {
		t.Errorf("Bar2 Open = %v, want 102.0", bar2.Open)
	}
	if bar2.Close != 103.0 {
		t.Errorf("Bar2 Close = %v, want 103.0", bar2.Close)
	}
	if bar2.High != 103.0 {
		t.Errorf("Bar2 High = %v, want 103.0", bar2.High)
	}
	if bar2.Low != 102.0 {
		t.Errorf("Bar2 Low = %v, want 102.0", bar2.Low)
	}
	if bar2.Volume != 3.0 {
		t.Errorf("Bar2 Volume = %v, want 3.0", bar2.Volume)
	}
	if bar2.TickCount != 2 {
		t.Errorf("Bar2 TickCount = %v, want 2", bar2.TickCount)
	}
	if bar2.BarTime != secondBar {
		t.Errorf("Bar2 BarTime = %v, want %v", bar2.BarTime, secondBar)
	}
}

func TestAggregatorEmptyBar(t *testing.T) {
	agg := NewBarAggregator("BTC-USDT", time.Minute, 0.1, nil, nil, nil)

	bar := agg.SealBar()

	if bar.TickCount != 0 {
		t.Errorf("Empty bar TickCount = %v, want 0", bar.TickCount)
	}
	if bar.Volume != 0 {
		t.Errorf("Empty bar Volume = %v, want 0", bar.Volume)
	}
	if bar.Footprint != nil {
		t.Errorf("Empty bar Footprint = %v, want nil", bar.Footprint)
	}
}

func TestAggregatorTickerNoSide(t *testing.T) {
	agg := NewBarAggregator("BTC-USDT", time.Minute, 0.1, nil, nil, nil)

	baseTime := int64(60000)
	ticks := []okxclient.Tick{
		makeTick(100.0, 1.0, 0, baseTime+1000),
		makeTick(101.0, 2.0, 0, baseTime+2000),
	}

	for i := range ticks {
		agg.Feed(&ticks[i])
	}

	bar := agg.SealBar()

	if bar.Volume != 3.0 {
		t.Errorf("Volume = %v, want 3.0", bar.Volume)
	}
	// ticks with no side go to BuyVolume (default case in footprint)
	if bar.BuyVolume != 3.0 {
		t.Errorf("BuyVolume = %v, want 3.0", bar.BuyVolume)
	}
	if bar.SellVolume != 0 {
		t.Errorf("SellVolume = %v, want 0", bar.SellVolume)
	}
}
