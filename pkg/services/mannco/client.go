// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mannco provides a client for interacting with the Mannco.store API.
package mannco

import (
	"context"
	"sync"

	"github.com/lemon4ksan/aoni"
)

// BaseURL is the default endpoint host for the Mannco.store API.
const BaseURL = "https://api.mannco.store"

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
// It wraps aoni.Client and synchronizes token updates.
type Client struct {
	mu         sync.Mutex
	restClient *aoni.Client
}

// NewClient initializes a new client with the predefined Mannco.store host,
// standard User-Agent, and BaseResponse envelope configurations.
func NewClient(restClient *aoni.Client) *Client {
	return &Client{
		restClient: restClient.
			WithUserAgent("G-man Bot/1.0").
			WithBaseURL("https://api.mannco.store/").
			WithBaseResponse(func() aoni.BaseResponse { return new(BaseResponse) }),
	}
}

// getClient retrieves the underlying aoni.Client pointer thread-safely.
func (c *Client) getClient() *aoni.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.restClient
}

// Login authenticates the client using a Mannco API key.
// It retrieves a JWT bearer token and configures the client to send it
// in the 'Authorization: Bearer <jwt>' header for all subsequent Connected+API calls.
//
// Route: POST /user/login
// Permission: Connected + API
func (c *Client) Login(ctx context.Context, apiKey string) error {
	body := struct {
		APIKey string `json:"apiKey"`
	}{APIKey: apiKey}

	type resp struct {
		JWT string `json:"jwt"`
	}

	r, err := aoni.PostJSON[resp](ctx, c.getClient(), "user/login", body)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.restClient = c.restClient.WithBearer(r.JWT)
	c.mu.Unlock()

	return nil
}
