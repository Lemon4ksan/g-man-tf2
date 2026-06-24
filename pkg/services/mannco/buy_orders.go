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

// CreateBuyOrderReq represents the request body to create a buy order.
type CreateBuyOrderReq struct {
	ItemID int `json:"itemid"` // Catalog item ID
	Value  int `json:"value"`  // Order price in cents (must be positive)
	Amount int `json:"amount"` // Quantity to buy (max 499,999)
}

// UpdateBuyOrderReq represents the request body to update an existing buy order.
type UpdateBuyOrderReq struct {
	ItemID int `json:"itemid"` // Catalog item ID
	Value  int `json:"value"`  // New order price in cents (must be positive)
	Amount int `json:"amount"` // New quantity (max 4,999)
}

// RemoveBuyOrderReq represents the request body to remove a buy order.
type RemoveBuyOrderReq struct {
	ItemID int `json:"itemid"` // Catalog item ID
}

// DetailsResponse holds generic response details message.
type DetailsResponse struct {
	Details string `json:"details"` // Status details (e.g. "Inserted", "Removed")
}

// UserBuyOrder represents user's active buy order for a specific item.
type UserBuyOrder struct {
	ID        int    `json:"id"`        // Buy order ID
	SteamID   string `json:"steamid"`   // User's Steam ID
	ItemID    int    `json:"itemid"`    // Catalog item ID
	Price     int    `json:"price"`     // Buy order price in cents
	Amount    int    `json:"amount"`    // Quantity of items to buy
	Timestamp string `json:"timestamp"` // Unix timestamp of creation
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on UserBuyOrder.
func (ubo *UserBuyOrder) UnmarshalJSON(data []byte) error {
	type Alias UserBuyOrder

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*ubo = UserBuyOrder(aux)
	ubo.SteamID = strings.TrimRight(ubo.SteamID, " ;")
	ubo.Timestamp = strings.TrimRight(ubo.Timestamp, " ;")

	return nil
}

// UserBuyOrderResponse wraps active user buy order.
type UserBuyOrderResponse struct {
	Informations UserBuyOrder `json:"informations"`
}

// UserAllBuyOrdersItem contains information about user's active buy order with joined catalog item details.
type UserAllBuyOrdersItem struct {
	ID              int             `json:"id"`              // Buy order ID
	SteamID         string          `json:"steamid"`         // User's Steam ID
	ItemID          int             `json:"itemid"`          // Catalog item ID
	Price           int             `json:"price"`           // Buy order price in cents
	Amount          int             `json:"amount"`          // Quantity remaining
	Name            string          `json:"name"`            // Custom buy order or item name
	Effect          string          `json:"effect"`          // Particle effect (TF2)
	URL             string          `json:"url"`             // Slug URL identifier
	Game            int             `json:"game"`            // Game ID
	Quality         string          `json:"quality"`         // Quality label
	Image           string          `json:"image"`           // Image URL
	Type            string          `json:"type"`            // Category description
	Craftable       Craftable       `json:"craftable"`       // Craftability status
	SKU             string          `json:"SKU"`             // Stock Keeping Unit
	TypeSteam       *string         `json:"type_steam"`      // Steam-specific type label
	Class           *string         `json:"class"`           // Item class (TF2)
	ImagePertinence int             `json:"imagePertinence"` // Image relevance
	Rarity          string          `json:"rarity"`          // Rarity class
	Featured        int             `json:"featured"`        // Featured flag
	Deal            *int            `json:"deal"`            // SCM price hint in cents
	Color           string          `json:"color"`           // Hex color code
	Slot            string          `json:"slot"`            // Equipment slot
	Hero            *string         `json:"hero"`            // Dota 2 hero
	Weapon          *string         `json:"weapon"`          // Weapon name
	Exterior        *string         `json:"exterior"`        // Wear exterior (CS2)
	Description     *string         `json:"description"`     // HTML description text
	TF2Shop         json.RawMessage `json:"tf2shop"`         // TF2 shop values
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on UserAllBuyOrdersItem.
func (u *UserAllBuyOrdersItem) UnmarshalJSON(data []byte) error {
	type Alias UserAllBuyOrdersItem

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*u = UserAllBuyOrdersItem(aux)
	u.SteamID = strings.TrimRight(u.SteamID, " ;")
	u.Name = strings.TrimRight(u.Name, " ;")
	u.Effect = strings.TrimRight(u.Effect, " ;")
	u.URL = strings.TrimRight(u.URL, " ;")
	u.Quality = strings.TrimRight(u.Quality, " ;")
	u.Image = strings.TrimRight(u.Image, " ;")
	u.Type = strings.TrimRight(u.Type, " ;")

	u.SKU = strings.TrimRight(u.SKU, " ;")
	if u.TypeSteam != nil {
		trimmed := strings.TrimRight(*u.TypeSteam, " ;")
		u.TypeSteam = &trimmed
	}

	if u.Class != nil {
		trimmed := strings.TrimRight(*u.Class, " ;")
		u.Class = &trimmed
	}

	u.Rarity = strings.TrimRight(u.Rarity, " ;")
	u.Color = strings.TrimRight(u.Color, " ;")

	u.Slot = strings.TrimRight(u.Slot, " ;")
	if u.Hero != nil {
		trimmed := strings.TrimRight(*u.Hero, " ;")
		u.Hero = &trimmed
	}

	if u.Weapon != nil {
		trimmed := strings.TrimRight(*u.Weapon, " ;")
		u.Weapon = &trimmed
	}

	if u.Exterior != nil {
		trimmed := strings.TrimRight(*u.Exterior, " ;")
		u.Exterior = &trimmed
	}

	if u.Description != nil {
		trimmed := strings.TrimRight(*u.Description, " ;")
		u.Description = &trimmed
	}

	return nil
}

// UserAllBuyOrdersCount counts total active buy orders.
type UserAllBuyOrdersCount struct {
	Nb int `json:"nb"` // Total count
}

// UserAllBuyOrdersResponse represents the response containing user buy orders.
type UserAllBuyOrdersResponse struct {
	Values []UserAllBuyOrdersItem `json:"values"`
	Count  UserAllBuyOrdersCount  `json:"count"`
}

// CreateBuyOrder creates a buy order for an item.
// One buy order allowed per item per user. Balance equivalent to (value * amount) is reserved.
// Create supports quantities up to 499,999.
//
// Route: POST /item/buyorder
// Permission: Connected + API
func (c *Client) CreateBuyOrder(ctx context.Context, itemID, value, amount int) (*DetailsResponse, error) {
	req := CreateBuyOrderReq{
		ItemID: itemID,
		Value:  value,
		Amount: amount,
	}

	return aoni.PostJSON[DetailsResponse](ctx, c.getClient(), "/item/buyorder", req)
}

// UpdateBuyOrder updates an existing buy order price and/or quantity.
// The new total balance requirement is reserved, and the previous reservation is released.
// Update supports quantities up to 4,999.
//
// Route: POST /item/buyorder/update
// Permission: Connected + API
func (c *Client) UpdateBuyOrder(ctx context.Context, itemID, value, amount int) (string, error) {
	req := UpdateBuyOrderReq{
		ItemID: itemID,
		Value:  value,
		Amount: amount,
	}

	resp, err := aoni.PostJSON[string](ctx, c.getClient(), "/item/buyorder/update", req)
	if err != nil {
		return "", err
	}

	return *resp, nil
}

// RemoveBuyOrder cancels a buy order and releases the reserved balance.
//
// Route: POST /item/buyorder/remove
// Permission: Connected + API
func (c *Client) RemoveBuyOrder(ctx context.Context, itemID int) (*DetailsResponse, error) {
	req := RemoveBuyOrderReq{
		ItemID: itemID,
	}

	return aoni.PostJSON[DetailsResponse](ctx, c.getClient(), "/item/buyorder/remove", req)
}

// GetUserBuyOrdersForItem returns user's active buy orders for a specific item ID.
//
// Route: GET /user/buyorder/{item}
// Permission: Connected + API
func (c *Client) GetUserBuyOrdersForItem(ctx context.Context, item string) (*UserBuyOrderResponse, error) {
	return aoni.GetJSON[UserBuyOrderResponse](ctx, c.getClient(), "/user/buyorder/{item}", aoni.WithVar("item", item))
}

// GetUserBuyOrdersQuery holds filtering arguments for GetUserBuyOrders.
type GetUserBuyOrdersQuery struct {
	Page     int    `url:"page,omitempty"`     // Page index
	Count    int    `url:"count,omitempty"`    // Results per page (max 1000)
	Search   string `url:"search,omitempty"`   // Filter by item name or effect (min 3 chars)
	Undercut any    `url:"undercut,omitempty"` // Pass true/1 to filter orders currently below lowest sale price
}

// GetUserBuyOrders returns user's active buy orders across all catalog items.
//
// Route: GET /user/getBuyorder
// Permission: Connected + API
func (c *Client) GetUserBuyOrders(ctx context.Context, query GetUserBuyOrdersQuery) (*UserAllBuyOrdersResponse, error) {
	return aoni.GetJSON[UserAllBuyOrdersResponse](ctx, c.getClient(), "/user/getBuyorder", aoni.WithQuery(query))
}
