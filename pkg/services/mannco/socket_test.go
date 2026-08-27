// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mannco

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLogRecorder struct {
	log.Logger
	messages []string
}

func (m *mockLogRecorder) Warn(msg string, fields ...any) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogRecorder) With(fields ...any) log.Logger {
	return m
}

func TestSocketManager_New_Defaults(t *testing.T) {
	t.Parallel()

	sm := NewSocketManager("", nil, nil)
	assert.NotNil(t, sm.r)
	assert.Equal(t, DefaultWSURL, sm.url)
}

func TestSocketManager_Run_Cancellation(t *testing.T) {
	t.Parallel()

	sm := NewSocketManager("ws://127.0.0.1:49151/ws", nil, log.Discard)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	err := sm.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSocketManager_DialFail(t *testing.T) {
	t.Parallel()

	sm := NewSocketManager("ws://127.0.0.1:49151/invalid", nil, log.Discard)
	err := sm.connectAndListen(t.Context())
	assert.Error(t, err)
}

func TestSocketManager_URLErr(t *testing.T) {
	t.Parallel()

	sm := NewSocketManager(":%:invalid", nil, log.Discard)
	err := sm.connectAndListen(t.Context())
	assert.Error(t, err)
}

func TestSocketManager_Events(t *testing.T) {
	t.Parallel()

	upgrader := websocket.Upgrader{}
	eventsSent := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send price_changed event
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{
			"event": "price_changed",
			"ts": 1782403724,
			"data": {
				"id": 57322926,
				"assetId": "17188460072",
				"itemId": 1536,
				"oldPrice": 1527,
				"newPrice": 1525
			}
		}`))

		// Send listing_added event
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{
			"event": "listing_added",
			"ts": 1782403704,
			"data": {
				"id": 57348505,
				"assetId": "17190178976",
				"itemId": 16145,
				"price": 61
			}
		}`))

		// Send listing_removed event
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{
			"event": "listing_removed",
			"ts": 1782403724,
			"data": {
				"id": 57348505
			}
		}`))

		// Send buyorder_added event
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{
			"event": "buyorder_added",
			"ts": 1782403724,
			"data": {
				"id": 880421,
				"itemId": 1536,
				"price": 1525,
				"amount": 5
			}
		}`))

		// Send buyorder_removed event
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{
			"event": "buyorder_removed",
			"ts": 1782403724,
			"data": {
				"id": 880421,
				"itemId": 1536,
				"oldPrice": 1525
			}
		}`))

		// Send buyorder_updated event
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{
			"event": "buyorder_updated",
			"ts": 1782403724,
			"data": {
				"id": 880421,
				"itemId": 1536,
				"oldPrice": 1525,
				"newPrice": 1530,
				"oldAmount": 5,
				"newAmount": 3
			}
		}`))

		// Send buyorder_activated event
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{
			"event": "buyorder_activated",
			"ts": 1782403724,
			"data": {
				"id": 880421,
				"itemId": 1536,
				"price": 1525,
				"amount": 5
			}
		}`))

		// Send buyorder_deactivated event
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{
			"event": "buyorder_deactivated",
			"ts": 1782403724,
			"data": {
				"id": 880421,
				"itemId": 1536,
				"price": 1525
			}
		}`))

		close(eventsSent)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	sm := NewSocketManager(wsURL, nil, log.Discard)

	var (
		mu                     sync.Mutex
		receivedGeneric        int
		gotPriceChanged        *PriceChangedEvent
		gotListingAdded        *ListingAddedEvent
		gotListingRemoved      *ListingRemovedEvent
		gotBuyOrderAdded       *BuyOrderAddedEvent
		gotBuyOrderRemoved     *BuyOrderRemovedEvent
		gotBuyOrderUpdated     *BuyOrderUpdatedEvent
		gotBuyOrderActivated   *BuyOrderActivatedEvent
		gotBuyOrderDeactivated *BuyOrderDeactivatedEvent
	)

	sm.OnEvent(func(e *WSEvent) {
		mu.Lock()
		defer mu.Unlock()

		receivedGeneric++
	})

	sm.OnPriceChanged(func(e *PriceChangedEvent) {
		mu.Lock()
		defer mu.Unlock()

		gotPriceChanged = e
	})

	sm.OnListingAdded(func(e *ListingAddedEvent) {
		mu.Lock()
		defer mu.Unlock()

		gotListingAdded = e
	})

	sm.OnListingRemoved(func(e *ListingRemovedEvent) {
		mu.Lock()
		defer mu.Unlock()

		gotListingRemoved = e
	})

	sm.OnBuyOrderAdded(func(e *BuyOrderAddedEvent) {
		mu.Lock()
		defer mu.Unlock()

		gotBuyOrderAdded = e
	})

	sm.OnBuyOrderRemoved(func(e *BuyOrderRemovedEvent) {
		mu.Lock()
		defer mu.Unlock()

		gotBuyOrderRemoved = e
	})

	sm.OnBuyOrderUpdated(func(e *BuyOrderUpdatedEvent) {
		mu.Lock()
		defer mu.Unlock()

		gotBuyOrderUpdated = e
	})

	sm.OnBuyOrderActivated(func(e *BuyOrderActivatedEvent) {
		mu.Lock()
		defer mu.Unlock()

		gotBuyOrderActivated = e
	})

	sm.OnBuyOrderDeactivated(func(e *BuyOrderDeactivatedEvent) {
		mu.Lock()
		defer mu.Unlock()

		gotBuyOrderDeactivated = e
	})

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	go func() {
		_ = sm.connectAndListen(ctx)
	}()

	select {
	case <-eventsSent:
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		assert.Equal(t, 8, receivedGeneric)

		require.NotNil(t, gotPriceChanged)
		assert.Equal(t, int64(57322926), gotPriceChanged.Data.ID)
		assert.Equal(t, "17188460072", gotPriceChanged.Data.AssetID)
		assert.Equal(t, int64(1536), gotPriceChanged.Data.ItemID)
		assert.Equal(t, int64(1527), gotPriceChanged.Data.OldPrice)
		assert.Equal(t, int64(1525), gotPriceChanged.Data.NewPrice)

		require.NotNil(t, gotListingAdded)
		assert.Equal(t, int64(57348505), gotListingAdded.Data.ID)
		assert.Equal(t, "17190178976", gotListingAdded.Data.AssetID)
		assert.Equal(t, int64(16145), gotListingAdded.Data.ItemID)
		assert.Equal(t, int64(61), gotListingAdded.Data.Price)

		require.NotNil(t, gotListingRemoved)
		assert.Equal(t, int64(57348505), gotListingRemoved.Data.ID)

		require.NotNil(t, gotBuyOrderAdded)
		assert.Equal(t, int64(880421), gotBuyOrderAdded.Data.ID)
		assert.Equal(t, int64(1536), gotBuyOrderAdded.Data.ItemID)
		assert.Equal(t, int64(1525), gotBuyOrderAdded.Data.Price)
		assert.Equal(t, 5, gotBuyOrderAdded.Data.Amount)

		require.NotNil(t, gotBuyOrderRemoved)
		assert.Equal(t, int64(880421), gotBuyOrderRemoved.Data.ID)
		assert.Equal(t, int64(1536), gotBuyOrderRemoved.Data.ItemID)
		assert.Equal(t, int64(1525), gotBuyOrderRemoved.Data.OldPrice)

		require.NotNil(t, gotBuyOrderUpdated)
		assert.Equal(t, int64(880421), gotBuyOrderUpdated.Data.ID)
		assert.Equal(t, int64(1536), gotBuyOrderUpdated.Data.ItemID)
		assert.Equal(t, int64(1525), gotBuyOrderUpdated.Data.OldPrice)
		assert.Equal(t, int64(1530), gotBuyOrderUpdated.Data.NewPrice)
		assert.Equal(t, 5, gotBuyOrderUpdated.Data.OldAmount)
		assert.Equal(t, 3, gotBuyOrderUpdated.Data.NewAmount)

		require.NotNil(t, gotBuyOrderActivated)
		assert.Equal(t, int64(880421), gotBuyOrderActivated.Data.ID)
		assert.Equal(t, int64(1536), gotBuyOrderActivated.Data.ItemID)
		assert.Equal(t, int64(1525), gotBuyOrderActivated.Data.Price)
		assert.Equal(t, 5, gotBuyOrderActivated.Data.Amount)

		require.NotNil(t, gotBuyOrderDeactivated)
		assert.Equal(t, int64(880421), gotBuyOrderDeactivated.Data.ID)
		assert.Equal(t, int64(1536), gotBuyOrderDeactivated.Data.ItemID)
		assert.Equal(t, int64(1525), gotBuyOrderDeactivated.Data.Price)

	case <-ctx.Done():
		t.Fatal("timed out waiting for events")
	}
}

func TestSocketManager_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("invalid_json", func(t *testing.T) {
		logger := &mockLogRecorder{}
		sm := NewSocketManager("", nil, logger)

		sm.handleMessage([]byte("{invalid_json"))

		require.NotEmpty(t, logger.messages)
		assert.Contains(t, logger.messages[0], "Failed to unmarshal market stream event from socket")
	})

	t.Run("invalid_data_payload", func(t *testing.T) {
		logger := &mockLogRecorder{}
		sm := NewSocketManager("", nil, logger)

		called := false
		sm.OnPriceChanged(func(e *PriceChangedEvent) {
			called = true
		})

		sm.handleMessage([]byte(`{"event":"price_changed","ts":123,"data":"invalid_data"}`))

		assert.False(t, called)
		require.NotEmpty(t, logger.messages)
		assert.Contains(t, logger.messages[0], "Failed to unmarshal price_changed data")
	})

	t.Run("unhandled_event_name", func(t *testing.T) {
		sm := NewSocketManager("", nil, log.Discard)
		calledGeneric := false
		sm.OnEvent(func(e *WSEvent) {
			calledGeneric = true
		})

		sm.handleMessage([]byte(`{"event":"unknown_event","ts":123,"data":{}}`))

		assert.True(t, calledGeneric)
	})
}
