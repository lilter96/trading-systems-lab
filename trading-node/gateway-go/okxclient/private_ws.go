package okxclient

import (
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/gobwas/ws/wsutil"
)

func (c *PrivateClient) readLoop(conn net.Conn, done chan struct{}) {
	defer func() {
		c.mu.Lock()
		if c.conn == conn {
			c.isConnected = false
			c.conn = nil
		}
		c.mu.Unlock()
		conn.Close()
		close(done)
	}()

	for {
		msgBytes, err := wsutil.ReadServerText(conn)
		if err != nil {
			log.Printf("okxclient private: read error: %v", err)
			return
		}

		c.handleMessage(msgBytes)
	}
}

func (c *PrivateClient) handleMessage(msg []byte) {
	if len(msg) == 0 {
		return
	}

	if string(msg) == "pong" {
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(msg, &raw); err != nil {
		return
	}

	if event, ok := raw["event"]; ok {
		eventStr := string(event)
		eventStr = eventStr[1 : len(eventStr)-1]

		switch eventStr {
		case "login":
			var code string
			if cRaw, ok := raw["code"]; ok {
				code = string(cRaw)
				code = code[1 : len(code)-1]
			}
			if code == "0" || code == "" {
				c.loginCh <- nil
			} else {
				var msgStr string
				if mRaw, ok := raw["msg"]; ok {
					msgStr = string(mRaw)
				}
				c.loginCh <- fmt.Errorf("login failed: code=%s msg=%s", code, msgStr)
			}

		case "subscribe", "unsubscribe":
			log.Printf("okxclient private: %s", msg)

		case "error":
			var code, msgStr string
			if cRaw, ok := raw["code"]; ok {
				code = string(cRaw)
				code = code[1 : len(code)-1]
			}
			if mRaw, ok := raw["msg"]; ok {
				msgStr = string(mRaw)
			}
			log.Printf("okxclient private: error code=%s msg=%s", code, msgStr)
		}

		return
	}

	if argRaw, ok := raw["arg"]; ok {
		var arg struct {
			Channel string `json:"channel"`
		}
		if err := json.Unmarshal(argRaw, &arg); err != nil {
			return
		}

		dataRaw, ok := raw["data"]
		if !ok {
			return
		}

		var data []json.RawMessage
		if err := json.Unmarshal(dataRaw, &data); err != nil {
			return
		}

		switch arg.Channel {
		case "orders":
			for _, item := range data {
				update := parseOrderUpdate(item)
				select {
				case c.Orders <- update:
				default:
				}
			}

		case "positions":
			for _, item := range data {
				update := parsePositionUpdate(item)
				select {
				case c.Positions <- update:
				default:
				}
			}
		}
	}
}

func parseOrderUpdate(raw json.RawMessage) OrderUpdate {
	var data struct {
		InstID    string `json:"instId"`
		OrdID     string `json:"ordId"`
		ClOrdID   string `json:"clOrdId"`
		State     string `json:"state"`
		Side      string `json:"side"`
		PosSide   string `json:"posSide"`
		FillPx    string `json:"fillPx"`
		FillSz    string `json:"fillSz"`
		AvgPx     string `json:"avgPx"`
		AccFillSz string `json:"accFillSz"`
		Fee       string `json:"fee"`
		FeeCcy    string `json:"feeCcy"`
		TradeID   string `json:"tradeId"`
		Px        string `json:"px"`
		Sz        string `json:"sz"`
		OrdType   string `json:"ordType"`
		Category  string `json:"category"`
		UTime     string `json:"uTime"`
		CTime     string `json:"cTime"`
	}
	json.Unmarshal(raw, &data)

	return OrderUpdate{
		InstID:    data.InstID,
		OrdID:     data.OrdID,
		ClOrdID:   data.ClOrdID,
		State:     data.State,
		Side:      data.Side,
		PosSide:   data.PosSide,
		FillPx:    parseFloatStr(data.FillPx),
		FillSz:    parseFloatStr(data.FillSz),
		AvgPx:     parseFloatStr(data.AvgPx),
		AccFillSz: parseFloatStr(data.AccFillSz),
		Fee:       parseFloatStr(data.Fee),
		FeeCcy:    data.FeeCcy,
		TradeID:   data.TradeID,
		Px:        parseFloatStr(data.Px),
		Sz:        parseFloatStr(data.Sz),
		OrdType:   data.OrdType,
		Category:  data.Category,
		UTime:     parseInt64Str(data.UTime),
		CTime:     parseInt64Str(data.CTime),
	}
}

func parsePositionUpdate(raw json.RawMessage) PositionUpdate {
	var data struct {
		InstID   string `json:"instId"`
		InstType string `json:"instType"`
		PosSide  string `json:"posSide"`
		Pos      string `json:"pos"`
		AvgPx    string `json:"avgPx"`
		Upl      string `json:"upl"`
		UplRatio string `json:"uplRatio"`
		Lever    string `json:"lever"`
		MgnMode  string `json:"mgnMode"`
		Ccy      string `json:"ccy"`
		PosID    string `json:"posId"`
		TradeID  string `json:"tradeId"`
		UTime    string `json:"uTime"`
	}
	json.Unmarshal(raw, &data)

	return PositionUpdate{
		InstID:    data.InstID,
		InstType:  data.InstType,
		PosSide:   data.PosSide,
		Pos:       parseFloatStr(data.Pos),
		AvgPx:     parseFloatStr(data.AvgPx),
		Upl:       parseFloatStr(data.Upl),
		UplRatio:  parseFloatStr(data.UplRatio),
		Lever:     data.Lever,
		MgnMode:   data.MgnMode,
		Ccy:       data.Ccy,
		PosID:     data.PosID,
		TradeID:   data.TradeID,
		UTime:     parseInt64Str(data.UTime),
	}
}

func parseFloatStr(s string) float64 {
	if s == "" {
		return 0
	}
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

func parseInt64Str(s string) int64 {
	if s == "" {
		return 0
	}
	var v int64
	fmt.Sscanf(s, "%d", &v)
	return v
}
