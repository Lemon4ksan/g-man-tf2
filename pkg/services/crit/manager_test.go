// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat/commands"
	"github.com/lemon4ksan/g-man/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

type mockBackpackProvider struct {
	items []uint64
}

func (m *mockBackpackProvider) GetTotalCount() int {
	return len(m.items)
}

func (m *mockBackpackProvider) GetStock(sku string) int {
	return len(m.items)
}

func (m *mockBackpackProvider) GetItemsBySKU(targetSKU string) []uint64 {
	return m.items
}

func (m *mockBackpackProvider) GetItem(id uint64) (*tf2.Item, bool) {
	return &tf2.Item{ID: id, SKU: "5021;6"}, true
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

func (m *mockPriceProvider) Watch(sku string) {}

func (m *mockPriceProvider) Fetch(ctx context.Context, skus []string) (map[string]*pricedb.Price, error) {
	return map[string]*pricedb.Price{
		skus[0]: {
			SKU: skus[0],
			Sell: pricedb.Currencies{
				Keys:  m.sellKey,
				Metal: m.sellMetal,
			},
		},
	}, nil
}

type mockConfigProviderWithVal struct {
	cfg trading.Config
}

func (m *mockConfigProviderWithVal) GetConfig() trading.Config {
	return m.cfg
}

type statefulDoer struct {
	mu             sync.Mutex
	refreshCount   int
	createAttempts int
}

func (d *statefulDoer) Do(req *http.Request) (*http.Response, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "inventory/refresh") {
		d.refreshCount++
		resp := InventoryResponse{
			Response:  Response{Success: true},
			ItemCount: 15,
		}
		b, _ := json.Marshal(resp)

		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(b)),
		}, nil
	}

	if req.Method == http.MethodPost && strings.HasSuffix(req.URL.Path, "listings") {
		d.createAttempts++
		if d.createAttempts == 1 {
			resp := Response{Success: false, Message: "item_not_found"}
			b, _ := json.Marshal(resp)

			return &http.Response{
				StatusCode: 400,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}

		resp := ListingsResponse{
			Response: Response{Success: true},
			Listing: &Listing{
				ID:      777,
				AssetID: "88888",
			},
		}
		b, _ := json.Marshal(resp)

		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(b)),
		}, nil
	}

	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(bytes.NewReader([]byte("{}"))),
	}, nil
}

func TestManager(t *testing.T) {
	t.Parallel()

	t.Run("init_and_lifecycle", func(t *testing.T) {
		t.Parallel()

		stub := mock.NewHTTPStub()
		client := NewClient(aoni.NewClient(stub), "mock-api-key")
		mgr := NewManager(client)

		ictx := mock.NewInitContext()
		err := mgr.Init(ictx)
		require.NoError(t, err)

		err = mgr.Start(t.Context())
		require.NoError(t, err)

		err = mgr.Close()
		require.NoError(t, err)
	})

	t.Run("bootstrap_and_storefront_url", func(t *testing.T) {
		t.Parallel()

		stub := mock.NewHTTPStub()

		stub.SetJSONResponse("api/v2/listings/my", 200, ListingsResponse{
			Response: Response{Success: true},
			Listings: []Listing{
				{ID: 101, AssetID: "10001", SKU: "5021;6", PriceKeys: 1, PriceMetal: 15.0},
			},
		})
		stub.SetJSONResponse("api/v2/groups/my", 404, nil)

		client := NewClient(aoni.NewClient(stub), "mock-api-key")
		mgr := NewManager(client)
		mgr.steamID = id.ID(76561198033830321)

		mgr.bootstrap(t.Context())

		assert.True(t, mgr.groupCheckFailed)
		assert.Empty(t, mgr.customStoreSlug)

		urlStr := mgr.GetStorefrontURL(t.Context())
		assert.Equal(t, "https://crit.tf/profile/76561198033830321", urlStr)

		mgr.steamID = id.ID(0)
		urlNoSteam := mgr.GetStorefrontURL(t.Context())
		assert.Equal(t, "https://crit.tf", urlNoSteam)

		mgr.customStoreSlug = "cool-store"
		urlSlug := mgr.GetStorefrontURL(t.Context())
		assert.Equal(t, "https://crit.tf/group/cool-store", urlSlug)
	})

	t.Run("refresh_inventory_cooldown", func(t *testing.T) {
		t.Parallel()

		mgr := NewManager(nil)
		mgr.lastInvRefresh = time.Now()

		_, err := mgr.RefreshInventory(t.Context())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cooldown")
	})

	t.Run("queue_and_rate_limiter", func(t *testing.T) {
		t.Parallel()

		stub := mock.NewHTTPStub()

		stub.SetJSONResponse("api/v2/listings", 200, ListingsResponse{
			Response: Response{Success: true},
			Listing: &Listing{
				ID:      202,
				AssetID: "99999",
				SKU:     "5021;6",
			},
		})

		client := NewClient(aoni.NewClient(stub), "mock-api-key")
		mgr := NewManager(client)

		ictx := mock.NewInitContext()
		err := mgr.Init(ictx)
		require.NoError(t, err)

		ctx := t.Context()

		err = mgr.Start(ctx)
		require.NoError(t, err)

		ch := mgr.EnqueueCreateListing(ctx, "99999", pricedb.Currencies{Keys: 1, Metal: 10.0})
		select {
		case res := <-ch:
			require.NoError(t, res.Err)
			assert.Equal(t, "99999", res.Listing.AssetID)
			assert.Equal(t, aoni.Int64String(202), res.Listing.ID)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for transaction worker")
		}
	})

	t.Run("item_not_found_self_healing_without_server", func(t *testing.T) {
		t.Parallel()

		stateful := &statefulDoer{}
		client := NewClient(aoni.NewClient(stateful), "mock-api-key")
		mgr := NewManager(client)

		ictx := mock.NewInitContext()
		err := mgr.Init(ictx)
		require.NoError(t, err)

		ctx := t.Context()
		err = mgr.Start(ctx)
		require.NoError(t, err)

		ch := mgr.EnqueueCreateListing(ctx, "88888", pricedb.Currencies{})
		select {
		case res := <-ch:
			require.NoError(t, res.Err)
			assert.Equal(t, "88888", res.Listing.AssetID)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for self-healing logic")
		}

		stateful.mu.Lock()
		assert.Equal(t, 1, stateful.refreshCount)
		assert.Equal(t, 2, stateful.createAttempts)
		stateful.mu.Unlock()
	})

	t.Run("chat_commands_and_authorization", func(t *testing.T) {
		t.Parallel()

		stub := mock.NewHTTPStub()

		stub.SetJSONResponse("api/v2/groups/my", 200, GroupResponse{
			Response: Response{Success: true},
			Group: &Group{
				ID:              123,
				GroupName:       "Trading Hub",
				CustomStoreSlug: "hub",
				Members: []GroupMember{
					{DisplayName: "", SteamID: "76561198000000002", Role: "member", InviteStatus: "accepted"},
				},
			},
		})
		stub.SetJSONResponse("api/v2/groups/invites", 200, InvitesResponse{
			Response: Response{Success: true},
			Invites:  []Invite{{StoreGroupID: 456, GroupName: "Other Hub"}},
		})
		stub.SetJSONResponse("api/v2/groups/456/accept", 200, Response{Success: true})
		stub.SetJSONResponse("api/v2/groups/123/leave", 200, Response{Success: true})

		client := NewClient(aoni.NewClient(stub), "mock-api-key")
		mgr := NewManager(client)

		ictx := mock.NewInitContext()
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

		ctx := t.Context()

		info, err := mgr.handleGroupInfoCommand(ctx, 123, nil)
		require.NoError(t, err)
		assert.Contains(t, info, "76561198000000002")

		gID := 456
		acceptMsg, err := mgr.handleAcceptInviteCommand(ctx, 123, &gID)
		require.NoError(t, err)
		assert.Contains(t, acceptMsg, "Successfully accepted storefront group")

		leaveMsg, err := mgr.handleLeaveGroupCommand(ctx, 123, nil)
		require.NoError(t, err)
		assert.Contains(t, leaveMsg, "Successfully left storefront group ID 123")
	})

	t.Run("init_without_chat_commands", func(t *testing.T) {
		t.Parallel()

		mgr := NewManager(nil)
		ictx := mock.NewInitContext()

		err := mgr.Init(ictx)
		assert.NoError(t, err)
		assert.Nil(t, mgr.commands)
	})

	t.Run("bootstrap_group_check_fails", func(t *testing.T) {
		t.Parallel()

		communityMock := mock.NewHTTPStub()
		communityMock.MockSessionID = "mock_session"
		communityMock.SetJSONResponse("inventory/my", 200, ListingsResponse{})

		client := NewClient(aoni.NewClient(communityMock), "api-key")
		communityMock.ResponseErrs["groups/my"] = errors.New("internal server error")

		mgr := NewManager(client)
		mgr.listings = make(map[string]Listing)
		mgr.mu.Lock()
		mgr.steamID = id.ID(123)
		mgr.mu.Unlock()

		mgr.bootstrap(t.Context())

		mgr.mu.RLock()
		assert.True(t, mgr.groupCheckFailed)
		assert.Empty(t, mgr.customStoreSlug)
		mgr.mu.RUnlock()
	})

	t.Run("storefront_url_cooldown_expiry_refreshes", func(t *testing.T) {
		t.Parallel()

		stub := mock.NewHTTPStub()
		stub.SetJSONResponse("api/v2/groups/my", 200, GroupResponse{
			Response: Response{Success: true},
			Group:    &Group{CustomStoreSlug: "new-slug"},
		})
		client := NewClient(aoni.NewClient(stub), "mock-api-key")
		mgr := NewManager(client)

		mgr.steamID = id.New(76561198033830321)
		mgr.groupCheckFailed = true
		mgr.lastGroupCheck = time.Now().Add(-10 * time.Minute)

		urlStr := mgr.GetStorefrontURL(t.Context())
		assert.Equal(t, "https://crit.tf/profile/76561198033830321", urlStr)

		assert.Eventually(t, func() bool {
			mgr.mu.RLock()
			defer mgr.mu.RUnlock()
			return !mgr.groupCheckFailed && mgr.customStoreSlug == "new-slug"
		}, 1*time.Second, 10*time.Millisecond)
	})

	t.Run("refresh_inventory_errors", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)
		mgr := NewManager(client)

		mgr.lastInvRefresh = time.Time{}

		mgr.rateLimiter = rate.NewLimiter(0, 0)
		_, err := mgr.RefreshInventory(t.Context())
		assert.Error(t, err)

		mgr.rateLimiter = rate.NewLimiter(10, 10)
		stub.ResponseErrs["api/v2/inventory/refresh"] = errors.New("network error")
		_, err = mgr.RefreshInventory(t.Context())
		assert.Error(t, err)
	})

	t.Run("fetch_my_listings_errors", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)
		mgr := NewManager(client)

		mgr.rateLimiter = rate.NewLimiter(0, 0)
		_, err := mgr.FetchMyListings(t.Context())
		assert.Error(t, err)

		mgr.rateLimiter = rate.NewLimiter(10, 10)
		stub.ResponseErrs["api/v2/listings/my"] = errors.New("network error")
		_, err = mgr.FetchMyListings(t.Context())
		assert.Error(t, err)
	})

	t.Run("transaction_canceled_context", func(t *testing.T) {
		t.Parallel()
		client, _ := setupTestClient(t)
		mgr := NewManager(client)

		ictx := mock.NewInitContext()
		err := mgr.Init(ictx)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err = mgr.Start(ctx)
		require.NoError(t, err)

		_, err = mgr.CreateListing(ctx, "123", pricedb.Currencies{})
		assert.ErrorIs(t, err, context.Canceled)

		_, err = mgr.UpdateListing(ctx, "listing-123", pricedb.Currencies{})
		assert.ErrorIs(t, err, context.Canceled)

		err = mgr.DeleteListing(ctx, "listing-123")
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("handle_accept_invite_errors", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)
		mgr := NewManager(client)

		ctx := t.Context()

		stub.ResponseErrs["api/v2/groups/invites"] = errors.New("api error")
		_, err := mgr.handleAcceptInviteCommand(ctx, 123, nil)
		assert.Error(t, err)

		stub.ResponseErrs["api/v2/groups/invites"] = nil
		stub.SetJSONResponse("api/v2/groups/invites", 200, InvitesResponse{
			Response: Response{Success: true},
			Invites:  nil,
		})

		msg, err := mgr.handleAcceptInviteCommand(ctx, 123, nil)
		require.NoError(t, err)
		assert.Contains(t, msg, "No pending storefront invitations")

		stub.ResponseErrs["api/v2/groups/789/accept"] = errors.New("api error")
		gID := 789
		_, err = mgr.handleAcceptInviteCommand(ctx, 123, &gID)
		assert.Error(t, err)
	})

	t.Run("handle_leave_group_errors", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)
		mgr := NewManager(client)

		ctx := t.Context()

		stub.ResponseErrs["api/v2/groups/my"] = errors.New("api error")
		msg, err := mgr.handleLeaveGroupCommand(ctx, 123, nil)
		require.NoError(t, err)
		assert.Contains(t, msg, "Usage: !crit_leave")

		stub.ResponseErrs["api/v2/groups/101/leave"] = errors.New("api error")
		gID := 101
		_, err = mgr.handleLeaveGroupCommand(ctx, 123, &gID)
		assert.Error(t, err)
	})

	t.Run("handle_group_info_errors_and_formatting", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)
		mgr := NewManager(client)

		ctx := t.Context()

		stub.ResponseErrs["api/v2/groups/my"] = errors.New("api error")
		_, err := mgr.handleGroupInfoCommand(ctx, 123, nil)
		assert.Error(t, err)

		stub.ResponseErrs["api/v2/groups/my"] = nil
		stub.SetJSONResponse("api/v2/listings/my", 200, map[string]any{"success": true, "listings": []any{}})
		stub.SetJSONResponse("api/v2/groups/my", 200, GroupResponse{
			Response: Response{Success: true},
			Group: &Group{
				ID:        123,
				GroupName: "Trading Hub",
				Members: []GroupMember{
					{
						DisplayName:  "Accepted Member",
						SteamID:      "76561198000000002",
						Role:         "member",
						InviteStatus: "accepted",
					},
					{
						DisplayName:  "Pending Member",
						SteamID:      "76561198000000003",
						Role:         "member",
						InviteStatus: "pending",
					},
				},
			},
		})

		msg, err := mgr.handleGroupInfoCommand(ctx, 123, nil)
		require.NoError(t, err)
		assert.Contains(t, msg, "Accepted Member")
		assert.Contains(t, msg, "Pending Member")
	})

	t.Run("with_module_and_from", func(t *testing.T) {
		t.Parallel()
		client, _ := setupTestClient(t)
		opt := WithModule(client)
		assert.NotNil(t, opt)

		steamClient, err := steam.NewClient(steam.Config{})
		if err == nil && steamClient != nil {
			opt(steamClient)
			retrieved := From(steamClient)
			assert.NotNil(t, retrieved)
		}
	})

	t.Run("set_providers_with_command_descriptions", func(t *testing.T) {
		t.Parallel()

		mgr := NewManager(nil)

		bp := &mockBackpackProvider{}
		pm := &mockPriceProvider{}

		cmdDispatcher := commands.NewManager()
		cmdDispatcher.Register("crit_url", func(ctx context.Context, senderID uint64, args []string) (string, error) {
			return "", nil
		})

		mgr.commands = cmdDispatcher

		cfgVal := trading.Config{
			CritCommandDescriptions: map[string]string{
				"crit_url": "custom description",
			},
		}

		cfg := &mockConfigProviderWithVal{
			cfg: cfgVal,
		}

		mgr.SetProviders(bp, pm, cfg)

		assert.Equal(t, bp, mgr.bp)
		assert.Equal(t, pm, mgr.priceMgr)
		assert.Equal(t, cfg, mgr.cfgMgr)
	})

	t.Run("start_authed", func(t *testing.T) {
		t.Parallel()

		stub := mock.NewHTTPStub()

		stub.SetJSONResponse("api/v2/listings/my", 200, ListingsResponse{})
		stub.SetJSONResponse("api/v2/groups/my", 200, GroupResponse{
			Response: Response{Success: true},
			Group:    &Group{CustomStoreSlug: "my-store"},
		})

		client := NewClient(aoni.NewClient(stub), "mock-api-key")
		mgr := NewManager(client)

		ictx := mock.NewInitContext()
		err := mgr.Init(ictx)
		require.NoError(t, err)

		ctx := t.Context()
		err = mgr.Start(ctx)
		require.NoError(t, err)

		authCtx := mock.NewAuthContext(id.ID(76561198033830321))

		err = mgr.StartAuthed(ctx, authCtx)
		require.NoError(t, err)

		assert.Eventually(t, func() bool {
			mgr.mu.RLock()
			defer mgr.mu.RUnlock()
			return mgr.customStoreSlug == "my-store"
		}, 2*time.Second, 10*time.Millisecond)
	})

	t.Run("listings_cache_operations", func(t *testing.T) {
		t.Parallel()

		mgr := NewManager(nil)

		mgr.listingsMu.Lock()
		mgr.listings["asset-1"] = Listing{ID: 123, AssetID: "asset-1"}
		mgr.listings["asset-2"] = Listing{ID: 456, AssetID: "asset-2"}
		mgr.listingsMu.Unlock()

		l, ok := mgr.FindListingByAssetID("asset-1")
		assert.True(t, ok)
		assert.Equal(t, aoni.Int64String(123), l.ID)

		_, ok = mgr.FindListingByAssetID("asset-unknown")
		assert.False(t, ok)

		cached := mgr.FetchCachedListings()
		assert.Len(t, cached, 2)
	})

	t.Run("worker_self_healing_queue_full", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)
		mgr := NewManager(client)

		// Set capacity to 0 so the worker's attempt to re-enqueue always hits default/fails
		mgr.txQueue = make(chan Transaction)

		ictx := mock.NewInitContext()
		err := mgr.Init(ictx)
		require.NoError(t, err)

		ctx := t.Context()
		err = mgr.Start(ctx)
		require.NoError(t, err)

		stub.SetJSONResponse("api/v2/listings", 400, Response{Success: false, Message: "item_not_found"})
		stub.SetJSONResponse("api/v2/inventory/refresh", 200, InventoryResponse{Response: Response{Success: true}})

		// Enqueue listing creation task. This will write to txQueue (blocks until worker reads it)
		ch := mgr.EnqueueCreateListing(ctx, "999", pricedb.Currencies{})
		select {
		case res := <-ch:
			assert.Error(t, res.Err)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for queue full branch")
		}
	})

	t.Run("event_loop_all_cases", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)
		mgr := NewManager(client)

		ictx := mock.NewInitContext()
		err := mgr.Init(ictx)
		require.NoError(t, err)

		ctx := t.Context()

		stub.SetJSONResponse("api/v2/listings", 200, ListingsResponse{
			Response: Response{Success: true},
			Listing:  &Listing{ID: 111, AssetID: "999"},
		})
		stub.SetJSONResponse("api/v2/listings/111", 200, Response{Success: true})
		stub.SetJSONResponse("api/v2/listings/555", 200, ListingsResponse{
			Response: Response{Success: true},
			Listing: &Listing{
				ID:         555,
				AssetID:    "777",
				SKU:        "5021;6",
				PriceKeys:  1,
				PriceMetal: 15.0,
			},
		})

		bp := &mockBackpackProvider{
			items: []uint64{888, 999},
		}
		pm := &mockPriceProvider{
			sellKey:   1,
			sellMetal: 15.5,
		}
		cfgVal := trading.Config{
			Items: map[string]trading.ItemConfig{
				"5021;6": {
					SKU:        "5021;6",
					EnableSell: true,
					MinStock:   1,
				},
				"5002;6": {
					SKU:        "5002;6",
					EnableSell: false,
				},
			},
		}
		cfg := &mockConfigProviderWithVal{cfg: cfgVal}

		mgr.SetProviders(bp, pm, cfg)

		err = mgr.Start(ctx)
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		mgr.Bus.Publish(&tf2.ItemAcquiredEvent{Item: nil})

		mgr.Bus.Publish(&tf2.ItemAcquiredEvent{
			Item: &tf2.Item{
				ID:       999,
				DefIndex: 5021,
				Quality:  6,
				SKU:      "",
			},
		})

		assert.Eventually(t, func() bool {
			l, ok := mgr.FindListingByAssetID("999")
			return ok && l.ID == 111
		}, 2*time.Second, 10*time.Millisecond)

		mgr.Bus.Publish(&tf2.ItemAcquiredEvent{
			Item: &tf2.Item{
				ID:       9999,
				DefIndex: 5002,
				Quality:  6,
				SKU:      "5002;6",
			},
		})

		time.Sleep(100 * time.Millisecond)

		_, ok := mgr.FindListingByAssetID("9999")
		assert.False(t, ok)

		mgr.Bus.Publish(&tf2.ItemRemovedEvent{
			ItemID: 999,
		})

		assert.Eventually(t, func() bool {
			_, ok := mgr.FindListingByAssetID("999")
			return !ok
		}, 2*time.Second, 10*time.Millisecond)

		mgr.Bus.Publish(&tf2.ItemRemovedEvent{
			ItemID: 999999,
		})

		mgr.listingsMu.Lock()
		mgr.listings["777"] = Listing{
			ID:         555,
			AssetID:    "777",
			SKU:        "5021;6",
			PriceKeys:  1,
			PriceMetal: 10.0,
		}
		mgr.listingsMu.Unlock()

		mgr.Bus.Publish(&pricedb.PricelistUpdatedEvent{
			SKU: "5021;6",
			Sell: pricedb.Currencies{
				Keys:  1,
				Metal: 15.0,
			},
		})

		assert.Eventually(t, func() bool {
			l, ok := mgr.FindListingByAssetID("777")
			return ok && float64(l.PriceMetal) == 15.0
		}, 2*time.Second, 10*time.Millisecond)

		mgr.listingsMu.Lock()
		mgr.listings["777"] = Listing{
			ID:         555,
			AssetID:    "777",
			SKU:        "5021;6",
			PriceKeys:  1,
			PriceMetal: 15.0,
		}
		mgr.listingsMu.Unlock()

		mgr.Bus.Publish(&pricedb.PricelistUpdatedEvent{
			SKU: "5021;6",
			Sell: pricedb.Currencies{
				Keys:  1,
				Metal: 15.0,
			},
		})

		mgr.listingsMu.Lock()
		mgr.pendingUpdates["777"] = pricedb.Currencies{
			Keys:  1,
			Metal: 20.0,
		}
		mgr.listingsMu.Unlock()

		mgr.Bus.Publish(&pricedb.PricelistUpdatedEvent{
			SKU: "5021;6",
			Sell: pricedb.Currencies{
				Keys:  1,
				Metal: 20.0,
			},
		})
	})

	t.Run("fetch_my_listings_cached_and_fresh", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)
		mgr := NewManager(client)

		ctx := t.Context()

		stub.SetJSONResponse("api/v2/listings/my", 200, ListingsResponse{
			Response: Response{Success: true},
			Listings: []Listing{
				{ID: 111, AssetID: "999"},
			},
		})

		listings, err := mgr.FetchMyListings(ctx)
		require.NoError(t, err)
		assert.Len(t, listings, 1)
		assert.Equal(t, aoni.Int64String(111), listings[0].ID)

		stub.ClearCalls()

		cachedListings, err := mgr.FetchMyListings(ctx)
		require.NoError(t, err)
		assert.Len(t, cachedListings, 1)
		assert.Equal(t, 0, stub.CallsCount())
	})

	t.Run("manager_api_done_contexts", func(t *testing.T) {
		t.Parallel()
		client, _ := setupTestClient(t)
		mgr := NewManager(client)

		ictx := mock.NewInitContext()
		err := mgr.Init(ictx)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		_, err = mgr.CreateListing(ctx, "asset-123", pricedb.Currencies{})
		assert.ErrorIs(t, err, context.Canceled)

		_, err = mgr.UpdateListing(ctx, "listing-123", pricedb.Currencies{})
		assert.ErrorIs(t, err, context.Canceled)

		err = mgr.DeleteListing(ctx, "listing-123")
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("handle_invite_command_all_paths", func(t *testing.T) {
		t.Parallel()
		client, stub := setupTestClient(t)
		mgr := NewManager(client)

		ctx := t.Context()

		stub.ResponseErrs["api/v2/groups/my"] = errors.New("api error")
		_, err := mgr.handleInviteCommand(ctx, 123, id.ID(76561198000000002))
		assert.Error(t, err)

		stub.ResponseErrs["api/v2/groups/my"] = nil
		stub.SetJSONResponse("api/v2/groups/my", 200, GroupResponse{
			Response: Response{Success: true},
			Group: &Group{
				ID:        123,
				GroupName: "Trading Hub",
			},
		})
		stub.ResponseErrs["api/v2/groups/123/invite"] = errors.New("invite failed")
		_, err = mgr.handleInviteCommand(ctx, 123, id.ID(76561198000000002))
		assert.Error(t, err)

		stub.ResponseErrs["api/v2/groups/123/invite"] = nil
		stub.SetJSONResponse("api/v2/groups/123/invite", 200, Response{Success: true})

		msg, err := mgr.handleInviteCommand(ctx, 123, id.ID(76561198000000002))
		require.NoError(t, err)
		assert.Contains(t, msg, "Successfully invited")
	})

	t.Run("handle_url_command", func(t *testing.T) {
		t.Parallel()

		mgr := NewManager(nil)
		mgr.customStoreSlug = "cool-store"

		msg, err := mgr.handleURLCommand(t.Context(), 123, nil)
		require.NoError(t, err)
		assert.Equal(t, "Storefront: https://crit.tf/group/cool-store", msg)
	})

	t.Run("crit_response_unmarshal_and_error_cases", func(t *testing.T) {
		t.Parallel()

		var r critResponse

		err := json.Unmarshal([]byte(`{"success":false,"message":"some_api_error"}`), &r)
		require.NoError(t, err)
		assert.False(t, r.IsSuccess())
		assert.EqualError(t, r.Error(), "crit: api error: some_api_error")

		var target Listing
		r.SetData(&target)
		err = json.Unmarshal([]byte(`{"success":true,"id":999}`), &r)
		require.NoError(t, err)
		assert.Equal(t, aoni.Int64String(999), target.ID)
	})

	t.Run("event_loop_target_for_sale_false", func(t *testing.T) {
		t.Parallel()
		client, _ := setupTestClient(t)
		mgr := NewManager(client)

		ictx := mock.NewInitContext()
		err := mgr.Init(ictx)
		require.NoError(t, err)

		bp := &mockBackpackProvider{
			items: []uint64{9999},
		}
		pm := &mockPriceProvider{
			sellKey:   1,
			sellMetal: 15.5,
		}
		cfgVal := trading.Config{
			Items: map[string]trading.ItemConfig{
				"5021;6": {
					SKU:        "5021;6",
					EnableSell: true,
					MinStock:   1,
				},
			},
		}
		cfg := &mockConfigProviderWithVal{cfg: cfgVal}
		mgr.SetProviders(bp, pm, cfg)

		err = mgr.Start(t.Context())
		require.NoError(t, err)

		mgr.Bus.Publish(&tf2.ItemAcquiredEvent{
			Item: &tf2.Item{
				ID:       9999,
				DefIndex: 5021,
				Quality:  6,
				SKU:      "5021;6",
			},
		})

		time.Sleep(100 * time.Millisecond)

		_, ok := mgr.FindListingByAssetID("9999")
		assert.False(t, ok)
	})

	t.Run("event_loop_price_missing_or_negative", func(t *testing.T) {
		t.Parallel()
		client, _ := setupTestClient(t)
		mgr := NewManager(client)

		ictx := mock.NewInitContext()
		err := mgr.Init(ictx)
		require.NoError(t, err)

		bp := &mockBackpackProvider{
			items: []uint64{9999, 8888},
		}
		pm := &mockPriceProvider{
			sellKey:   0,
			sellMetal: -1.0,
		}
		cfgVal := trading.Config{
			Items: map[string]trading.ItemConfig{
				"5021;6": {
					SKU:        "5021;6",
					EnableSell: true,
					MinStock:   0,
				},
			},
		}
		cfg := &mockConfigProviderWithVal{cfg: cfgVal}
		mgr.SetProviders(bp, pm, cfg)

		err = mgr.Start(t.Context())
		require.NoError(t, err)

		mgr.Bus.Publish(&tf2.ItemAcquiredEvent{
			Item: &tf2.Item{
				ID:       9999,
				DefIndex: 5021,
				Quality:  6,
				SKU:      "5021;6",
			},
		})

		time.Sleep(100 * time.Millisecond)

		_, ok := mgr.FindListingByAssetID("9999")
		assert.False(t, ok)
	})

	t.Run("event_loop_uninitialized_providers", func(t *testing.T) {
		t.Parallel()
		client, _ := setupTestClient(t)
		mgr := NewManager(client)

		ictx := mock.NewInitContext()
		err := mgr.Init(ictx)
		require.NoError(t, err)

		err = mgr.Start(t.Context())
		require.NoError(t, err)

		mgr.Bus.Publish(&tf2.ItemAcquiredEvent{
			Item: &tf2.Item{
				ID:       9999,
				DefIndex: 5021,
				Quality:  6,
				SKU:      "5021;6",
			},
		})

		time.Sleep(100 * time.Millisecond)

		_, ok := mgr.FindListingByAssetID("9999")
		assert.False(t, ok)
	})
}
