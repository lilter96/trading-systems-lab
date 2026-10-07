package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"watchdog-go/pkg/config"
	"watchdog-go/pkg/emergency"
	"watchdog-go/pkg/monitor"
	"watchdog-go/pkg/okx"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[FATAL] config load: %v", err)
	}

	log.Printf("[INFO] Watchdog starting (port=%d, missed=%ds, poll=%ds, floor=%.0f%%)",
		cfg.HeartbeatPort, cfg.AllowedMissedSeconds, cfg.BalancePollSeconds,
		cfg.EquityFloorFraction*100)

	beatChan := make(chan monitor.HeartbeatMessage, 64)
	equityChan := make(chan float64, 8)

	okxClient := okx.New(cfg.OkxApiKey, cfg.OkxApiSecret, cfg.OkxPassphrase, cfg.OkxIsDemo)

	pipeline, err := emergency.New(
		cfg.EmergencyServiceTargets,
		cfg.GrpcCaCert, cfg.GrpcWatchdogCert, cfg.GrpcWatchdogKey,
		okxClient,
		cfg.TelegramBotToken, cfg.TelegramChatId,
	)
	if err != nil {
		log.Fatalf("[FATAL] emergency pipeline init: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer pipeline.Close()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		udpListener(ctx, cfg.HeartbeatPort, beatChan)
	}()

	if cfg.OkxApiKey != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			balancePoller(ctx, okxClient, equityChan, cfg.BalancePollSeconds)
		}()
	} else {
		log.Println("[WARN] No OKX credentials — equity monitoring disabled")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("[INFO] Shutdown signal received")
		cancel()
	}()

	runCoordinator(ctx, cfg, beatChan, equityChan, func(reason string) {
		pipeline.ExecuteHaltSequence(ctx, reason)
	})

	wg.Wait()
	log.Println("[INFO] Watchdog shut down")
}

func udpListener(ctx context.Context, port int, beatChan chan<- monitor.HeartbeatMessage) {
	addr := &net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("[ERROR] UDP listen on :%d failed: %v", port, err)
		return
	}
	defer conn.Close()

	log.Printf("[INFO] UDP heartbeat listener on :%d", port)

	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn.SetDeadline(time.Now().Add(1 * time.Second))
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				select {
				case <-ctx.Done():
					return
				default:
					log.Printf("[ERROR] UDP read: %v", err)
					continue
				}
			}
			var msg monitor.HeartbeatMessage
			if err := json.Unmarshal(buf[:n], &msg); err != nil {
				log.Printf("[WARN] UDP parse error: %v", err)
				continue
			}
			if msg.Sender != "gateway-go" && msg.Sender != "engine-net" {
				log.Printf("[WARN] Unknown sender: %s", msg.Sender)
				continue
			}
			select {
			case beatChan <- msg:
			default:
				log.Println("[WARN] beat channel full, dropping heartbeat")
			}
		}
	}
}

func balancePoller(ctx context.Context, client *okx.Client, equityChan chan<- float64, intervalSec int) {
	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	log.Printf("[INFO] Balance poller every %ds", intervalSec)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			eq, err := client.GetBalance(ctx)
			if err != nil {
				log.Printf("[WARN] Balance poll error: %v", err)
				continue
			}
			log.Printf("[INFO] Balance: %.2f", eq)
			select {
			case equityChan <- eq:
			default:
				log.Println("[WARN] equity channel full, dropping value")
			}
		}
	}
}

func runCoordinator(
	ctx context.Context,
	cfg *config.Config,
	beatChan <-chan monitor.HeartbeatMessage,
	equityChan <-chan float64,
	haltFn func(string),
) {
	state := monitor.StateInitializing
	tracker := monitor.HeartbeatTracker{}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[INFO] Coordinator shutting down")
			return

		case beat := <-beatChan:
			if state == monitor.StateHalted {
				continue
			}
			now := time.Now()
			switch beat.Sender {
			case "gateway-go":
				tracker.LastGatewayBeat = now
			case "engine-net":
				tracker.LastEngineBeat = now
			}
			if state == monitor.StateInitializing && tracker.BothReceived() {
				state = monitor.StateHealthy
				tracker.LastGatewayBeat = now
				tracker.LastEngineBeat = now
				log.Println("[INFO] Watchdog ARMED — all services online")
			}

		case equity := <-equityChan:
			if state == monitor.StateHalted {
				continue
			}
			if state == monitor.StateHealthy {
				floorLimit := cfg.StartingCapital * cfg.EquityFloorFraction
				if equity < floorLimit {
					log.Printf("[EMERGENCY] Equity floor breached: %.2f < %.2f", equity, floorLimit)
					state = monitor.StateHalting
					haltFn("equity_floor_breach")
					state = monitor.StateHalted
				}
			}

		case <-ticker.C:
			if state != monitor.StateHealthy {
				continue
			}
			now := time.Now()
			missedLimit := time.Duration(cfg.AllowedMissedSeconds) * time.Second

			if now.Sub(tracker.LastGatewayBeat) > missedLimit {
				log.Printf("[EMERGENCY] Heartbeat lost for gateway-go. Last: %v", tracker.LastGatewayBeat)
				state = monitor.StateHalting
				haltFn("heartbeat_lost_gateway")
				state = monitor.StateHalted
			} else if now.Sub(tracker.LastEngineBeat) > missedLimit {
				log.Printf("[EMERGENCY] Heartbeat lost for engine-net. Last: %v", tracker.LastEngineBeat)
				state = monitor.StateHalting
				haltFn("heartbeat_lost_engine")
				state = monitor.StateHalted
			}
		}
	}
}
