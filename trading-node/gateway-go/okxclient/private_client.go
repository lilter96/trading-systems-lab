package okxclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type PrivateClient struct {
	creds  Credentials
	wsAddr string
	isDemo bool

	mu          sync.Mutex
	conn        net.Conn
	isConnected bool
	isClosed    bool
	done        chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc

	loginCh chan error

	Orders    chan OrderUpdate
	Positions chan PositionUpdate

	subscriptions []PrivateSub

	Rest *RestClient
}

func NewPrivateClient(creds Credentials, isDemo bool) *PrivateClient {
	wsAddr := "wss://ws.okx.com:8443/ws/v5/private"
	if isDemo {
		wsAddr = "wss://wspap.okx.com:8443/ws/v5/private"
	}

	return &PrivateClient{
		creds:  creds,
		wsAddr: wsAddr,
		isDemo: isDemo,
		Rest:   NewRestClient(creds, isDemo),

		Orders:    make(chan OrderUpdate, 1024),
		Positions: make(chan PositionUpdate, 1024),

		done: make(chan struct{}),
	}
}

func (c *PrivateClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.isConnected {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	if c.ctx == nil {
		c.ctx, c.cancel = context.WithCancel(context.Background())
	}

	dialer := ws.Dialer{}
	conn, _, _, err := dialer.Dial(ctx, c.wsAddr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", c.wsAddr, err)
	}

	c.mu.Lock()
	c.conn = conn
	c.isConnected = true
	c.isClosed = false
	done := make(chan struct{})
	c.done = done
	c.mu.Unlock()

	c.loginCh = make(chan error, 1)

	go c.readLoop(conn, done)

	if err := c.login(); err != nil {
		c.mu.Lock()
		c.isConnected = false
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
		conn.Close()
		close(done)
		return fmt.Errorf("login: %w", err)
	}

	for _, sub := range c.subscriptions {
		if err := c.sendSubscribe(sub); err != nil {
			log.Printf("okxclient private: subscribe %s/%s/%s failed: %v", sub.Channel, sub.InstType, sub.InstID, err)
		}
	}

	log.Printf("okxclient private: connected to %s", c.wsAddr)
	return nil
}

func (c *PrivateClient) login() error {
	ts, sig := c.creds.WsLoginSign()

	loginReq := map[string]interface{}{
		"op": "login",
		"args": []map[string]string{
			{
				"apiKey":     c.creds.APIKey,
				"passphrase": c.creds.Passphrase,
				"timestamp":  ts,
				"sign":       sig,
			},
		},
	}

	payload, err := json.Marshal(loginReq)
	if err != nil {
		return fmt.Errorf("marshal login: %w", err)
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	if err := wsutil.WriteClientMessage(conn, ws.OpText, payload); err != nil {
		return fmt.Errorf("send login: %w", err)
	}

	select {
	case err := <-c.loginCh:
		return err
	case <-time.After(10 * time.Second):
		return fmt.Errorf("login timeout")
	}
}

func (c *PrivateClient) Subscribe(channel, instType, instID string) error {
	sub := PrivateSub{Channel: channel, InstType: instType, InstID: instID}

	c.mu.Lock()
	c.subscriptions = append(c.subscriptions, sub)
	connected := c.isConnected
	c.mu.Unlock()

	if !connected {
		log.Printf("okxclient private: queued subscription %s/%s/%s", channel, instType, instID)
		return nil
	}

	return c.sendSubscribe(sub)
}

func (c *PrivateClient) sendSubscribe(sub PrivateSub) error {
	args := map[string]string{"channel": sub.Channel}
	if sub.InstType != "" {
		args["instType"] = sub.InstType
	}
	if sub.InstID != "" {
		args["instId"] = sub.InstID
	}

	req := map[string]interface{}{
		"op":   "subscribe",
		"args": []map[string]string{args},
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal subscribe: %w", err)
	}

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	return wsutil.WriteClientMessage(conn, ws.OpText, payload)
}

func (c *PrivateClient) Disconnect() {
	c.mu.Lock()
	if !c.isConnected {
		c.mu.Unlock()
		return
	}
	c.isConnected = false
	conn := c.conn
	done := c.done
	c.conn = nil
	c.mu.Unlock()

	if conn != nil {
		conn.Close()
	}
	select {
	case <-done:
	default:
		close(done)
	}

	log.Println("okxclient private: disconnected")
}

func (c *PrivateClient) Close() {
	c.mu.Lock()
	c.isClosed = true
	c.mu.Unlock()
	c.Disconnect()
}
