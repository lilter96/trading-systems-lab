package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trading-node/gateway-go/clickhouse"
	"trading-node/gateway-go/emergency"
	"trading-node/gateway-go/exchangestate"
	"trading-node/gateway-go/okxclient"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	wsClient := okxclient.NewOKXClient("wss://ws.okx.com:8443/ws/v5/public")

	if err := wsClient.Connect(ctx); err != nil {
		fmt.Println("Initial connection failed:", err)
		return
	}

	wsClient.StartSupervisor(ctx)
	defer wsClient.Close()

	btcQueue := okxclient.NewSPSCTickQueue(16384)
	historyBuffer := okxclient.NewTickRingBuffer(16384)

	err := wsClient.Subscribe("tickers", "BTC-USDT", btcQueue)
	if err != nil {
		fmt.Println("Can't subscribe to BTC-USDT:", err)
		return
	}

	chWriter := clickhouse.NewWriter("")
	go func() {
		if err := chWriter.Start(ctx); err != nil {
			log.Printf("clickhouse writer error: %v", err)
		}
	}()

	engineBarCh := make(chan BarData, 64)

	aggregator := NewBarAggregator("BTC-USDT", time.Minute, 0.1, chWriter.TickCh, chWriter.BarCh, engineBarCh)

	var tickCount int64

	go aggregator.Run(ctx, btcQueue, historyBuffer, &tickCount)
	go runReporter(ctx, wsClient, historyBuffer, &tickCount)

	go func() {
		addr := ":50052"
		log.Printf("starting exchangestate gRPC server on %s", addr)
		if err := exchangestate.StartServer(addr); err != nil {
			log.Printf("exchangestate server error: %v", err)
		}
	}()

	go func() {
		caCert := os.Getenv("GRPC_CA_CERT")
		serverCert := os.Getenv("GRPC_GATEWAY_CERT")
		serverKey := os.Getenv("GRPC_GATEWAY_KEY")
		port := os.Getenv("GRPC_EMERGENCY_PORT")
		if port == "" {
			port = "50050"
		}
		addr := ":" + port

		if caCert != "" && serverCert != "" && serverKey != "" {
			log.Printf("starting emergency halt gRPC server on %s (mTLS)", addr)
			if _, err := emergency.StartServer(addr, caCert, serverCert, serverKey); err != nil {
				log.Printf("emergency server error: %v", err)
			}
		} else {
			log.Println("emergency halt gRPC server not started: missing mTLS cert env vars")
		}
	}()

	target := os.Getenv("GRPC_TARGET")
	if target == "" {
		target = "localhost:50051"
	}
	caCert := os.Getenv("GRPC_CA_CERT")
	clientCert := os.Getenv("GRPC_GATEWAY_CERT")
	clientKey := os.Getenv("GRPC_GATEWAY_KEY")
	if caCert != "" && clientCert != "" && clientKey != "" {
		go startBarForwarder(ctx, engineBarCh, target, caCert, clientCert, clientKey)
	} else {
		log.Println("bar forwarder not started: missing GRPC_CA_CERT / GRPC_GATEWAY_CERT / GRPC_GATEWAY_KEY env vars")
	}

	log.Println("gateway started — waiting for ticks")

	<-sigCh
	log.Println("shutting down...")
	cancel()
	time.Sleep(500 * time.Millisecond)
}
