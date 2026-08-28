// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package crit provides a client for interacting with the crit.tf API.
package crit

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/fast"
	"github.com/lemon4ksan/aoni/mod"
	"github.com/lemon4ksan/aoni/option"
	"github.com/lemon4ksan/aoni/realtime/stream"
	"github.com/lemon4ksan/g-man/pkg/steam/id"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
)

// BaseURL is the default base URL for the crit.tf API.
const BaseURL = "https://crit.tf/api/v2/"

// Config defines the configuration for the crit.tf API client.
type Config struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

// Client interacts with the crit.tf v2 API.
type Client struct {
	API
	mu    sync.Mutex
	token string
	r     *aoni.Client
}

// NewClient creates a new crit.tf API client targeting v2 endpoints by default using fast.Client.
func NewClient(doer aoni.RequestDoer, apiKey string, opts ...aoni.ClientOption) *Client {
	c := &Client{}

	if doer == nil {
		doer = fast.NewClient()
	}

	defaultOpts := []aoni.ClientOption{
		option.WithBaseURL(BaseURL),
		option.WithHeaderFunc("X-Short-Lived-Token", c.Token),
		option.WithUserAgent("G-man Bot/1.0"),
		option.WithBaseResponse(func() aoni.BaseResponse {
			return &critResponse{}
		}),
	}

	if apiKey != "" {
		defaultOpts = append(defaultOpts, option.WithHeader("X-API-Key", apiKey))
	}

	defaultOpts = append(defaultOpts, opts...)
	r := aoni.NewClient(doer, defaultOpts...)
	c.r = r
	c.API = New(r)

	return c
}

// Token returns the current token used for authentication.
func (c *Client) Token() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

func (c *Client) setToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.token = token
}

// With applies the given options to the client, returning a new client with the updated configuration.
func (c *Client) With(opts ...aoni.ClientOption) *Client {
	if len(opts) == 0 {
		return c
	}

	r := c.r.With(opts...)

	return &Client{
		r:   r,
		API: New(r),
	}
}

// R yields the underlying low-level Client.
func (c *Client) R() *aoni.Client {
	return c.r
}

// FetchMyListings retrieves all active listings for the authenticated user.
func (c *Client) FetchMyListings(ctx context.Context, mods ...aoni.RequestModifier) ([]Listing, error) {
	resp, err := c.API.FetchMyListings(ctx, mods...)
	if err != nil {
		return nil, err
	}

	return resp.Listings, nil
}

// CreateListing creates a new sell listing on crit.tf.
func (c *Client) CreateListing(
	ctx context.Context,
	assetID string,
	currencies pricedb.Currencies,
	mods ...aoni.RequestModifier,
) (*Listing, error) {
	req := CreateListingRequest{
		AssetID:    assetID,
		PriceKeys:  currencies.Keys,
		PriceMetal: currencies.Metal,
	}

	resp, err := c.CreateListingDirect(ctx, req, mods...)
	if err != nil {
		return nil, err
	}

	if resp.Listing == nil {
		return nil, errors.New("crit: listing is missing in response")
	}

	return resp.Listing, nil
}

// UpdateListing updates an existing listing by its database ID.
func (c *Client) UpdateListing(
	ctx context.Context,
	listingID string,
	currencies pricedb.Currencies,
	mods ...aoni.RequestModifier,
) (*Listing, error) {
	req := UpdateListingRequest{
		PriceKeys:  currencies.Keys,
		PriceMetal: currencies.Metal,
	}

	resp, err := c.UpdateListingDirect(ctx, listingID, req, mods...)
	if err != nil {
		return nil, err
	}

	if resp.Listing == nil {
		return nil, errors.New("crit: listing is missing in response")
	}

	return resp.Listing, nil
}

// DeleteListing deletes an active listing by its database ID.
func (c *Client) DeleteListing(ctx context.Context, listingID string, mods ...aoni.RequestModifier) error {
	_, err := c.API.DeleteListing(ctx, listingID, mods...)
	return err
}

// RefreshInventory requests crit.tf to sync the latest inventory status from Steam.
func (c *Client) RefreshInventory(ctx context.Context, mods ...aoni.RequestModifier) (*InventoryResponse, error) {
	return c.API.RefreshInventory(ctx, mods...)
}

// GetMyGroup retrieves store group details of the authenticated bot.
func (c *Client) GetMyGroup(ctx context.Context, mods ...aoni.RequestModifier) (*Group, error) {
	resp, err := c.API.GetMyGroup(ctx, mods...)
	if err != nil {
		return nil, err
	}

	if resp.Group == nil {
		return nil, errors.New("crit: group is missing in response")
	}

	return resp.Group, nil
}

// InviteToGroup sends a store group membership invite to a user.
func (c *Client) InviteToGroup(
	ctx context.Context,
	groupID int,
	targetSteamID id.ID,
	mods ...aoni.RequestModifier,
) error {
	req := InviteGroupRequest{SteamID: targetSteamID.String()}
	_, err := c.InviteToGroupDirect(ctx, groupID, req, mods...)
	return err
}

// GetPendingInvites retrieves pending store group invitations.
func (c *Client) GetPendingInvites(ctx context.Context, mods ...aoni.RequestModifier) ([]Invite, error) {
	resp, err := c.API.GetPendingInvites(ctx, mods...)
	if err != nil {
		return nil, err
	}

	return resp.Invites, nil
}

// AcceptGroupInvite accepts a pending group invite.
func (c *Client) AcceptGroupInvite(ctx context.Context, groupID int, mods ...aoni.RequestModifier) error {
	_, err := c.API.AcceptGroupInvite(ctx, groupID, mods...)
	return err
}

// LeaveGroup leaves a store group.
func (c *Client) LeaveGroup(ctx context.Context, groupID int, mods ...aoni.RequestModifier) error {
	_, err := c.API.LeaveGroup(ctx, groupID, mods...)
	return err
}

// FetchAuthToken requests an SSE auth token from Crit.tf API.
// The token is stored in the client and can be retrieved using [Client.Token].
func (c *Client) FetchAuthToken(ctx context.Context, mods ...aoni.RequestModifier) (string, error) {
	allMods := append([]aoni.RequestModifier{
		mod.WithoutBaseResponse(),
	}, mods...)

	data, err := c.FetchAuthTokenDirect(ctx, allMods...)
	if err != nil {
		return "", fmt.Errorf("crit: auth token request failed: %w", err)
	}

	if !data.OK {
		if data.Reason != "" {
			return "", fmt.Errorf("crit: api error: %s", data.Reason)
		}

		return "", errors.New("crit: api error: unknown reason")
	}

	c.setToken(data.Token)

	return data.Token, nil
}

// StreamEvents connects to the SSE endpoint and returns a channel of SSEEvent.
func (c *Client) StreamEvents(
	ctx context.Context,
	streamURL, token string,
	mods ...aoni.RequestModifier,
) (<-chan SSEEvent, <-chan error, error) {
	var allMods []aoni.RequestModifier
	if token != "" {
		req := struct {
			Token string `query:"token"`
		}{Token: token}
		allMods = append(allMods, mod.WithQuery(req))
	}

	allMods = append(allMods, mods...)

	out, errs, err := stream.SSE[SSEEvent](ctx, c.r, streamURL, allMods...)
	if err != nil {
		return nil, nil, fmt.Errorf("crit: failed to start sse stream: %w", err)
	}

	return out, errs, nil
}

// SendDeadMansRequest sends a heartbeat signal to Crit.tf backend to indicate the bot is alive.
func (c *Client) SendDeadMansRequest(ctx context.Context, mods ...aoni.RequestModifier) (bool, error) {
	payload := map[string]bool{"alive": true}

	resp, err := c.r.Raw().Post(ctx, "bot-api/alive", append(mods, mod.WithJSON(payload))...)
	if err != nil {
		return false, fmt.Errorf("crit: dead man request failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}

// GetInventory retrieves the cached inventory of the bot from Crit.tf backend.
func (c *Client) GetInventory(ctx context.Context, mods ...aoni.RequestModifier) ([]any, error) {
	resp, err := c.API.GetInventory(ctx, mods...)
	if err != nil {
		return nil, err
	}

	return resp.Items, nil
}

// UpdateTradeURL updates the bot's trade URL on Crit.tf.
func (c *Client) UpdateTradeURL(ctx context.Context, tradeURL string, mods ...aoni.RequestModifier) (bool, error) {
	resp, err := c.UpdateTradeURLDirect(ctx, UpdateTradeURLRequest{TradeURL: tradeURL}, mods...)
	if err != nil {
		return false, err
	}

	return resp.Success, nil
}

// GetUserInfo retrieves the authenticated user information from Crit.tf.
func (c *Client) GetUserInfo(ctx context.Context, mods ...aoni.RequestModifier) (*User, error) {
	resp, err := c.API.GetUserInfo(ctx, mods...)
	if err != nil {
		return nil, err
	}

	return resp.User, nil
}
