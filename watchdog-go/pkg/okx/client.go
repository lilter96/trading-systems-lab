package okx

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Client struct {
	apiKey     string
	apiSecret  string
	passphrase string
	baseURL    string
	httpClient *http.Client
	demo       bool
}

type apiCode string

func (c *apiCode) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*c = apiCode(s)
		return nil
	}
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		*c = apiCode(fmt.Sprintf("%.0f", n))
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into apiCode", string(b))
}

type Position struct {
	InstID  string `json:"instId"`
	MgnMode string `json:"mgnMode"`
	Pos     string `json:"pos"`
	PosSide string `json:"posSide"`
}

type AlgoOrder struct {
	AlgoId string `json:"algoId"`
	InstId string `json:"instId"`
}

type balanceResponse struct {
	Code apiCode       `json:"code"`
	Msg  string        `json:"msg"`
	Data []balanceData `json:"data"`
}

type balanceData struct {
	TotalEq string `json:"totalEq"`
}

type cancelAllResponse struct {
	Code apiCode `json:"code"`
	Msg  string  `json:"msg"`
}

type positionsResponse struct {
	Code apiCode    `json:"code"`
	Msg  string     `json:"msg"`
	Data []Position `json:"data"`
}

type closePositionResponse struct {
	Code apiCode `json:"code"`
	Msg  string  `json:"msg"`
}

type algoPendingResponse struct {
	Code apiCode     `json:"code"`
	Msg  string      `json:"msg"`
	Data []AlgoOrder `json:"data"`
}

type cancelAlgoResponse struct {
	Code apiCode `json:"code"`
	Msg  string  `json:"msg"`
}

// SetTestBaseURL overrides the base URL and HTTP client for testing.
func (c *Client) SetTestBaseURL(url string, client *http.Client) {
	c.baseURL = url
	c.httpClient = client
}

func New(apiKey, apiSecret, passphrase string, isDemo bool) *Client {
	baseURL := "https://www.okx.com"
	if isDemo {
		baseURL = "https://www.okx.cab"
	}
	return &Client{
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		passphrase: passphrase,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		demo:       isDemo,
	}
}

func (c *Client) GetBalance(ctx context.Context) (float64, error) {
	body, err := c.do(ctx, "GET", "/api/v5/account/balance", "")
	if err != nil {
		return 0, fmt.Errorf("balance request: %w", err)
	}

	var resp balanceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("balance parse: %w", err)
	}
	if resp.Code != "0" {
		return 0, fmt.Errorf("balance API error code=%s msg=%s", string(resp.Code), resp.Msg)
	}
	if len(resp.Data) == 0 {
		return 0, fmt.Errorf("balance response empty")
	}

	var totalEq float64
	if _, err := fmt.Sscanf(resp.Data[0].TotalEq, "%f", &totalEq); err != nil {
		return 0, fmt.Errorf("totalEq parse: %w", err)
	}
	return totalEq, nil
}

func (c *Client) EmergencyHalt(ctx context.Context) error {
	var errs []error
	var mu sync.Mutex
	var wg sync.WaitGroup

	instTypes := []string{"SWAP", "FUTURES", "MARGIN"}
	for _, instType := range instTypes {
		wg.Add(1)
		go func(it string) {
			defer wg.Done()
			if err := c.CancelAllOrders(ctx, it); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("cancel %s: %w", it, err))
				mu.Unlock()
			}
		}(instType)
	}
	wg.Wait()

	// Steps 4+5 are independent — run in parallel
	var wg2 sync.WaitGroup
	wg2.Add(2)
	go func() {
		defer wg2.Done()
		if err := c.CancelAllAlgoOrders(ctx); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("cancel algos: %w", err))
			mu.Unlock()
		}
	}()
	go func() {
		defer wg2.Done()
		if err := c.FlattenAllPositions(ctx); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("flatten: %w", err))
			mu.Unlock()
		}
	}()
	wg2.Wait()

	return errors.Join(errs...)
}

func (c *Client) GetPendingOrders(ctx context.Context, instType string) (int, error) {
	body, err := c.do(ctx, "GET", "/api/v5/trade/orders-pending?instType="+instType, "")
	if err != nil {
		if strings.HasPrefix(err.Error(), "http status 404") {
			return 0, nil
		}
		return 0, fmt.Errorf("pending %s request: %w", instType, err)
	}
	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct{} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("pending %s parse: %w", instType, err)
	}
	if resp.Code != "0" {
		return 0, fmt.Errorf("pending %s API error code=%s msg=%s", instType, string(resp.Code), resp.Msg)
	}
	return len(resp.Data), nil
}

func (c *Client) CancelAllOrders(ctx context.Context, instType string) error {
	payloadBytes, err := json.Marshal(struct {
		InstType string `json:"instType"`
	}{InstType: instType})
	if err != nil {
		return fmt.Errorf("cancel-all %s marshal: %w", instType, err)
	}
	payload := string(payloadBytes)
	body, err := c.do(ctx, "POST", "/api/v5/trade/cancel-all-orders", payload)
	if err != nil {
		// 404 = no orders to cancel, treat as success
		if strings.HasPrefix(err.Error(), "http status 404") {
			return nil
		}
		return fmt.Errorf("cancel-all %s request: %w", instType, err)
	}
	var resp cancelAllResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("cancel-all %s parse: %w", instType, err)
	}
	// API returns 200 with code=404 when no orders exist
	if resp.Code == "404" || resp.Code == "0" {
		return nil
	}
	return fmt.Errorf("cancel-all %s API error code=%s msg=%s", instType, string(resp.Code), resp.Msg)
}

func (c *Client) GetPositions(ctx context.Context) ([]Position, error) {
	body, err := c.do(ctx, "GET", "/api/v5/account/positions", "")
	if err != nil {
		return nil, fmt.Errorf("positions request: %w", err)
	}
	var resp positionsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("positions parse: %w", err)
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("positions API error code=%s msg=%s", string(resp.Code), resp.Msg)
	}
	return resp.Data, nil
}

func (c *Client) ClosePosition(ctx context.Context, instId, mgnMode, posSide string) error {
	payload := struct {
		InstID  string `json:"instId"`
		MgnMode string `json:"mgnMode"`
		PosSide string `json:"posSide,omitempty"`
	}{InstID: instId, MgnMode: mgnMode}
	if posSide != "" && posSide != "net" {
		payload.PosSide = posSide
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("close %s marshal: %w", instId, err)
	}
	payloadBody := string(payloadBytes)
	body, err := c.do(ctx, "POST", "/api/v5/trade/close-position", payloadBody)
	if err != nil {
		return fmt.Errorf("close %s request: %w", instId, err)
	}
	var resp closePositionResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("close %s parse: %w", instId, err)
	}
	if resp.Code != "0" {
		return fmt.Errorf("close %s API error code=%s msg=%s", instId, string(resp.Code), resp.Msg)
	}
	return nil
}

type OrderResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		OrdID string `json:"ordId"`
	} `json:"data"`
}

func (c *Client) PlaceLimitOrder(ctx context.Context, instId, side, sz, px string) error {
	payload := struct {
		InstID  string `json:"instId"`
		TdMode  string `json:"tdMode"`
		Side    string `json:"side"`
		OrdType string `json:"ordType"`
		Sz      string `json:"sz"`
		Px      string `json:"px"`
	}{InstID: instId, TdMode: "cross", Side: side, OrdType: "limit", Sz: sz, Px: px}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal limit order: %w", err)
	}
	body, err := c.do(ctx, "POST", "/api/v5/trade/order", string(b))
	if err != nil {
		return fmt.Errorf("place limit %s: %w", instId, err)
	}
	var resp OrderResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("place limit %s parse: %w", instId, err)
	}
	if resp.Code != "0" {
		return fmt.Errorf("place limit %s API error code=%s msg=%s", instId, string(resp.Code), resp.Msg)
	}
	return nil
}

func (c *Client) PlaceAlgoOrder(ctx context.Context, instId, side, sz, triggerPx string) error {
	payload := struct {
		InstID    string `json:"instId"`
		TdMode    string `json:"tdMode"`
		Side      string `json:"side"`
		OrdType   string `json:"ordType"`
		Sz        string `json:"sz"`
		SlTriggerPx string `json:"slTriggerPx"`
		SlOrdPx   string `json:"slOrdPx"`
	}{InstID: instId, TdMode: "cross", Side: side, OrdType: "conditional", Sz: sz,
		SlTriggerPx: triggerPx, SlOrdPx: "-1"}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal algo order: %w", err)
	}
	body, err := c.do(ctx, "POST", "/api/v5/trade/order-algo", string(b))
	if err != nil {
		return fmt.Errorf("place algo %s: %w", instId, err)
	}
	var resp OrderResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("place algo %s parse: %w", instId, err)
	}
	if resp.Code != "0" {
		return fmt.Errorf("place algo %s API error code=%s msg=%s", instId, string(resp.Code), resp.Msg)
	}
	return nil
}

func (c *Client) GetPendingAlgoOrders(ctx context.Context, instType string) ([]AlgoOrder, error) {
	path := "/api/v5/trade/orders-algo-pending?" + url.Values{"instType": {instType}, "ordType": {"conditional"}}.Encode()
	body, err := c.do(ctx, "GET", path, "")
	if err != nil {
		return nil, fmt.Errorf("pending algos %s request: %w", instType, err)
	}
	var resp algoPendingResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("pending algos %s parse: %w", instType, err)
	}
	if resp.Code != "0" {
		return nil, fmt.Errorf("pending algos %s API error code=%s msg=%s", instType, string(resp.Code), resp.Msg)
	}
	return resp.Data, nil
}

func (c *Client) CancelAlgoOrders(ctx context.Context, orders []AlgoOrder) error {
	payloadBytes, err := json.Marshal(orders)
	if err != nil {
		return fmt.Errorf("cancel algos marshal: %w", err)
	}
	body, err := c.do(ctx, "POST", "/api/v5/trade/cancel-algos", string(payloadBytes))
	if err != nil {
		return fmt.Errorf("cancel algos request: %w", err)
	}
	var resp cancelAlgoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("cancel algos parse: %w", err)
	}
	if resp.Code != "0" {
		return fmt.Errorf("cancel algos API error code=%s msg=%s", string(resp.Code), resp.Msg)
	}
	return nil
}

type fetchResult struct {
	instType string
	orders   []AlgoOrder
	err      error
}

func (c *Client) CancelAllAlgoOrders(ctx context.Context) error {
	instTypes := []string{"SWAP", "FUTURES", "MARGIN"}
	ch := make(chan fetchResult, len(instTypes))

	// Fetch all instTypes concurrently
	for _, instType := range instTypes {
		go func(it string) {
			orders, err := c.GetPendingAlgoOrders(ctx, it)
			ch <- fetchResult{instType: it, orders: orders, err: err}
		}(instType)
	}

	var errs []error
	done := 0
	for res := range ch {
		done++
		if res.err != nil {
			errs = append(errs, fmt.Errorf("fetch pending %s: %w", res.instType, res.err))
		} else {
			// Cancel algos sequentially per instType (batches of 10)
			for i := 0; i < len(res.orders); i += 10 {
				end := i + 10
				if end > len(res.orders) {
					end = len(res.orders)
				}
				if err := c.CancelAlgoOrders(ctx, res.orders[i:end]); err != nil {
					errs = append(errs, fmt.Errorf("cancel algo batch %s: %w", res.instType, err))
				}
			}
		}
		if done == len(instTypes) {
			break
		}
	}
	return errors.Join(errs...)
}

func (c *Client) FlattenAllPositions(ctx context.Context) error {
	positions, err := c.GetPositions(ctx)
	if err != nil {
		return fmt.Errorf("get positions: %w", err)
	}

	var errs []error
	var mu sync.Mutex
	var wg sync.WaitGroup
	seen := make(map[string]bool)
	for _, pos := range positions {
		if pos.InstID == "" {
			continue
		}
		key := pos.InstID + ":" + pos.PosSide
		if seen[key] {
			continue
		}
		seen[key] = true

		mgnMode := pos.MgnMode
		if mgnMode == "" {
			mgnMode = "cross"
		}
		wg.Add(1)
		go func(instId, mgnMode, posSide string) {
			defer wg.Done()
			if err := c.ClosePosition(ctx, instId, mgnMode, posSide); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("close %s: %w", instId, err))
				mu.Unlock()
			}
		}(pos.InstID, mgnMode, pos.PosSide)
	}
	wg.Wait()
	return errors.Join(errs...)
}

func (c *Client) sign(method, requestPath, body string) (string, string) {
	t := time.Now().UTC()
	timestamp := t.Format("2006-01-02T15:04:05.000Z")

	signPayload := timestamp + method + requestPath + body
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(signPayload))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return timestamp, signature
}

func (c *Client) do(ctx context.Context, method, requestPath, body string) ([]byte, error) {
	url := c.baseURL + requestPath

	var reqBody io.Reader
	if body != "" {
		reqBody = bytes.NewBufferString(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	timestamp, signature := c.sign(method, requestPath, body)
	req.Header.Set("OK-ACCESS-KEY", c.apiKey)
	req.Header.Set("OK-ACCESS-SIGN", signature)
	req.Header.Set("OK-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("OK-ACCESS-PASSPHRASE", c.passphrase)
	req.Header.Set("Content-Type", "application/json")

	if c.demo {
		req.Header.Set("x-simulated-trading", "1")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}
