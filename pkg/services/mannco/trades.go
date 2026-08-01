// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mannco

import (
	"context"
	"strings"

	json "github.com/goccy/go-json"
	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/mod"
	"github.com/lemon4ksan/aoni/request"
)

// GetTradesResponse wraps a list of user bot trades.
type GetTradesResponse struct {
	Trades []TradeInfo `json:"trades"` // Slices of trade rows
}

// ResendTradeResponse details outcome from resending a trade offer.
type ResendTradeResponse struct {
	Message string `json:"message"`  // Confirmation message (e.g. "Trade resent successfully")
	TradeID int    `json:"trade_id"` // Trade ID
	Code    string `json:"code"`     // Resent security verification code
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
type ResendTradeQuery struct {
	ID int `url:"id"` // Internal trade ID
}

// GetActiveTrades returns pending, incomplete trades for the user (withdrawals/deposits).
//
// Route: GET /trades/active
// Permission: Connected + API
func (c *Client) GetActiveTrades(ctx context.Context, mods ...aoni.RequestModifier) (*GetTradesResponse, error) {
	return request.GetTo[GetTradesResponse](ctx, c.r, "/trades/active", mods...)
}

// GetAllTrades returns up to the last 500 historical trades (completed, failed) for the user.
// Ordered by timestamp descending.
//
// Route: GET /trades/all
// Permission: Connected + API
func (c *Client) GetAllTrades(ctx context.Context, mods ...aoni.RequestModifier) (*GetTradesResponse, error) {
	return request.GetTo[GetTradesResponse](ctx, c.r, "/trades/all", mods...)
}

// ResendTrade triggers a retry for a failed or pending withdrawal trade.
// Query parameter name is 'id'.
//
// Route: GET /trade/resend?id={tradeId}
// Permission: Connected + API
func (c *Client) ResendTrade(
	ctx context.Context,
	tradeID int,
	mods ...aoni.RequestModifier,
) (*ResendTradeResponse, error) {
	req := ResendTradeQuery{ID: tradeID}

	allMods := append([]aoni.RequestModifier{
		mod.WithQuery(req),
	}, mods...)

	return request.GetTo[ResendTradeResponse](ctx, c.r, "/trade/resend", allMods...)
}
