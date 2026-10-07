package okxclient

import (
	"bytes"
	"time"
)

const (
	DefaultRingBufferCapacity  = 10000
	LongTermRingBufferCapacity = 36000000
)

type Tick struct {
	Channel      [16]byte
	InstID       [32]byte
	Price        float64
	Quantity     float64
	Side         byte
	ExchangeTime int64
	ReceiptTime  int64
}

func (t *Tick) GetChannel() string {
	return string(bytes.Trim(t.Channel[:], "\x00"))
}

func (t *Tick) GetInstID() string {
	return string(bytes.Trim(t.InstID[:], "\x00"))
}

func (t *Tick) GetSide() string {
	if t.Side == 0 {
		return ""
	}
	return string([]byte{t.Side})
}

func (t *Tick) GetExchangeTime() time.Time {
	return time.UnixMilli(t.ExchangeTime)
}

func (t *Tick) GetReceiptTime() time.Time {
	return time.Unix(0, t.ReceiptTime)
}

type RouteKey struct {
	Channel [16]byte
	InstID  [32]byte
}

type Slot struct {
	Tick Tick
}
