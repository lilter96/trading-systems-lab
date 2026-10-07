package okxclient

import (
	"context"
	"log"
	"time"
)

type clientState struct {
	connected bool
	closed    bool
}

type reconnectConfig struct {
	Name        string
	GetState    func() clientState
	Connect     func(ctx context.Context) error
	Close       func()
	OnReconnect func()
}

func runReconnectLoop(ctx context.Context, cfg reconnectConfig) {
	const (
		minBackoff = 1 * time.Second
		maxBackoff = 64 * time.Second
	)
	backoff := minBackoff

	for {
		select {
		case <-ctx.Done():
			log.Printf("%s: supervisor stopped (context canceled)", cfg.Name)
			cfg.Close()
			return
		default:
		}

		st := cfg.GetState()

		if st.closed {
			log.Printf("%s: supervisor stopped (client closed intentionally)", cfg.Name)
			return
		}

		if st.connected {
			select {
			case <-ctx.Done():
				cfg.Close()
				return
			case <-time.After(1 * time.Second):
			}
			continue
		}

		log.Printf("%s: attempting to (re)connect...", cfg.Name)
		if err := cfg.Connect(ctx); err != nil {
			log.Printf("%s: connect failed: %v — retrying in %v", cfg.Name, err, backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		backoff = minBackoff

		if cfg.OnReconnect != nil {
			cfg.OnReconnect()
		}
	}
}

func (w *WSClient) StartSupervisor(ctx context.Context) {
	go runReconnectLoop(ctx, reconnectConfig{
		Name: "okxclient",
		GetState: func() clientState {
			w.mu.Lock()
			defer w.mu.Unlock()
			return clientState{connected: w.isConnected, closed: w.isClosed}
		},
		Connect:     w.Connect,
		Close:       w.Close,
		OnReconnect: w.restoreSubscriptions,
	})
}
