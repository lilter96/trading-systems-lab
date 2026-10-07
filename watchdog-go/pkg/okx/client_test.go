package okx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSigning_ProducesValidSignature(t *testing.T) {
	c := New("test-key", "test-secret", "test-pass", false)

	method := "GET"
	path := "/api/v5/account/balance"
	body := ""

	timestamp, signature := c.sign(method, path, body)

	if timestamp == "" {
		t.Fatal("timestamp should not be empty")
	}
	if signature == "" {
		t.Fatal("signature should not be empty")
	}

	signPayload := timestamp + method + path + body
	mac := hmac.New(sha256.New, []byte("test-secret"))
	mac.Write([]byte(signPayload))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if signature != expected {
		t.Fatalf("signature mismatch:\n  got:  %s\n  want: %s", signature, expected)
	}
}

func TestSigning_DifferentBodiesProduceDifferentSignatures(t *testing.T) {
	c := New("key", "secret", "pass", false)

	_, sig1 := c.sign("POST", "/api/v5/trade/close-position", `{"instId":"BTC-USDT-SWAP"}`)
	time.Sleep(2 * time.Millisecond)
	_, sig2 := c.sign("POST", "/api/v5/trade/close-position", `{"instId":"ETH-USDT-SWAP"}`)

	if sig1 == sig2 {
		t.Fatal("different bodies should produce different signatures")
	}
}

func TestGetBalance_ParsesTotalEq(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/account/balance" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("OK-ACCESS-KEY") != "test-key" {
			t.Fatal("missing or wrong API key header")
		}
		if r.Header.Get("OK-ACCESS-SIGN") == "" {
			t.Fatal("missing signature header")
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":"0","msg":"","data":[{"totalEq":"12345.67"}]}`)
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "test-key",
		apiSecret:  "test-secret",
		passphrase: "test-pass",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}

	eq, err := c.GetBalance(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eq != 12345.67 {
		t.Fatalf("expected 12345.67, got %f", eq)
	}
}

func TestGetBalance_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":"50001","msg":"Invalid API Key","data":[]}`)
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "bad-key",
		apiSecret:  "bad-secret",
		passphrase: "bad-pass",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}

	_, err := c.GetBalance(context.Background())
	if err == nil {
		t.Fatal("expected error for bad API key")
	}
	if !strings.Contains(err.Error(), "50001") {
		t.Fatalf("expected error code in message, got: %v", err)
	}
}

func TestCancelAllOrders_SendsCorrectPayload(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v5/trade/cancel-all-orders" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		buf := make([]byte, 256)
		n, _ := r.Body.Read(buf)
		gotBody = strings.TrimSpace(string(buf[:n]))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":"0","msg":""}`)
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "k",
		apiSecret:  "s",
		passphrase: "p",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}

	if err := c.CancelAllOrders(context.Background(), "SWAP"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotBody, `"instType"`) || !strings.Contains(gotBody, "SWAP") {
		t.Fatalf("payload missing instType=SWAP: %s", gotBody)
	}
}

func TestEmergencyHalt_FullSequence(t *testing.T) {
	var mu sync.Mutex
	var callLog []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case path == "/api/v5/trade/cancel-all-orders" && r.Method == "POST":
			buf := make([]byte, 256)
			n, _ := r.Body.Read(buf)
			mu.Lock()
			callLog = append(callLog, "cancel:"+strings.TrimSpace(string(buf[:n])))
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":""}`)

		case strings.HasPrefix(path, "/api/v5/trade/orders-algo-pending") && r.Method == "GET":
			instType := r.URL.Query().Get("instType")
			mu.Lock()
			callLog = append(callLog, "pending_algos:"+instType)
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":"","data":[]}`)

		case path == "/api/v5/trade/cancel-algos" && r.Method == "POST":
			buf := make([]byte, 1024)
			n, _ := r.Body.Read(buf)
			mu.Lock()
			callLog = append(callLog, "cancel_algos:"+strings.TrimSpace(string(buf[:n])))
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":""}`)

		case path == "/api/v5/account/positions" && r.Method == "GET":
			mu.Lock()
			callLog = append(callLog, "get_positions")
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","mgnMode":"cross","pos":"1","posSide":"long"},{"instId":"ETH-USDT-SWAP","mgnMode":"cross","pos":"-2","posSide":"short"}]}`)

		case path == "/api/v5/trade/close-position" && r.Method == "POST":
			buf := make([]byte, 256)
			n, _ := r.Body.Read(buf)
			mu.Lock()
			callLog = append(callLog, "close:"+strings.TrimSpace(string(buf[:n])))
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":""}`)

		default:
			t.Fatalf("unexpected call: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "k",
		apiSecret:  "s",
		passphrase: "p",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}

	if err := c.EmergencyHalt(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedCalls := []string{
		`cancel:{"instType":"SWAP"}`,
		`cancel:{"instType":"FUTURES"}`,
		`cancel:{"instType":"MARGIN"}`,
		"pending_algos:SWAP",
		"pending_algos:FUTURES",
		"pending_algos:MARGIN",
		"get_positions",
		`close:{"instId":"BTC-USDT-SWAP","mgnMode":"cross","posSide":"long"}`,
		`close:{"instId":"ETH-USDT-SWAP","mgnMode":"cross","posSide":"short"}`,
	}

	if len(callLog) != len(expectedCalls) {
		t.Fatalf("expected %d calls, got %d: %v", len(expectedCalls), len(callLog), callLog)
	}
	// Verify expected entries (order-independent due to concurrent calls)
	remaining := make([]string, len(expectedCalls))
	copy(remaining, expectedCalls)
	for _, got := range callLog {
		found := false
		for i, exp := range remaining {
			if exp == got {
				remaining = append(remaining[:i], remaining[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("unexpected call: %q (remaining: %v)", got, remaining)
		}
	}
}

func TestEmergencyHalt_EmptyPositions(t *testing.T) {
	var mu sync.Mutex
	var callLog []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "cancel-all-orders"):
			mu.Lock()
			callLog = append(callLog, "cancel")
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":""}`)
		case strings.HasPrefix(path, "/api/v5/trade/orders-algo-pending"):
			mu.Lock()
			callLog = append(callLog, "algos")
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":"","data":[]}`)
		case strings.HasSuffix(path, "positions"):
			mu.Lock()
			callLog = append(callLog, "positions")
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":"","data":[]}`)
		default:
			t.Fatalf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "k",
		apiSecret:  "s",
		passphrase: "p",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}

	if err := c.EmergencyHalt(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedCount := 7 // 3 cancel + 3 algo pending + 1 positions
	if len(callLog) != expectedCount {
		t.Fatalf("expected %d calls, got %d: %v", expectedCount, len(callLog), callLog)
	}
}

func TestDemoMode_SetsSimulatedHeader(t *testing.T) {
	var gotSimHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSimHeader = r.Header.Get("x-simulated-trading")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":"0","msg":"","data":[{"totalEq":"10000"}]}`)
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "k",
		apiSecret:  "s",
		passphrase: "p",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
		demo:       true,
	}

	c.GetBalance(context.Background())

	if gotSimHeader != "1" {
		t.Fatalf("expected x-simulated-trading:1 header, got: %s", gotSimHeader)
	}
}

func TestOkxCab_DomainUsedForDemo(t *testing.T) {
	c := New("k", "s", "p", true)
	if !strings.Contains(c.baseURL, "okx.cab") {
		t.Fatalf("expected demo base URL with okx.cab, got %s", c.baseURL)
	}
}

func TestOkxCom_DomainUsedForLive(t *testing.T) {
	c := New("k", "s", "p", false)
	if !strings.Contains(c.baseURL, "okx.com") || strings.Contains(c.baseURL, "okx.cab") {
		t.Fatalf("expected live base URL with okx.com, got %s", c.baseURL)
	}
}

func TestClosePosition_SendsCorrectPayload(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v5/trade/close-position" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		buf := make([]byte, 256)
		n, _ := r.Body.Read(buf)
		gotBody = strings.TrimSpace(string(buf[:n]))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":"0","msg":""}`)
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "k",
		apiSecret:  "s",
		passphrase: "p",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}

	if err := c.ClosePosition(context.Background(), "BTC-USDT-SWAP", "cross", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal([]byte(gotBody), &parsed); err != nil {
		t.Fatalf("body parse: %v", err)
	}
	if parsed["instId"] != "BTC-USDT-SWAP" {
		t.Fatalf("expected instId BTC-USDT-SWAP, got %s", parsed["instId"])
	}
	if parsed["mgnMode"] != "cross" {
		t.Fatalf("expected mgnMode cross, got %s", parsed["mgnMode"])
	}
	if _, exists := parsed["posSide"]; exists {
		t.Fatal("posSide should be omitted for net mode (empty string)")
	}
}

func TestClosePosition_WithPosSide(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 256)
		n, _ := r.Body.Read(buf)
		gotBody = strings.TrimSpace(string(buf[:n]))
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":"0","msg":""}`)
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "k",
		apiSecret:  "s",
		passphrase: "p",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}

	if err := c.ClosePosition(context.Background(), "BTC-USDT-SWAP", "cross", "long"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal([]byte(gotBody), &parsed); err != nil {
		t.Fatalf("body parse: %v", err)
	}
	if parsed["posSide"] != "long" {
		t.Fatalf("expected posSide long, got %s", parsed["posSide"])
	}
}

func TestCancelAllAlgoOrders_Empty(t *testing.T) {
	var mu sync.Mutex
	var callLog []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callLog = append(callLog, r.URL.Path+"?"+r.URL.RawQuery)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":"0","msg":"","data":[]}`)
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "k",
		apiSecret:  "s",
		passphrase: "p",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}

	if err := c.CancelAllAlgoOrders(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mu.Lock()
	callCount := len(callLog)
	mu.Unlock()
	if callCount != 3 {
		t.Fatalf("expected 3 pending algo calls, got %d", callCount)
	}
}

func TestCancelAllAlgoOrders_BatchesAtTen(t *testing.T) {
	var mu sync.Mutex
	var fetchCount, cancelBatchCount int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v5/trade/orders-algo-pending"):
			mu.Lock()
			fetchCount++
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":"","data":[
				{"algoId":"1","instId":"A"},
				{"algoId":"2","instId":"B"},
				{"algoId":"3","instId":"C"},
				{"algoId":"4","instId":"D"},
				{"algoId":"5","instId":"E"},
				{"algoId":"6","instId":"F"},
				{"algoId":"7","instId":"G"},
				{"algoId":"8","instId":"H"},
				{"algoId":"9","instId":"I"},
				{"algoId":"10","instId":"J"},
				{"algoId":"11","instId":"K"},
				{"algoId":"12","instId":"L"}
			]}`)
		case strings.HasSuffix(r.URL.Path, "cancel-algos"):
			buf := make([]byte, 1024)
			n, _ := r.Body.Read(buf)
			var orders []AlgoOrder
			json.Unmarshal(buf[:n], &orders)
			mu.Lock()
			cancelBatchCount++
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":""}`)
		default:
			t.Fatalf("unexpected: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "k",
		apiSecret:  "s",
		passphrase: "p",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}

	if err := c.CancelAllAlgoOrders(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fetchCount != 3 {
		t.Fatalf("expected 3 algo fetches, got %d", fetchCount)
	}
	if cancelBatchCount != 6 {
		t.Fatalf("expected 6 cancel batches (2 per instType), got %d", cancelBatchCount)
	}
}

func TestFlattenAllPositions_PosSideInClose(t *testing.T) {
	var mu sync.Mutex
	var callLog []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "positions"):
			mu.Lock()
			callLog = append(callLog, "positions")
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":"","data":[
				{"instId":"BTC-USDT-SWAP","mgnMode":"cross","pos":"1","posSide":"long"},
				{"instId":"ETH-USDT-SWAP","mgnMode":"cross","pos":"-2","posSide":"short"}
			]}`)
		case strings.HasSuffix(r.URL.Path, "close-position"):
			buf := make([]byte, 256)
			n, _ := r.Body.Read(buf)
			mu.Lock()
			callLog = append(callLog, "close:"+strings.TrimSpace(string(buf[:n])))
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":""}`)
		default:
			t.Fatalf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "k",
		apiSecret:  "s",
		passphrase: "p",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}

	if err := c.FlattenAllPositions(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(callLog) != 3 {
		t.Fatalf("expected 3 calls (positions + 2 close), got %d: %v", len(callLog), callLog)
	}
	// Close-position calls are concurrent — verify both payloads regardless of order
	var sawLong, sawShort bool
	for _, entry := range callLog {
		if strings.Contains(entry, `"posSide":"long"`) {
			sawLong = true
		}
		if strings.Contains(entry, `"posSide":"short"`) {
			sawShort = true
		}
	}
	if !sawLong {
		t.Fatalf("expected long posSide in close calls: %v", callLog)
	}
	if !sawShort {
		t.Fatalf("expected short posSide in close calls: %v", callLog)
	}
}

func TestFlattenAllPositions_DedupByInstIdAndPosSide(t *testing.T) {
	var callLog []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "positions"):
			callLog = append(callLog, "positions")
			// Duplicate position entries for the same instId+posSide
			fmt.Fprint(w, `{"code":"0","msg":"","data":[
				{"instId":"BTC-USDT-SWAP","mgnMode":"cross","pos":"1","posSide":"long"},
				{"instId":"BTC-USDT-SWAP","mgnMode":"cross","pos":"2","posSide":"long"},
				{"instId":"ETH-USDT-SWAP","mgnMode":"cross","pos":"-1","posSide":"short"}
			]}`)
		case strings.HasSuffix(r.URL.Path, "close-position"):
			buf := make([]byte, 256)
			n, _ := r.Body.Read(buf)
			callLog = append(callLog, "close:"+strings.TrimSpace(string(buf[:n])))
			fmt.Fprint(w, `{"code":"0","msg":""}`)
		default:
			t.Fatalf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "k",
		apiSecret:  "s",
		passphrase: "p",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}

	if err := c.FlattenAllPositions(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1 positions call + 2 close calls (BTC dedup'd, ETH separate)
	if len(callLog) != 3 {
		t.Fatalf("expected 3 calls (dedup), got %d: %v", len(callLog), callLog)
	}
}

func TestContextCancellation_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":"0","msg":"","data":[{"totalEq":"10000"}]}`)
	}))
	defer srv.Close()

	c := &Client{
		apiKey:     "k",
		apiSecret:  "s",
		passphrase: "p",
		baseURL:    srv.URL,
		httpClient: srv.Client(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetBalance(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestEmergencyHalt_CancelsAlgoOrdersAndClosesPositions(t *testing.T) {
	var mu sync.Mutex
	var cancelAllInstTypes []string
	var cancelAlgoPayload []AlgoOrder
	var closePayload map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v5/trade/cancel-all-orders" && r.Method == "POST":
			var payload struct {
				InstType string `json:"instType"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode cancel-all payload: %v", err)
			}
			mu.Lock()
			cancelAllInstTypes = append(cancelAllInstTypes, payload.InstType)
			mu.Unlock()
			fmt.Fprint(w, `{"code":"0","msg":""}`)

		case r.URL.Path == "/api/v5/trade/orders-algo-pending" && r.Method == "GET":
			if r.URL.Query().Get("instType") == "SWAP" {
				fmt.Fprint(w, `{"code":"0","msg":"","data":[{"algoId":"1836487817828872192","instId":"BTC-USDT-SWAP"},{"algoId":"1836487817828872195","instId":"ETH-USDT-SWAP"}]}`)
				return
			}
			fmt.Fprint(w, `{"code":"0","msg":"","data":[]}`)

		case r.URL.Path == "/api/v5/trade/cancel-algos" && r.Method == "POST":
			if err := json.NewDecoder(r.Body).Decode(&cancelAlgoPayload); err != nil {
				t.Fatalf("decode cancel-algos payload: %v", err)
			}
			fmt.Fprint(w, `{"code":"0","msg":""}`)

		case r.URL.Path == "/api/v5/account/positions" && r.Method == "GET":
			fmt.Fprint(w, `{"code":"0","msg":"","data":[{"instId":"BTC-USDT-SWAP","mgnMode":"cross","pos":"0.5","posSide":"long"}]}`)

		case r.URL.Path == "/api/v5/trade/close-position" && r.Method == "POST":
			if err := json.NewDecoder(r.Body).Decode(&closePayload); err != nil {
				t.Fatalf("decode close-position payload: %v", err)
			}
			fmt.Fprint(w, `{"code":"0","msg":""}`)

		default:
			t.Fatalf("unexpected call: %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	c := New("k", "s", "p", false)
	c.SetTestBaseURL(srv.URL, srv.Client())

	if err := c.EmergencyHalt(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedInstTypes := []string{"SWAP", "FUTURES", "MARGIN"}
	if len(cancelAllInstTypes) != len(expectedInstTypes) {
		t.Fatalf("cancel-all instTypes = %v, want %v", cancelAllInstTypes, expectedInstTypes)
	}
	for _, et := range expectedInstTypes {
		found := false
		for _, gt := range cancelAllInstTypes {
			if gt == et {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("cancel-all missing %s in %v", et, cancelAllInstTypes)
		}
	}
	if len(cancelAlgoPayload) != 2 || cancelAlgoPayload[0].AlgoId != "1836487817828872192" || cancelAlgoPayload[0].InstId != "BTC-USDT-SWAP" || cancelAlgoPayload[1].AlgoId != "1836487817828872195" || cancelAlgoPayload[1].InstId != "ETH-USDT-SWAP" {
		t.Fatalf("unexpected cancel-algos payload: %+v", cancelAlgoPayload)
	}
	if closePayload["instId"] != "BTC-USDT-SWAP" || closePayload["mgnMode"] != "cross" || closePayload["posSide"] != "long" {
		t.Fatalf("unexpected close-position payload: %+v", closePayload)
	}
}

func TestEmergencyHalt_DoesNotFailFast(t *testing.T) {
	var sawPositions bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v5/trade/cancel-all-orders":
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"code":"500","msg":"cancel failed"}`)
		case r.URL.Path == "/api/v5/trade/orders-algo-pending":
			fmt.Fprint(w, `{"code":"0","msg":"","data":[]}`)
		case r.URL.Path == "/api/v5/account/positions":
			sawPositions = true
			fmt.Fprint(w, `{"code":"0","msg":"","data":[]}`)
		default:
			t.Fatalf("unexpected call: %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	c := New("k", "s", "p", false)
	c.SetTestBaseURL(srv.URL, srv.Client())

	err := c.EmergencyHalt(context.Background())
	if err == nil {
		t.Fatal("expected aggregated cancel-all error")
	}
	if !sawPositions {
		t.Fatal("expected positions fetch despite cancel-all failures")
	}
}
