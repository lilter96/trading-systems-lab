package okxclient

import (
	"context"
	"encoding/json"
	"fmt"
)

type OrderRequest struct {
	InstID  string `json:"instId"`
	TDMode  string `json:"tdMode"`
	Side    string `json:"side"`
	OrdType string `json:"ordType"`
	Sz      string `json:"sz"`
	PosSide string `json:"posSide,omitempty"`
	TgtCcy  string `json:"tgtCcy,omitempty"`
	Px      string `json:"px,omitempty"`
	ClOrdID string `json:"clOrdId,omitempty"`
}

type AlgoOrderRequest struct {
	InstID      string `json:"instId"`
	TDMode      string `json:"tdMode"`
	Side        string `json:"side"`
	OrdType     string `json:"ordType"`
	Sz          string `json:"sz"`
	PosSide     string `json:"posSide,omitempty"`
	TPTriggerPx string `json:"tpTriggerPx,omitempty"`
	TPOrdPx     string `json:"tpOrdPx,omitempty"`
	SLTriggerPx string `json:"slTriggerPx,omitempty"`
	SLOrdPx     string `json:"slOrdPx,omitempty"`
}

type CancelOrderRequest struct {
	InstID string `json:"instId"`
	OrdID  string `json:"ordId"`
}

func (c *RestClient) PlaceOrder(ctx context.Context, req *OrderRequest) (*RestResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal order request: %w", err)
	}
	return c.Do(ctx, "POST", "/api/v5/trade/order", string(body))
}

func (c *RestClient) PlaceAlgoOrder(ctx context.Context, req *AlgoOrderRequest) (*RestResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal algo order request: %w", err)
	}
	return c.Do(ctx, "POST", "/api/v5/trade/order-algo", string(body))
}

func (c *RestClient) CancelOrder(ctx context.Context, instID, ordID string) (*RestResponse, error) {
	req := CancelOrderRequest{InstID: instID, OrdID: ordID}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal cancel request: %w", err)
	}
	return c.Do(ctx, "POST", "/api/v5/trade/cancel-order", string(body))
}

func (c *RestClient) CancelAlgoOrder(ctx context.Context, instID, algoID string) (*RestResponse, error) {
	body := fmt.Sprintf(`{"instId":"%s","algoId":"%s"}`, instID, algoID)
	return c.Do(ctx, "POST", "/api/v5/trade/cancel-algos", body)
}

func (c *RestClient) GetOrder(ctx context.Context, instID, ordID string) (*RestResponse, error) {
	path := fmt.Sprintf("/api/v5/trade/order?instId=%s&ordId=%s", instID, ordID)
	return c.Do(ctx, "GET", path, "")
}

func (c *RestClient) GetPendingOrders(ctx context.Context) (*RestResponse, error) {
	return c.Do(ctx, "GET", "/api/v5/trade/orders-pending", "")
}

func (c *RestClient) GetPendingAlgoOrders(ctx context.Context, ordType string) (*RestResponse, error) {
	return c.Do(ctx, "GET", "/api/v5/trade/orders-algo-pending?ordType="+ordType, "")
}
