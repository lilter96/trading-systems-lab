package okxclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL    = "https://www.okx.com"
	defaultDemoBaseURL = "https://www.okx.cab"
)

type RestResponse struct {
	Code string            `json:"code"`
	Msg  string            `json:"msg"`
	Data []json.RawMessage `json:"data"`
}

type RestClient struct {
	creds   Credentials
	baseURL string
	isDemo  bool
	http    *http.Client
}

func NewRestClient(creds Credentials, isDemo bool) *RestClient {
	baseURL := defaultBaseURL
	if isDemo {
		baseURL = defaultDemoBaseURL
	}
	return &RestClient{
		creds:   creds,
		baseURL: baseURL,
		isDemo:  isDemo,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *RestClient) Do(ctx context.Context, method, requestPath, body string) (*RestResponse, error) {
	ts, sig := c.creds.Sign(method, requestPath, body)

	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+requestPath, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OK-ACCESS-KEY", c.creds.APIKey)
	req.Header.Set("OK-ACCESS-SIGN", sig)
	req.Header.Set("OK-ACCESS-TIMESTAMP", ts)
	req.Header.Set("OK-ACCESS-PASSPHRASE", c.creds.Passphrase)
	if c.isDemo {
		req.Header.Set("x-simulated-trading", "1")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var restResp RestResponse
	if err := json.Unmarshal(raw, &restResp); err != nil {
		return nil, fmt.Errorf("decode response: %s: %w", string(raw), err)
	}

	return &restResp, nil
}
