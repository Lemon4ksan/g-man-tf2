// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"testing"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestClient(t *testing.T) (*Client, *mock.HTTPStub) {
	t.Helper()

	stub := mock.NewHTTPStub()
	client := New(aoni.NewClient(stub), "api-key", "user-token")

	return client, stub
}

func TestClient(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	steamID := id.New(76561198033830321)

	t.Run("prices_and_currencies", func(t *testing.T) {
		t.Parallel()

		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/IGetPrices/v4", 200, PricesResponseV4{Success: 1})
		stub.SetJSONResponse("api/IGetCurrencies/v1", 200, CurrenciesResponseV1{Success: 1})

		prices, err := client.GetPricesV4(ctx, 1, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, prices.Success)

		currencies, err := client.GetCurrencies(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, 1, currencies.Success)
	})

	t.Run("listings_lifecycle", func(t *testing.T) {
		t.Parallel()

		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/v2/classifieds/listings", 200, ListingResponse{ID: "123"})

		listing, err := client.CreateListing(ctx, ListingResolvable{})
		require.NoError(t, err)
		assert.Equal(t, "123", listing.ID)

		stub.SetJSONResponse(
			"api/v2/classifieds/listings/batch",
			200,
			[]ListingBatchCreateResult{{Result: &ListingResponse{ID: "123"}}},
		)

		batch, err := client.BatchCreateListings(ctx, []ListingResolvable{{}})
		require.NoError(t, err)
		assert.Len(t, batch, 1)
		assert.Equal(t, "123", batch[0].Result.ID)

		stub.SetJSONResponse("api/v2/classifieds/listings", 200, ListingsResponse{})

		listings, err := client.GetListings(ctx, 0, 100)
		require.NoError(t, err)
		assert.Empty(t, listings.Results)

		stub.SetRawResponse("api/v2/classifieds/listings/123", 200, nil)

		err = client.DeleteListing(ctx, "123")
		require.NoError(t, err)

		stub.SetRawResponse("api/v2/classifieds/listings/batch", 200, nil)

		err = client.BatchDeleteListings(ctx, []string{"123"})
		require.NoError(t, err)

		stub.SetRawResponse("api/v2/classifieds/listings", 200, nil)

		err = client.DeleteAllListings(ctx, ListingDropRequest{})
		require.NoError(t, err)

		stub.SetJSONResponse("api/v2/classifieds/listings/batch", 200, map[string]any{"limit": 100})

		limit, err := client.GetListingsBatchLimit(ctx)
		require.NoError(t, err)
		assert.Equal(t, float64(100), limit["limit"])

		stub.SetJSONResponse("api/v2/classifieds/listings/list_123", 200, ListingResponse{ID: "list_123"})

		gotList, err := client.GetListing(ctx, "list_123")
		require.NoError(t, err)
		assert.Equal(t, "list_123", gotList.ID)

		stub.SetJSONResponse("api/v2/classifieds/listings/list_123", 200, ListingResponse{ID: "list_123"})

		patchedList, err := client.PatchListing(ctx, "list_123", ListingPatchRequest{})
		require.NoError(t, err)
		assert.Equal(t, "list_123", patchedList.ID)
	})

	t.Run("inventory", func(t *testing.T) {
		t.Parallel()

		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/inventory/76561198033830321/status", 200, InventoryStatus{CurrentTime: 1600000000})

		status, err := client.GetInventoryStatus(ctx, steamID)
		require.NoError(t, err)
		assert.Equal(t, int64(1600000000), status.CurrentTime)

		stub.SetJSONResponse("api/inventory/76561198033830321/values", 200, InventoryValues{Value: 123.45})

		values, err := client.GetInventoryValues(ctx, steamID)
		require.NoError(t, err)
		assert.Equal(t, 123.45, values.Value)

		stub.SetJSONResponse("api/inventory/76561198033830321/refresh", 200, InventoryStatus{CurrentTime: 1600000000})

		refreshed, err := client.RefreshInventory(ctx, steamID)
		require.NoError(t, err)
		assert.Equal(t, int64(1600000000), refreshed.CurrentTime)
	})

	t.Run("users_and_agents", func(t *testing.T) {
		t.Parallel()

		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/users/info/v1", 200, V1UserResponse{})

		_, err := client.GetUsersInfo(ctx, []id.ID{steamID})
		require.NoError(t, err)

		stub.SetJSONResponse("api/agent/pulse", 200, UserAgentStatus{Status: "active"})

		pulse, err := client.Pulse(ctx)
		require.NoError(t, err)
		assert.Equal(t, "active", pulse.Status)

		stub.SetJSONResponse("api/agent/stop", 200, UserAgentStatus{Status: "inactive"})

		stop, err := client.StopAgent(ctx)
		require.NoError(t, err)
		assert.Equal(t, "inactive", stop.Status)

		stub.SetJSONResponse("api/agent/status", 200, UserAgentStatus{Status: "active"})

		status, err := client.GetAgentStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, "active", status.Status)
	})

	t.Run("alerts", func(t *testing.T) {
		t.Parallel()

		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/classifieds/alerts", 200, AlertsResponse{})

		alerts, err := client.GetAlerts(ctx, 0, 100)
		require.NoError(t, err)
		assert.Empty(t, alerts.Results)

		stub.SetJSONResponse("api/classifieds/alerts", 200, Alert{ID: "alert_123"})

		alert, err := client.CreateAlert(ctx, "Key", "buy", "ref", 1, 10)
		require.NoError(t, err)
		assert.Equal(t, "alert_123", alert.ID)

		stub.SetRawResponse("api/classifieds/alerts/alert_123", 200, nil)

		err = client.DeleteAlertByID(ctx, "alert_123")
		require.NoError(t, err)

		stub.SetRawResponse("api/classifieds/alerts", 200, nil)

		err = client.DeleteAlertByItem(ctx, "Key", "buy")
		require.NoError(t, err)
	})

	t.Run("notifications", func(t *testing.T) {
		t.Parallel()

		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/notifications", 200, NotificationsResponse{})

		notifs, err := client.GetNotifications(ctx, 0, 100, true)
		require.NoError(t, err)
		assert.Empty(t, notifs.Results)

		stub.SetJSONResponse("api/notifications/mark", 200, NotificationMarkResponse{Modified: 1})

		mark, err := client.MarkNotificationsRead(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, mark.Modified)

		stub.SetRawResponse("api/notifications/notif_123", 200, nil)

		err = client.DeleteNotification(ctx, "notif_123")
		require.NoError(t, err)
	})

	t.Run("history_and_classifieds", func(t *testing.T) {
		t.Parallel()

		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/IGetPriceHistory/v1", 200, PriceHistoryResponse{Success: 1})

		history, err := client.GetPriceHistory(ctx, 440, "Key", "6", "1", "1", "0")
		require.NoError(t, err)
		assert.Equal(t, 1, history.Success)

		stub.SetJSONResponse("api/v2/classifieds/archive", 200, ListingsResponse{})

		archive, err := client.GetArchiveListings(ctx, 0, 100)
		require.NoError(t, err)
		assert.Empty(t, archive.Results)

		stub.SetJSONResponse("api/classifieds/listings/snapshot", 200, SnapshotResponse{
			Listings: []ListingResponse{
				{ID: "123", Intent: "buy"},
				{ID: "456", Intent: "sell"},
			},
		})

		search, err := client.SearchClassifieds(ctx, "5021;6", "buy")
		require.NoError(t, err)
		assert.Len(t, search.Listings, 1)

		stub.SetRawResponse("api/v2/classifieds/archive", 200, nil)

		err = client.DeleteArchiveListings(ctx, ListingDropRequest{})
		require.NoError(t, err)

		stub.SetJSONResponse("api/v2/classifieds/archive/batch", 200, map[string]any{"limit": 100})

		archLimit, err := client.GetArchiveBatchLimit(ctx)
		require.NoError(t, err)
		assert.Equal(t, float64(100), archLimit["limit"])

		stub.SetJSONResponse("api/v2/classifieds/archive/batch", 200, map[string]any{"success": true})

		_, err = client.BatchDeleteArchiveListings(ctx)
		require.NoError(t, err)

		stub.SetJSONResponse("api/v2/classifieds/archive/arch_123", 200, ListingResponse{ID: "arch_123"})

		archList, err := client.GetArchiveListing(ctx, "arch_123")
		require.NoError(t, err)
		assert.Equal(t, "arch_123", archList.ID)

		stub.SetRawResponse("api/v2/classifieds/archive/arch_123", 200, nil)

		err = client.DeleteArchiveListing(ctx, "arch_123")
		require.NoError(t, err)

		stub.SetJSONResponse("api/v2/classifieds/archive/arch_123", 200, ListingResponse{ID: "arch_123"})

		patchedArch, err := client.PatchArchiveListing(ctx, "arch_123", ListingPatchRequest{})
		require.NoError(t, err)
		assert.Equal(t, "arch_123", patchedArch.ID)

		stub.SetJSONResponse("api/v2/classifieds/archive/arch_123/publish", 200, ListingResponse{ID: "arch_123"})

		published, err := client.PublishArchiveListing(ctx, "arch_123")
		require.NoError(t, err)
		assert.Equal(t, "arch_123", published.ID)
	})

	t.Run("low_level_rest_accessor", func(t *testing.T) {
		t.Parallel()

		client, _ := setupTestClient(t)
		rest := client.REST()
		assert.NotNil(t, rest)
	})
}
