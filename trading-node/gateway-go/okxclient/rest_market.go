package okxclient

import (
	"context"
	"fmt"
)

func (c *RestClient) GetTicker(ctx context.Context, instID string) (*RestResponse, error) {
	path := fmt.Sprintf("/api/v5/market/ticker?instId=%s", instID)
	return c.Do(ctx, "GET", path, "")
}

func (c *RestClient) GetInstruments(ctx context.Context, instType, instID string) (*RestResponse, error) {
	path := fmt.Sprintf("/api/v5/public/instruments?instType=%s&instId=%s", instType, instID)
	return c.Do(ctx, "GET", path, "")
}
