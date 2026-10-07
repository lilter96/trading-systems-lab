package okxclient

import (
	"context"
)

func (c *PrivateClient) StartSupervisor(ctx context.Context) {
	go runReconnectLoop(ctx, reconnectConfig{
		Name: "okxclient private",
		GetState: func() clientState {
			c.mu.Lock()
			defer c.mu.Unlock()
			return clientState{connected: c.isConnected, closed: c.isClosed}
		},
		Connect: c.Connect,
		Close:   c.Close,
	})
}
