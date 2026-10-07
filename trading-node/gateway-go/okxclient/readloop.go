package okxclient

import (
	"bytes"
	"encoding/json"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

func (w *WSClient) keepalive(conn net.Conn, done chan struct{}, activity <-chan struct{}) {
	timer := time.NewTimer(25 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-done:
			return

		case <-activity:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(25 * time.Second)

		case <-timer.C:
			if err := wsutil.WriteClientMessage(conn, ws.OpText, []byte("ping")); err != nil {
				log.Printf("okxclient: send ping failed: %v", err)
				w.Disconnect()
				return
			}
			timer.Reset(25 * time.Second)
		}
	}
}

func (w *WSClient) readLoop(conn net.Conn, done chan struct{}, activity chan<- struct{}) {
	defer w.Disconnect()

	for {
		msgBytes, err := wsutil.ReadServerText(conn)
		if err != nil {
			log.Printf("okxclient: read error: %v", err)
			return
		}

		select {
		case activity <- struct{}{}:
		default:
		}

		w.routeMessage(msgBytes)
	}
}

func (w *WSClient) routeMessage(msgBytes []byte) {
	if len(msgBytes) == 0 {
		return
	}
	if len(msgBytes) == 4 && string(msgBytes) == "pong" {
		return
	}

	if bytes.Contains(msgBytes, []byte(`"event"`)) || bytes.Contains(msgBytes, []byte(`"code"`)) {
		w.handleAdministrativeMessage(msgBytes)
		return
	}

	if msgBytes[0] != '{' {
		return
	}

	channelBytes := findJSONStringValue(msgBytes, "channel")
	instIDBytes := findJSONStringValue(msgBytes, "instId")
	if len(channelBytes) == 0 || len(instIDBytes) == 0 {
		return
	}

	if len(channelBytes) > 16 || len(instIDBytes) > 32 {
		log.Printf("okxclient: channel or instId exceeds fixed boundaries (channel=%d/16, instId=%d/32)", len(channelBytes), len(instIDBytes))
		return
	}

	var key RouteKey
	copy(key.Channel[:], channelBytes)
	copy(key.InstID[:], instIDBytes)

	routingMap, _ := w.routingTable.Load().(map[RouteKey]*SPSCTickQueue)
	queue, exists := routingMap[key]
	if !exists {
		return
	}

	receipt := time.Now().UnixNano()

	dataIdx := bytes.Index(msgBytes, []byte(`"data":[`))
	if dataIdx == -1 {
		return
	}

	dataArray := msgBytes[dataIdx+len(`"data":[`):]
	for len(dataArray) > 0 {
		start := bytes.IndexByte(dataArray, '{')
		if start == -1 {
			break
		}
		dataArray = dataArray[start:]
		depth := 0
		end := -1
		for i, b := range dataArray {
			if b == '{' {
				depth++
			} else if b == '}' {
				depth--
				if depth == 0 {
					end = i
					break
				}
			}
		}
		if end == -1 {
			break
		}
		rawObj := dataArray[:end+1]
		dataArray = dataArray[end+1:]

		tick, err := parseTickZeroAlloc(channelBytes, instIDBytes, rawObj, receipt)
		if err != nil {
			log.Printf("okxclient: parseTick error: %v", err)
			continue
		}

		if !queue.Push(&tick) {
			atomic.AddUint64(&w.droppedTicks, 1)
		}
	}
}

func (w *WSClient) handleAdministrativeMessage(msgBytes []byte) {
	var admin struct {
		Event string `json:"event"`
		Code  string `json:"code"`
		Msg   string `json:"msg"`
	}
	if err := json.Unmarshal(msgBytes, &admin); err == nil {
		log.Printf("okxclient [admin]: event=%s, code=%s, msg=%s", admin.Event, admin.Code, admin.Msg)
	}
}
