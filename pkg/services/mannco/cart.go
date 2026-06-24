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

// CartItem represents an item row currently inside the user's cart.
// Cart items are grouped by price point; the row lists one or more asset IDs.
type CartItem struct {
	CartID       int             `json:"cartId"`           // Cart row ID
	AssetID      string          `json:"assetId"`          // Comma-separated asset IDs in this cart row
	Count        int             `json:"count"`            // Quantity in this row
	ItemID       int             `json:"item_id"`          // Catalog item ID
	Price        int             `json:"price"`            // Unit price in cents (recorded at add time)
	Name         string          `json:"name"`             // Item display name
	Image        string          `json:"image"`            // Image reference URL
	Effect       string          `json:"effect"`           // Unusual particle effect (TF2)
	Rarity       string          `json:"rarity"`           // Item rarity tier
	Color        string          `json:"color"`            // Hex color value representing quality or rarity
	URL          string          `json:"url"`              // Slug URL identifier
	Quality      string          `json:"quality"`          // Quality label
	TypeSteam    string          `json:"type_steam"`       // Steam category type
	Class        string          `json:"class"`            // Character class (TF2)
	Craftable    Craftable       `json:"craftable"`        // Craftability representation
	Slot         string          `json:"slot"`             // TF2 slot
	Game         int             `json:"game"`             // Game App ID
	Sheen        string          `json:"sheen"`            // Sheen killstreak (TF2)
	Killstreaker string          `json:"killstreaker"`     // Killstreaker effect (TF2)
	Spell        string          `json:"spell"`            // Spells applied (TF2)
	Parts        string          `json:"parts"`            // Strange parts (TF2)
	Paint        string          `json:"paint"`            // Paint hex color (TF2)
	Level        *int            `json:"level"`            // Item level
	Festivized   int             `json:"festivized"`       // TF2 Festivized flag (1 = festivized)
	Inspect      string          `json:"inspect"`          // Inspect link string
	Values       json.RawMessage `json:"values,omitempty"` // CS2 stickers, wear, index
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on CartItem.
func (ci *CartItem) UnmarshalJSON(data []byte) error {
	type Alias CartItem

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*ci = CartItem(aux)
	ci.AssetID = strings.TrimRight(ci.AssetID, " ;")
	ci.Name = strings.TrimRight(ci.Name, " ;")
	ci.Image = strings.TrimRight(ci.Image, " ;")
	ci.Effect = strings.TrimRight(ci.Effect, " ;")
	ci.Rarity = strings.TrimRight(ci.Rarity, " ;")
	ci.Color = strings.TrimRight(ci.Color, " ;")
	ci.URL = strings.TrimRight(ci.URL, " ;")
	ci.Quality = strings.TrimRight(ci.Quality, " ;")
	ci.TypeSteam = strings.TrimRight(ci.TypeSteam, " ;")
	ci.Class = strings.TrimRight(ci.Class, " ;")
	ci.Slot = strings.TrimRight(ci.Slot, " ;")
	ci.Sheen = strings.TrimRight(ci.Sheen, " ;")
	ci.Killstreaker = strings.TrimRight(ci.Killstreaker, " ;")
	ci.Spell = strings.TrimRight(ci.Spell, " ;")
	ci.Parts = strings.TrimRight(ci.Parts, " ;")
	ci.Paint = strings.TrimRight(ci.Paint, " ;")
	ci.Inspect = strings.TrimRight(ci.Inspect, " ;")

	return nil
}

// GetCartResponse wraps current cart response.
type GetCartResponse struct {
	Cart []CartItem `json:"cart"` // Cart row items
}

// AddCartReq represents request body to add a single item to cart.
type AddCartReq struct {
	AssetID string `json:"assetId"` // Steam asset ID of listing to buy
}

// BulkAddCartReq represents request body to add multiple items of the same type in bulk.
type BulkAddCartReq struct {
	ItemID       int    `json:"itemId"`                 // Catalog item ID
	Count        int    `json:"count"`                  // Quantity to add
	SellerUserID string `json:"sellerUserId,omitempty"` // Steam ID of seller (Optional filter)
}

// RemoveCartReq represents request body to remove a cart row.
type RemoveCartReq struct {
	CartID int `json:"cartId"` // Cart row ID
}

// InvalidItem lists invalid items found during cart integrity verification.
type InvalidItem struct {
	CartID        int    `json:"cartId"`                  // Cart row ID
	AssetID       string `json:"assetId"`                 // Steam asset ID
	Reason        string `json:"reason"`                  // Reason (not_found, unavailable, price_changed, own_item)
	CurrentPrice  *int   `json:"currentPrice,omitempty"`  // Actual price in SCM (cents), if price_changed
	ExpectedPrice *int   `json:"expectedPrice,omitempty"` // Cart recorded price (cents), if price_changed
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on InvalidItem.
func (ii *InvalidItem) UnmarshalJSON(data []byte) error {
	type Alias InvalidItem

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*ii = InvalidItem(aux)
	ii.AssetID = strings.TrimRight(ii.AssetID, " ;")
	ii.Reason = strings.TrimRight(ii.Reason, " ;")

	return nil
}

// IntegrityInfo details results of cart integrity checks.
type IntegrityInfo struct {
	Valid        bool          `json:"valid"`        // True if cart integrity is intact
	InvalidItems []InvalidItem `json:"invalidItems"` // Details of invalid items
}

// ReplacedItem represents an invalid plain item replaced with an equivalent listing.
type ReplacedItem struct {
	CartID     int    `json:"cartId"`     // Cart row ID
	OldAssetID string `json:"oldAssetId"` // Replaced asset ID
	NewAssetID string `json:"newAssetId"` // Substituted asset ID
	Reason     string `json:"reason"`     // Replacement trigger reason
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on ReplacedItem.
func (ri *ReplacedItem) UnmarshalJSON(data []byte) error {
	type Alias ReplacedItem

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*ri = ReplacedItem(aux)
	ri.OldAssetID = strings.TrimRight(ri.OldAssetID, " ;")
	ri.NewAssetID = strings.TrimRight(ri.NewAssetID, " ;")
	ri.Reason = strings.TrimRight(ri.Reason, " ;")

	return nil
}

// RemovedItem represents an invalid item removed from the cart without replacement.
type RemovedItem struct {
	CartID  int    `json:"cartId"`  // Cart row ID
	AssetID string `json:"assetId"` // Removed asset ID
	Reason  string `json:"reason"`  // Removal trigger reason
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on RemovedItem.
func (rm *RemovedItem) UnmarshalJSON(data []byte) error {
	type Alias RemovedItem

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*rm = RemovedItem(aux)
	rm.AssetID = strings.TrimRight(rm.AssetID, " ;")
	rm.Reason = strings.TrimRight(rm.Reason, " ;")

	return nil
}

// UpdateCartResponse outlines cart validation, replacements, and finalized cart items.
type UpdateCartResponse struct {
	Integrity IntegrityInfo  `json:"integrity"` // Integrity results
	Replaced  []ReplacedItem `json:"replaced"`  // Replaced items list
	Removed   []RemovedItem  `json:"removed"`   // Removed items list
	Cart      []CartItem     `json:"cart"`      // Active cart rows after cleanup
}

// GetCart retrieves user's current shopping cart with full item metadata.
//
// Route: GET /cart/get
// Permission: Connected + API
func (c *Client) GetCart(ctx context.Context) (*GetCartResponse, error) {
	return aoni.GetJSON[GetCartResponse](ctx, c.getClient(), "/cart/get")
}

// AddToCart inserts a single listing into user's shopping cart by Steam asset ID.
// The asset must be currently on sale (state = 1) and not owned by the buyer.
//
// Route: POST /cart/add
// Permission: Connected + API
func (c *Client) AddToCart(ctx context.Context, assetID string) (*GetCartResponse, error) {
	req := AddCartReq{AssetID: assetID}
	return aoni.PostJSON[GetCartResponse](ctx, c.getClient(), "/cart/add", req)
}

// BulkAddToCart searches the cheapest marketplace listings for an item, excluding
// items already in the cart, and inserts them. Price groupings are aggregated
// into cart rows automatically.
//
// Route: POST /cart/bulk
// Permission: Connected + API
func (c *Client) BulkAddToCart(ctx context.Context, itemID, count int, sellerUserID string) (*GetCartResponse, error) {
	req := BulkAddCartReq{
		ItemID:       itemID,
		Count:        count,
		SellerUserID: sellerUserID,
	}

	return aoni.PostJSON[GetCartResponse](ctx, c.getClient(), "/cart/bulk", req)
}

// RemoveFromCart deletes an entire cart row (and all its associated asset IDs) from the cart.
//
// Route: POST /cart/remove
// Permission: Connected + API
func (c *Client) RemoveFromCart(ctx context.Context, cartID int) (*GetCartResponse, error) {
	req := RemoveCartReq{CartID: cartID}
	return aoni.PostJSON[GetCartResponse](ctx, c.getClient(), "/cart/remove", req)
}

// UpdateCart triggers an integrity scan on the cart. Invalid items (sold, price altered)
// are flagged. Plain invalid items are automatically replaced by equivalent plain items at the same price
// if available. Non-plain invalid items are removed. Empty rows are deleted.
// Use this prior to initiating payment checkout.
//
// Route: POST /cart/update
// Permission: Connected + API
func (c *Client) UpdateCart(ctx context.Context) (*UpdateCartResponse, error) {
	return aoni.PostJSON[UpdateCartResponse](ctx, c.getClient(), "/cart/update", nil)
}
