// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mannco

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/lemon4ksan/aoni"
)

// DepositInfoItem represents an item in user's Steam inventory with pricing hints and stock limits.
type DepositInfoItem struct {
	AssetID        string            `json:"assetid"`          // Comma-separated asset IDs matching this item
	Count          int               `json:"count"`            // Total count of matching items in Steam inventory
	MarketHashName string            `json:"market_hash_name"` // Steam Market Hash Name
	ItemID         int               `json:"item_id"`          // Catalog item ID
	URL            string            `json:"url"`              // Slug URL identifier
	DepositKey     map[string]string `json:"depositkey"`       // Map of asset ID to verification key
	NbHighStock    int               `json:"nb_high_stock"`    // Current number in platform inventory
	HighStockLimit int               `json:"high_stock_limit"` // High stock limit of this item type
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on DepositInfoItem.
func (d *DepositInfoItem) UnmarshalJSON(data []byte) error {
	type Alias DepositInfoItem

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*d = DepositInfoItem(aux)
	d.AssetID = strings.TrimRight(d.AssetID, " ;")
	d.MarketHashName = strings.TrimRight(d.MarketHashName, " ;")
	d.URL = strings.TrimRight(d.URL, " ;")

	return nil
}

// GetDepositInfoResponse wraps deposit inventory information list.
type GetDepositInfoResponse struct {
	Informations []DepositInfoItem `json:"informations"` // Inventory list
}

// CreateDepositTradeReq represents request body to create a deposit trade.
type CreateDepositTradeReq struct {
	Prices      map[string]int    `json:"prices"`      // Map of assetId to price in cents (USD)
	DepositKeys map[string]string `json:"depositKeys"` // Map of assetId to the verification key from GET deposit response
	Game        int               `json:"game"`        // Game ID (440 = TF2, 730 = CS2, 570 = Dota2, 252490 = Rust)
}

// CreateDepositTradeResponse wraps the created trade ID.
type CreateDepositTradeResponse struct {
	ID int `json:"id"` // Deposit trade ID (use for GetDepositTradeStatus)
}

// CreateInstantSellTradeReq represents request body to create an instant sell trade.
type CreateInstantSellTradeReq struct {
	Prices        map[string]int    `json:"prices"`         // Map of assetId to instant price in cents (USD)
	DepositKeys   map[string]string `json:"depositKeys"`    // Map of assetId to verification key
	Game          int               `json:"game"`           // Game ID (TF2 440 only)
	CashoutMethod string            `json:"cashout_method"` // Instant cashout provider method
}

// TradeInfo represents detailed metrics for a deposit trade row.
type TradeInfo struct {
	ID            int    `json:"id"`             // Trade transaction ID
	ItemsReceived string `json:"items_received"` // Comma-separated asset IDs received
	ItemsSend     string `json:"items_send"`     // Comma-separated asset IDs sent
	Status        int    `json:"status"`         // Trade Status (0 = Pending, 3 = Completed, -1 = Failed)
	User          string `json:"user"`           // User Steam ID
	Bot           string `json:"bot"`            // Bot Steam ID
	Code          string `json:"code"`           // Security verification code
	LastError     string `json:"lasterror"`      // Error details, if failed
	OfferID       string `json:"offerid"`        // Steam trade offer ID
	Timestamp     string `json:"timestamp"`      // Creation timestamp (Unix ms)
	Game          int    `json:"game"`           // Game ID
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on TradeInfo.
func (ti *TradeInfo) UnmarshalJSON(data []byte) error {
	type Alias TradeInfo

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*ti = TradeInfo(aux)
	ti.ItemsReceived = strings.TrimRight(ti.ItemsReceived, " ;")
	ti.ItemsSend = strings.TrimRight(ti.ItemsSend, " ;")
	ti.User = strings.TrimRight(ti.User, " ;")
	ti.Bot = strings.TrimRight(ti.Bot, " ;")
	ti.Code = strings.TrimRight(ti.Code, " ;")
	ti.LastError = strings.TrimRight(ti.LastError, " ;")
	ti.OfferID = strings.TrimRight(ti.OfferID, " ;")
	ti.Timestamp = strings.TrimRight(ti.Timestamp, " ;")

	return nil
}

// TradeStatusResponse wraps deposit trade details.
type TradeStatusResponse struct {
	Trade TradeInfo `json:"trade"` // Trade information
}

// GetDepositInfo returns user's Steam inventory for a game with deposit keys per asset ID.
//
// Route: GET /deposit/{game}
// Permission: Connected + API
func (c *Client) GetDepositInfo(ctx context.Context, game int) (*GetDepositInfoResponse, error) {
	return aoni.GetJSON[GetDepositInfoResponse](ctx, c.getClient(), "/deposit/{game}", aoni.WithVar("game", game))
}

// CreateDepositTrade initiates a trade offer to deposit items onto the site.
// Verification keys are verified server-side.
//
// Route: POST /deposit/trade
// Permission: Connected + API
func (c *Client) CreateDepositTrade(
	ctx context.Context,
	req CreateDepositTradeReq,
) (*CreateDepositTradeResponse, error) {
	return aoni.PostJSON[CreateDepositTradeResponse](ctx, c.getClient(), "/deposit/trade", req)
}

// GetInstantSellInfo returns enriched inventory data with instant-sell price values.
// Supports Team Fortress 2 (game 440) only.
//
// Route: GET /deposit/instantSell/{game}
// Permission: Connected + API
func (c *Client) GetInstantSellInfo(ctx context.Context, game int) (*GetDepositInfoResponse, error) {
	return aoni.GetJSON[GetDepositInfoResponse](
		ctx, c.getClient(), "/deposit/instantSell/{game}",
		aoni.WithVar("game", game),
	)
}

// CreateInstantSellTrade creates a trade to instantly sell inventory items.
// Supports TF2 (game 440) only. Requires a valid cashout payout method configuration.
//
// Route: POST /deposit/trade/instant
// Permission: Connected + API
func (c *Client) CreateInstantSellTrade(
	ctx context.Context,
	req CreateInstantSellTradeReq,
) (*CreateDepositTradeResponse, error) {
	return aoni.PostJSON[CreateDepositTradeResponse](ctx, c.getClient(), "/deposit/trade/instant", req)
}

// GetDepositTradeStatus returns the current status and bot metadata of a deposit trade row.
//
// Route: GET /deposit/tradeStatus/{tradeid}
// Permission: Connected + API
func (c *Client) GetDepositTradeStatus(ctx context.Context, tradeID int) (*TradeStatusResponse, error) {
	return aoni.GetJSON[TradeStatusResponse](
		ctx, c.getClient(), "/deposit/tradeStatus/{tradeid}",
		aoni.WithVar("tradeid", tradeID),
	)
}
