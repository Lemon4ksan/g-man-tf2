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

// ItemInfo represents the metadata of a catalog item in the database.
type ItemInfo struct {
	ID              int             `json:"id"`              // Unique item type identifier
	Name            string          `json:"name"`            // Item display name
	Effect          string          `json:"effect"`          // Unusual effect description (TF2)
	SKU             string          `json:"SKU"`             // Stock Keeping Unit string
	Quality         string          `json:"quality"`         // Item quality (Unique, Strange, Unusual, Souvenir, etc.)
	Type            string          `json:"type"`            // Item type/category (e.g. Rifle, Hat)
	TypeSteam       *string         `json:"type_steam"`      // Steam-specific type label
	Class           *string         `json:"class"`           // Item class (TF2)
	Image           string          `json:"image"`           // Image URL or reference code
	ImagePertinence int             `json:"imagePertinence"` // Image relevance scoring
	Craftable       int             `json:"craftable"`       // Craftable state: 1 = Craftable, 0 = Uncraftable
	Rarity          string          `json:"rarity"`          // Rarity class (CS:GO / CS2 covert, classified, etc.)
	Featured        int             `json:"featured"`        // 1 if item is featured, 0 otherwise
	Deal            int             `json:"deal"`            // Steam community market price in cents (USD)
	T               json.RawMessage `json:"t"`               // Game specific extra attributes
	Color           string          `json:"color"`           // Hex color code representing rarity or quality
	Game            int             `json:"game"`            // Game App ID
	Slot            string          `json:"slot"`            // TF2 equipment slot
	Hero            *string         `json:"hero"`            // Dota 2 hero name
	Weapon          string          `json:"weapon"`          // CS:GO weapon name
	Exterior        string          `json:"exterior"`        // CS:GO exterior wear condition
	URL             string          `json:"url"`             // Slug URL identifier
	Description     *string         `json:"description"`     // HTML description text
	TF2Shop         json.RawMessage `json:"tf2shop"`         // TF2 shop details
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on ItemInfo.
func (i *ItemInfo) UnmarshalJSON(data []byte) error {
	type Alias ItemInfo

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*i = ItemInfo(aux)
	i.Name = strings.TrimRight(i.Name, " ;")
	i.Effect = strings.TrimRight(i.Effect, " ;")
	i.SKU = strings.TrimRight(i.SKU, " ;")
	i.Quality = strings.TrimRight(i.Quality, " ;")

	i.Type = strings.TrimRight(i.Type, " ;")
	if i.TypeSteam != nil {
		trimmed := strings.TrimRight(*i.TypeSteam, " ;")
		i.TypeSteam = &trimmed
	}

	if i.Class != nil {
		trimmed := strings.TrimRight(*i.Class, " ;")
		i.Class = &trimmed
	}

	i.Image = strings.TrimRight(i.Image, " ;")
	i.Rarity = strings.TrimRight(i.Rarity, " ;")
	i.Color = strings.TrimRight(i.Color, " ;")

	i.Slot = strings.TrimRight(i.Slot, " ;")
	if i.Hero != nil {
		trimmed := strings.TrimRight(*i.Hero, " ;")
		i.Hero = &trimmed
	}

	i.Weapon = strings.TrimRight(i.Weapon, " ;")
	i.Exterior = strings.TrimRight(i.Exterior, " ;")

	i.URL = strings.TrimRight(i.URL, " ;")
	if i.Description != nil {
		trimmed := strings.TrimRight(*i.Description, " ;")
		i.Description = &trimmed
	}

	return nil
}

// ItemDetails contains the catalog information for a queried item.
type ItemDetails struct {
	Informations ItemInfo `json:"informations"`
}

// SalesGraphValue contains daily or weekly historical sales summary for chart points.
type SalesGraphValue struct {
	// Daily fields (returned for periods 1M, 3M, 6M, 1Y)
	Date  string `json:"date,omitempty"`  // Timestamp of sales (YYYY-MM-DD HH:MM:SS)
	Price int    `json:"price,omitempty"` // Total value in cents sold on this day
	Nb    int    `json:"nb,omitempty"`    // Number of sales on this day

	// Weekly fields (returned for periods 5Y, ALL)
	FirstDate  string `json:"first_date,omitempty"`  // Monday start of the week (YYYY-MM-DD)
	Week       string `json:"week,omitempty"`        // Week identifier (YYYYWW)
	TotalPrice int    `json:"total_price,omitempty"` // Total value in cents sold this week
	TotalNb    int    `json:"total_nb,omitempty"`    // Number of sales this week
}

// ItemSalesGraph represents sales graph history response.
type ItemSalesGraph struct {
	Values []SalesGraphValue `json:"values"`
}

// ListingCount holds the count of active marketplace listings.
type ListingCount struct {
	Count int `json:"count"`
}

// Listing represents a seller listing in the marketplace.
type Listing struct {
	ID           int             `json:"id"`           // Marketplace row ID
	AssetID      string          `json:"assetId"`      // Steam asset ID
	ItemID       int             `json:"item_id"`      // Item type ID
	User         string          `json:"user"`         // Seller Steam ID (omitted for non-admin requests)
	State        int             `json:"state"`        // Item state (1 = on sale)
	Price        int             `json:"price"`        // Market price in cents
	Bot          string          `json:"bot"`          // Holding bot Steam ID
	Game         int             `json:"game"`         // Game ID
	Wear         *float64        `json:"wear"`         // Wear float (CS2)
	Sheen        string          `json:"sheen"`        // Killstreak Sheen effect (TF2)
	Killstreaker string          `json:"killstreaker"` // Active Killstreaker effect (TF2)
	Spell        string          `json:"spell"`        // Active Halloween spells (TF2)
	Parts        string          `json:"parts"`        // Applied Strange Parts (TF2)
	HTML         string          `json:"html"`         // Custom seller description HTML
	Paint        string          `json:"paint"`        // Paint hex color (TF2)
	Values       json.RawMessage `json:"values"`       // CS2 inspection or float attributes
	GetImage     *string         `json:"getImage"`     // Override image URL
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on Listings.
func (l *Listing) UnmarshalJSON(data []byte) error {
	type Alias Listing

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*l = Listing(aux)
	l.AssetID = strings.TrimRight(l.AssetID, " ;")
	l.User = strings.TrimRight(l.User, " ;")
	l.Bot = strings.TrimRight(l.Bot, " ;")
	l.Sheen = strings.TrimRight(l.Sheen, " ;")
	l.Killstreaker = strings.TrimRight(l.Killstreaker, " ;")
	l.Spell = strings.TrimRight(l.Spell, " ;")
	l.Parts = strings.TrimRight(l.Parts, " ;")
	l.HTML = strings.TrimRight(l.HTML, " ;")

	l.Paint = strings.TrimRight(l.Paint, " ;")
	if l.GetImage != nil {
		trimmed := strings.TrimRight(*l.GetImage, " ;")
		l.GetImage = &trimmed
	}

	return nil
}

// BuyOrderTier represents a single buy order grouping at a specific price point.
type BuyOrderTier struct {
	Count int `json:"count"` // Total active buy orders matching this price
	Price int `json:"price"` // Buy order price in cents
}

// BuyOrderList maps active buy orders by grouping tiers.
// Keys are strings "0" through "4", representing the top 5 highest price points,
// with remaining buy orders accumulated under the "more" key.
type BuyOrderList struct {
	Informations map[string]BuyOrderTier `json:"informations"`
}

// LastSale records the price and timestamp of the latest successful checkout.
type LastSale struct {
	Price float64 `json:"price"` // Sale price in cents
	Date  int64   `json:"date"`  // Unix timestamp of transaction
}

// PricingDetail outlines pricing thresholds for an item.
type PricingDetail struct {
	LowestSalePrice float64   `json:"lowest_sale_price"` // Price of the cheapest listing on sale (cents)
	LowestBuyOrder  float64   `json:"lowest_buy_order"`  // Price of the highest buy order (cents)
	SteamPrice      float64   `json:"steam_price"`       // Steam community market price (cents)
	SuggestedPrice  float64   `json:"suggested_price"`   // Platform suggested price (cents)
	LastSale        *LastSale `json:"last_sale"`         // Most recent sale, if available
}

// ItemPricing represents pricing data.
type ItemPricing struct {
	ItemID      int           `json:"item_id"`      // Item ID
	Cached      bool          `json:"cached"`       // True if served from caching layers
	LastUpdated string        `json:"last_updated"` // Calculation timestamp (Y-m-d H:i:s)
	Pricing     PricingDetail `json:"pricing"`      // Pricing breakdown details
}

// BulkPricingItem wraps item pricing metadata for bulk queries.
type BulkPricingItem struct {
	ItemID      int           `json:"item_id"`      // Item ID
	FromCache   bool          `json:"from_cache"`   // True if row was read from cache (5-minute window)
	LastUpdated string        `json:"last_updated"` // Persistent cache timestamp (Y-m-d H:i:s)
	Pricing     PricingDetail `json:"pricing"`      // Pricing details
}

// BulkPricing is the response for a bulk pricing inquiry.
type BulkPricing struct {
	TotalItems     int               `json:"total_items"`     // Total items processed
	CachedItems    int               `json:"cached_items"`    // Sum of items served from cache
	RefreshedItems int               `json:"refreshed_items"` // Sum of items recalculated
	Items          []BulkPricingItem `json:"items"`           // List of pricing data per item
}

// BackpackItemDetails contains full properties of a TF2 or CS2 inventory item.
type BackpackItemDetails struct {
	ID           int             `json:"id"`           // Database entry ID
	AssetID      string          `json:"assetId"`      // Steam asset ID
	User         string          `json:"user"`         // Owner's Steam ID
	ItemID       int             `json:"item_id"`      // Item type ID
	Price        *int            `json:"price"`        // Sale price in cents (null if not listed)
	State        int             `json:"state"`        // Item state (0 = inventory, 1 = sale, 2 = withdrawing, 3 = active trade)
	Bot          string          `json:"bot"`          // Steam ID of the holding bot
	Game         int             `json:"game"`         // Game App ID
	Wear         *float64        `json:"wear"`         // CS2 wear float
	Sheen        string          `json:"sheen"`        // TF2 Sheen
	Killstreaker string          `json:"killstreaker"` // TF2 Killstreaker
	Spell        string          `json:"spell"`        // TF2 Spells
	Parts        *string         `json:"parts"`        // TF2 Strange Parts
	Paint        *string         `json:"paint"`        // TF2 Paint
	LastUpdate   int64           `json:"lastupdate"`   // Unix timestamp of last update
	Name         string          `json:"name"`         // Item display name
	Effect       string          `json:"effect"`       // Unusual effect name
	Quality      string          `json:"quality"`      // Quality label
	Type         string          `json:"type"`         // Item category (e.g. Weapon, Hat)
	Craftable    Craftable       `json:"craftable"`    // Craftability representation
	Image        string          `json:"image"`        // Image URL
	Rarity       string          `json:"rarity"`       // Item rarity tier
	URL          string          `json:"url"`          // Slug URL identifier
	Color        string          `json:"color"`        // Hex color value
	Values       json.RawMessage `json:"values"`       // CS2 sticker, pattern, or float metrics
	GetImage     *int            `json:"getImage"`     // Custom image index
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on BackpackItemDetails.
func (b *BackpackItemDetails) UnmarshalJSON(data []byte) error {
	type Alias BackpackItemDetails

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*b = BackpackItemDetails(aux)
	b.AssetID = strings.TrimRight(b.AssetID, " ;")
	b.User = strings.TrimRight(b.User, " ;")
	b.Bot = strings.TrimRight(b.Bot, " ;")
	b.Sheen = strings.TrimRight(b.Sheen, " ;")
	b.Killstreaker = strings.TrimRight(b.Killstreaker, " ;")

	b.Spell = strings.TrimRight(b.Spell, " ;")
	if b.Parts != nil {
		trimmed := strings.TrimRight(*b.Parts, " ;")
		b.Parts = &trimmed
	}

	if b.Paint != nil {
		trimmed := strings.TrimRight(*b.Paint, " ;")
		b.Paint = &trimmed
	}

	b.Name = strings.TrimRight(b.Name, " ;")
	b.Effect = strings.TrimRight(b.Effect, " ;")
	b.Quality = strings.TrimRight(b.Quality, " ;")
	b.Type = strings.TrimRight(b.Type, " ;")
	b.Image = strings.TrimRight(b.Image, " ;")
	b.Rarity = strings.TrimRight(b.Rarity, " ;")
	b.URL = strings.TrimRight(b.URL, " ;")
	b.Color = strings.TrimRight(b.Color, " ;")

	return nil
}

// BackpackDetailsResponse wraps backpack item informations.
type BackpackDetailsResponse struct {
	Informations BackpackItemDetails `json:"informations"`
}

// GetItemDetails queries the items table and returns full metadata details.
//
// Route: GET /item/details/{item}
// Permission: API Only (No session required)
func (c *Client) GetItemDetails(ctx context.Context, item string) (*ItemDetails, error) {
	return aoni.GetJSON[ItemDetails](ctx, c.getClient(), "/item/details/{item}", aoni.WithVar("item", item))
}

// SalesGraphReq contains query parameters for GetItemSalesGraph.
type SalesGraphReq struct {
	Period string `url:"period"` // 1M, 3M, 6M, 1Y (daily data points) or 5Y, ALL (weekly grouped data points)
}

// GetItemSalesGraph returns historical sales graph trends for charts.
// Groupings (daily or weekly) are automatically selected based on the period query.
//
// Route: GET /item/salesGraph/{item}?period={period}
// Permission: Connected + API
func (c *Client) GetItemSalesGraph(ctx context.Context, item, period string) (*ItemSalesGraph, error) {
	req := SalesGraphReq{Period: period}

	return aoni.GetJSON[ItemSalesGraph](
		ctx, c.getClient(), "/item/salesGraph/{item}",
		aoni.WithVar("item", item),
		aoni.WithQuery(req),
	)
}

// GetListingCount returns the number of active sales listings for an item,
// optionally filtered by the seller's SteamID.
//
// Route: GET /item/listing/count/{item} or GET /item/listing/count/{item}/{userid}
// Permission: API Only
func (c *Client) GetListingCount(ctx context.Context, item, userID string) (*ListingCount, error) {
	path := "/item/listing/count/{item}"

	var opts []aoni.RequestModifier

	opts = append(opts, aoni.WithVar("item", item))
	if userID != "" {
		path = "/item/listing/count/{item}/{userid}"

		opts = append(opts, aoni.WithVar("userid", userID))
	}

	return aoni.GetJSON[ListingCount](ctx, c.getClient(), path, opts...)
}

// ListingsReq holds query arguments for GetItemListings.
type ListingsReq struct {
	Count int `url:"count,omitempty"` // Page size (default 10)
	Page  int `url:"page,omitempty"`  // Page index (default 0)
	Game  int `url:"game,omitempty"`  // Game ID filter (e.g. 440 or 730)
}

// GetItemListings fetches marketplace active listings sorted by price ascending.
// Listings are optionally filtered by seller SteamID. Seller identities and IP addresses
// are automatically redacted for public callers.
//
// Route: GET /item/listing/{item} or GET /item/listing/{item}/{userid}
// Permission: API Only
func (c *Client) GetItemListings(ctx context.Context, item, userID string, query ListingsReq) ([]Listing, error) {
	path := "/item/listing/{item}"

	var opts []aoni.RequestModifier

	opts = append(opts, aoni.WithVar("item", item))
	if userID != "" {
		path = "/item/listing/{item}/{userid}"

		opts = append(opts, aoni.WithVar("userid", userID))
	}

	opts = append(opts, aoni.WithQuery(query))

	resp, err := aoni.GetJSON[[]Listing](ctx, c.getClient(), path, opts...)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetBuyOrderList retrieves active buy orders grouped by pricing tier keys (0-4 and "more").
//
// Route: GET /item/buyorderList/{item}
// Permission: API Only
func (c *Client) GetBuyOrderList(ctx context.Context, item string) (*BuyOrderList, error) {
	return aoni.GetJSON[BuyOrderList](ctx, c.getClient(), "/item/buyorderList/{item}", aoni.WithVar("item", item))
}

// GetItemPricing returns calculated lowest sale, highest buy order, Steam price,
// suggested price, and optional last sale price. Results are cached for 20 minutes.
//
// Route: GET /item/pricing/{item}
// Permission: Connected + API
func (c *Client) GetItemPricing(ctx context.Context, item string) (*ItemPricing, error) {
	return aoni.GetJSON[ItemPricing](ctx, c.getClient(), "/item/pricing/{item}", aoni.WithVar("item", item))
}

// BulkPricingReq contains query parameters for GetBulkPricing.
type BulkPricingReq struct {
	Items string `url:"items"` // Comma-separated list of item definition IDs (max 100)
}

// GetBulkPricing returns calculated pricing for multiple items at once.
// Cached items are re-served if updated within the last 5 minutes.
//
// Route: GET /item/pricing/bulk?items=...
// Permission: Connected + API
func (c *Client) GetBulkPricing(ctx context.Context, items []string) (*BulkPricing, error) {
	req := BulkPricingReq{
		Items: strings.Join(items, ","),
	}

	return aoni.GetJSON[BulkPricing](ctx, c.getClient(), "/item/pricing/bulk", aoni.WithQuery(req))
}

// GetBackpackDetailsTF2 queries full item details and backpack metrics for a TF2 asset ID.
//
// Route: GET /item/details/fromid/{backpackid}
// Permission: API Only
func (c *Client) GetBackpackDetailsTF2(ctx context.Context, backpackID string) (*BackpackDetailsResponse, error) {
	return aoni.GetJSON[BackpackDetailsResponse](
		ctx, c.getClient(), "/item/details/fromid/{backpackid}",
		aoni.WithVar("backpackid", backpackID),
	)
}

// GetBackpackDetailsCS2 queries full details (including wear and sticker layers) for a CS2 asset ID.
//
// Route: GET /item/cs/details/fromid/{backpackid}
// Permission: API Only
func (c *Client) GetBackpackDetailsCS2(ctx context.Context, backpackID string) (*BackpackDetailsResponse, error) {
	return aoni.GetJSON[BackpackDetailsResponse](
		ctx, c.getClient(), "/item/cs/details/fromid/{backpackid}",
		aoni.WithVar("backpackid", backpackID),
	)
}
