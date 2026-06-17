// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package crit provides an integration client for crit.tf classifieds, group stores and showcase APIs.
package crit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/id"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
)

// Config defines the configuration for the crit.tf API client.
type Config struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

// Client interacts with the crit.tf v2 API.
type Client struct {
	config Config
	rest   *aoni.Client
}

// NewClient creates a new crit.tf API client targeting v2 endpoints by default.
func NewClient(httpClient aoni.HTTPDoer, apiKey string) *Client {
	cfg := Config{
		APIKey: apiKey,
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://crit.tf/api/v2"
	}

	restClient := aoni.NewClient(httpClient).
		WithBaseURL(cfg.BaseURL).
		WithUserAgent("G-man Bot/1.0").
		WithBaseResponse(func() aoni.BaseResponse {
			return &critResponse{}
		})

	if apiKey != "" {
		restClient = restClient.WithHeader("X-API-Key", apiKey)
	}

	return &Client{
		config: cfg,
		rest:   restClient,
	}
}

// FetchMyListings retrieves all active listings for the authenticated user.
func (c *Client) FetchMyListings(ctx context.Context) ([]Listing, error) {
	resp, err := aoni.GetJSON[ListingsResponse](ctx, c.rest, "/listings/my")
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

	resp, err := aoni.PostJSON[any, ListingsResponse](ctx, c.rest, "/listings", payload)
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

	resp, err := aoni.PutJSON[any, ListingsResponse](
		ctx, c.rest, "/listings/{listingID}", payload,
		aoni.WithVar("listingID", listingID),
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
	_, err := aoni.DeleteJSON[any, Response](
		ctx, c.rest, "/listings/{listingID}", nil,
		aoni.WithVar("listingID", listingID),
	)

	return err
}

// RefreshInventory requests crit.tf to sync the latest inventory status from Steam.
func (c *Client) RefreshInventory(ctx context.Context) (*InventoryResponse, error) {
	return aoni.PostJSON[any, InventoryResponse](ctx, c.rest, "/inventory/refresh", nil)
}

// GetMyGroup retrieves store group details of the authenticated bot.
func (c *Client) GetMyGroup(ctx context.Context) (*Group, error) {
	resp, err := aoni.GetJSON[GroupResponse](ctx, c.rest, "/groups/my")
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
	_, err := aoni.PostJSON[any, Response](
		ctx, c.rest, "/groups/{groupID}/invite",
		map[string]string{"steam_id": targetSteamID.String()},
		aoni.WithVar("groupID", groupID),
	)

	return err
}

// GetPendingInvites retrieves pending store group invitations.
func (c *Client) GetPendingInvites(ctx context.Context) ([]Invite, error) {
	resp, err := aoni.GetJSON[InvitesResponse](ctx, c.rest, "/groups/invites")
	if err != nil {
		return nil, err
	}

	return resp.Invites, nil
}

// AcceptGroupInvite accepts a pending group invite.
func (c *Client) AcceptGroupInvite(ctx context.Context, groupID int) error {
	_, err := aoni.PostJSON[any, Response](
		ctx, c.rest, "/groups/{groupID}/accept", nil,
		aoni.WithVar("groupID", groupID),
	)

	return err
}

// LeaveGroup leaves a store group.
func (c *Client) LeaveGroup(ctx context.Context, groupID int) error {
	_, err := aoni.PostJSON[any, Response](
		ctx, c.rest, "/groups/{groupID}/leave", nil,
		aoni.WithVar("groupID", groupID),
	)

	return err
}

// FetchAuthToken requests an SSE auth token from Crit.tf API.
func (c *Client) FetchAuthToken(ctx context.Context) (string, error) {
	resp, err := c.rest.Request(ctx, http.MethodGet, "/bot-api/auth-token")
	if err != nil {
		return "", fmt.Errorf("crit: auth token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("crit: auth token request failed with status %d", resp.StatusCode)
	}

	var data struct {
		OK     bool   `json:"ok"`
		Token  string `json:"token"`
		Reason string `json:"reason,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("crit: failed to decode auth token response: %w", err)
	}

	if !data.OK {
		if data.Reason != "" {
			return "", fmt.Errorf("crit: api error: %s", data.Reason)
		}

		return "", errors.New("crit: api error: unknown reason")
	}

	// Update client with Short-Lived Token header for all subsequent API requests
	c.rest = c.rest.WithHeader("X-Short-Lived-Token", data.Token)

	return data.Token, nil
}

// StreamEvents connects to the SSE endpoint and returns a channel of SSEEvent.
func (c *Client) StreamEvents(ctx context.Context, streamURL, token string) (<-chan SSEEvent, error) {
	var mods []aoni.RequestModifier
	if token != "" {
		mods = append(mods, aoni.WithQuery(map[string]string{"token": token}))
	}

	out, errs, err := aoni.StreamSSE[SSEEvent](ctx, c.rest, streamURL, mods...)
	if err != nil {
		return nil, fmt.Errorf("crit: failed to start sse stream: %w", err)
	}

	go func() {
		for err := range errs {
			if err != nil && !errors.Is(err, context.Canceled) {
				c.rest.Logger().Error("SSE stream error", log.Err(err))
			}
		}
	}()

	return out, nil
}

// SendDeadMansRequest sends a heartbeat signal to Crit.tf backend to indicate the bot is alive.
func (c *Client) SendDeadMansRequest(ctx context.Context) (bool, error) {
	payload := map[string]bool{"alive": true}

	resp, err := c.rest.Request(ctx, http.MethodPost, "/bot-api/alive", aoni.WithJSONBody(payload))
	if err != nil {
		return false, fmt.Errorf("crit: dead man request failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// GetInventory retrieves the cached inventory of the bot from Crit.tf backend.
func (c *Client) GetInventory(ctx context.Context) ([]any, error) {
	resp, err := aoni.GetJSON[InventoryResponse](ctx, c.rest, "/inventory")
	if err != nil {
		return nil, err
	}

	return resp.Items, nil
}

// UpdateTradeURL updates the bot's trade URL on Crit.tf.
func (c *Client) UpdateTradeURL(ctx context.Context, tradeURL string) (bool, error) {
	payload := map[string]string{"trade_url": tradeURL}

	resp, err := aoni.PutJSON[any, Response](ctx, c.rest, "/user/trade-url", payload)
	if err != nil {
		return false, err
	}

	return resp.Success, nil
}

// GetUserInfo retrieves the authenticated user information from Crit.tf.
func (c *Client) GetUserInfo(ctx context.Context) (*User, error) {
	resp, err := aoni.GetJSON[UserResponse](ctx, c.rest, "/user")
	if err != nil {
		return nil, err
	}

	return resp.User, nil
}
