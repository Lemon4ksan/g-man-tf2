// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"context"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/option"
)

// BaseURL is the default API base endpoint.
const BaseURL = "https://backpack.tf/api"

// API defines the declarative REST API endpoints for the Backpack.tf platform.
//
// @aoni:service casing=snake_case
// @base_url "https://backpack.tf/api"
// @version "v1.0.0"
// @source "https://api.backpack.tf/api/swagger.json"
type API interface {
	// Get — Get client info
	//
	// @get ""
	Get(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error)

	// GetIgetCurrenciesV1 — Returns internal currency data for a given game
	//
	// @get "IGetCurrencies/v1"
	GetIgetCurrenciesV1(ctx context.Context, raw int, mods ...aoni.RequestModifier) (map[string]any, error)

	// GetIgetPriceHistoryV1 — Returns price history for an item
	//
	// @get "IGetPriceHistory/v1"
	GetIgetPriceHistoryV1(
		ctx context.Context,
		appid string,
		item string,
		quality string,
		tradable string,
		craftable string,
		priceindex string,
		mods ...aoni.RequestModifier,
	) (*V1priceHistoryResponse, error)

	// GetIgetPricesV4 — Get price schema
	//
	// @get "IGetPrices/v4"
	GetIgetPricesV4(ctx context.Context, raw, since int, mods ...aoni.RequestModifier) (*V4pricesResponse, error)

	// GetIgetSpecialItemsV1 — Get special internal items
	//
	// @get "IGetSpecialItems/v1"
	GetIgetSpecialItemsV1(ctx context.Context, appid int, mods ...aoni.RequestModifier) (*SpecialItemsResponse, error)

	// GetIgetUsersGetImpersonatedUsers — Get impersonated users
	//
	// @get "IGetUsers/GetImpersonatedUsers"
	GetIgetUsersGetImpersonatedUsers(
		ctx context.Context,
		limit, skip int,
		mods ...aoni.RequestModifier,
	) (map[string]any, error)

	// GetIgetUsersV3 — Get user data
	//
	// @get "IGetUsers/v3"
	GetIgetUsersV3(ctx context.Context, steamid, steamids []string, mods ...aoni.RequestModifier) (any, error)

	// PostAgentPulse — (Re)-register a user agent
	//
	// @post "agent/pulse"
	PostAgentPulse(ctx context.Context, mods ...aoni.RequestModifier) (*UserAgentStatus, error)

	// PostAgentStatus — Get agent status
	//
	// @post "agent/status"
	PostAgentStatus(ctx context.Context, mods ...aoni.RequestModifier) (*UserAgentStatus, error)

	// PostAgentStop — Unregister a user agent
	//
	// @post "agent/stop"
	PostAgentStop(ctx context.Context, mods ...aoni.RequestModifier) (*UserAgentStatus, error)

	// GetClassifiedsAlerts — Get alerts
	//
	// @get "classifieds/alerts"
	GetClassifiedsAlerts(ctx context.Context, limit, skip int, mods ...aoni.RequestModifier) (map[string]any, error)

	// PostClassifiedsAlerts — Create alert
	//
	// @post "classifieds/alerts"
	// @query casing=snake_case
	PostClassifiedsAlerts(
		ctx context.Context,
		blanket int,
		currency string,
		intent string,
		itemName string,
		max int,
		min int,
		mods ...aoni.RequestModifier,
	) (map[string]any, error)

	// DeleteClassifiedsAlerts — Delete an alert by item name and intent
	//
	// @delete "classifieds/alerts"
	// @query casing=snake_case
	DeleteClassifiedsAlerts(
		ctx context.Context,
		intent, itemName string,
		mods ...aoni.RequestModifier,
	) (map[string]any, error)

	// GetClassifiedsAlertsByID — Get alert
	//
	// @get "classifieds/alerts/{id}"
	GetClassifiedsAlertsByID(ctx context.Context, iD string, mods ...aoni.RequestModifier) (map[string]any, error)

	// DeleteClassifiedsAlertsByID — Delete alert
	//
	// @delete "classifieds/alerts/{id}"
	DeleteClassifiedsAlertsByID(ctx context.Context, iD string, mods ...aoni.RequestModifier) (map[string]any, error)

	// DeleteClassifiedsDeleteV1 — Bulk delete listings
	//
	// @delete "classifieds/delete/v1"
	// @json
	DeleteClassifiedsDeleteV1(ctx context.Context, req []string, mods ...aoni.RequestModifier) (map[string]any, error)

	// PostClassifiedsListV1 — Bulk create classifieds listings
	//
	// @post "classifieds/list/v1"
	PostClassifiedsListV1(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error)

	// GetClassifiedsListingsV1 — Get session user listings
	//
	// @get "classifieds/listings/v1"
	GetClassifiedsListingsV1(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error)

	// PostInventoryBySteamidRefresh — Refresh an inventory.
	//
	// @post "inventory/{steamid}/refresh"
	PostInventoryBySteamidRefresh(
		ctx context.Context,
		steamid string,
		mods ...aoni.RequestModifier,
	) (*InventoryStatus, error)

	// GetInventoryBySteamidStatus — Get the status of an inventory.
	//
	// @get "inventory/{steamid}/status"
	GetInventoryBySteamidStatus(
		ctx context.Context,
		steamid string,
		mods ...aoni.RequestModifier,
	) (*InventoryStatus, error)

	// GetInventoryBySteamidValues — Get inventory values.
	//
	// @get "inventory/{steamid}/values"
	GetInventoryBySteamidValues(
		ctx context.Context,
		steamid string,
		mods ...aoni.RequestModifier,
	) (*InventoryValues, error)

	// GetNotifications — Get notifications
	//
	// @get "notifications"
	GetNotifications(
		ctx context.Context,
		limit int,
		skip int,
		unread int,
		mods ...aoni.RequestModifier,
	) (map[string]any, error)

	// PostNotificationsMark — Mark unread notifications as read
	//
	// @post "notifications/mark"
	PostNotificationsMark(ctx context.Context, mods ...aoni.RequestModifier) (*NotificationMarkState, error)

	// PostNotificationsUnread — Get unread notifications and mark them as read
	//
	// @post "notifications/unread"
	PostNotificationsUnread(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error)

	// GetNotificationsByID — Get notification
	//
	// @get "notifications/{id}"
	GetNotificationsByID(ctx context.Context, iD string, mods ...aoni.RequestModifier) (map[string]any, error)

	// DeleteNotificationsByID — Delete notification
	//
	// @delete "notifications/{id}"
	DeleteNotificationsByID(ctx context.Context, iD string, mods ...aoni.RequestModifier) (map[string]any, error)

	// GetUsersInfoV1 — Get user info
	//
	// @get "users/info/v1"
	GetUsersInfoV1(ctx context.Context, steamids string, mods ...aoni.RequestModifier) (map[string]any, error)

	// GetV2ClassifiedsArchive — Get account archived listings
	//
	// @get "v2/classifieds/archive"
	GetV2ClassifiedsArchive(
		ctx context.Context,
		limit, skip int,
		mods ...aoni.RequestModifier,
	) (*ListingScrollable, error)

	// DeleteV2ClassifiedsArchive — Delete all archived listings
	//
	// @delete "v2/classifieds/archive"
	// @json
	DeleteV2ClassifiedsArchive(
		ctx context.Context,
		req ListingDropRequest,
		mods ...aoni.RequestModifier,
	) (map[string]any, error)

	// GetV2ClassifiedsArchiveBatch — Get batch operation limit
	//
	// @get "v2/classifieds/archive/batch"
	GetV2ClassifiedsArchiveBatch(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error)

	// DeleteV2ClassifiedsArchiveBatch — Batch delete archived listings
	//
	// @delete "v2/classifieds/archive/batch"
	// @json
	DeleteV2ClassifiedsArchiveBatch(
		ctx context.Context,
		req ListingBatchDeleteRequest,
		mods ...aoni.RequestModifier,
	) (map[string]any, error)

	// GetV2ClassifiedsArchiveByListingID — Get one archived listing
	//
	// @get "v2/classifieds/archive/{listingId}"
	GetV2ClassifiedsArchiveByListingID(
		ctx context.Context,
		listingID string,
		mods ...aoni.RequestModifier,
	) (*Listing, error)

	// DeleteV2ClassifiedsArchiveByListingID — Delete one archived listing
	//
	// @delete "v2/classifieds/archive/{listingId}"
	DeleteV2ClassifiedsArchiveByListingID(
		ctx context.Context,
		listingID string,
		mods ...aoni.RequestModifier,
	) (map[string]any, error)

	// PatchV2ClassifiedsArchiveByListingID — Update one archived listing
	//
	// @patch "v2/classifieds/archive/{listingId}"
	// @json
	PatchV2ClassifiedsArchiveByListingID(
		ctx context.Context,
		listingID string,
		req ListingPatchRequest,
		mods ...aoni.RequestModifier,
	) (*Listing, error)

	// PostV2ClassifiedsArchiveByListingIDPublish — Publish one archived listing to the active pool
	//
	// @post "v2/classifieds/archive/{listingId}/publish"
	PostV2ClassifiedsArchiveByListingIDPublish(
		ctx context.Context,
		listingID string,
		mods ...aoni.RequestModifier,
	) (*Listing, error)

	// GetV2ClassifiedsListings — Get account listings
	//
	// @get "v2/classifieds/listings"
	GetV2ClassifiedsListings(
		ctx context.Context,
		bumpedSince int,
		createdSince int,
		limit int,
		skip int,
		mods ...aoni.RequestModifier,
	) (*ListingScrollable, error)

	// PostV2ClassifiedsListings — Create one listing
	//
	// @post "v2/classifieds/listings"
	// @json
	PostV2ClassifiedsListings(
		ctx context.Context,
		req ListingResolvable,
		mods ...aoni.RequestModifier,
	) (*Listing, error)

	// DeleteV2ClassifiedsListings — Delete all listings
	//
	// @delete "v2/classifieds/listings"
	// @json
	DeleteV2ClassifiedsListings(
		ctx context.Context,
		req ListingDropRequest,
		mods ...aoni.RequestModifier,
	) (map[string]any, error)

	// PostV2ClassifiedsListingsArchiveAll — Archive all listings
	//
	// @post "v2/classifieds/listings/archiveAll"
	PostV2ClassifiedsListingsArchiveAll(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error)

	// GetV2ClassifiedsListingsBatch — Get batch operation limit
	//
	// @get "v2/classifieds/listings/batch"
	GetV2ClassifiedsListingsBatch(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error)

	// PostV2ClassifiedsListingsBatch — Batch create listings
	//
	// @post "v2/classifieds/listings/batch"
	// @json
	PostV2ClassifiedsListingsBatch(
		ctx context.Context,
		req []ListingResolvable,
		mods ...aoni.RequestModifier,
	) ([]*ListingBatchCreateResult, error)

	// PatchV2ClassifiedsListingsBatch — Batch update listings
	//
	// @patch "v2/classifieds/listings/batch"
	// @json
	PatchV2ClassifiedsListingsBatch(
		ctx context.Context,
		req []ListingBatchUpdateItem,
		mods ...aoni.RequestModifier,
	) (*ListingBatchUpdateResponse, error)

	// DeleteV2ClassifiedsListingsBatch — Batch delete listings
	//
	// @delete "v2/classifieds/listings/batch"
	// @json
	DeleteV2ClassifiedsListingsBatch(
		ctx context.Context,
		req ListingBatchDeleteRequest,
		mods ...aoni.RequestModifier,
	) (map[string]any, error)

	// PostV2ClassifiedsListingsPublishAll — Publish all listings
	//
	// @post "v2/classifieds/listings/publishAll"
	PostV2ClassifiedsListingsPublishAll(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error)

	// GetV2ClassifiedsListingsByListingID — Get one listing
	//
	// @get "v2/classifieds/listings/{listingId}"
	GetV2ClassifiedsListingsByListingID(
		ctx context.Context,
		listingID string,
		mods ...aoni.RequestModifier,
	) (*Listing, error)

	// DeleteV2ClassifiedsListingsByListingID — Delete one listing
	//
	// @delete "v2/classifieds/listings/{listingId}"
	DeleteV2ClassifiedsListingsByListingID(
		ctx context.Context,
		listingID string,
		mods ...aoni.RequestModifier,
	) (map[string]any, error)

	// PatchV2ClassifiedsListingsByListingID — Update one listing
	//
	// @patch "v2/classifieds/listings/{listingId}"
	// @json
	PatchV2ClassifiedsListingsByListingID(
		ctx context.Context,
		listingID string,
		req ListingPatchRequest,
		mods ...aoni.RequestModifier,
	) (*Listing, error)

	// PostV2ClassifiedsListingsByListingIDArchive — Move listing to the archive
	//
	// @post "v2/classifieds/listings/{listingId}/archive"
	PostV2ClassifiedsListingsByListingIDArchive(
		ctx context.Context,
		listingID string,
		mods ...aoni.RequestModifier,
	) (*Listing, error)

	// PostV2ClassifiedsListingsByListingIDDemote — Demote this listing
	//
	// @post "v2/classifieds/listings/{listingId}/demote"
	PostV2ClassifiedsListingsByListingIDDemote(
		ctx context.Context,
		listingID string,
		mods ...aoni.RequestModifier,
	) (*Listing, error)

	// PostV2ClassifiedsListingsByListingIDPromote — Promote this listing
	//
	// @post "v2/classifieds/listings/{listingId}/promote"
	PostV2ClassifiedsListingsByListingIDPromote(
		ctx context.Context,
		listingID string,
		mods ...aoni.RequestModifier,
	) (*Listing, error)
}

// Client is an alias for API for backward compatibility.
type Client = API

// NewClient creates a new Backpack.tf API client configured with optional apiKey (for Web API endpoints)
// and userToken (for Classifieds listings management).
func NewClient(doer any, apiKey, userToken string, opts ...aoni.ClientOption) API {
	var defaultOpts []aoni.ClientOption

	defaultOpts = append(defaultOpts, option.WithBaseURL(BaseURL))
	if userToken != "" {
		defaultOpts = append(defaultOpts, option.WithHeader("X-Auth-Token", userToken))
	}

	if apiKey != "" {
		defaultOpts = append(defaultOpts, option.WithHeader("token", apiKey))
	}

	defaultOpts = append(defaultOpts, opts...)

	return New(doer, defaultOpts...)
}
