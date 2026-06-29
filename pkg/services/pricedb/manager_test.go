// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/test/mock"
	"github.com/lemon4ksan/miyako/bus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPriceManager_WatchAndGet(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))
	client := NewClient(nil)

	manager := NewManager(client, logger)
	assert.Equal(t, BehaviorName, manager.Name())

	assert.Len(t, manager.GetWatchedSKUs(), 0)

	manager.Watch("5021;6")
	assert.ElementsMatch(t, []string{"5021;6"}, manager.GetWatchedSKUs())

	manager.Watch("5021;6")
	assert.ElementsMatch(t, []string{"5021;6"}, manager.GetWatchedSKUs())

	manager.Watch("5002;6")
	assert.ElementsMatch(t, []string{"5021;6", "5002;6"}, manager.GetWatchedSKUs())

	manager.Unwatch("5021;6")
	assert.ElementsMatch(t, []string{"5002;6"}, manager.GetWatchedSKUs())

	_, ok := manager.GetPrice("5002;6")
	assert.False(t, ok)
}

func TestPriceManager_UpdatesAndFetch(t *testing.T) {
	t.Parallel()

	stub := mock.NewHTTPStub()

	allPrices := []*Price{
		{
			SKU:  "5021;6",
			Name: "Key",
			Buy:  Currencies{Metal: 75},
			Sell: Currencies{Metal: 75.11},
			Time: 1600000000,
		},
		{
			SKU:  "5002;6",
			Name: "Refined",
			Buy:  Currencies{Metal: 1},
			Sell: Currencies{Metal: 1},
			Time: 1600000000,
		},
	}
	stub.SetJSONResponse("api/items-bulk", 200, allPrices)

	logger := log.New(log.DefaultConfig(log.LevelError))
	client := NewClient(aoni.NewClient(stub))
	manager := NewManager(client, logger)

	t.Run("seed_empty", func(t *testing.T) {
		err := manager.SeedFromBackpack(t.Context(), nil)
		require.NoError(t, err)
	})

	t.Run("seed_from_backpack", func(t *testing.T) {
		err := manager.SeedFromBackpack(t.Context(), []string{"5021;6", "5002;6"})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"5021;6", "5002;6"}, manager.GetWatchedSKUs())

		p1, ok := manager.GetPrice("5021;6")
		assert.True(t, ok)
		assert.Equal(t, 75.0, p1.Buy.Metal)

		p2, ok := manager.GetPrice("5002;6")
		assert.True(t, ok)
		assert.Equal(t, 1.0, p2.Buy.Metal)
	})

	t.Run("update", func(t *testing.T) {
		err := manager.Update(t.Context())
		require.NoError(t, err)
	})

	t.Run("fetch", func(t *testing.T) {
		fetched, err := manager.Fetch(t.Context(), []string{"5021;6"})
		require.NoError(t, err)
		assert.Len(t, fetched, 1)
		assert.Equal(t, 75.0, fetched["5021;6"].Buy.Metal)
	})

	t.Run("fetch_empty", func(t *testing.T) {
		fetchedEmpty, err := manager.Fetch(t.Context(), nil)
		require.NoError(t, err)
		assert.Len(t, fetchedEmpty, 0)
	})

	t.Run("get_all_prices", func(t *testing.T) {
		all := manager.GetAllPrices()
		assert.Len(t, all, 2)
	})
}

func TestPriceManager_SocketUpdates(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))
	client := NewClient(nil)
	manager := NewManager(client, logger)

	priceUpdate := &Price{
		SKU:  "5021;6",
		Name: "Key",
		Buy:  Currencies{Metal: 80},
		Sell: Currencies{Metal: 80.11},
		Time: 1700000000,
	}

	manager.socket.onPrice(priceUpdate)

	p, ok := manager.GetPrice("5021;6")
	assert.True(t, ok)
	assert.Equal(t, 80.0, p.Buy.Metal)

	invalidPrice := &Price{
		SKU:  "5021;6",
		Buy:  Currencies{Metal: -5},
		Sell: Currencies{Metal: 80.11},
	}
	manager.socket.onPrice(invalidPrice)

	p, ok = manager.GetPrice("5021;6")
	assert.True(t, ok)
	assert.Equal(t, 80.0, p.Buy.Metal)
}

func TestPriceManager_OrchestratorOption(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))
	client := NewClient(nil)
	b := bus.New()
	orchestrator := behavior.NewOrchestrator(b, logger)

	assert.NotPanics(t, func() {
		WithPriceManager(orchestrator, client)
	})
}

func TestPriceManager_RealtimeWebsocketHandshake(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{}

	var mockPriceSent sync.WaitGroup
	mockPriceSent.Add(1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		err = conn.WriteMessage(websocket.TextMessage, []byte(`0{"sid":"123"}`))
		if err != nil {
			return
		}

		_, p, err := conn.ReadMessage()
		if err != nil || string(p) != "40" {
			return
		}

		priceUpdatePayload := `42["price",{"sku":"5021;6","name":"Key","buy":{"metal":82},"sell":{"metal":82.11},"time":1800000000}]`

		err = conn.WriteMessage(websocket.TextMessage, []byte(priceUpdatePayload))
		if err != nil {
			return
		}

		mockPriceSent.Done()
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)

	logger := log.New(log.DefaultConfig(log.LevelError))
	client := NewClient(nil)

	manager := &Manager{
		client:       client,
		logger:       logger.With(log.Module(BehaviorName)),
		cache:        make(map[string]*Price),
		watchedSKUs:  make(map[string]struct{}),
		syncInterval: 10 * time.Millisecond,
	}

	var priceUpdated sync.WaitGroup
	priceUpdated.Add(1)

	manager.socket = NewSocketManager(wsURL, client.rest, manager.logger)
	manager.socket.OnPrice(func(p *Price) {
		if p.SKU == "5021;6" && p.Buy.Metal == 82.0 {
			manager.mu.Lock()
			manager.cache[p.SKU] = p
			manager.mu.Unlock()
			priceUpdated.Done()
		}
	})

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		_ = manager.socket.Run(ctx)
	}()

	select {
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for websocket price update event")
	case <-func() chan struct{} {
		ch := make(chan struct{})
		go func() {
			mockPriceSent.Wait()
			priceUpdated.Wait()
			close(ch)
		}()

		return ch
	}():
	}

	p, ok := manager.GetPrice("5021;6")
	assert.True(t, ok)
	assert.Equal(t, 82.0, p.Buy.Metal)
}

func TestPriceManager_EventPublication(t *testing.T) {
	t.Parallel()

	logger := log.New(log.DefaultConfig(log.LevelError))
	eventBus := bus.New()
	manager := NewManager(nil, logger).WithBus(eventBus)

	sub := eventBus.Subscribe(&PricelistUpdatedEvent{})
	defer sub.Unsubscribe()

	manager.SetPrice("5021;6", Currencies{Metal: 50.0}, Currencies{Metal: 55.0}, "Manual")

	select {
	case ev := <-sub.C():
		updatedEv, ok := ev.(*PricelistUpdatedEvent)
		require.True(t, ok)
		assert.Equal(t, "5021;6", updatedEv.SKU)
		assert.Equal(t, 50.0, updatedEv.Buy.Metal)
		assert.Equal(t, 55.0, updatedEv.Sell.Metal)

	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for event publication")
	}

	manager.SetPrice("5021;6", Currencies{Metal: 52.0}, Currencies{Metal: 57.0}, "Autokeys")

	select {
	case ev := <-sub.C():
		updatedEv, ok := ev.(*PricelistUpdatedEvent)
		require.True(t, ok)
		assert.Equal(t, "5021;6", updatedEv.SKU)
		assert.Equal(t, 52.0, updatedEv.Buy.Metal)
		assert.Equal(t, 57.0, updatedEv.Sell.Metal)

	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for event publication")
	}

	manager.SetPrice("5021;6", Currencies{Metal: 52.0}, Currencies{Metal: 57.0}, "Autokeys")

	select {
	case <-sub.C():
		t.Fatal("should not receive redundant event")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}
