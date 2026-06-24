// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mannco

import (
	"context"

	"github.com/lemon4ksan/aoni"
)

// PaymentReq represents the request body to initiate a checkout session.
type PaymentReq struct {
	Type          string `json:"type"`                     // Required: "balance" (top-up) or "items" (pay cart items)
	TOSTimestamp  any    `json:"tos_timestamp"`            // Required: Client Unix timestamp of Terms of Service acceptance
	Value         *int   `json:"value,omitempty"`          // Balance deposit amount in cents (USD, max 2,500,000 cents)
	Items         string `json:"items,omitempty"`          // Comma-separated asset IDs for cart checkout purchase
	PaymentMethod string `json:"payment_method,omitempty"` // Optional provider-specific method identifier
}

// PaymentResponse represents payment processing checkout result.
type PaymentResponse struct {
	URL     string `json:"url,omitempty"`     // Payment redirect URL for the user's browser, if redirect needed
	Message string `json:"message,omitempty"` // Status message, if processed immediately
}

// InitiatePayment creates a checkout session to add balance credit or purchase cart listings.
//
// Route: POST /payment/{provider}
// Permission: Connected + API
// Payout Providers: "payviox" (Card redirect checkout) or "mannco" (Internal account balance payment)
func (c *Client) InitiatePayment(ctx context.Context, provider string, req PaymentReq) (*PaymentResponse, error) {
	return aoni.PostJSON[PaymentResponse](
		ctx, c.getClient(), "/payment/{provider}", req,
		aoni.WithVar("provider", provider),
	)
}
