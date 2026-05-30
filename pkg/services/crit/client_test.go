// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
)

func TestClient_FetchMyListings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/listings/my", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
			"listings": []any{
				map[string]any{},
				map[string]any{},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	listings, err := client.FetchMyListings(context.Background())
	require.NoError(t, err)
	assert.Len(t, listings, 2)
}

func TestClient_CreateListing(t *testing.T) {
	assetID := "123456"
	currencies := pricedb.Currencies{Keys: 2, Metal: 15.5}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/listings", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))

		var payload map[string]any

		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		assert.Equal(t, assetID, payload["asset_id"])
		assert.Equal(t, float64(2), payload["price_keys"])
		assert.Equal(t, 15.5, payload["price_metal"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
			"listing": map[string]any{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	listing, err := client.CreateListing(context.Background(), assetID, currencies)
	require.NoError(t, err)
	assert.NotNil(t, listing)
}

func TestClient_CreateListing_MissingListing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
			"listing": nil,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	_, err := client.CreateListing(context.Background(), "123", pricedb.Currencies{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "crit: listing is missing in response")
}

func TestClient_UpdateListing(t *testing.T) {
	listingID := "listing-123"
	currencies := pricedb.Currencies{Keys: 3, Metal: 25.0}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/listings/listing-123", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))

		var payload map[string]any

		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)

		assert.Equal(t, float64(3), payload["price_keys"])
		assert.Equal(t, float64(25), payload["price_metal"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
			"listing": map[string]any{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	listing, err := client.UpdateListing(context.Background(), listingID, currencies)
	require.NoError(t, err)
	assert.NotNil(t, listing)
}

func TestClient_UpdateListing_MissingListing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
			"listing": nil,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	_, err := client.UpdateListing(context.Background(), "listing-123", pricedb.Currencies{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "crit: listing is missing in response")
}

func TestClient_DeleteListing(t *testing.T) {
	listingID := "listing-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/listings/listing-123", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	err := client.DeleteListing(context.Background(), listingID)
	require.NoError(t, err)
}

func TestClient_RefreshInventory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/inventory/refresh", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	resp, err := client.RefreshInventory(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestClient_GetMyGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groups/my", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
			"group":   map[string]any{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	group, err := client.GetMyGroup(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, group)
}

func TestClient_GetMyGroup_MissingGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
			"group":   nil,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	_, err := client.GetMyGroup(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "crit: group is missing in response")
}

func TestClient_InviteToGroup(t *testing.T) {
	groupID := 456
	targetSteamID := id.ID(76561198000000000)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groups/456/invite", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))

		var payload map[string]string

		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)
		assert.Equal(t, "76561198000000000", payload["steam_id"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	err := client.InviteToGroup(context.Background(), groupID, targetSteamID)
	require.NoError(t, err)
}

func TestClient_GetPendingInvites(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groups/invites", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
			"invites": []any{
				map[string]any{},
				map[string]any{},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	invites, err := client.GetPendingInvites(context.Background())
	require.NoError(t, err)
	assert.Len(t, invites, 2)
}

func TestClient_AcceptGroupInvite(t *testing.T) {
	groupID := 789

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groups/789/accept", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	err := client.AcceptGroupInvite(context.Background(), groupID)
	require.NoError(t, err)
}

func TestClient_LeaveGroup(t *testing.T) {
	groupID := 101

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/groups/101/leave", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-API-Key"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := map[string]any{
			"success": true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(nil, "test-api-key")
	client.rest = client.rest.WithBaseURL(server.URL)

	err := client.LeaveGroup(context.Background(), groupID)
	require.NoError(t, err)
}
