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

// OfferItem represents a summary of item metadata associated with a trade offer.
type OfferItem struct {
	Name         string          `json:"name"`                   // Item name
	Effect       string          `json:"effect"`                 // Particle effect (TF2)
	URL          string          `json:"url"`                    // Slug identifier
	Game         int             `json:"game"`                   // Game ID
	Quality      string          `json:"quality"`                // Item Quality
	Image        string          `json:"image"`                  // Image URL
	Type         string          `json:"type"`                   // Type description
	Craftable    Craftable       `json:"craftable"`              // Craftability status
	AssetID      string          `json:"assetId"`                // Steam asset ID
	Wear         *float64        `json:"wear,omitempty"`         // Wear float (CS2)
	Sheen        string          `json:"sheen,omitempty"`        // Sheen (TF2)
	Killstreaker string          `json:"killstreaker,omitempty"` // Killstreaker (TF2)
	Spell        string          `json:"spell,omitempty"`        // Spells (TF2)
	Parts        json.RawMessage `json:"parts,omitempty"`        // Strange parts (TF2)
	HTML         *string         `json:"html,omitempty"`         // Description HTML
	User         string          `json:"user,omitempty"`         // Owner ID
	State        *int            `json:"state,omitempty"`        // Item state
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
type OfferUser struct {
	Username string `json:"username"` // Steam username
	Avatar   string `json:"avatar"`   // Steam avatar URL
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
type Offer struct {
	ID         int         `json:"id"`         // Internal offer ID
	From       string      `json:"from"`       // Buyer's Steam ID
	To         string      `json:"to"`         // Seller's Steam ID
	Status     OfferStatus `json:"status"`     // Offer status code
	Time       int64       `json:"time"`       // Expiration timestamp (Unix time)
	Price      int         `json:"price"`      // Price in cents
	Read       int         `json:"read"`       // Read flag (0 = unread, 1 = read)
	CreatedAt  int64       `json:"createdAt"`  // Creation timestamp (Unix time)
	BackpackID int64       `json:"backpackid"` // Backpack item row ID
	User       OfferUser   `json:"user"`       // Counterparty metadata
	Item       OfferItem   `json:"item"`       // Item details associated with this offer
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
type GetReceivedOffersResponse struct {
	Offers []Offer `json:"offers"`
}

// CreateOfferReq represents payload to create a trade offer.
type CreateOfferReq struct {
	ID    int64 `json:"id"`    // Steam assetId of the item
	Price int   `json:"price"` // Price in cents
}

// OfferMessageResponse holds generic confirmation text from offer actions.
type OfferMessageResponse struct {
	Message string `json:"message"`
}

// OfferActionReq represents the payload for accept/decline/remove requests.
type OfferActionReq struct {
	ID int64 `json:"id"` // Offer ID
}

// GetReceivedOffers returns trade offers received from other buyers.
// Results automatically validate offer status constraints. Omitted / cancelled offers (status < 0) are filtered out.
//
// Route: GET /offers/received
// Permission: Connected + API
func (c *Client) GetReceivedOffers(ctx context.Context) ([]Offer, error) {
	resp, err := aoni.GetJSON[GetReceivedOffersResponse](ctx, c.getClient(), "/offers/received")
	if err != nil {
		return nil, err
	}

	return resp.Offers, nil
}

// GetMyOffers returns active trade offers sent to other sellers.
//
// Route: GET /offers/my
// Permission: Connected + API
func (c *Client) GetMyOffers(ctx context.Context) ([]Offer, error) {
	resp, err := aoni.GetJSON[[]Offer](ctx, c.getClient(), "/offers/my")
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// CreateOffer initiates a purchase trade offer for an item on sale.
// The asset must be currently listed (state = 1) and not owned by the caller.
// The total price is validated against user balance. Limit: Max 10 active offers per user.
// Expiration defaults to 24 hours (86,400 seconds).
//
// Route: POST /offers/create
// Permission: Connected + API
func (c *Client) CreateOffer(ctx context.Context, itemAssetID int64, priceCents int) (*OfferMessageResponse, error) {
	req := CreateOfferReq{
		ID:    itemAssetID,
		Price: priceCents,
	}

	return aoni.PostJSON[OfferMessageResponse](ctx, c.getClient(), "/offers/create", req)
}

// AcceptOffer accepts a received offer and completes the checkout transaction (Seller action).
//
// Route: POST /offers/accept
// Permission: Connected + API
func (c *Client) AcceptOffer(ctx context.Context, offerID int64) (*OfferMessageResponse, error) {
	req := OfferActionReq{ID: offerID}
	return aoni.PostJSON[OfferMessageResponse](ctx, c.getClient(), "/offers/accept", req)
}

// DeclineOffer declines an incoming trade offer (Seller action).
//
// Route: POST /offers/decline
// Permission: Connected + API
func (c *Client) DeclineOffer(ctx context.Context, offerID int64) (*OfferMessageResponse, error) {
	req := OfferActionReq{ID: offerID}
	return aoni.PostJSON[OfferMessageResponse](ctx, c.getClient(), "/offers/decline", req)
}

// RemoveOffer cancels and removes an outgoing trade offer (Buyer action).
//
// Route: POST /offers/remove
// Permission: Connected + API
func (c *Client) RemoveOffer(ctx context.Context, offerID int64) (*OfferMessageResponse, error) {
	req := OfferActionReq{ID: offerID}
	return aoni.PostJSON[OfferMessageResponse](ctx, c.getClient(), "/offers/remove", req)
}
