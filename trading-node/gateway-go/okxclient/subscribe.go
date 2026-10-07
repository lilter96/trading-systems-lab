package okxclient

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type SubscribeReq struct {
	Op   string `json:"op"`
	Args []Arg  `json:"args"`
}

type Arg struct {
	Channel string `json:"channel"`
	InstID  string `json:"instId"`
}

type Subscription struct {
	Channel string
	InstID  string
	Queue   *SPSCTickQueue
}

func subKey(channel, instID string) string {
	return channel + ":" + instID
}

func (w *WSClient) Subscribe(channel, instID string, queue *SPSCTickQueue) error {
	keyStr := subKey(channel, instID)

	w.mu.Lock()
	if _, exists := w.registry[keyStr]; exists {
		w.mu.Unlock()
		return fmt.Errorf("already subscribed to %s", keyStr)
	}
	w.registry[keyStr] = Subscription{Channel: channel, InstID: instID, Queue: queue}
	w.rebuildRoutingTable()

	if !w.isConnected {
		w.mu.Unlock()
		log.Printf("okxclient: not connected yet — subscription for %s queued", keyStr)
		return nil
	}
	conn := w.conn
	w.mu.Unlock()

	req := SubscribeReq{Op: "subscribe", Args: []Arg{{Channel: channel, InstID: instID}}}
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal subscribe request: %w", err)
	}
	if err := wsutil.WriteClientMessage(conn, ws.OpText, payload); err != nil {
		w.mu.Lock()
		delete(w.registry, keyStr)
		w.rebuildRoutingTable()
		w.mu.Unlock()
		return fmt.Errorf("send subscribe request for %s: %w", keyStr, err)
	}

	return nil
}

func (w *WSClient) restoreSubscriptions() {
	w.mu.Lock()
	conn := w.conn
	subs := make([]Subscription, 0, len(w.registry))
	for _, sub := range w.registry {
		subs = append(subs, sub)
	}
	w.mu.Unlock()

	if conn == nil {
		return
	}

	for _, sub := range subs {
		keyStr := subKey(sub.Channel, sub.InstID)

		req := SubscribeReq{Op: "subscribe", Args: []Arg{{Channel: sub.Channel, InstID: sub.InstID}}}
		payload, err := json.Marshal(req)
		if err != nil {
			log.Printf("okxclient: marshal subscribe request for %s: %v", keyStr, err)
			continue
		}
		if err := wsutil.WriteClientMessage(conn, ws.OpText, payload); err != nil {
			log.Printf("okxclient: send subscribe request for %s: %v", keyStr, err)
			continue
		}

		w.mu.Lock()
		if w.conn == conn {
			w.registry[keyStr] = sub
			w.rebuildRoutingTable()
			log.Printf("okxclient: restored subscription %s", keyStr)
		}
		w.mu.Unlock()
	}
}

func (w *WSClient) rebuildRoutingTable() {
	routingMap := make(map[RouteKey]*SPSCTickQueue, len(w.registry))
	for _, sub := range w.registry {
		var rKey RouteKey
		if len(sub.Channel) > 16 || len(sub.InstID) > 32 {
			log.Printf("okxclient: skipping subscription rebuild due to field limit overflow: %s:%s", sub.Channel, sub.InstID)
			continue
		}
		copy(rKey.Channel[:], sub.Channel)
		copy(rKey.InstID[:], sub.InstID)
		routingMap[rKey] = sub.Queue
	}
	w.routingTable.Store(routingMap)
}
