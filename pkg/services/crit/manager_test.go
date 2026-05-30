// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/rest"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat/commands"
	"github.com/lemon4ksan/g-man/test/module"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

// Mock providers for event loop testing
type mockBackpackProvider struct {
	items []uint64
}

func (m *mockBackpackProvider) GetStock(sku string) int {
	return len(m.items)
}

func (m *mockBackpackProvider) GetItemsBySKU(targetSKU string) []uint64 {
	return m.items
}

type mockPriceProvider struct {
	sellKey   int
	sellMetal float64
}

func (m *mockPriceProvider) GetPrice(sku string) (*pricedb.Price, bool) {
	return &pricedb.Price{
		SKU: sku,
		Sell: pricedb.Currencies{
			Keys:  m.sellKey,
			Metal: m.sellMetal,
		},
	}, true
}

type mockConfigProvider struct {
	items map[string]trading.ItemConfig
}

func (m *mockConfigProvider) GetConfig() trading.Config {
	return trading.Config{
		Items: m.items,
	}
}

// Helper to construct a mock HTTP client
type mockHTTPClient struct {
	mu           sync.Mutex
	reqCount     int
	mockResponse func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	m.reqCount++
	m.mu.Unlock()

	return m.mockResponse(req)
}

func newMockResponse(statusCode int, bodyObj any) (*http.Response, error) {
	bodyBytes, err := json.Marshal(bodyObj)
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: statusCode,
		Status:     fmt.Sprintf("%d Status", statusCode),
		Body:       io.NopCloser(bytes.NewBuffer(bodyBytes)),
		Header:     make(http.Header),
	}, nil
}

func TestManager_InitAndLifecycle(t *testing.T) {
	httpClient := &mockHTTPClient{
		mockResponse: func(req *http.Request) (*http.Response, error) {
			return newMockResponse(200, Response{Success: true})
		},
	}
	client := NewClient(httpClient, "mock-api-key")
	mgr := NewManager(client)

	ictx := module.NewInitContext()
	err := mgr.Init(ictx)
	require.NoError(t, err)

	err = mgr.Start(t.Context())
	require.NoError(t, err)

	err = mgr.Close()
	require.NoError(t, err)
}

func TestManager_Bootstrap_Success(t *testing.T) {
	httpClient := &mockHTTPClient{
		mockResponse: func(req *http.Request) (*http.Response, error) {
			if strings.HasSuffix(req.URL.Path, "/listings/my") {
				return newMockResponse(200, ListingsResponse{
					Response: Response{Success: true},
					Listings: []Listing{
						{ID: 101, AssetID: "10001", SKU: "5021;6", PriceKeys: 1, PriceMetal: 15.0},
					},
				})
			}

			if strings.HasSuffix(req.URL.Path, "/groups/my") {
				return newMockResponse(200, GroupResponse{
					Response: Response{Success: true},
					Group: &Group{
						ID:              42,
						GroupName:       "Epic Store",
						CustomStoreSlug: "epic-store",
					},
				})
			}

			return newMockResponse(404, nil)
		},
	}

	client := NewClient(httpClient, "mock-api-key")
	mgr := NewManager(client)

	ictx := module.NewInitContext()
	err := mgr.Init(ictx)
	require.NoError(t, err)

	// Call bootstrap directly
	mgr.bootstrap(context.Background())

	// Assert cache is populated
	listing, exists := mgr.FindListingByAssetID("10001")
	assert.True(t, exists)
	assert.Equal(t, rest.Int64String(101), listing.ID)

	// Assert slug is cached
	assert.Equal(t, "epic-store", mgr.customStoreSlug)
	assert.False(t, mgr.groupCheckFailed)

	// Verify storefront URL generation
	url := mgr.GetStorefrontURL(context.Background())
	assert.Equal(t, "https://crit.tf/group/epic-store", url)
}

func TestManager_Bootstrap_Group404(t *testing.T) {
	httpClient := &mockHTTPClient{
		mockResponse: func(req *http.Request) (*http.Response, error) {
			if strings.HasSuffix(req.URL.Path, "/listings/my") {
				return newMockResponse(200, ListingsResponse{
					Response: Response{Success: true},
					Listings: []Listing{},
				})
			}

			if strings.HasSuffix(req.URL.Path, "/groups/my") {
				// Simulate 404 group missing
				return &http.Response{
					StatusCode: 404,
					Status:     "404 Not Found",
					Body:       io.NopCloser(bytes.NewBufferString(`{"success":false,"message":"Not Found"}`)),
				}, nil
			}

			return newMockResponse(500, nil)
		},
	}

	client := NewClient(httpClient, "mock-api-key")
	mgr := NewManager(client)
	mgr.steamID = id.ID(76561198000000001)

	ictx := module.NewInitContext()
	err := mgr.Init(ictx)
	require.NoError(t, err)

	mgr.bootstrap(context.Background())

	// Assert slug is empty and 404 check flag is set
	assert.Empty(t, mgr.customStoreSlug)
	assert.True(t, mgr.groupCheckFailed)

	// Assert storefront URL fell back to profile link
	url := mgr.GetStorefrontURL(context.Background())
	assert.Equal(t, "https://crit.tf/profile/76561198000000001", url)
}

func TestManager_QueueAndRateLimiter(t *testing.T) {
	httpClient := &mockHTTPClient{
		mockResponse: func(req *http.Request) (*http.Response, error) {
			if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/listings") {
				var body map[string]any

				_ = json.NewDecoder(req.Body).Decode(&body)
				assetID := body["asset_id"].(string)

				return newMockResponse(200, ListingsResponse{
					Response: Response{Success: true},
					Listing: &Listing{
						ID:      202,
						AssetID: assetID,
						SKU:     "5021;6",
					},
				})
			}

			return newMockResponse(500, nil)
		},
	}

	client := NewClient(httpClient, "mock-api-key")
	mgr := NewManager(client)

	ictx := module.NewInitContext()
	err := mgr.Init(ictx)
	require.NoError(t, err)

	ctx := t.Context()

	err = mgr.Start(ctx)
	require.NoError(t, err)

	// Enqueue a listing creation and wait for result
	ch := mgr.EnqueueCreateListing(ctx, "99999", pricedb.Currencies{Keys: 1, Metal: 10.0})
	select {
	case res := <-ch:
		require.NoError(t, res.Err)
		assert.Equal(t, "99999", res.Listing.AssetID)
		assert.Equal(t, rest.Int64String(202), res.Listing.ID)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for transaction worker")
	}

	// Verify item was cached
	listing, exists := mgr.FindListingByAssetID("99999")
	assert.True(t, exists)
	assert.Equal(t, rest.Int64String(202), listing.ID)
}

func TestManager_ItemNotFoundSelfHealing(t *testing.T) {
	var mu sync.Mutex

	refreshCount := 0
	createAttempts := 0

	httpClient := &mockHTTPClient{
		mockResponse: func(req *http.Request) (*http.Response, error) {
			mu.Lock()
			defer mu.Unlock()

			if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/inventory/refresh") {
				refreshCount++

				return newMockResponse(200, InventoryResponse{
					Response:  Response{Success: true},
					ItemCount: 15,
				})
			}

			if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/listings") {
				createAttempts++
				if createAttempts == 1 {
					// First attempt: return HTTP 400 Item Not Found
					return &http.Response{
						StatusCode: 400,
						Status:     "400 Bad Request",
						Body:       io.NopCloser(bytes.NewBufferString(`{"success":false,"message":"item_not_found"}`)),
					}, nil
				}

				// Second attempt: succeed
				return newMockResponse(200, ListingsResponse{
					Response: Response{Success: true},
					Listing: &Listing{
						ID:      777,
						AssetID: "88888",
					},
				})
			}

			return newMockResponse(500, nil)
		},
	}

	client := NewClient(httpClient, "mock-api-key")
	mgr := NewManager(client)

	ictx := module.NewInitContext()
	err := mgr.Init(ictx)
	require.NoError(t, err)

	ctx := t.Context()

	err = mgr.Start(ctx)
	require.NoError(t, err)

	ch := mgr.EnqueueCreateListing(ctx, "88888", pricedb.Currencies{Metal: 15.0})
	select {
	case res := <-ch:
		require.NoError(t, res.Err)
		assert.Equal(t, "88888", res.Listing.AssetID)
		assert.Equal(t, rest.Int64String(777), res.Listing.ID)
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for recovery self-healing")
	}

	mu.Lock()
	assert.Equal(t, 1, refreshCount, "Inventory refresh should have been called exactly once for self-healing")
	assert.Equal(t, 2, createAttempts, "CreateListing should have been attempted twice")
	mu.Unlock()
}

func TestManager_EventLoop(t *testing.T) {
	httpClient := &mockHTTPClient{
		mockResponse: func(req *http.Request) (*http.Response, error) {
			if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "/listings") {
				return newMockResponse(200, ListingsResponse{
					Response: Response{Success: true},
					Listing: &Listing{
						ID:      99,
						AssetID: "12345",
						SKU:     "5021;6",
					},
				})
			}

			return newMockResponse(500, nil)
		},
	}

	client := NewClient(httpClient, "mock-api-key")
	mgr := NewManager(client)

	ictx := module.NewInitContext()
	eb := ictx.Bus()

	err := mgr.Init(ictx)
	require.NoError(t, err)

	// Setup providers and configs
	bp := &mockBackpackProvider{items: []uint64{12345}}
	price := &mockPriceProvider{sellKey: 1, sellMetal: 10.0}
	cfg := &mockConfigProvider{items: map[string]trading.ItemConfig{
		"5021;6": {
			SKU:        "5021;6",
			EnableSell: true,
			MinStock:   0,
		},
	}}

	mgr.SetProviders(bp, price, cfg)

	err = mgr.Start(t.Context())
	require.NoError(t, err)

	// Wait for event loop subscription to be registered
	time.Sleep(100 * time.Millisecond)

	// Publish ItemAcquiredEvent
	eb.Publish(&tf2.ItemAcquiredEvent{
		Item: &tf2.Item{
			ID:       12345,
			DefIndex: 5021,
			Quality:  6,
			SKU:      "5021;6",
		},
	})

	// Wait for worker to create and cache listing
	time.Sleep(200 * time.Millisecond)

	listing, exists := mgr.FindListingByAssetID("12345")
	assert.True(t, exists, "Acquired item should have been processed and cached by event loop")
	assert.Equal(t, rest.Int64String(99), listing.ID)
}

func TestManager_ChatCommands(t *testing.T) {
	httpClient := &mockHTTPClient{
		mockResponse: func(req *http.Request) (*http.Response, error) {
			if strings.HasSuffix(req.URL.Path, "/groups/my") {
				return newMockResponse(200, GroupResponse{
					Response: Response{Success: true},
					Group: &Group{
						ID:              123,
						GroupName:       "Trading Hub",
						CustomStoreSlug: "hub",
						OwnerName:       "G-man Owner",
						OwnerSteamID:    "76561198000000001",
						ViewCount:       1500,
						Description:     "Official Trading Bot Group",
						Members: []GroupMember{
							{
								DisplayName:  "Trader 1",
								SteamID:      "76561198000000002",
								Role:         "member",
								InviteStatus: "accepted",
							},
							{
								DisplayName:  "Trader 2",
								SteamID:      "76561198000000003",
								Role:         "member",
								InviteStatus: "pending",
							},
						},
					},
				})
			}

			if strings.HasSuffix(req.URL.Path, "/groups/invites") {
				return newMockResponse(200, InvitesResponse{
					Response: Response{Success: true},
					Count:    1,
					Invites: []Invite{
						{StoreGroupID: 456, GroupName: "Other Hub", InviterName: "Friend"},
					},
				})
			}

			return newMockResponse(200, Response{Success: true})
		},
	}

	client := NewClient(httpClient, "mock-api-key")
	mgr := NewManager(client)

	ctx := context.Background()

	// 1. Test crit_group command output formatting
	groupInfo, err := mgr.handleGroupInfoCommand(ctx, 123, nil)
	require.NoError(t, err)
	assert.Contains(t, groupInfo, "Store Group: Trading Hub")
	assert.Contains(t, groupInfo, "Slug: hub")
	assert.Contains(t, groupInfo, "Trader 1")
	assert.Contains(t, groupInfo, "Trader 2")
	assert.Contains(t, groupInfo, "Members (Accepted)")
	assert.Contains(t, groupInfo, "Members (Pending)")

	// 2. Test crit_accept listing invites command
	acceptList, err := mgr.handleAcceptInviteCommand(ctx, 123, nil)
	require.NoError(t, err)
	assert.Contains(t, acceptList, "Other Hub")
	assert.Contains(t, acceptList, "Friend")

	// 3. Test leaving group command
	gID := 123
	leaveMsg, err := mgr.handleLeaveGroupCommand(ctx, 123, &gID)
	require.NoError(t, err)
	assert.Contains(t, leaveMsg, "Successfully left storefront group ID 123")

	// 4. Test crit_url public command
	urlMsg, err := mgr.handleURLCommand(ctx, 123, nil)
	require.NoError(t, err)
	assert.Contains(t, urlMsg, "Storefront:")
}

func TestManager_CommandAuthorization(t *testing.T) {
	client := NewClient(nil, "mock-api-key")
	mgr := NewManager(client)

	ictx := module.NewInitContext()
	chatClient := chat.New()
	cmdDispatcher := commands.NewManager()

	ictx.SetModule("chat", chatClient)
	ictx.SetModule("chat_commands", cmdDispatcher)

	err := chatClient.Init(ictx)
	require.NoError(t, err)

	err = cmdDispatcher.Init(ictx)
	require.NoError(t, err)

	err = mgr.Init(ictx)
	require.NoError(t, err)

	// Verify command registration metadata on dispatcher
	urlCmd, exists := cmdDispatcher.GetCommand("crit_url")
	assert.True(t, exists)
	assert.False(t, urlCmd.IsAdmin, "crit_url should be a public user command")

	groupCmd, exists := cmdDispatcher.GetCommand("crit_group")
	assert.True(t, exists)
	assert.True(t, groupCmd.IsAdmin, "crit_group should be an admin command")

	inviteCmd, exists := cmdDispatcher.GetCommand("crit_invite")
	assert.True(t, exists)
	assert.True(t, inviteCmd.IsAdmin, "crit_invite should be an admin command")
}
