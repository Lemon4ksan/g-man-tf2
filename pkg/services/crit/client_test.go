// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crit

import (
	"errors"
	"testing"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
)

func setupTestClient(t *testing.T) (*Client, *mock.HTTPStub) {
	t.Helper()

	stub := mock.NewHTTPStub()
	client := NewClient(aoni.NewClient(stub), "test-api-key")

	return client, stub
}

func TestClient(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("fetch_my_listings", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		resp := map[string]any{
			"success": true,
			"listings": []any{
				map[string]any{},
				map[string]any{},
			},
		}
		stub.SetJSONResponse("api/v2/listings/my", 200, resp)

		listings, err := client.FetchMyListings(ctx)
		require.NoError(t, err)
		assert.Len(t, listings, 2)
	})

	t.Run("create_listing", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/v2/listings", 200, map[string]any{
			"success": true,
			"listing": map[string]any{},
		})

		listing, err := client.CreateListing(ctx, "123456", pricedb.Currencies{Keys: 2, Metal: 15.5})
		require.NoError(t, err)
		assert.NotNil(t, listing)

		stub.SetJSONResponse("api/v2/listings", 200, map[string]any{
			"success": true,
			"listing": nil,
		})

		_, err = client.CreateListing(ctx, "123", pricedb.Currencies{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "crit: listing is missing")
	})

	t.Run("update_listing", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/v2/listings/listing-123", 200, map[string]any{
			"success": true,
			"listing": map[string]any{},
		})

		listing, err := client.UpdateListing(ctx, "listing-123", pricedb.Currencies{Keys: 3, Metal: 25.0})
		require.NoError(t, err)
		assert.NotNil(t, listing)

		stub.SetJSONResponse("api/v2/listings/listing-123", 200, map[string]any{
			"success": true,
			"listing": nil,
		})

		_, err = client.UpdateListing(ctx, "listing-123", pricedb.Currencies{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "crit: listing is missing")

		// Error path for UpdateListing
		stub.ResponseErrs["api/v2/listings/listing-123"] = errors.New("network error")
		_, err = client.UpdateListing(ctx, "listing-123", pricedb.Currencies{})
		assert.Error(t, err)
	})

	t.Run("delete_listing", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/v2/listings/listing-123", 200, map[string]any{"success": true})

		err := client.DeleteListing(ctx, "listing-123")
		require.NoError(t, err)
	})

	t.Run("refresh_inventory", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/v2/inventory/refresh", 200, map[string]any{"success": true})

		resp, err := client.RefreshInventory(ctx)
		require.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("groups_management", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/v2/groups/my", 200, map[string]any{
			"success": true,
			"group":   map[string]any{},
		})

		group, err := client.GetMyGroup(ctx)
		require.NoError(t, err)
		assert.NotNil(t, group)

		stub.SetJSONResponse("api/v2/groups/my", 200, map[string]any{
			"success": true,
			"group":   nil,
		})

		_, err = client.GetMyGroup(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "crit: group is missing")

		stub.SetJSONResponse("api/v2/groups/456/invite", 200, map[string]any{"success": true})

		err = client.InviteToGroup(ctx, 456, id.ID(76561198000000000))
		require.NoError(t, err)

		stub.SetJSONResponse("api/v2/groups/invites", 200, map[string]any{
			"success": true,
			"invites": []any{map[string]any{}, map[string]any{}},
		})

		invites, err := client.GetPendingInvites(ctx)
		require.NoError(t, err)
		assert.Len(t, invites, 2)

		stub.SetJSONResponse("api/v2/groups/789/accept", 200, map[string]any{"success": true})

		err = client.AcceptGroupInvite(ctx, 789)
		require.NoError(t, err)

		stub.SetJSONResponse("api/v2/groups/101/leave", 200, map[string]any{"success": true})

		err = client.LeaveGroup(ctx, 101)
		require.NoError(t, err)
	})

	t.Run("missing_api_methods", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/v2/bot-api/auth-token", 200, map[string]any{
			"ok":    true,
			"token": "token_abc",
		})

		token, err := client.FetchAuthToken(ctx)
		require.NoError(t, err)
		assert.Equal(t, "token_abc", token)

		stub.SetRawResponse("api/v2/bot-api/alive", 200, []byte(`{"success":true}`))

		ok, err := client.SendDeadMansRequest(ctx)
		require.NoError(t, err)
		assert.True(t, ok)

		// GetInventory and error path
		stub.SetJSONResponse("api/v2/inventory", 200, map[string]any{
			"success": true,
			"items":   []any{"item1"},
		})

		inv, err := client.GetInventory(ctx)
		require.NoError(t, err)
		assert.Len(t, inv, 1)

		stub.ResponseErrs["api/v2/inventory"] = errors.New("network error")
		_, err = client.GetInventory(ctx)
		assert.Error(t, err)

		// UpdateTradeURL and error path
		stub.SetJSONResponse("api/v2/user/trade-url", 200, map[string]any{
			"success": true,
		})

		ok, err = client.UpdateTradeURL(ctx, "trade_url_123")
		require.NoError(t, err)
		assert.True(t, ok)

		stub.ResponseErrs["api/v2/user/trade-url"] = errors.New("network error")
		_, err = client.UpdateTradeURL(ctx, "trade_url_123")
		assert.Error(t, err)

		// GetUserInfo and error path
		stub.SetJSONResponse("api/v2/user", 200, map[string]any{
			"success": true,
			"user":    map[string]any{"steam_id": "123"},
		})

		usr, err := client.GetUserInfo(ctx)
		require.NoError(t, err)
		assert.Equal(t, "123", usr.SteamID)

		stub.ResponseErrs["api/v2/user"] = errors.New("network error")
		_, err = client.GetUserInfo(ctx)
		assert.Error(t, err)
	})

	t.Run("stream_events", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		stub.SetRawResponse("api/v2/events", 200, []byte("event: test\ndata: payload\n\n"))

		ch, err := client.StreamEvents(ctx, "events", "token_123")
		require.NoError(t, err)
		assert.NotNil(t, ch)
	})

	t.Run("crit_response_errors", func(t *testing.T) {
		t.Parallel()

		r := &critResponse{Success: false, Message: ""}
		assert.EqualError(t, r.Error(), "crit: api error")

		err := r.UnmarshalJSON([]byte("{invalid-json}"))
		assert.Error(t, err)
	})

	t.Run("fetch_auth_token_errors", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		stub.SetRawResponse("api/v2/bot-api/auth-token", 500, []byte(`Internal Server Error`))

		_, err := client.FetchAuthToken(ctx)
		assert.Error(t, err)

		stub.SetRawResponse("api/v2/bot-api/auth-token", 200, []byte(`{invalid-json}`))

		_, err = client.FetchAuthToken(ctx)
		assert.Error(t, err)

		stub.SetJSONResponse("api/v2/bot-api/auth-token", 200, map[string]any{
			"ok":     false,
			"reason": "rate_limited",
		})

		_, err = client.FetchAuthToken(ctx)
		assert.ErrorContains(t, err, "crit: api error: rate_limited")

		stub.SetJSONResponse("api/v2/bot-api/auth-token", 200, map[string]any{
			"ok": false,
		})

		_, err = client.FetchAuthToken(ctx)
		assert.ErrorContains(t, err, "crit: api error: unknown reason")

		// Network request failure
		stub.ResponseErrs["api/v2/bot-api/auth-token"] = errors.New("connection failed")
		_, err = client.FetchAuthToken(ctx)
		assert.Error(t, err)
	})

	t.Run("stream_events_token_empty_and_error", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		stub.SetRawResponse("api/v2/events", 200, []byte("event: test\ndata: payload\n\n"))

		ch, err := client.StreamEvents(ctx, "events", "")
		require.NoError(t, err)
		assert.NotNil(t, ch)

		stub.ResponseErrs["api/v2/events-error"] = errors.New("network error")
		_, err = client.StreamEvents(ctx, "events-error", "token")
		assert.Error(t, err)
	})

	t.Run("send_dead_mans_request_errors", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)

		stub.ResponseErrs["api/v2/bot-api/alive"] = errors.New("network error")
		_, err := client.SendDeadMansRequest(ctx)
		assert.Error(t, err)

		stub.ResponseErrs["api/v2/bot-api/alive"] = nil
		stub.SetRawResponse("api/v2/bot-api/alive", 500, []byte(`Internal Server Error`))

		ok, err := client.SendDeadMansRequest(ctx)
		require.NoError(t, err)
		assert.False(t, ok)
	})
}
