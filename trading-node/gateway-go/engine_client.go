package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"log"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"trading-system/contracts/go/marketdatapb"
)

func startBarForwarder(ctx context.Context, barCh <-chan BarData, target, caCertPath, clientCertPath, clientKeyPath string) {
	log.Printf("bar forwarder: connecting to engine at %s", target)

	conn := dialEngine(ctx, target, caCertPath, clientCertPath, clientKeyPath)
	if conn == nil {
		return
	}
	defer conn.Close()

	client := marketdatapb.NewMarketDataFeedClient(conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		stream, err := client.StreamBars(ctx)
		if err != nil {
			log.Printf("bar forwarder: StreamBars error: %v (retrying in 3s)", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}

		log.Printf("bar forwarder: stream established")

		go func() {
			for {
				ack, err := stream.Recv()
				if err != nil {
					log.Printf("bar forwarder: recv ack error: %v", err)
					return
				}
				log.Printf("bar forwarder: ack id=%s status=%s", ack.CommandId, ack.Status)
			}
		}()

		err = forwardBars(ctx, stream, barCh)
		log.Printf("bar forwarder: stream closed (%v), reconnecting...", err)
	}
}

func dialEngine(ctx context.Context, target, caCertPath, clientCertPath, clientKeyPath string) *grpc.ClientConn {
	caPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		log.Printf("bar forwarder: read CA cert: %v", err)
		return nil
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		log.Printf("bar forwarder: append CA cert failed")
		return nil
	}

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		log.Printf("bar forwarder: load client cert/key: %v", err)
		return nil
	}

	tlsCfg := &tls.Config{
		ServerName:   "localhost",
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS12,
	}

	conn, err := grpc.DialContext(ctx, target,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
	)
	if err != nil {
		log.Printf("bar forwarder: dial %s: %v", target, err)
		return nil
	}
	return conn
}

func forwardBars(ctx context.Context, stream marketdatapb.MarketDataFeed_StreamBarsClient, barCh <-chan BarData) error {
	for {
		select {
		case <-ctx.Done():
			stream.CloseSend()
			return ctx.Err()
		case bar, ok := <-barCh:
			if !ok {
				stream.CloseSend()
				return nil
			}
			pb := &marketdatapb.BarData{
				Timestamp: bar.BarTime,
				Timeframe: "1m",
				Open:      strconv.FormatFloat(bar.Open, 'f', -1, 64),
				High:      strconv.FormatFloat(bar.High, 'f', -1, 64),
				Low:       strconv.FormatFloat(bar.Low, 'f', -1, 64),
				Close:     strconv.FormatFloat(bar.Close, 'f', -1, 64),
				Volume:    strconv.FormatFloat(bar.Volume, 'f', -1, 64),
			}
			if err := stream.Send(pb); err != nil {
				return err
			}
		}
	}
}
