// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package crit provides an integration client for crit.tf classifieds, group stores and showcase APIs.
package crit

import (
	"context"
	"errors"
	"net/http"

	"github.com/lemon4ksan/g-man/pkg/rest"
	"github.com/lemon4ksan/g-man/pkg/steam/id"

	"github.com/lemon4ksan/g-man-tf2/pkg/pricedb"
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
