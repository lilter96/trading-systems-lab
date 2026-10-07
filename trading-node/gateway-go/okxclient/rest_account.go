package okxclient

import (
	"context"
	"fmt"
)

func (c *RestClient) GetBalance(ctx context.Context, ccy string) (*RestResponse, error) {
	path := "/api/v5/account/balance"
	if ccy != "" {
		path += "?ccy=" + ccy
	}
	return c.Do(ctx, "GET", path, "")
}

func (c *RestClient) GetPositions(ctx context.Context, instID string) (*RestResponse, error) {
	path := fmt.Sprintf("/api/v5/account/positions?instId=%s", instID)
	return c.Do(ctx, "GET", path, "")
}

func (c *RestClient) GetAllPositions(ctx context.Context) (*RestResponse, error) {
	return c.Do(ctx, "GET", "/api/v5/account/positions", "")
}

func (c *RestClient) GetMaxAvailSize(ctx context.Context, instID, tdMode string) (*RestResponse, error) {
	path := fmt.Sprintf("/api/v5/account/max-avail-size?instId=%s&tdMode=%s", instID, tdMode)
	return c.Do(ctx, "GET", path, "")
}

func (c *RestClient) SetLeverage(ctx context.Context, instID, lever, mgnMode string) (*RestResponse, error) {
	body := fmt.Sprintf(`{"instId":"%s","lever":"%s","mgnMode":"%s"}`, instID, lever, mgnMode)
	return c.Do(ctx, "POST", "/api/v5/account/set-leverage", body)
}

func (c *RestClient) GetAccountConfig(ctx context.Context) (*RestResponse, error) {
	return c.Do(ctx, "GET", "/api/v5/account/config", "")
}

func (c *RestClient) GetTradeFee(ctx context.Context, instType, instID string) (*RestResponse, error) {
	path := fmt.Sprintf("/api/v5/account/trade-fee?instType=%s&instId=%s", instType, instID)
	return c.Do(ctx, "GET", path, "")
}
