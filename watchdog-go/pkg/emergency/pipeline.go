package emergency

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	pb "trading-system/contracts/go/commonpb"
	"watchdog-go/pkg/okx"
	"watchdog-go/pkg/telegram"
)

type ExecutionPipeline struct {
	grpcClients    []pb.EmergencyServiceClient
	grpcConns      []*grpc.ClientConn
	okxClient      *okx.Client
	telegramToken  string
	telegramChatId string
}

func New(
	grpcTargets []string,
	caCertPath, clientCertPath, clientKeyPath string,
	okxClient *okx.Client,
	telegramToken, telegramChatId string,
) (*ExecutionPipeline, error) {
	if len(grpcTargets) == 0 {
		log.Println("[WARN] No gRPC targets configured — gRPC broadcast disabled")
		return &ExecutionPipeline{
			okxClient:      okxClient,
			telegramToken:  telegramToken,
			telegramChatId: telegramChatId,
		}, nil
	}

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("read CA cert %q: %w", caCertPath, err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA cert %q", caCertPath)
	}

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load client cert/key %q/%q: %w", clientCertPath, clientKeyPath, err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS12,
	}

	var clients []pb.EmergencyServiceClient
	var conns []*grpc.ClientConn

	for _, target := range grpcTargets {
		creds := credentials.NewTLS(tlsConfig)
		conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(creds))
		if err != nil {
			log.Printf("[WARN] gRPC dial %s failed: %v — skipping", target, err)
			continue
		}
		conns = append(conns, conn)
		clients = append(clients, pb.NewEmergencyServiceClient(conn))
	}

	return &ExecutionPipeline{
		grpcClients:    clients,
		grpcConns:      conns,
		okxClient:      okxClient,
		telegramToken:  telegramToken,
		telegramChatId: telegramChatId,
	}, nil
}

func (p *ExecutionPipeline) Close() {
	for _, conn := range p.grpcConns {
		conn.Close()
	}
}

func (p *ExecutionPipeline) ExecuteHaltSequence(ctx context.Context, reason string) {
	log.Printf("[CRITICAL] EMERGENCY HALT SEQUENCE INITIATED. Reason: %s", reason)

	// Steps 1-2: broadcast the halt command. The .NET engine owns the durable
	// database lockdown through its EmergencyService implementation, so the
	// watchdog never writes directly to the engine database.
	var wg sync.WaitGroup
	for i, client := range p.grpcClients {
		wg.Add(1)
		go func(idx int, cli pb.EmergencyServiceClient) {
			defer wg.Done()
			grpcCtx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()

			_, err := cli.HaltSystem(grpcCtx, &pb.HaltRequest{
				Reason:      reason,
				Timestamp:   time.Now().UnixNano() / int64(time.Millisecond),
				TriggeredBy: "watchdog-go",
			})
			if err != nil {
				log.Printf("[ERROR] gRPC Halt failed for client %d: %v. Continuing...", idx, err)
			} else {
				log.Printf("[HALT-1,2/6] gRPC Halt pushed to client %d.", idx)
			}
		}(i, client)
	}
	wg.Wait()
	log.Println("[HALT-1,2/6] gRPC broadcasts complete; durable DB halt delegated to engine service.")

	if p.okxClient == nil {
		log.Println("[HALT-3,4,5/6] OKX client not configured — skipping exchange liquidation.")
	} else {
		log.Println("[HALT-3,4,5/6] Executing OKX halt (cancel orders + flatten positions)…")
		if err := p.okxClient.EmergencyHalt(ctx); err != nil {
			log.Printf("[ERROR] OKX halt: %v", err)
		}
	}

	// Step 6: Telegram alert
	if p.telegramToken != "" && p.telegramChatId != "" {
		if err := telegram.SendAlert(p.telegramToken, p.telegramChatId,
			fmt.Sprintf("[HALT] Emergency halt triggered. Reason: %s", reason)); err != nil {
			log.Printf("[ERROR] Telegram alert: %v. Continuing...", err)
		} else {
			log.Println("[HALT-6/6] Telegram alert dispatched.")
		}
	} else {
		log.Println("[HALT-6/6] Telegram not configured — skipping alert.")
	}
}
