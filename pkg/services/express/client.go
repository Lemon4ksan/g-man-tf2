// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package express

import (
	"context"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/mod"
	"github.com/lemon4ksan/aoni/option"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
)

// V2AccountResponse is an alias for V2accountResponse.
type V2AccountResponse = V2accountResponse

// V2InventoryResponse is an alias for V2inventoryResponse.
type V2InventoryResponse = V2inventoryResponse

// V2MarketPriceResponse is an alias for V2marketPriceResponse.
type V2MarketPriceResponse = V2marketPriceResponse

// V2ErrorResponse is an alias for V2errorResponse.
type V2ErrorResponse = V2errorResponse

// GetSteamUsersInventoryV2Query wraps query parameters for inventory requests.
type GetSteamUsersInventoryV2Query struct {
	Cursor   string        `query:"cursor,omitempty"`
	Language SteamLanguage `query:"language,omitempty"`
}

// GetSteamMarketPriceV2Query wraps query parameters for market price requests.
type GetSteamMarketPriceV2Query struct {
	MarketHashName string        `query:"market_hash_name,omitempty"`
	AppID          int           `query:"app_id,omitempty"`
	Currency       SteamCurrency `query:"currency,omitempty"`
}

// Client wraps API with optional client-level ergonomics.
type Client struct {
	API
}

// NewClient creates a new Express client instance.
func NewClient(doer any, apiKey string, opts ...aoni.ClientOption) *Client {
	if apiKey != "" {
		opts = append(opts, option.WithHeader("X-API-Key", apiKey))
	}

	return &Client{
		API: New(doer, opts...),
	}
}

// GetAccountV2 fetches the API account details.
func (c *Client) GetAccountV2(ctx context.Context, mods ...aoni.RequestModifier) (*V2AccountResponse, error) {
	return c.GetAccount(ctx, mods...)
}

// GetSteamMarketPriceV2 fetches the market price for an item.
func (c *Client) GetSteamMarketPriceV2(
	ctx context.Context,
	q GetSteamMarketPriceV2Query,
	mods ...aoni.RequestModifier,
) (*V2MarketPriceResponse, error) {
	return c.GetSteamMarketPrice(ctx, q.MarketHashName, q.AppID, int(q.Currency), mods...)
}

// GetSteamUsersInventoryV2 loads a Steam user's inventory.
func (c *Client) GetSteamUsersInventoryV2(
	ctx context.Context,
	steamID id.ID,
	appID, contextID int,
	q GetSteamUsersInventoryV2Query,
	mods ...aoni.RequestModifier,
) (*V2InventoryResponse, error) {
	return c.GetSteamInventory(ctx, steamID, appID, contextID, q.Cursor, string(q.Language), mods...)
}

// GetStatus returns the public service status.
func (c *Client) GetStatus(ctx context.Context, mods ...aoni.RequestModifier) (*PublicStatus, error) {
	return c.GetPublicStatus(ctx, mods...)
}

// CheckReady verifies service readiness.
func (c *Client) CheckReady(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error) {
	return c.GetReadyHealth(ctx, mods...)
}

// CheckLive verifies service liveness.
func (c *Client) CheckLive(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error) {
	return c.GetLiveHealth(ctx, mods...)
}

// WithIdempotencyKey returns a RequestModifier that sets the Idempotency-Key header.
func WithIdempotencyKey(value string) aoni.RequestModifier {
	return mod.WithHeader("Idempotency-Key", value)
}
