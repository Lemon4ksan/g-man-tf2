// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crit

import (
	"context"

	"github.com/lemon4ksan/aoni"
)

// API defines the declarative REST contract for the Crit.tf v2 API.
//
// @aoni:service casing=snake_case
// @base_url "https://crit.tf/api/v2/"
// @version "v2.0.0"
type API interface {
	// FetchMyListings retrieves all active listings for the authenticated user.
	// @get "listings/my"
	FetchMyListings(ctx context.Context, mods ...aoni.RequestModifier) (*ListingsResponse, error)

	// CreateListingDirect creates a new sell listing on crit.tf.
	// @post "listings"
	CreateListingDirect(ctx context.Context, req CreateListingRequest, mods ...aoni.RequestModifier) (*ListingsResponse, error)

	// UpdateListingDirect updates an existing listing by its database ID.
	// @put "listings/{listing_id}"
	UpdateListingDirect(ctx context.Context, listingID string, req UpdateListingRequest, mods ...aoni.RequestModifier) (*ListingsResponse, error)

	// DeleteListing deletes an active listing by its database ID.
	// @delete "listings/{listing_id}"
	DeleteListing(ctx context.Context, listingID string, mods ...aoni.RequestModifier) (*Response, error)

	// RefreshInventory requests crit.tf to sync the latest inventory status from Steam.
	// @post "inventory/refresh"
	RefreshInventory(ctx context.Context, mods ...aoni.RequestModifier) (*InventoryResponse, error)

	// GetMyGroup retrieves store group details of the authenticated bot.
	// @get "groups/my"
	GetMyGroup(ctx context.Context, mods ...aoni.RequestModifier) (*GroupResponse, error)

	// InviteToGroupDirect sends a store group membership invite to a user.
	// @post "groups/{group_id}/invite"
	InviteToGroupDirect(ctx context.Context, groupID int, req InviteGroupRequest, mods ...aoni.RequestModifier) (*Response, error)

	// GetPendingInvites retrieves pending store group invitations.
	// @get "groups/invites"
	GetPendingInvites(ctx context.Context, mods ...aoni.RequestModifier) (*InvitesResponse, error)

	// AcceptGroupInvite accepts a pending group invite.
	// @post "groups/{group_id}/accept"
	AcceptGroupInvite(ctx context.Context, groupID int, mods ...aoni.RequestModifier) (*Response, error)

	// LeaveGroup leaves a store group.
	// @post "groups/{group_id}/leave"
	LeaveGroup(ctx context.Context, groupID int, mods ...aoni.RequestModifier) (*Response, error)

	// FetchAuthTokenDirect requests an SSE auth token from Crit.tf API.
	// @get "bot-api/auth-token"
	FetchAuthTokenDirect(ctx context.Context, mods ...aoni.RequestModifier) (*AuthTokenResponse, error)

	// SendDeadMansRequestDirect sends a heartbeat signal to Crit.tf backend to indicate the bot is alive.
	// @post "bot-api/alive"
	SendDeadMansRequestDirect(ctx context.Context, req DeadMansRequest, mods ...aoni.RequestModifier) (*Response, error)

	// GetInventory retrieves the cached inventory of the bot from Crit.tf backend.
	// @get "inventory"
	GetInventory(ctx context.Context, mods ...aoni.RequestModifier) (*InventoryResponse, error)

	// UpdateTradeURLDirect updates the bot's trade URL on Crit.tf.
	// @put "user/trade-url"
	UpdateTradeURLDirect(ctx context.Context, req UpdateTradeURLRequest, mods ...aoni.RequestModifier) (*Response, error)

	// GetUserInfo retrieves the authenticated user information from Crit.tf.
	// @get "user"
	GetUserInfo(ctx context.Context, mods ...aoni.RequestModifier) (*UserResponse, error)
}
