// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mannco provides the TF2 Mannco.store API client.
package mannco

import (
	"context"
	"strings"
	"sync"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/option"
)

// BaseURL is the default endpoint host for the Mannco.store API.
const BaseURL = "https://api.mannco.store/"

// Game App IDs supported by Mannco.store API.
const (
	GameIDTF2   = 440    // Team Fortress 2
	GameIDCS2   = 730    // Counter-Strike 2 / CS:GO
	GameIDDota2 = 570    // Dota 2
	GameIDRust  = 252490 // Rust
)

// ItemState defines the sale and lock status of items in user inventory.
type ItemState int

// List of possible item states.
const (
	ItemStateInInventory       ItemState = iota // 0: Available in inventory (Can Sell & Withdraw)
	ItemStateListedForSale                      // 1: Currently listed for sale (Can Sell & Withdraw)
	ItemStatePendingWithdrawal                  // 2: Locked in withdrawal process (Cannot sell/withdraw)
	ItemStateInActiveTrade                      // 3: Locked in active trade queue (Cannot sell/withdraw)
	ItemStateReserved                           // 4: Reserved / in tradehold
)

// OfferStatus defines the state of a trade offer between users.
type OfferStatus int

// List of offer status codes. Auto-validated by GET queries.
const (
	OfferStatusAutoCancelledItemUnavailable OfferStatus = iota - 3 // -3: Item no longer available or not owned
	OfferStatusAutoCancelledExpired                                // -2: Offer expired
	OfferStatusAutoCancelledBalance                                // -1: Buyer has insufficient balance
	OfferStatusActive                                              // 0: Active, awaiting response
	OfferStatusAccepted                                            // 1: Accepted and completed
	OfferStatusDeclined                                            // 2: Declined by recipient
	OfferStatusRemoved                                             // 3: Cancelled/removed by sender
)

// TradeStatus defines the state of a deposit or withdraw bot trade.
type TradeStatus int

// List of trade status codes.
const (
	TradeStatusPending   TradeStatus = 0   // Pending trade creation or delivery
	TradeStatusCompleted TradeStatus = 3   // Trade completed successfully
	TradeStatusFailed    TradeStatus = -1  // Trade failed
	TradeStatusHidden    TradeStatus = -11 // Hidden from user listings
	TradeStatusReverted  TradeStatus = -12 // Reverted trade
)

// Client is a thread-safe client for the Mannco.store API.
// It wraps generated API services and manages session tokens.
type Client struct {
	API
	mu    sync.Mutex
	token string
	r     *aoni.Client
}

// NewClient initializes a new client with the predefined Mannco.store host,
// standard User-Agent, and BaseResponse envelope configurations, backed by fast.Client by default.
func NewClient(rest any, opts ...aoni.ClientOption) *Client {
	c := &Client{}

	defaultOpts := []aoni.ClientOption{
		option.WithUserAgent("G-man Bot/1.0"),
		option.WithHeaderFunc("Authorization", c.Token),
		option.WithBaseURL(BaseURL),
	}

	allOpts := append(defaultOpts, opts...)
	r := aoni.NewClient(rest, allOpts...)
	c.r = r
	c.API = New(r)

	return c
}

// With applies the given options to the client, returning a new client with the updated configuration.
func (c *Client) With(opts ...aoni.ClientOption) *Client {
	if len(opts) == 0 {
		return c
	}

	return NewClient(c.r, opts...)
}

// Token returns the current token used for authentication with "Bearer " prefix.
func (c *Client) Token() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token == "" {
		return ""
	}

	return "Bearer " + c.token
}

// SetToken updates the current authentication token.
func (c *Client) SetToken(token string) {
	c.mu.Lock()
	c.token = token
	c.mu.Unlock()
}

// Login authenticates the client using a Mannco API key and stores the JWT bearer token.
func (c *Client) Login(ctx context.Context, apiKey string, mods ...aoni.RequestModifier) error {
	res, err := c.PostLogin(ctx, LoginRequest{APIKey: apiKey}, mods...)
	if err != nil {
		return err
	}

	c.SetToken(res.JWT)
	return nil
}

// R returns the underlying *aoni.Client used by the client.
func (c *Client) R() *aoni.Client {
	return c.r
}

// SetItemPrice updates pricing for a list of inventory item asset IDs.
func (c *Client) SetItemPrice(
	ctx context.Context,
	ids []string,
	price int,
	mods ...aoni.RequestModifier,
) (*InventoryMessageResponse, error) {
	req := SetPriceReq{
		IDs:   strings.Join(ids, ","),
		Price: price,
	}

	return c.SetPriceDirect(ctx, req, mods...)
}

// WithdrawItems pulls items from Mannco.store inventory to the user's Steam Account.
func (c *Client) WithdrawItems(
	ctx context.Context,
	ids []string,
	mods ...aoni.RequestModifier,
) (*WithdrawResponse, error) {
	req := WithdrawReq{
		IDs: strings.Join(ids, ","),
	}

	return c.WithdrawDirect(ctx, req, mods...)
}

// GetBulkPricing returns calculated pricing for multiple items at once.
func (c *Client) GetBulkPricing(
	ctx context.Context,
	items []string,
	mods ...aoni.RequestModifier,
) (*BulkPricing, error) {
	return c.GetBulkPricingDirect(ctx, strings.Join(items, ","), mods...)
}

// GetListingCount returns the number of active sales listings for an item,
// optionally filtered by the seller's SteamID.
func (c *Client) GetListingCount(
	ctx context.Context,
	item, userID string,
	mods ...aoni.RequestModifier,
) (*ListingCount, error) {
	if userID != "" {
		return c.GetListingCountForUser(ctx, item, userID, mods...)
	}

	return c.GetListingCountDirect(ctx, item, mods...)
}

// GetItemListings fetches marketplace active listings sorted by price ascending.
func (c *Client) GetItemListings(
	ctx context.Context,
	item, userID string,
	query ListingsReq,
	mods ...aoni.RequestModifier,
) ([]Listing, error) {
	if userID != "" {
		return c.GetItemListingsForUser(ctx, item, userID, query, mods...)
	}

	return c.GetItemListingsDirect(ctx, item, query, mods...)
}

// CreateBuyOrder creates a buy order for an item.
func (c *Client) CreateBuyOrder(
	ctx context.Context,
	itemID, value, amount int,
	mods ...aoni.RequestModifier,
) (*DetailsResponse, error) {
	req := CreateBuyOrderReq{
		ItemID: itemID,
		Value:  value,
		Amount: amount,
	}

	return c.CreateBuyOrderDirect(ctx, req, mods...)
}

// UpdateBuyOrder updates an existing buy order price and/or quantity.
func (c *Client) UpdateBuyOrder(
	ctx context.Context,
	itemID, value, amount int,
	mods ...aoni.RequestModifier,
) (string, error) {
	req := UpdateBuyOrderReq{
		ItemID: itemID,
		Value:  value,
		Amount: amount,
	}

	return c.UpdateBuyOrderDirect(ctx, req, mods...)
}

// RemoveBuyOrder cancels a buy order and releases the reserved balance.
func (c *Client) RemoveBuyOrder(
	ctx context.Context,
	itemID int,
	mods ...aoni.RequestModifier,
) (*DetailsResponse, error) {
	req := RemoveBuyOrderReq{
		ItemID: itemID,
	}

	return c.RemoveBuyOrderDirect(ctx, req, mods...)
}

// AddToCart inserts a single listing into user's shopping cart by Steam asset ID.
func (c *Client) AddToCart(
	ctx context.Context,
	assetID string,
	mods ...aoni.RequestModifier,
) (*GetCartResponse, error) {
	req := AddCartReq{AssetID: assetID}
	return c.AddToCartDirect(ctx, req, mods...)
}

// BulkAddToCart searches the cheapest marketplace listings for an item and inserts them into cart.
func (c *Client) BulkAddToCart(
	ctx context.Context,
	itemID, count int,
	sellerUserID string,
	mods ...aoni.RequestModifier,
) (*GetCartResponse, error) {
	req := BulkAddCartReq{
		ItemID:       itemID,
		Count:        count,
		SellerUserID: sellerUserID,
	}

	return c.BulkAddToCartDirect(ctx, req, mods...)
}

// RemoveFromCart deletes an entire cart row from the cart.
func (c *Client) RemoveFromCart(
	ctx context.Context,
	cartID int,
	mods ...aoni.RequestModifier,
) (*GetCartResponse, error) {
	req := RemoveCartReq{CartID: cartID}
	return c.RemoveFromCartDirect(ctx, req, mods...)
}

// CreateOffer initiates a purchase trade offer for an item on sale.
func (c *Client) CreateOffer(
	ctx context.Context,
	itemAssetID int64,
	priceCents int,
	mods ...aoni.RequestModifier,
) (*OfferMessageResponse, error) {
	req := CreateOfferReq{
		ID:    itemAssetID,
		Price: priceCents,
	}

	return c.CreateOfferDirect(ctx, req, mods...)
}

// AcceptOffer accepts a received offer and completes the checkout transaction (Seller action).
func (c *Client) AcceptOffer(
	ctx context.Context,
	offerID int64,
	mods ...aoni.RequestModifier,
) (*OfferMessageResponse, error) {
	req := OfferActionReq{ID: offerID}
	return c.AcceptOfferDirect(ctx, req, mods...)
}

// DeclineOffer declines an incoming trade offer (Seller action).
func (c *Client) DeclineOffer(
	ctx context.Context,
	offerID int64,
	mods ...aoni.RequestModifier,
) (*OfferMessageResponse, error) {
	req := OfferActionReq{ID: offerID}
	return c.DeclineOfferDirect(ctx, req, mods...)
}

// RemoveOffer cancels and removes an outgoing trade offer (Buyer action).
func (c *Client) RemoveOffer(
	ctx context.Context,
	offerID int64,
	mods ...aoni.RequestModifier,
) (*OfferMessageResponse, error) {
	req := OfferActionReq{ID: offerID}
	return c.RemoveOfferDirect(ctx, req, mods...)
}
