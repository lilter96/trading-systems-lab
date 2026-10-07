package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"trading-system/contracts/go/marketdatapb"
)

func main() {
	caCertPath := getEnv("GRPC_CA_CERT", "deployment/certs/ca.crt")
	clientCertPath := getEnv("GRPC_CLIENT_CERT", "deployment/certs/client.crt")
	clientKeyPath := getEnv("GRPC_CLIENT_KEY", "deployment/certs/client.key")
	target := getEnv("GRPC_TARGET", "localhost:50051")

	caPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		log.Fatalf("read CA cert: %v", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		log.Fatalf("append CA cert to pool")
	}

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		log.Fatalf("load client cert/key: %v", err)
	}

	tlsCfg := &tls.Config{
		ServerName:   "localhost",
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS12,
	}

	conn, err := grpc.Dial(target,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
	)
	if err != nil {
		log.Fatalf("dial %s: %v", target, err)
	}
	defer conn.Close()

	client := marketdatapb.NewMarketDataFeedClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.StreamBars(ctx)
	if err != nil {
		log.Fatalf("StreamBars: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var barID int64

	go func() {
		for {
			ack, err := stream.Recv()
			if err != nil {
				log.Printf("recv ack error: %v", err)
				cancel()
				return
			}
			log.Printf("ACK: id=%s status=%s msg=%q ts=%d",
				ack.CommandId, ack.Status, ack.Message, ack.Timestamp)
		}
	}()

	log.Printf("streaming bars to %s ...", target)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	price := 50000.0

	for {
		select {
		case <-ctx.Done():
			stream.CloseSend()
			log.Println("shutting down")
			return

		case <-sigCh:
			log.Println("interrupted")
			cancel()

		case <-ticker.C:
			// Simulate OHLC movement
			open := price
			high := open + rand.Float64()*20
			low := open - rand.Float64()*20
			close := low + rand.Float64()*(high-low)
			volume := rand.Float64() * 100
			price = close

			id := atomic.AddInt64(&barID, 1)
			ts := time.Now().UnixMilli()

			bar := &marketdatapb.BarData{
				Timestamp:           ts,
				Timeframe:           "1m",
				Open:                fmt.Sprintf("%.2f", math.Round(open*100)/100),
				High:                fmt.Sprintf("%.2f", math.Round(high*100)/100),
				Low:                 fmt.Sprintf("%.2f", math.Round(low*100)/100),
				Close:               fmt.Sprintf("%.2f", math.Round(close*100)/100),
				Volume:              fmt.Sprintf("%.2f", math.Round(volume*100)/100),
				OpenInterest:        "0",
				Vwap:                "0",
				BuySellVolumeDelta:  "0",
				Footprint:           nil,
			}

			if err := stream.Send(bar); err != nil {
				log.Printf("send bar %d error: %v", id, err)
				cancel()
				return
			}

			log.Printf("sent bar %d: O=%.2f H=%.2f L=%.2f C=%.2f V=%.2f",
				id, bar.Open, bar.High, bar.Low, bar.Close, bar.Volume)
		}
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
