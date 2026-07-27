// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/fast"
	"github.com/lemon4ksan/aoni/mod"
	"github.com/lemon4ksan/aoni/option"
	"github.com/lemon4ksan/aoni/realtime/stream"
	"github.com/lemon4ksan/aoni/request"
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
	mu    sync.Mutex
	token string
	r     request.Requester
}

// NewClient creates a new crit.tf API client targeting v2 endpoints by default using fast.Client.
func NewClient(rest aoni.RequestDoer, apiKey string) *Client {
	c := &Client{}

	if rest == nil {
		rest = fast.NewClient()
	}

	opts := []option.Option{
		option.WithBaseURL(BaseURL),
		option.WithHeaderFunc("X-Short-Lived-Token", c.Token),
		option.WithUserAgent("G-man Bot/1.0"),
		option.WithBaseResponse(func() aoni.BaseResponse {
			return &critResponse{}
		}),
	}

	if apiKey != "" {
		opts = append(opts, option.WithHeader("X-API-Key", apiKey))
	}

	c.r = request.AsRequester(aoni.Configure(rest, opts...))

	return c
}

// Token returns the current token used for authentication.
// Use [Client.FetchAuthToken] to obtain a new token.
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

	return &Client{
		r: request.AsRequester(aoni.Configure(c.r, opts...)),
	}
}

// FetchMyListings retrieves all active listings for the authenticated user.
func (c *Client) FetchMyListings(ctx context.Context) ([]Listing, error) {
	resp, err := request.GetTo[ListingsResponse](ctx, c.r, "listings/my")
	if err != nil {
		return nil, err
	}

	return resp.Listings, nil
}

// CreateListing creates a new sell listing on crit.tf.
func (c *Client) CreateListing(ctx context.Context, assetID string, currencies pricedb.Currencies) (*Listing, error) {
	payload := map[string]any{
		"asset_id":    assetID,
		"price_keys":  currencies.Keys,
		"price_metal": currencies.Metal,
	}

	resp, err := request.PostTo[ListingsResponse](ctx, c.r, "listings", payload)
	if err != nil {
		return nil, err
	}

	if resp.Listing == nil {
		return nil, errors.New("crit: listing is missing in response")
	}

	return resp.Listing, nil
}

// UpdateListing updates an existing listing by its database ID.
func (c *Client) UpdateListing(ctx context.Context, listingID string, currencies pricedb.Currencies) (*Listing, error) {
	payload := map[string]any{
		"price_keys":  currencies.Keys,
		"price_metal": currencies.Metal,
	}

	resp, err := request.PutTo[ListingsResponse](
		ctx, c.r, "listings/{listingID}", payload,
		mod.WithVar("listingID", listingID),
	)
	if err != nil {
		return nil, err
	}

	if resp.Listing == nil {
		return nil, errors.New("crit: listing is missing in response")
	}

	return resp.Listing, nil
}

// DeleteListing deletes an active listing by its database ID.
func (c *Client) DeleteListing(ctx context.Context, listingID string) error {
	_, err := request.DeleteTo[Response](
		ctx, c.r, "listings/{listingID}", nil,
		mod.WithVar("listingID", listingID),
	)

	return err
}

// RefreshInventory requests crit.tf to sync the latest inventory status from Steam.
func (c *Client) RefreshInventory(ctx context.Context) (*InventoryResponse, error) {
	return request.PostTo[InventoryResponse](ctx, c.r, "inventory/refresh", nil)
}

// GetMyGroup retrieves store group details of the authenticated bot.
func (c *Client) GetMyGroup(ctx context.Context) (*Group, error) {
	resp, err := request.GetTo[GroupResponse](ctx, c.r, "groups/my")
	if err != nil {
		return nil, err
	}

	if resp.Group == nil {
		return nil, errors.New("crit: group is missing in response")
	}

	return resp.Group, nil
}

// InviteToGroup sends a store group membership invite to a user.
func (c *Client) InviteToGroup(ctx context.Context, groupID int, targetSteamID id.ID) error {
	_, err := request.PostTo[Response](
		ctx, c.r, "groups/{groupID}/invite",
		map[string]string{"steam_id": targetSteamID.String()},
		mod.WithVar("groupID", groupID),
	)

	return err
}

// GetPendingInvites retrieves pending store group invitations.
func (c *Client) GetPendingInvites(ctx context.Context) ([]Invite, error) {
	resp, err := request.GetTo[InvitesResponse](ctx, c.r, "groups/invites")
	if err != nil {
		return nil, err
	}

	return resp.Invites, nil
}

// AcceptGroupInvite accepts a pending group invite.
func (c *Client) AcceptGroupInvite(ctx context.Context, groupID int) error {
	_, err := request.PostTo[Response](
		ctx, c.r, "groups/{groupID}/accept", nil,
		mod.WithVar("groupID", groupID),
	)

	return err
}

// LeaveGroup leaves a store group.
func (c *Client) LeaveGroup(ctx context.Context, groupID int) error {
	_, err := request.PostTo[Response](
		ctx, c.r, "groups/{groupID}/leave", nil,
		mod.WithVar("groupID", groupID),
	)

	return err
}

// FetchAuthToken requests an SSE auth token from Crit.tf API.
// The token is stored in the client and can be retrieved using [Client.Token].
func (c *Client) FetchAuthToken(ctx context.Context) (string, error) {
	type tokenResp struct {
		OK     bool   `json:"ok"`
		Token  string `json:"token"`
		Reason string `json:"reason,omitempty"`
	}

	data, err := request.GetTo[tokenResp](ctx, c.r, "bot-api/auth-token", mod.WithoutBaseResponse())
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
func (c *Client) StreamEvents(ctx context.Context, streamURL, token string) (<-chan SSEEvent, <-chan error, error) {
	var mods []aoni.RequestModifier
	if token != "" {
		req := struct {
			Token string `url:"token"`
		}{Token: token}
		mods = append(mods, mod.WithQuery(req))
	}

	out, errs, err := stream.SSE[SSEEvent](ctx, c.r, streamURL, mods...)
	if err != nil {
		return nil, nil, fmt.Errorf("crit: failed to start sse stream: %w", err)
	}

	return out, errs, nil
}

// SendDeadMansRequest sends a heartbeat signal to Crit.tf backend to indicate the bot is alive.
func (c *Client) SendDeadMansRequest(ctx context.Context) (bool, error) {
	payload := map[string]bool{"alive": true}

	resp, err := request.Post(ctx, c.r, "bot-api/alive", payload)
	if err != nil {
		return false, fmt.Errorf("crit: dead man request failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// GetInventory retrieves the cached inventory of the bot from Crit.tf backend.
func (c *Client) GetInventory(ctx context.Context) ([]any, error) {
	resp, err := request.GetTo[InventoryResponse](ctx, c.r, "inventory")
	if err != nil {
		return nil, err
	}

	return resp.Items, nil
}

// UpdateTradeURL updates the bot's trade URL on Crit.tf.
func (c *Client) UpdateTradeURL(ctx context.Context, tradeURL string) (bool, error) {
	payload := map[string]string{"trade_url": tradeURL}

	resp, err := request.PutTo[Response](ctx, c.r, "user/trade-url", payload)
	if err != nil {
		return false, err
	}

	return resp.Success, nil
}

// GetUserInfo retrieves the authenticated user information from Crit.tf.
func (c *Client) GetUserInfo(ctx context.Context) (*User, error) {
	resp, err := request.GetTo[UserResponse](ctx, c.r, "user")
	if err != nil {
		return nil, err
	}

	return resp.User, nil
}
