package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"trading-node/gateway-go/okxclient"
)

type telemetryPayload struct {
	Sender         string  `json:"sender"`
	Timestamp      int64   `json:"timestamp"`
	Status         string  `json:"status"`
	TicksPerSecond int64   `json:"ticks_per_second"`
	TotalTicks     int64   `json:"total_ticks"`
	LastPrice      float64 `json:"last_price"`
}

var udpAddr = &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5555}

func sendTelemetryUdp(payload telemetryPayload) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.Write(data)
}

func runReporter(
	ctx context.Context,
	wsClient *okxclient.WSClient,
	historyBuffer *okxclient.TickRingBuffer,
	tickCount *int64,
) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastCount int64
	singleTickDest := make([]okxclient.Tick, 1)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentCount := atomic.LoadInt64(tickCount)
			ticksPerSecond := currentCount - lastCount
			lastCount = currentCount

			droppedCount := wsClient.GetDroppedTicks()

			var lastPrice float64
			n := historyBuffer.GetRecent(singleTickDest)
			if n > 0 {
				latest := &singleTickDest[0]
				lastPrice = latest.Price
				fmt.Printf("--- [LATEST TICK] ---\n")
				fmt.Printf("Channel:       %s\n", latest.GetChannel())
				fmt.Printf("Instrument:    %s\n", latest.GetInstID())
				fmt.Printf("Last Price:    %.2f\n", latest.Price)
				fmt.Printf("Last Quantity: %.4f\n", latest.Quantity)
				fmt.Printf("ExchangeTime:  %s\n", latest.GetExchangeTime().Format("15:04:05.000"))
				fmt.Printf("ReceiptTime:   %s\n", latest.GetReceiptTime().Format("15:04:05.000000"))
			}

			fmt.Printf("Throughput:    %d ticks/second\n", ticksPerSecond)
			fmt.Printf("Total Processed: %d ticks\n", currentCount)

			if droppedCount > 0 {
				fmt.Printf("\033[31mWARNING: Dropped %d ticks (queue overflow)\033[0m\n", droppedCount)
			} else {
				fmt.Printf("Dropped Ticks: 0 (healthy queue)\n")
			}
			fmt.Printf("---------------------\n\n")

			sendTelemetryUdp(telemetryPayload{
				Sender:         "gateway-go",
				Timestamp:      time.Now().UnixMilli(),
				Status:         "OK",
				TicksPerSecond: ticksPerSecond,
				TotalTicks:     currentCount,
				LastPrice:      lastPrice,
			})
		}
	}
}
