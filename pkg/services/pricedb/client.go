// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

import (
	"context"

	"github.com/lemon4ksan/g-man/pkg/rest"
)

const (
	// BaseURL is the base URL for the PriceDB API.
	BaseURL = "https://api.pricedb.io"
	// SKUURL is the base URL for the PriceDB SKU API.
	SKUURL = "https://sku.pricedb.io"
)

// Client is a thread-safe HTTP client for interacting with PriceDB.
type Client struct {
	restClient *rest.Client
	skuClient  *rest.Client
}

// NewClient creates a new PriceDB API client.
// If httpClient is nil, a default robust client is created.
func NewClient(httpClient rest.HTTPDoer) *Client {
	return &Client{
		restClient: rest.NewClient(httpClient).WithBaseURL(BaseURL).WithUserAgent("G-man Bot/1.0"),
		skuClient:  rest.NewClient(httpClient).WithBaseURL(SKUURL).WithUserAgent("G-man Bot/1.0"),
	}
}

// WithUserAgent returns a new Client configured with a custom User-Agent.
func (c *Client) WithUserAgent(ua string) *Client {
	return &Client{
		restClient: c.restClient.WithUserAgent(ua),
		skuClient:  c.skuClient.WithUserAgent(ua),
	}
}

// UserAgent returns the configured User-Agent for this client.
func (c *Client) UserAgent() string {
	return c.restClient.UserAgent()
}

// GetItem fetches the latest price for a specific item SKU.
func (c *Client) GetItem(ctx context.Context, sku string) (*Price, error) {
	return rest.GetJSON[Price](ctx, c.restClient, "/api/item/{sku}", nil, rest.WithVar("sku", sku))
}

// GetItemsBulk fetches the latest prices for an array of SKUs in a single request.
func (c *Client) GetItemsBulk(ctx context.Context, skus []string) ([]*Price, error) {
	req := bulkRequest{SKUs: skus}

	resp, err := rest.PostJSON[bulkRequest, []*Price](ctx, c.restClient, "/api/items-bulk", req, nil)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// Search performs a fuzzy search for items by name.
func (c *Client) Search(ctx context.Context, query string, limit int) (*SearchResult, error) {
	req := struct {
		Q     string `url:"q"`
		Limit int    `url:"limit,omitempty"`
	}{query, limit}

	return rest.GetJSON[SearchResult](ctx, c.restClient, "/api/search", req)
}

// GetHistory returns the price history for a specific SKU.
// start and end are optional Unix timestamps (use 0 to ignore).
func (c *Client) GetHistory(ctx context.Context, sku string, start, end int64) ([]*Price, error) {
	req := struct {
		Start int64 `url:"start,omitempty"`
		End   int64 `url:"end,omitempty"`
	}{start, end}

	resp, err := rest.GetJSON[[]*Price](ctx, c.restClient, "/api/item-history/{sku}", req, rest.WithVar("sku", sku))
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetStats returns statistics (min, max, avg) for an item's price history.
func (c *Client) GetStats(ctx context.Context, sku string) (*ItemStats, error) {
	return rest.GetJSON[ItemStats](ctx, c.restClient, "/api/item-stats/{sku}", nil, rest.WithVar("sku", sku))
}

// Compare compares two items side by side, returning the price differences.
func (c *Client) Compare(ctx context.Context, sku1, sku2 string) (*CompareResult, error) {
	return rest.GetJSON[CompareResult](
		ctx,
		c.restClient,
		"/api/compare/{sku1}/{sku2}",
		nil,
		rest.WithVar("sku1", sku1),
		rest.WithVar("sku2", sku2),
	)
}

// TriggerPriceCheck requests PriceDB to update the price for a specific SKU.
// This hits the Autobot integration endpoint.
func (c *Client) TriggerPriceCheck(ctx context.Context, sku string) error {
	// We don't care about the response body, just the HTTP status code
	_, err := rest.PostJSON[any, any](ctx, c.restClient, "/api/autob/items/{sku}", nil, nil, rest.WithVar("sku", sku))

	return err
}

// HealthCheck returns the current system statistics and health of the API.
func (c *Client) HealthCheck(ctx context.Context) (*CacheStats, error) {
	return rest.GetJSON[CacheStats](ctx, c.restClient, "/api/cache-stats", nil)
}

// ResolveName looks up an item by name using the SKU Service.
func (c *Client) ResolveName(ctx context.Context, name string) (map[string]any, error) {
	resp, err := rest.GetJSON[map[string]any](ctx, c.skuClient, "/api/name/{name}", nil, rest.WithVar("name", name))
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// ResolveSKU looks up item properties by its SKU using the SKU Service.
func (c *Client) ResolveSKU(ctx context.Context, sku string) (map[string]any, error) {
	resp, err := rest.GetJSON[map[string]any](ctx, c.skuClient, "/api/sku/{sku}", nil, rest.WithVar("sku", sku))
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetSchema fetches the complete TF2 schema from PriceDB.
func (c *Client) GetSchema(ctx context.Context) (map[string]any, error) {
	resp, err := rest.GetJSON[map[string]any](ctx, c.skuClient, "/api/schema", nil)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}
