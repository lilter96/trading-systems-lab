package okxclient

import (
	"bytes"
	"fmt"
	"strconv"
	"unsafe"
)

func parseTickZeroAlloc(channel, instID, raw []byte, receipt int64) (Tick, error) {
	var t Tick

	if len(channel) > len(t.Channel) {
		return t, fmt.Errorf("channel length %d exceeds static limit %d", len(channel), len(t.Channel))
	}
	if len(instID) > len(t.InstID) {
		return t, fmt.Errorf("instrument ID length %d exceeds static limit %d", len(instID), len(t.InstID))
	}

	copy(t.Channel[:], channel)
	copy(t.InstID[:], instID)

	if bytes.Equal(channel, []byte("trades")) {
		pxBytes := findJSONStringValue(raw, "px")
		szBytes := findJSONStringValue(raw, "sz")
		sideBytes := findJSONStringValue(raw, "side")
		tsBytes := findJSONStringValue(raw, "ts")

		if len(pxBytes) == 0 || len(szBytes) == 0 || len(tsBytes) == 0 {
			return t, fmt.Errorf("missing trade fields")
		}

		price, err := strconv.ParseFloat(unsafeString(pxBytes), 64)
		if err != nil {
			return t, err
		}
		qty, err := strconv.ParseFloat(unsafeString(szBytes), 64)
		if err != nil {
			return t, err
		}
		ts, err := strconv.ParseInt(unsafeString(tsBytes), 10, 64)
		if err != nil {
			return t, err
		}

		t.Price = price
		t.Quantity = qty
		if len(sideBytes) > 0 {
			t.Side = sideBytes[0]
		}
		t.ExchangeTime = ts
		t.ReceiptTime = receipt
		return t, nil

	} else if bytes.Equal(channel, []byte("tickers")) {
		lastBytes := findJSONStringValue(raw, "last")
		tsBytes := findJSONStringValue(raw, "ts")

		if len(lastBytes) == 0 || len(tsBytes) == 0 {
			return t, fmt.Errorf("missing ticker fields")
		}

		price, err := strconv.ParseFloat(unsafeString(lastBytes), 64)
		if err != nil {
			return t, err
		}
		ts, err := strconv.ParseInt(unsafeString(tsBytes), 10, 64)
		if err != nil {
			return t, err
		}

		t.Price = price
		t.ExchangeTime = ts
		t.ReceiptTime = receipt
		return t, nil
	}

	return t, fmt.Errorf("unrecognized channel")
}

func unsafeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}

func findJSONStringValue(data []byte, key string) []byte {
	keyBytes := []byte(`"` + key + `"`)
	pos := 0
	for {
		idx := bytes.Index(data[pos:], keyBytes)
		if idx == -1 {
			return nil
		}
		actualPos := pos + idx

		afterKey := data[actualPos+len(keyBytes):]
		afterKeyTrimmed := bytes.TrimSpace(afterKey)
		if len(afterKeyTrimmed) > 0 && afterKeyTrimmed[0] == ':' {
			sub := afterKeyTrimmed[1:]
			valStart := bytes.TrimSpace(sub)
			if len(valStart) == 0 {
				return nil
			}
			if valStart[0] == '"' {
				valStart = valStart[1:]
				end := bytes.IndexByte(valStart, '"')
				if end == -1 {
					return nil
				}
				return valStart[:end]
			} else {
				end := bytes.IndexAny(valStart, ", \t\r\n}")
				if end == -1 {
					return valStart
				}
				return valStart[:end]
			}
		}
		pos = actualPos + 1
	}
}
