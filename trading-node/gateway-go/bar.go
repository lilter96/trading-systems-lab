package main

import "sort"

type FootprintBucket struct {
	PriceLevel float64 `json:"price"`
	BuyVolume  float64 `json:"buyVol"`
	SellVolume float64 `json:"sellVol"`
}

type BarData struct {
	Instrument string           `json:"inst"`
	BarTime    int64            `json:"time"`
	Open       float64          `json:"open"`
	High       float64          `json:"high"`
	Low        float64          `json:"low"`
	Close      float64          `json:"close"`
	Volume     float64          `json:"vol"`
	BuyVolume  float64          `json:"buyVol"`
	SellVolume float64          `json:"sellVol"`
	OIChange   float64          `json:"oi,omitempty"`
	TickCount  int64            `json:"n"`
	Complete   bool             `json:"complete"`
	Footprint  []FootprintBucket `json:"fp,omitempty"`
}

func (b *BarData) SumFootprintVolume() float64 {
	var total float64
	for i := range b.Footprint {
		total += b.Footprint[i].BuyVolume + b.Footprint[i].SellVolume
	}
	return total
}

func sortedKeys(m map[int64]*priceLevel) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
