// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crit

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/lemon4ksan/g-man/pkg/rest"
)

// critResponse implements rest.BaseResponse to automatically parse API wrappers.
type critResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	target  any
}

func (r *critResponse) IsSuccess() bool {
	return r.Success
}

func (r *critResponse) Error() error {
	if r.Message != "" {
		return fmt.Errorf("crit: api error: %s", r.Message)
	}

	return errors.New("crit: api error")
}

func (r *critResponse) SetData(data any) {
	r.target = data
}

func (r *critResponse) UnmarshalJSON(data []byte) error {
	type Alias critResponse

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	r.Success = aux.Success
	r.Message = aux.Message

	if r.target != nil {
		if err := json.Unmarshal(data, r.target); err != nil {
			return err
		}
	}

	return nil
}

// Listing represents a crit.tf showcase classified listing.
type Listing struct {
	ID         rest.Int64String   `json:"id,omitempty"`
	SteamID    string             `json:"steam_id,omitempty"`
	ItemName   string             `json:"item_name,omitempty"`
	ItemImage  string             `json:"item_image,omitempty"`
	AssetID    string             `json:"asset_id"`
	PriceKeys  int                `json:"price_keys"`
	PriceMetal rest.Float64String `json:"price_metal"`
	Quality    string             `json:"quality,omitempty"`
	Type       string             `json:"type,omitempty"`
	SKU        string             `json:"sku,omitempty"`
	CreatedAt  string             `json:"created_at,omitempty"`
}

// GroupMember defines a store group membership state.
type GroupMember struct {
	ID           int    `json:"id"`
	StoreGroupID int    `json:"store_group_id"`
	SteamID      string `json:"steam_id"`
	Role         string `json:"role"`          // "owner" | "member"
	InviteStatus string `json:"invite_status"` // "pending" | "accepted" | "declined"
	InvitedBy    string `json:"invited_by"`
	InvitedAt    string `json:"invited_at"`
	RespondedAt  string `json:"responded_at,omitempty"`
	DisplayName  string `json:"display_name"`
	AvatarURL    string `json:"avatar_url"`
}

// Group represents store group metrics and themes.
type Group struct {
	ID              int           `json:"id"`
	OwnerSteamID    string        `json:"owner_steam_id"`
	GroupName       string        `json:"group_name"`
	Description     string        `json:"description"`
	BannerURL       string        `json:"banner_url,omitempty"`
	CustomStoreSlug string        `json:"custom_store_slug,omitempty"`
	ViewCount       int           `json:"view_count"`
	CreatedAt       string        `json:"created_at"`
	UpdatedAt       string        `json:"updated_at"`
	OwnerName       string        `json:"owner_name"`
	OwnerAvatar     string        `json:"owner_avatar"`
	Members         []GroupMember `json:"members"`
}

// Invite represents a pending group invitation.
type Invite struct {
	ID            int    `json:"id"`
	StoreGroupID  int    `json:"store_group_id"`
	SteamID       string `json:"steam_id"`
	Role          string `json:"role"`
	InviteStatus  string `json:"invite_status"`
	InvitedBy     string `json:"invited_by"`
	InvitedAt     string `json:"invited_at"`
	GroupName     string `json:"group_name"`
	Description   string `json:"description"`
	InviterName   string `json:"inviter_name"`
	InviterAvatar string `json:"inviter_avatar"`
}

// Response defines a generic API wrapper response.
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ListingsResponse wraps a list of classifieds.
type ListingsResponse struct {
	Response
	Listings []Listing `json:"listings,omitempty"`
	Listing  *Listing  `json:"listing,omitempty"`
}

// InventoryResponse wraps inventory refresh metadata.
type InventoryResponse struct {
	Response
	ItemCount    int  `json:"item_count,omitempty"`
	RefreshCount int  `json:"refresh_count,omitempty"`
	FromCache    bool `json:"from_cache,omitempty"`
}

// GroupResponse wraps a store group object.
type GroupResponse struct {
	Response
	Group *Group `json:"group,omitempty"`
}

// InvitesResponse wraps incoming invites list.
type InvitesResponse struct {
	Response
	Count   int      `json:"count"`
	Invites []Invite `json:"invites"`
}

// SSEEvent represents a single Server-Sent Event block.
type SSEEvent struct {
	Event string
	Data  string
}

// TradeRequestItem represents a trade request item from SSE stream.
type TradeRequestItem struct {
	Kind    string `json:"kind"` // "sku" or "assetid"
	SKU     string `json:"sku,omitempty"`
	Amount  int    `json:"amount,omitempty"`
	AssetID string `json:"assetid,omitempty"`
}

// TradeRequestPayload is the trade request event payload.
type TradeRequestPayload struct {
	TradeOfferURL  string             `json:"trade_offer_url"`
	ItemsToGive    []TradeRequestItem `json:"items_to_give"`
	ItemsToReceive []TradeRequestItem `json:"items_to_receive"`
	ReservedAssets []string           `json:"reserved_assets"`
}

// TradeRequestEventEnvelope wraps the trade request payload.
type TradeRequestEventEnvelope struct {
	Kind         string               `json:"kind"`
	TradeRequest *TradeRequestPayload `json:"trade_request,omitempty"`
}
