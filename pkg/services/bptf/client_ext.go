// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"context"
	"strings"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/mod"
	"github.com/lemon4ksan/aoni/request"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
)

// V4PricesBaseItemDocExt represents the item details in backpack.tf V4 pricing schema.
type V4PricesBaseItemDocExt struct {
	Defindex []string                                                  `json:"defindex,omitempty"`
	Prices   map[string]map[string]map[string]map[string]V4PricesEntry `json:"prices,omitempty"`
}

// V4PricesResponseExt represents the full response from IGetPrices/v4.
type V4PricesResponseExt struct {
	CurrentTime      int                               `json:"current_time,omitempty"`
	Items            map[string]V4PricesBaseItemDocExt `json:"items,omitempty"`
	Message          string                            `json:"message,omitempty"`
	RawUsdValue      int                               `json:"raw_usd_value,omitempty"`
	Success          SuccessBoolProp                   `json:"success,omitempty"`
	UsdCurrency      string                            `json:"usd_currency,omitempty"`
	UsdCurrencyIndex int                               `json:"usd_currency_index,omitempty"`
}

// UserBans contains ban indicators from backpack.tf and SteamRep.
type UserBans struct {
	All             string `json:"all,omitempty"`
	BPTF            string `json:"bptf,omitempty"`
	SteamRepScammer int    `json:"steamrep_scammer,omitempty"`
}

// UserTrust contains trust ratings on backpack.tf.
type UserTrust struct {
	Positive int `json:"positive,omitempty"`
	Negative int `json:"negative,omitempty"`
}

// UserInfo represents detailed user information on backpack.tf.
type UserInfo struct {
	Name       string     `json:"name,omitempty"`
	Avatar     string     `json:"avatar,omitempty"`
	LastOnline int64      `json:"last_online,omitempty"`
	Bans       *UserBans  `json:"bans,omitempty"`
	Trust      *UserTrust `json:"trust,omitempty"`
}

// V1UserResponse represents the response wrapper for /users/info/v1.
type V1UserResponse struct {
	Users map[id.ID]UserInfo `json:"users,omitempty"`
}

// GetPricesV4 returns the current pricing scheme (IGetPrices/v4).
func (c *Client) GetPricesV4(ctx context.Context, raw int, since int64) (*V4PricesResponseExt, error) {
	mods := []aoni.RequestModifier{
		mod.WithQuery(GetIgetPricesV4Query{
			Raw:   raw,
			Since: int(since),
		}),
	}

	return request.GetTo[V4PricesResponseExt](ctx, c.r, "IGetPrices/v4", mods...)
}

// GetCurrencies returns a list of currencies (IGetCurrencies/v1).
func (c *Client) GetCurrencies(ctx context.Context, raw int) (map[string]any, error) {
	return c.GetIgetCurrenciesV1(ctx, GetIgetCurrenciesV1Query{
		Raw: raw,
	})
}

// CreateListing creates a buy or sell listing.
func (c *Client) CreateListing(ctx context.Context, listing ListingResolvable) (*Listing, error) {
	return c.PostClassifiedsListingsV2(ctx, listing)
}

// BatchCreateListings allows creating up to 100 listings in one request.
func (c *Client) BatchCreateListings(
	ctx context.Context,
	listings []ListingResolvable,
) ([]ListingBatchCreateResult, error) {
	resp, err := c.PostClassifiedsListingsBatchV2(ctx, listings)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetInventoryStatus returns the status of a user's inventory on backpack.tf.
func (c *Client) GetInventoryStatus(ctx context.Context, steamID id.ID) (*InventoryStatus, error) {
	return c.GetInventoryBySteamIDStatus(ctx, mod.WithVar("steamid", steamID.String()))
}

// GetInventoryValues returns the total value of a user's inventory.
func (c *Client) GetInventoryValues(ctx context.Context, steamID id.ID) (*InventoryValues, error) {
	return c.GetInventoryBySteamIDValues(ctx, mod.WithVar("steamid", steamID.String()))
}

// RefreshInventory requests backpack.tf to fetch the latest data from Steam.
func (c *Client) RefreshInventory(ctx context.Context, steamID id.ID) (*InventoryStatus, error) {
	return c.PostInventoryBySteamIDRefresh(ctx, mod.WithVar("steamid", steamID.String()))
}

// GetUsersInfo returns detailed information for a list of SteamIDs.
func (c *Client) GetUsersInfo(ctx context.Context, steamIDs []id.ID) (*V1UserResponse, error) {
	ids := make([]string, len(steamIDs))
	for i, sID := range steamIDs {
		ids[i] = sID.String()
	}

	mods := []aoni.RequestModifier{
		mod.WithQuery(GetUsersInfoV1Query{
			Steamids: strings.Join(ids, ","),
		}),
	}

	return request.GetTo[V1UserResponse](ctx, c.r, "users/info/v1", mods...)
}

// GetAlerts returns a list of active listing alerts for the current user.
func (c *Client) GetAlerts(ctx context.Context, skip, limit int) (map[string]any, error) {
	return c.GetClassifiedsAlerts(ctx, GetClassifiedsAlertsQuery{
		Skip:  skip,
		Limit: limit,
	})
}

// CreateAlert creates a new listing alert for a specific item.
func (c *Client) CreateAlert(
	ctx context.Context,
	itemName, intent, currency string,
	minVal, maxVal int,
) (map[string]any, error) {
	return c.PostClassifiedsAlerts(ctx, PostClassifiedsAlertsQuery{
		ItemName: itemName,
		Intent:   ListingIntent(intent),
		Currency: currency,
		Min:      minVal,
		Max:      maxVal,
	})
}

// GetListings returns a list of active listings for the current account.
func (c *Client) GetListings(ctx context.Context, skip, limit int) (*ListingScrollable, error) {
	return c.GetClassifiedsListingsV2(ctx, GetClassifiedsListingsV2Query{
		Skip:  skip,
		Limit: limit,
	})
}

// DeleteListing deletes a single listing by its ID.
func (c *Client) DeleteListing(ctx context.Context, listingID string) error {
	_, err := c.DeleteClassifiedsListingsByListingIDV2(ctx, listingID)
	return err
}

// BatchDeleteListings deletes multiple listings at once.
func (c *Client) BatchDeleteListings(ctx context.Context) (map[string]any, error) {
	return c.DeleteClassifiedsListingsBatchV2(ctx)
}

// Pulse sends a heartbeat to backpack.tf to keep the bot online and bump listings.
func (c *Client) Pulse(ctx context.Context) (*UserAgentStatus, error) {
	return c.PostAgentPulse(ctx)
}

// StopAgent declares the user as no longer under control of the agent.
func (c *Client) StopAgent(ctx context.Context) (*UserAgentStatus, error) {
	return c.PostAgentStop(ctx)
}

// GetAgentStatus returns the current status of the user agent.
func (c *Client) GetAgentStatus(ctx context.Context) (*UserAgentStatus, error) {
	return c.PostAgentStatus(ctx)
}

// GetNotificationsExt returns user notifications.
func (c *Client) GetNotificationsExt(ctx context.Context, skip, limit int, unread bool) (map[string]any, error) {
	unreadInt := 0
	if unread {
		unreadInt = 1
	}

	return c.GetNotifications(ctx, GetNotificationsQuery{
		Skip:   skip,
		Limit:  limit,
		Unread: unreadInt,
	})
}

// MarkNotificationsRead marks all unread notifications as read.
func (c *Client) MarkNotificationsRead(ctx context.Context) (*NotificationMarkState, error) {
	return c.PostNotificationsMark(ctx)
}

// DeleteNotification deletes a notification by ID.
func (c *Client) DeleteNotification(ctx context.Context, id string) error {
	_, err := c.DeleteNotificationsByID(ctx, id)
	return err
}

// DeleteArchiveListings deletes all archived listings for the account.
func (c *Client) DeleteArchiveListings(ctx context.Context, req ListingDropRequest) error {
	_, err := c.DeleteClassifiedsArchiveV2(ctx, req)
	return err
}

// DeleteAllListings deletes all active listings for the account.
func (c *Client) DeleteAllListings(ctx context.Context, req ListingDropRequest) error {
	_, err := c.DeleteClassifiedsListingsV2(ctx, req)
	return err
}
