package okxclient

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"github.com/gobwas/ws"
)

type WSClient struct {
	Addr string

	mu           sync.Mutex
	conn         net.Conn
	isConnected  bool
	isConnecting bool
	isClosed     bool
	done         chan struct{}

	registry     map[string]Subscription
	routingTable atomic.Value
	droppedTicks uint64
}

func NewOKXClient(addr string) *WSClient {
	client := &WSClient{
		Addr:     addr,
		registry: make(map[string]Subscription),
		done:     make(chan struct{}),
	}
	client.routingTable.Store(make(map[RouteKey]*SPSCTickQueue))
	return client
}

func (w *WSClient) GetDroppedTicks() uint64 {
	return atomic.LoadUint64(&w.droppedTicks)
}

func (w *WSClient) Connect(ctx context.Context) error {
	w.mu.Lock()
	if w.isConnected || w.isConnecting {
		w.mu.Unlock()
		return nil
	}
	w.isConnecting = true
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.isConnecting = false
		w.mu.Unlock()
	}()

	dialer := ws.Dialer{}
	conn, _, _, err := dialer.Dial(ctx, w.Addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", w.Addr, err)
	}

	done := make(chan struct{})
	activity := make(chan struct{}, 1)

	w.mu.Lock()
	w.conn = conn
	w.isConnected = true
	w.isClosed = false
	w.done = done
	w.mu.Unlock()

	log.Printf("okxclient: connected to %s", w.Addr)

	go w.keepalive(conn, done, activity)
	go w.readLoop(conn, done, activity)

	return nil
}

func (w *WSClient) Disconnect() {
	w.mu.Lock()
	if !w.isConnected {
		w.mu.Unlock()
		return
	}
	w.isConnected = false
	conn := w.conn
	done := w.done
	w.conn = nil
	w.rebuildRoutingTable()
	w.mu.Unlock()

	if conn != nil {
		conn.Close()
	}
	select {
	case <-done:
	default:
		close(done)
	}

	log.Println("okxclient: disconnected — waiting for supervisor to reconnect")
}

func (w *WSClient) Close() {
	w.mu.Lock()
	w.isClosed = true
	w.mu.Unlock()
	w.Disconnect()
}
