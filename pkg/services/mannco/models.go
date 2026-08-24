// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mannco

import (
	"errors"
	"strings"

	"github.com/lemon4ksan/foundation/codec/json"
)

// BaseResponse represents the standard Mannco.store API response envelope.
type BaseResponse struct {
	Err     bool            `json:"err"`
	Success bool            `json:"success"`
	Content json.RawMessage `json:"content"`
	target  any
}

// IsSuccess checks if the response indicates a successful operation.
func (b *BaseResponse) IsSuccess() bool {
	return b.Success
}

// Error formats and returns the error details from the Content payload.
func (b *BaseResponse) Error() error {
	errMsg, err := json.Marshal(b.Content)
	if err != nil {
		return errors.New("unknown error")
	}

	return errors.New(string(errMsg))
}

// SetData binds the output destination struct target for automatic unmarshaling.
func (b *BaseResponse) SetData(data any) {
	b.target = data
}

// UnmarshalJSON handles decoding the outer envelope and automatically unmarshals
// the Content field into the configured target destination.
func (b *BaseResponse) UnmarshalJSON(data []byte) error {
	type Alias BaseResponse

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	b.Err = aux.Err
	b.Success = aux.Success

	b.Content = aux.Content
	if b.target != nil {
		if err := json.Unmarshal(b.Content, b.target); err != nil {
			return err
		}
	}

	return nil
}

// Craftable represents the craftability state of a TF2 item (1 = craftable, 0 = uncraftable).
type Craftable int

// UnmarshalJSON parses both integer representations and fallback string values ("").
func (c *Craftable) UnmarshalJSON(data []byte) error {
	var val any
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}

	switch v := val.(type) {
	case float64:
		*c = Craftable(int(v))
	case int:
		*c = Craftable(v)
	case string:
		*c = 0
	default:
		*c = 0
	}

	return nil
}

// LoginRequest represents request payload to authenticate with an API key.
// @aoni:dto casing=camel_case omitempty=true
type LoginRequest struct {
	APIKey string `json:"apiKey"`
}

// LoginContent represents JWT token response.
// @aoni:dto casing=camel_case omitempty=true
type LoginContent struct {
	JWT string `json:"jwt"`
}

// DisconnectResponse outlines session invalidation results.
// @aoni:dto casing=camel_case omitempty=true
type DisconnectResponse struct {
	Disconnect bool `json:"disconnect"`
}

// UserInfo represents authenticated user profile parameters.
// @aoni:dto casing=camel_case omitempty=true
type UserInfo struct {
	SteamID      string          `json:"steamId"`
	Balance      int             `json:"balance"`
	TradeURL     string          `json:"tradeurl"`
	Name         string          `json:"name"`
	Image        string          `json:"image"`
	TwoFA        string          `json:"2fa"`
	ShortURL     string          `json:"shorturl"`
	Notification json.RawMessage `json:"notification"`
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on UserInfo.
func (ui *UserInfo) UnmarshalJSON(data []byte) error {
	type Alias UserInfo

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*ui = UserInfo(aux)
	ui.SteamID = strings.TrimRight(ui.SteamID, " ;")
	ui.TradeURL = strings.TrimRight(ui.TradeURL, " ;")
	ui.Name = strings.TrimRight(ui.Name, " ;")
	ui.Image = strings.TrimRight(ui.Image, " ;")
	ui.TwoFA = strings.TrimRight(ui.TwoFA, " ;")
	ui.ShortURL = strings.TrimRight(ui.ShortURL, " ;")

	return nil
}

// UserInfoResponse wraps user account details.
// @aoni:dto casing=camel_case omitempty=true
type UserInfoResponse struct {
	Informations UserInfo `json:"informations"`
}

// UserBalanceResponse wraps account balance.
// @aoni:dto casing=camel_case omitempty=true
type UserBalanceResponse struct {
	Balance int `json:"balance"`
}

// NotificationsResponse summarizes unread notifications count.
// @aoni:dto casing=camel_case omitempty=true
type NotificationsResponse struct {
	Alerts              []json.RawMessage `json:"alerts"`
	AlertCount          int               `json:"alertCount"`
	PaymentReviews      []json.RawMessage `json:"paymentReviews"`
	PaymentReviewsCount int               `json:"paymentReviewsCount"`
	Messages            []json.RawMessage `json:"messages"`
	MessagesCount       int               `json:"messagesCount"`
	Offers              []json.RawMessage `json:"offers"`
	OffersCount         int               `json:"offersCount"`
	Trades              []json.RawMessage `json:"trades"`
	TradesCount         int               `json:"tradesCount"`
}

// SessionRow represents user login session details.
// @aoni:dto casing=camel_case omitempty=true
type SessionRow struct {
	IP        string `json:"ip"`
	Location  string `json:"location"`
	ISP       string `json:"isp"`
	Current   int    `json:"current"`
	Timestamp int64  `json:"timestamp"`
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on SessionRow.
func (s *SessionRow) UnmarshalJSON(data []byte) error {
	type Alias SessionRow

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*s = SessionRow(aux)
	s.IP = strings.TrimRight(s.IP, " ;")
	s.Location = strings.TrimRight(s.Location, " ;")
	s.ISP = strings.TrimRight(s.ISP, " ;")

	return nil
}

// IPSessionListResponse represents active and historical login sessions.
// @aoni:dto casing=camel_case omitempty=true
type IPSessionListResponse struct {
	Values []SessionRow `json:"values"`
	Count  int          `json:"count"`
}

// IPSessionListQuery holds filtering queries for GetIPSessionList.
// @aoni:dto casing=camel_case omitempty=true
type IPSessionListQuery struct {
	Page    int    `url:"page,omitempty"`
	PerPage int    `url:"perPage,omitempty"`
	Expire  string `url:"expire,omitempty"`
}

// PublicStoreProfile contains public metadata for user store lookup.
// @aoni:dto casing=camel_case omitempty=true
type PublicStoreProfile struct {
	SteamID string `json:"steamId"`
	Image   string `json:"image"`
	Name    string `json:"name"`
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on PublicStoreProfile.
func (p *PublicStoreProfile) UnmarshalJSON(data []byte) error {
	type Alias PublicStoreProfile

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*p = PublicStoreProfile(aux)
	p.SteamID = strings.TrimRight(p.SteamID, " ;")
	p.Image = strings.TrimRight(p.Image, " ;")
	p.Name = strings.TrimRight(p.Name, " ;")

	return nil
}

// SalesInfosResponse summarizes sale metrics.
type SalesInfosResponse map[string]any

// SalesChartQuery holds query parameters for GetSalesChartInfos.
// @aoni:dto casing=snake_case omitempty=true
type SalesChartQuery struct {
	Period    string `url:"period,omitempty"`
	Chart     any    `url:"chart,omitempty"`
	ChartOnly any    `url:"chart_only,omitempty"`
}

// SalesChartResponse contains chart data points.
type SalesChartResponse map[string]any

// BalanceHistoryRow represents balance history entry.
type BalanceHistoryRow map[string]any

// BalanceHistoryResponse represents list of balance deposits, withdrawals, and updates.
// @aoni:dto casing=camel_case omitempty=true
type BalanceHistoryResponse struct {
	Values []BalanceHistoryRow `json:"values"`
	Count  int                 `json:"count,omitempty"`
}

// BalanceHistoryQuery holds query parameters for GetBalanceHistory.
// @aoni:dto casing=camel_case omitempty=true
type BalanceHistoryQuery struct {
	Page  int `url:"page,omitempty"`
	Limit int `url:"limit,omitempty"`
}

// PurchaseHistoryRow represents purchase history entry.
type PurchaseHistoryRow map[string]any

// PurchaseHistoryResponse represents purchase checkouts logs.
// @aoni:dto casing=camel_case omitempty=true
type PurchaseHistoryResponse struct {
	Values []PurchaseHistoryRow `json:"values"`
	Count  int                  `json:"count,omitempty"`
}

// PurchaseHistoryQuery holds query parameters for GetPurchaseHistory.
// @aoni:dto casing=camel_case omitempty=true
type PurchaseHistoryQuery struct {
	Page  int `url:"page,omitempty"`
	Count int `url:"count,omitempty"`
}

// SalesHistoryQuery holds search and paging parameters for GetSalesHistory.
// @aoni:dto casing=camel_case omitempty=true
type SalesHistoryQuery struct {
	Page    int    `url:"page,omitempty"`
	PerPage int    `url:"perPage,omitempty"`
	Range   string `url:"range,omitempty"`
	Search  string `url:"search,omitempty"`
}

// SalesHistoryRow represents item sales checkout entry.
type SalesHistoryRow map[string]any

// SalesHistoryResponse represents listings sales transactions.
// @aoni:dto casing=camel_case omitempty=true
type SalesHistoryResponse struct {
	Values []SalesHistoryRow `json:"values"`
	Count  int               `json:"count,omitempty"`
}

// CashoutHistoryRow represents cashout transaction log.
type CashoutHistoryRow map[string]any

// CashoutHistoryResponse represents balance cashouts history.
// @aoni:dto casing=camel_case omitempty=true
type CashoutHistoryResponse struct {
	Values []CashoutHistoryRow `json:"values"`
	Count  int                 `json:"count,omitempty"`
}

// CashoutHistoryQuery holds paging configuration for GetCashoutHistory.
// @aoni:dto casing=camel_case omitempty=true
type CashoutHistoryQuery struct {
	Page    int `url:"page,omitempty"`
	Count   int `url:"count,omitempty"`
	PerPage int `url:"perpage,omitempty"`
	Limit   int `url:"limit,omitempty"`
}

// TransactionHistoryRow represents general ledger transaction.
type TransactionHistoryRow map[string]any

// TransactionHistoryResponse represents ledger details.
// @aoni:dto casing=camel_case omitempty=true
type TransactionHistoryResponse struct {
	Values []TransactionHistoryRow `json:"values"`
	Count  int                     `json:"count,omitempty"`
}

// TransactionHistoryQuery holds query configurations for GetTransactionHistory.
// @aoni:dto casing=camel_case omitempty=true
type TransactionHistoryQuery struct {
	Page   int    `url:"page,omitempty"`
	Limit  int    `url:"limit,omitempty"`
	Search string `url:"search,omitempty"`
}

// TransactionDetailsQuery represents query params for GetTransactionDetails.
// @aoni:dto casing=camel_case omitempty=true
type TransactionDetailsQuery struct {
	TransactionID string `url:"transactionId"`
}

// TransactionDetailsResponse wraps ledger entry transaction details.
// @aoni:dto casing=camel_case omitempty=true
type TransactionDetailsResponse struct {
	Transaction json.RawMessage `json:"transaction"`
}

// CryptoDepositHistoryRow represents crypto ledger log.
type CryptoDepositHistoryRow map[string]any

// CryptoDepositHistoryResponse represents cryptocurrency top-ups logs.
// @aoni:dto casing=camel_case omitempty=true
type CryptoDepositHistoryResponse struct {
	Values []CryptoDepositHistoryRow `json:"values"`
	Count  int                       `json:"count,omitempty"`
}

// CryptoDepositHistoryQuery holds query parameters for GetCryptoDepositHistory.
// @aoni:dto casing=camel_case omitempty=true
type CryptoDepositHistoryQuery struct {
	Page    int `url:"page,omitempty"`
	Count   int `url:"count,omitempty"`
	PerPage int `url:"perpage,omitempty"`
	Limit   int `url:"limit,omitempty"`
}

// ItemInfo represents the metadata of a catalog item in the database.
// @aoni:dto casing=camel_case omitempty=true
type ItemInfo struct {
	ID              int             `json:"id"`
	Name            string          `json:"name"`
	Effect          string          `json:"effect"`
	SKU             string          `json:"SKU"`
	Quality         string          `json:"quality"`
	Type            string          `json:"type"`
	TypeSteam       *string         `json:"type_steam"`
	Class           *string         `json:"class"`
	Image           string          `json:"image"`
	ImagePertinence int             `json:"imagePertinence"`
	Craftable       int             `json:"craftable"`
	Rarity          string          `json:"rarity"`
	Featured        int             `json:"featured"`
	Deal            int             `json:"deal"`
	T               json.RawMessage `json:"t"`
	Color           string          `json:"color"`
	Game            int             `json:"game"`
	Slot            string          `json:"slot"`
	Hero            *string         `json:"hero"`
	Weapon          string          `json:"weapon"`
	Exterior        string          `json:"exterior"`
	URL             string          `json:"url"`
	Description     *string         `json:"description"`
	TF2Shop         json.RawMessage `json:"tf2shop"`
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
// @aoni:dto casing=camel_case omitempty=true
type ItemDetails struct {
	Informations ItemInfo `json:"informations"`
}

// SalesGraphValue contains daily or weekly historical sales summary for chart points.
// @aoni:dto casing=snake_case omitempty=true
type SalesGraphValue struct {
	Day        string   `json:"day,omitempty"`
	Week       string   `json:"week,omitempty"`
	Median     *float64 `json:"median,omitempty"`
	Sold       *int     `json:"sold,omitempty"`
	Mean       *float64 `json:"mean,omitempty"`
	Percentile *float64 `json:"percentile,omitempty"`
}

// ItemSalesGraph contains historical sales summary chart points.
// @aoni:dto casing=snake_case omitempty=true
type ItemSalesGraph struct {
	Period string            `json:"period,omitempty"`
	Values []SalesGraphValue `json:"values"`
}

// SalesGraphReq contains query parameters for GetItemSalesGraph.
// @aoni:dto casing=snake_case omitempty=true
type SalesGraphReq struct {
	Period string `url:"period"`
}

// ListingCount returns the number of active sales listings for an item.
// @aoni:dto casing=camel_case omitempty=true
type ListingCount struct {
	Count int `json:"count"`
}

// Listing represents a user marketplace listing for an item.
// @aoni:dto casing=camel_case omitempty=true
type Listing struct {
	ID           int64           `json:"id"`
	AssetID      string          `json:"assetid"`
	Price        int             `json:"price"`
	Created      int64           `json:"created"`
	State        int             `json:"state"`
	Time         int64           `json:"time"`
	User         string          `json:"user"`
	Name         string          `json:"name"`
	Effect       string          `json:"effect"`
	Quality      string          `json:"quality"`
	Type         string          `json:"type"`
	Craftable    Craftable       `json:"craftable"`
	Image        string          `json:"image"`
	Rarity       string          `json:"rarity"`
	URL          string          `json:"url"`
	Color        string          `json:"color"`
	Values       json.RawMessage `json:"values"`
	Bot          *string         `json:"bot"`
	Sheen        string          `json:"sheen"`
	Killstreaker string          `json:"killstreaker"`
	Spell        string          `json:"spell"`
	Parts        *string         `json:"parts"`
	HTML         string          `json:"html"`
	Paint        *string         `json:"paint"`
	GetImage     *string         `json:"getImage"`
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on Listing.
func (l *Listing) UnmarshalJSON(data []byte) error {
	type Alias Listing

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*l = Listing(aux)
	l.AssetID = strings.TrimRight(l.AssetID, " ;")
	l.User = strings.TrimRight(l.User, " ;")
	if l.Bot != nil {
		trimmed := strings.TrimRight(*l.Bot, " ;")
		l.Bot = &trimmed
	}
	l.Sheen = strings.TrimRight(l.Sheen, " ;")
	l.Killstreaker = strings.TrimRight(l.Killstreaker, " ;")
	l.Spell = strings.TrimRight(l.Spell, " ;")
	if l.Parts != nil {
		trimmed := strings.TrimRight(*l.Parts, " ;")
		l.Parts = &trimmed
	}
	l.HTML = strings.TrimRight(l.HTML, " ;")

	if l.Paint != nil {
		trimmed := strings.TrimRight(*l.Paint, " ;")
		l.Paint = &trimmed
	}
	if l.GetImage != nil {
		trimmed := strings.TrimRight(*l.GetImage, " ;")
		l.GetImage = &trimmed
	}

	return nil
}

// ListingsReq holds query arguments for GetItemListings.
// @aoni:dto casing=camel_case omitempty=true
type ListingsReq struct {
	Count int `url:"count,omitempty"`
	Page  int `url:"page,omitempty"`
	Game  int `url:"game,omitempty"`
}

// BuyOrderTier represents a single buy order grouping at a specific price point.
// @aoni:dto casing=camel_case omitempty=true
type BuyOrderTier struct {
	Count int `json:"count"`
	Price int `json:"price"`
}

// BuyOrderList maps active buy orders by grouping tiers.
// @aoni:dto casing=camel_case omitempty=true
type BuyOrderList struct {
	Informations map[string]BuyOrderTier `json:"informations"`
}

// LastSale records the price and timestamp of the latest successful checkout.
// @aoni:dto casing=camel_case omitempty=true
type LastSale struct {
	Price float64 `json:"price"`
	Date  int64   `json:"date"`
}

// PricingDetail outlines pricing thresholds for an item.
// @aoni:dto casing=snake_case omitempty=true
type PricingDetail struct {
	LowestSalePrice float64   `json:"lowest_sale_price"`
	LowestBuyOrder  float64   `json:"lowest_buy_order"`
	SteamPrice      float64   `json:"steam_price"`
	SuggestedPrice  float64   `json:"suggested_price"`
	LastSale        *LastSale `json:"last_sale"`
}

// ItemPricing represents calculated prices for an item.
// @aoni:dto casing=snake_case omitempty=true
type ItemPricing struct {
	ItemID      int           `json:"item_id"`
	Cached      bool          `json:"cached"`
	LastUpdated string        `json:"last_updated"`
	Pricing     PricingDetail `json:"pricing"`
}

// BulkPricingItem represents pricing details for a single item within bulk results.
// @aoni:dto casing=snake_case omitempty=true
type BulkPricingItem struct {
	ItemID      int           `json:"item_id"`
	FromCache   bool          `json:"from_cache"`
	LastUpdated string        `json:"last_updated"`
	Pricing     PricingDetail `json:"pricing"`
}

// BulkPricing maps catalog item IDs to their respective pricing details.
// @aoni:dto casing=snake_case omitempty=true
type BulkPricing struct {
	TotalItems     int               `json:"total_items"`
	CachedItems    int               `json:"cached_items"`
	RefreshedItems int               `json:"refreshed_items"`
	Items          []BulkPricingItem `json:"items"`
}

// BulkPricingReq contains query parameters for GetBulkPricing.
// @aoni:dto casing=camel_case omitempty=true
type BulkPricingReq struct {
	Items string `url:"items"`
}

// BackpackItemDetails contains full properties of a TF2 or CS2 inventory item.
// @aoni:dto casing=camel_case omitempty=true
type BackpackItemDetails struct {
	ID           int             `json:"id"`
	AssetID      string          `json:"assetId"`
	User         string          `json:"user"`
	ItemID       int             `json:"item_id"`
	Price        *int            `json:"price"`
	State        int             `json:"state"`
	Bot          string          `json:"bot"`
	Game         int             `json:"game"`
	Wear         *float64        `json:"wear"`
	Sheen        string          `json:"sheen"`
	Killstreaker string          `json:"killstreaker"`
	Spell        string          `json:"spell"`
	Parts        *string         `json:"parts"`
	Paint        *string         `json:"paint"`
	LastUpdate   int64           `json:"lastupdate"`
	Name         string          `json:"name"`
	Effect       string          `json:"effect"`
	Quality      string          `json:"quality"`
	Type         string          `json:"type"`
	Craftable    Craftable       `json:"craftable"`
	Image        string          `json:"image"`
	Rarity       string          `json:"rarity"`
	URL          string          `json:"url"`
	Color        string          `json:"color"`
	Values       json.RawMessage `json:"values"`
	GetImage     *int            `json:"getImage"`
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
// @aoni:dto casing=camel_case omitempty=true
type BackpackDetailsResponse struct {
	Informations BackpackItemDetails `json:"informations"`
}

// InventoryItem represents a user's backpack item in inventory or on sale.
// @aoni:dto casing=camel_case omitempty=true
type InventoryItem struct {
	IDs       string    `json:"ids"`
	Count     int       `json:"count"`
	ItemID    int       `json:"item_id"`
	AssetID   string    `json:"assetId"`
	Bot       string    `json:"bot"`
	Game      int       `json:"game"`
	State     int       `json:"state"`
	Price     int       `json:"price"`
	Name      string    `json:"name"`
	Effect    string    `json:"effect"`
	URL       string    `json:"url"`
	Quality   string    `json:"quality"`
	Image     string    `json:"image"`
	Type      string    `json:"type"`
	Craftable Craftable `json:"craftable"`
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
// @aoni:dto casing=camel_case omitempty=true
type GetInventoryResponse struct {
	Items []InventoryItem `json:"items"`
	Count int             `json:"count"`
}

// SetPriceReq represents request payload to set item pricing.
// @aoni:dto casing=camel_case omitempty=true
type SetPriceReq struct {
	IDs   string `json:"ids"`
	Price int    `json:"price"`
}

// InventoryMessageResponse holds price set outcome text.
// @aoni:dto casing=camel_case omitempty=true
type InventoryMessageResponse struct {
	Message string `json:"message"`
}

// WithdrawReq represents request payload to withdraw items.
// @aoni:dto casing=camel_case omitempty=true
type WithdrawReq struct {
	IDs string `json:"ids"`
}

// WithdrawResponse outlines successfully withdrawal processing.
// @aoni:dto casing=camel_case omitempty=true
type WithdrawResponse struct {
	Message string `json:"message"`
	Updated int    `json:"updated"`
	Locked  int    `json:"locked"`
}

// CreateBuyOrderReq represents the request body to create a buy order.
// @aoni:dto casing=camel_case omitempty=true
type CreateBuyOrderReq struct {
	ItemID int `json:"itemid"`
	Value  int `json:"value"`
	Amount int `json:"amount"`
}

// UpdateBuyOrderReq represents the request body to update an existing buy order.
// @aoni:dto casing=camel_case omitempty=true
type UpdateBuyOrderReq struct {
	ItemID int `json:"itemid"`
	Value  int `json:"value"`
	Amount int `json:"amount"`
}

// RemoveBuyOrderReq represents the request body to remove a buy order.
// @aoni:dto casing=camel_case omitempty=true
type RemoveBuyOrderReq struct {
	ItemID int `json:"itemid"`
}

// DetailsResponse holds generic response details message.
// @aoni:dto casing=camel_case omitempty=true
type DetailsResponse struct {
	Details string `json:"details"`
}

// UserBuyOrder represents user's active buy order for a specific item.
// @aoni:dto casing=camel_case omitempty=true
type UserBuyOrder struct {
	ID        int    `json:"id"`
	SteamID   string `json:"steamid"`
	ItemID    int    `json:"itemid"`
	Price     int    `json:"price"`
	Amount    int    `json:"amount"`
	Timestamp string `json:"timestamp"`
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
// @aoni:dto casing=camel_case omitempty=true
type UserBuyOrderResponse struct {
	Informations UserBuyOrder `json:"informations"`
}

// UserAllBuyOrdersItem contains information about user's active buy order with joined catalog item details.
// @aoni:dto casing=camel_case omitempty=true
type UserAllBuyOrdersItem struct {
	ID              int             `json:"id"`
	SteamID         string          `json:"steamid"`
	ItemID          int             `json:"itemid"`
	Price           int             `json:"price"`
	Amount          int             `json:"amount"`
	Name            string          `json:"name"`
	Effect          string          `json:"effect"`
	URL             string          `json:"url"`
	Game            int             `json:"game"`
	Quality         string          `json:"quality"`
	Image           string          `json:"image"`
	Type            string          `json:"type"`
	Craftable       Craftable       `json:"craftable"`
	SKU             string          `json:"SKU"`
	TypeSteam       *string         `json:"type_steam"`
	Class           *string         `json:"class"`
	ImagePertinence int             `json:"imagePertinence"`
	Rarity          string          `json:"rarity"`
	Featured        int             `json:"featured"`
	Deal            *int            `json:"deal"`
	Color           string          `json:"color"`
	Slot            string          `json:"slot"`
	Hero            *string         `json:"hero"`
	Weapon          *string         `json:"weapon"`
	Exterior        *string         `json:"exterior"`
	Description     *string         `json:"description"`
	TF2Shop         json.RawMessage `json:"tf2shop"`
	Sheen           string          `json:"sheen"`
	Killstreaker    string          `json:"killstreaker"`
	Spell           string          `json:"spell"`
	Parts           string          `json:"parts"`
	Paint           string          `json:"paint"`
	Level           *int            `json:"level"`
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

	u.Sheen = strings.TrimRight(u.Sheen, " ;")
	u.Killstreaker = strings.TrimRight(u.Killstreaker, " ;")
	u.Spell = strings.TrimRight(u.Spell, " ;")
	u.Parts = strings.TrimRight(u.Parts, " ;")
	u.Paint = strings.TrimRight(u.Paint, " ;")

	return nil
}

// UserAllBuyOrdersCount represents buy orders count.
// @aoni:dto casing=camel_case omitempty=true
type UserAllBuyOrdersCount struct {
	Nb int `json:"nb"`
}

// UserAllBuyOrdersResponse wraps slice of user's active buy orders.
// @aoni:dto casing=camel_case omitempty=true
type UserAllBuyOrdersResponse struct {
	Values []UserAllBuyOrdersItem `json:"values"`
	Count  UserAllBuyOrdersCount  `json:"count"`
}

// GetUserBuyOrdersQuery holds filtering arguments for GetUserBuyOrders.
// @aoni:dto casing=camel_case omitempty=true
type GetUserBuyOrdersQuery struct {
	Page     int    `url:"page,omitempty"`
	Count    int    `url:"count,omitempty"`
	Search   string `url:"search,omitempty"`
	Undercut any    `url:"undercut,omitempty"`
}

// CartItem represents an item row currently inside the user's cart.
// @aoni:dto casing=camel_case omitempty=true
type CartItem struct {
	CartID       int             `json:"cartId"`
	AssetID      string          `json:"assetId"`
	Count        int             `json:"count"`
	ItemID       int             `json:"item_id"`
	Price        int             `json:"price"`
	Name         string          `json:"name"`
	Image        string          `json:"image"`
	Effect       string          `json:"effect"`
	Rarity       string          `json:"rarity"`
	Color        string          `json:"color"`
	URL          string          `json:"url"`
	Quality      string          `json:"quality"`
	TypeSteam    string          `json:"type_steam"`
	Class        string          `json:"class"`
	Craftable    Craftable       `json:"craftable"`
	Slot         string          `json:"slot"`
	Game         int             `json:"game"`
	Sheen        string          `json:"sheen"`
	Killstreaker string          `json:"killstreaker"`
	Spell        string          `json:"spell"`
	Parts        string          `json:"parts"`
	Paint        string          `json:"paint"`
	Level        *int            `json:"level"`
	Festivized   int             `json:"festivized"`
	Inspect      string          `json:"inspect"`
	Values       json.RawMessage `json:"values,omitempty"`
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
// @aoni:dto casing=camel_case omitempty=true
type GetCartResponse struct {
	Cart []CartItem `json:"cart"`
}

// AddCartReq represents request body to add a single item to cart.
// @aoni:dto casing=camel_case omitempty=true
type AddCartReq struct {
	AssetID string `json:"assetId"`
}

// BulkAddCartReq represents request body to add multiple items of the same type in bulk.
// @aoni:dto casing=camel_case omitempty=true
type BulkAddCartReq struct {
	ItemID       int    `json:"itemId"`
	Count        int    `json:"count"`
	SellerUserID string `json:"sellerUserId,omitempty"`
}

// RemoveCartReq represents request body to remove a cart row.
// @aoni:dto casing=camel_case omitempty=true
type RemoveCartReq struct {
	CartID int `json:"cartId"`
}

// InvalidItem lists invalid items found during cart integrity verification.
// @aoni:dto casing=camel_case omitempty=true
type InvalidItem struct {
	CartID        int    `json:"cartId"`
	AssetID       string `json:"assetId"`
	Reason        string `json:"reason"`
	CurrentPrice  *int   `json:"currentPrice,omitempty"`
	ExpectedPrice *int   `json:"expectedPrice,omitempty"`
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
// @aoni:dto casing=camel_case omitempty=true
type IntegrityInfo struct {
	Valid        bool          `json:"valid"`
	InvalidItems []InvalidItem `json:"invalidItems"`
}

// ReplacedItem represents an invalid plain item replaced with an equivalent listing.
// @aoni:dto casing=camel_case omitempty=true
type ReplacedItem struct {
	CartID     int    `json:"cartId"`
	OldAssetID string `json:"oldAssetId"`
	NewAssetID string `json:"newAssetId"`
	Reason     string `json:"reason"`
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
// @aoni:dto casing=camel_case omitempty=true
type RemovedItem struct {
	CartID  int    `json:"cartId"`
	AssetID string `json:"assetId"`
	Reason  string `json:"reason"`
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
// @aoni:dto casing=camel_case omitempty=true
type UpdateCartResponse struct {
	Integrity IntegrityInfo  `json:"integrity"`
	Replaced  []ReplacedItem `json:"replaced"`
	Removed   []RemovedItem  `json:"removed"`
	Cart      []CartItem     `json:"cart"`
}

// DepositInfoItem represents an item in user's Steam inventory with pricing hints and stock limits.
// @aoni:dto casing=snake_case omitempty=true
type DepositInfoItem struct {
	AssetID        string            `json:"assetid"`
	Count          int               `json:"count"`
	MarketHashName string            `json:"market_hash_name"`
	ItemID         int               `json:"item_id"`
	URL            string            `json:"url"`
	DepositKey     map[string]string `json:"depositkey"`
	NbHighStock    int               `json:"nb_high_stock"`
	HighStockLimit int               `json:"high_stock_limit"`
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
// @aoni:dto casing=camel_case omitempty=true
type GetDepositInfoResponse struct {
	Informations []DepositInfoItem `json:"informations"`
}

// CreateDepositTradeReq represents request body to create a deposit trade.
// @aoni:dto casing=camel_case omitempty=true
type CreateDepositTradeReq struct {
	Prices      map[string]int    `json:"prices"`
	DepositKeys map[string]string `json:"depositKeys"`
	Game        int               `json:"game"`
}

// CreateDepositTradeResponse wraps the created trade ID.
// @aoni:dto casing=camel_case omitempty=true
type CreateDepositTradeResponse struct {
	ID int `json:"id"`
}

// CreateInstantSellTradeReq represents request body to create an instant sell trade.
// @aoni:dto casing=snake_case omitempty=true
type CreateInstantSellTradeReq struct {
	Prices        map[string]int    `json:"prices"`
	DepositKeys   map[string]string `json:"depositKeys"`
	Game          int               `json:"game"`
	CashoutMethod string            `json:"cashout_method"`
}

// TradeInfo represents detailed metrics for a deposit trade row.
// @aoni:dto casing=snake_case omitempty=true
type TradeInfo struct {
	ID            int    `json:"id"`
	ItemsReceived string `json:"items_received"`
	ItemsSend     string `json:"items_send"`
	Status        int    `json:"status"`
	User          string `json:"user"`
	Bot           string `json:"bot"`
	Code          string `json:"code"`
	LastError     string `json:"lasterror"`
	OfferID       string `json:"offerid"`
	Timestamp     string `json:"timestamp"`
	Game          int    `json:"game"`
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
// @aoni:dto casing=camel_case omitempty=true
type TradeStatusResponse struct {
	Trade TradeInfo `json:"trade"`
}

// OfferItem represents a summary of item metadata associated with a trade offer.
// @aoni:dto casing=camel_case omitempty=true
type OfferItem struct {
	Name         string          `json:"name"`
	Effect       string          `json:"effect"`
	URL          string          `json:"url"`
	Game         int             `json:"game"`
	Quality      string          `json:"quality"`
	Image        string          `json:"image"`
	Type         string          `json:"type"`
	Craftable    Craftable       `json:"craftable"`
	AssetID      string          `json:"assetId"`
	Wear         *float64        `json:"wear,omitempty"`
	Sheen        string          `json:"sheen,omitempty"`
	Killstreaker string          `json:"killstreaker,omitempty"`
	Spell        string          `json:"spell,omitempty"`
	Parts        json.RawMessage `json:"parts,omitempty"`
	HTML         *string         `json:"html,omitempty"`
	User         string          `json:"user,omitempty"`
	State        *int            `json:"state,omitempty"`
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on OfferItem.
func (oi *OfferItem) UnmarshalJSON(data []byte) error {
	type Alias OfferItem

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*oi = OfferItem(aux)
	oi.Name = strings.TrimRight(oi.Name, " ;")
	oi.Effect = strings.TrimRight(oi.Effect, " ;")
	oi.URL = strings.TrimRight(oi.URL, " ;")
	oi.Quality = strings.TrimRight(oi.Quality, " ;")
	oi.Image = strings.TrimRight(oi.Image, " ;")
	oi.Type = strings.TrimRight(oi.Type, " ;")
	oi.AssetID = strings.TrimRight(oi.AssetID, " ;")
	oi.Sheen = strings.TrimRight(oi.Sheen, " ;")
	oi.Killstreaker = strings.TrimRight(oi.Killstreaker, " ;")
	oi.Spell = strings.TrimRight(oi.Spell, " ;")
	oi.User = strings.TrimRight(oi.User, " ;")

	if oi.HTML != nil {
		trimmed := strings.TrimRight(*oi.HTML, " ;")
		oi.HTML = &trimmed
	}

	return nil
}

// OfferUser represents buyer or seller identity metadata.
// @aoni:dto casing=camel_case omitempty=true
type OfferUser struct {
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on OfferUser.
func (ou *OfferUser) UnmarshalJSON(data []byte) error {
	type Alias OfferUser

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*ou = OfferUser(aux)
	ou.Username = strings.TrimRight(ou.Username, " ;")
	ou.Avatar = strings.TrimRight(ou.Avatar, " ;")

	return nil
}

// Offer represents a marketplace trade offer.
// @aoni:dto casing=camel_case omitempty=true
type Offer struct {
	ID         int         `json:"id"`
	From       string      `json:"from"`
	To         string      `json:"to"`
	Status     OfferStatus `json:"status"`
	Time       int64       `json:"time"`
	Price      int         `json:"price"`
	Read       int         `json:"read"`
	CreatedAt  int64       `json:"createdAt"`
	BackpackID int64       `json:"backpackid"`
	User       OfferUser   `json:"user"`
	Item       OfferItem   `json:"item"`
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on Offer.
func (o *Offer) UnmarshalJSON(data []byte) error {
	type Alias Offer

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*o = Offer(aux)
	o.From = strings.TrimRight(o.From, " ;")
	o.To = strings.TrimRight(o.To, " ;")

	return nil
}

// GetReceivedOffersResponse wraps the received offers slice.
// @aoni:dto casing=camel_case omitempty=true
type GetReceivedOffersResponse struct {
	Offers []Offer `json:"offers"`
}

// CreateOfferReq represents payload to create a trade offer.
// @aoni:dto casing=camel_case omitempty=true
type CreateOfferReq struct {
	ID    int64 `json:"id"`
	Price int   `json:"price"`
}

// OfferMessageResponse holds generic confirmation text from offer actions.
// @aoni:dto casing=camel_case omitempty=true
type OfferMessageResponse struct {
	Message string `json:"message"`
}

// OfferActionReq represents the payload for accept/decline/remove requests.
// @aoni:dto casing=camel_case omitempty=true
type OfferActionReq struct {
	ID int64 `json:"id"`
}

// PaymentReq represents the request body to initiate a checkout session.
// @aoni:dto casing=snake_case omitempty=true
type PaymentReq struct {
	Type          string `json:"type"`
	TOSTimestamp  any    `json:"tos_timestamp"`
	Value         *int   `json:"value,omitempty"`
	Items         string `json:"items,omitempty"`
	PaymentMethod string `json:"payment_method,omitempty"`
}

// PaymentResponse represents payment processing checkout result.
// @aoni:dto casing=camel_case omitempty=true
type PaymentResponse struct {
	URL     string `json:"url,omitempty"`
	Message string `json:"message,omitempty"`
}

// GetTradesResponse wraps a list of user bot trades.
// @aoni:dto casing=camel_case omitempty=true
type GetTradesResponse struct {
	Trades []TradeInfo `json:"trades"`
}

// ResendTradeResponse details outcome from resending a trade offer.
// @aoni:dto casing=snake_case omitempty=true
type ResendTradeResponse struct {
	Message string `json:"message"`
	TradeID int    `json:"trade_id"`
	Code    string `json:"code"`
}

// UnmarshalJSON implements custom unmarshaling to trim trailing characters on ResendTradeResponse.
func (r *ResendTradeResponse) UnmarshalJSON(data []byte) error {
	type Alias ResendTradeResponse

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	*r = ResendTradeResponse(aux)
	r.Message = strings.TrimRight(r.Message, " ;")
	r.Code = strings.TrimRight(r.Code, " ;")

	return nil
}

// ResendTradeQuery holds query arguments for ResendTrade.
// @aoni:dto casing=camel_case omitempty=true
type ResendTradeQuery struct {
	ID int `url:"id"`
}
