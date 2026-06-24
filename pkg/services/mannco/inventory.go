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

// InventoryItem represents a user's backpack item in inventory or on sale.
type InventoryItem struct {
	IDs       string    `json:"ids"`       // Comma-separated list of asset IDs matching this item
	Count     int       `json:"count"`     // Total quantity of items matching this group
	ItemID    int       `json:"item_id"`   // Catalog item ID
	AssetID   string    `json:"assetId"`   // Asset ID
	Bot       string    `json:"bot"`       // Steam ID of the holding bot
	Game      int       `json:"game"`      // Game App ID
	State     int       `json:"state"`     // Item state (0 = inventory, 1 = sale, 2 = withdraw, 3 = active trade)
	Price     int       `json:"price"`     // Price in cents
	Name      string    `json:"name"`      // Display name
	Effect    string    `json:"effect"`    // Unusual particle effect (TF2)
	URL       string    `json:"url"`       // Slug URL identifier
	Quality   string    `json:"quality"`   // Quality label
	Image     string    `json:"image"`     // Image URL
	Type      string    `json:"type"`      // Category type label
	Craftable Craftable `json:"craftable"` // Craftability representation
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on InventoryItem.
func (ii *InventoryItem) UnmarshalJSON(data []byte) error {
	type Alias InventoryItem

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*ii = InventoryItem(aux)
	ii.IDs = strings.TrimRight(ii.IDs, " ;")
	ii.AssetID = strings.TrimRight(ii.AssetID, " ;")
	ii.Bot = strings.TrimRight(ii.Bot, " ;")
	ii.Name = strings.TrimRight(ii.Name, " ;")
	ii.Effect = strings.TrimRight(ii.Effect, " ;")
	ii.URL = strings.TrimRight(ii.URL, " ;")
	ii.Quality = strings.TrimRight(ii.Quality, " ;")
	ii.Image = strings.TrimRight(ii.Image, " ;")
	ii.Type = strings.TrimRight(ii.Type, " ;")

	return nil
}

// GetInventoryResponse wraps inventory list response.
type GetInventoryResponse struct {
	Items []InventoryItem `json:"items"` // List of inventory items
	Count int             `json:"count"` // Total count
}

// SetPriceReq represents request payload to set item pricing.
type SetPriceReq struct {
	IDs   string `json:"ids"`   // Comma-separated list of asset IDs to price
	Price int    `json:"price"` // Price in cents (must be between 1 and 5,000,000 cents)
}

// InventoryMessageResponse holds price set outcome text.
type InventoryMessageResponse struct {
	Message string `json:"message"` // Success message (e.g. "ok")
}

// WithdrawReq represents request payload to withdraw items.
type WithdrawReq struct {
	IDs string `json:"ids"` // Comma-separated asset IDs to withdraw
}

// WithdrawResponse outlines successfully withdrawal processing.
type WithdrawResponse struct {
	Message string `json:"message"` // Status message (e.g. "Items withdrawal processed")
	Updated int    `json:"updated"` // Quantity successfully queued for withdrawal
	Locked  int    `json:"locked"`  // Quantity locked in active trades/holds
}

// GetItemsOnSale returns items listed for sale by the user (state = 1).
//
// Route: GET /inventory/onSale
// Permission: Connected + API
func (c *Client) GetItemsOnSale(ctx context.Context) (*GetInventoryResponse, error) {
	return aoni.GetJSON[GetInventoryResponse](ctx, c.getClient(), "/inventory/onSale")
}

// GetItemsInInventory returns items currently in user inventory (not on sale, state = 0).
//
// Route: GET /inventory/onInventory
// Permission: Connected + API
func (c *Client) GetItemsInInventory(ctx context.Context) (*GetInventoryResponse, error) {
	return aoni.GetJSON[GetInventoryResponse](ctx, c.getClient(), "/inventory/onInventory")
}

// SetItemPrice updates pricing for a list of inventory item asset IDs.
// Prices must be between 1 and 5,000,000 cents.
// Matching buy orders at or above the price trigger an instant sale (debited from buyer,
// credited to seller minus 5% fee). Active CS:GO tradehold items cannot be listed.
//
// Route: POST /inventory/price
// Permission: Connected + API
func (c *Client) SetItemPrice(ctx context.Context, ids []string, price int) (*InventoryMessageResponse, error) {
	req := SetPriceReq{
		IDs:   strings.Join(ids, ","),
		Price: price,
	}

	return aoni.PostJSON[InventoryMessageResponse](ctx, c.getClient(), "/inventory/price", req)
}

// WithdrawItems pulls items from Mannco.store inventory to the user's Steam Account.
// Items must have state = 0 (inventory) or state = 1 (on sale). Locked items (state 2 or 3) fail withdrawal.
//
// Route: POST /inventory/withdraw
// Permission: Connected + API
func (c *Client) WithdrawItems(ctx context.Context, ids []string) (*WithdrawResponse, error) {
	req := WithdrawReq{
		IDs: strings.Join(ids, ","),
	}

	return aoni.PostJSON[WithdrawResponse](ctx, c.getClient(), "/inventory/withdraw", req)
}
