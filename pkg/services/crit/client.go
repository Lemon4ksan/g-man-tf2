// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package crit provides an integration client for crit.tf classifieds, group stores and showcase APIs.
package crit

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/lemon4ksan/g-man/pkg/rest"
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
	rest   *rest.Client
}

// NewClient creates a new crit.tf API client targeting v2 endpoints by default.
func NewClient(httpClient rest.HTTPDoer, apiKey string) *Client {
	cfg := Config{
		APIKey: apiKey,
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://crit.tf/api/v2"
	}

	restClient := rest.NewClient(httpClient).
		WithBaseURL(cfg.BaseURL).
		WithUserAgent("G-man Bot/1.0").
		WithBaseResponse(func() rest.BaseResponse {
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
	resp, err := rest.GetJSON[ListingsResponse](ctx, c.rest, "/listings/my", nil)
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

	resp, err := rest.PostJSON[any, ListingsResponse](ctx, c.rest, "/listings", payload, nil)
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

	// Use method modifier to execute PUT request using PostJSON
	putMod := func(req *http.Request) {
		req.Method = http.MethodPut
	}

	resp, err := rest.PostJSON[any, ListingsResponse](
		ctx,
		c.rest,
		"/listings/{listingID}",
		payload,
		nil,
		rest.WithVar("listingID", listingID),
		putMod,
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
	_, err := rest.DeleteJSON[any, Response](
		ctx,
		c.rest,
		"/listings/{listingID}",
		nil,
		nil,
		rest.WithVar("listingID", listingID),
	)

	return err
}

// RefreshInventory requests crit.tf to sync the latest inventory status from Steam.
func (c *Client) RefreshInventory(ctx context.Context) (*InventoryResponse, error) {
	return rest.PostJSON[any, InventoryResponse](ctx, c.rest, "/inventory/refresh", nil, nil)
}

// GetMyGroup retrieves store group details of the authenticated bot.
func (c *Client) GetMyGroup(ctx context.Context) (*Group, error) {
	resp, err := rest.GetJSON[GroupResponse](ctx, c.rest, "/groups/my", nil)
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
	_, err := rest.PostJSON[any, Response](
		ctx,
		c.rest,
		"/groups/{groupID}/invite",
		map[string]string{"steam_id": targetSteamID.String()},
		nil,
		rest.WithVar("groupID", groupID),
	)

	return err
}

// GetPendingInvites retrieves pending store group invitations.
func (c *Client) GetPendingInvites(ctx context.Context) ([]Invite, error) {
	resp, err := rest.GetJSON[InvitesResponse](ctx, c.rest, "/groups/invites", nil)
	if err != nil {
		return nil, err
	}

	return resp.Invites, nil
}

// AcceptGroupInvite accepts a pending group invite.
func (c *Client) AcceptGroupInvite(ctx context.Context, groupID int) error {
	_, err := rest.PostJSON[any, Response](
		ctx,
		c.rest,
		"/groups/{groupID}/accept",
		nil,
		nil,
		rest.WithVar("groupID", groupID),
	)

	return err
}

// LeaveGroup leaves a store group.
func (c *Client) LeaveGroup(ctx context.Context, groupID int) error {
	_, err := rest.PostJSON[any, Response](
		ctx,
		c.rest,
		"/groups/{groupID}/leave",
		nil,
		nil,
		rest.WithVar("groupID", groupID),
	)

	return err
}

// FetchAuthToken requests an SSE auth token from Crit.tf API.
func (c *Client) FetchAuthToken(ctx context.Context) (string, error) {
	resp, err := c.rest.Request(ctx, http.MethodGet, "/bot-api/auth-token", nil, nil)
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
	reqURL := streamURL
	if token != "" {
		if strings.Contains(reqURL, "?") {
			reqURL += "&token=" + token
		} else {
			reqURL += "?token=" + token
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("crit: failed to create stream request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("User-Agent", c.rest.UserAgent())

	resp, err := c.rest.HTTP().
		Do(req) //nolint:bodyclose
	if err != nil {
		return nil, fmt.Errorf("crit: stream request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("crit: stream handshake failed with status %d", resp.StatusCode)
	}

	out := make(chan SSEEvent, 100)

	go func() {
		defer func() {
			_ = resp.Body.Close()
		}()
		defer close(out)

		scanner := bufio.NewScanner(resp.Body)

		var currentEvent SSEEvent

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				// Empty line indicates the end of an event block.
				if currentEvent.Event != "" || currentEvent.Data != "" {
					select {
					case <-ctx.Done():
						return
					case out <- currentEvent:
					}

					currentEvent = SSEEvent{}
				}

				continue
			}

			if strings.HasPrefix(line, ":") {
				// SSE comment, ignore
				continue
			}

			parts := strings.SplitN(line, ":", 2)
			if len(parts) < 2 {
				continue
			}

			key := parts[0]
			value := strings.TrimSpace(parts[1])

			switch key {
			case "event":
				currentEvent.Event = value
			case "data":
				if currentEvent.Data != "" {
					currentEvent.Data += "\n" + value
				} else {
					currentEvent.Data = value
				}
			}
		}
	}()

	return out, nil
}

// SendDeadMansRequest sends a heartbeat signal to Crit.tf backend to indicate the bot is alive.
func (c *Client) SendDeadMansRequest(ctx context.Context) (bool, error) {
	payload := map[string]bool{"alive": true}

	resp, err := c.rest.Request(ctx, http.MethodPost, "/bot-api/alive", payload, nil)
	if err != nil {
		return false, fmt.Errorf("crit: dead man request failed: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// GetInventory retrieves the cached inventory of the bot from Crit.tf backend.
func (c *Client) GetInventory(ctx context.Context) ([]any, error) {
	resp, err := rest.GetJSON[InventoryResponse](ctx, c.rest, "/inventory", nil)
	if err != nil {
		return nil, err
	}

	return resp.Items, nil
}

// UpdateTradeURL updates the bot's trade URL on Crit.tf.
func (c *Client) UpdateTradeURL(ctx context.Context, tradeURL string) (bool, error) {
	payload := map[string]string{"trade_url": tradeURL}
	putMod := func(req *http.Request) {
		req.Method = http.MethodPut
	}

	resp, err := rest.PostJSON[any, Response](ctx, c.rest, "/user/trade-url", payload, nil, putMod)
	if err != nil {
		return false, err
	}

	return resp.Success, nil
}

// GetUserInfo retrieves the authenticated user information from Crit.tf.
func (c *Client) GetUserInfo(ctx context.Context) (*User, error) {
	resp, err := rest.GetJSON[UserResponse](ctx, c.rest, "/user", nil)
	if err != nil {
		return nil, err
	}

	return resp.User, nil
}
