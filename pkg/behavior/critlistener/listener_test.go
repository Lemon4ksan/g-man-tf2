// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package critlistener_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/behavior/critlistener"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/crit"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	tf2trading "github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

type mockCritClient struct {
	token         string
	events        chan crit.SSEEvent
	refreshCalled chan bool
}

func (m *mockCritClient) FetchAuthToken(ctx context.Context) (string, error) {
	if m.token == "" {
		return "", errors.New("auth error")
	}

	return m.token, nil
}

func (m *mockCritClient) StreamEvents(ctx context.Context, streamURL, token string) (<-chan crit.SSEEvent, error) {
	return m.events, nil
}

func (m *mockCritClient) SendDeadMansRequest(ctx context.Context) (bool, error) {
	m.refreshCalled <- true
	return true, nil
}

type mockPriceProvider struct {
	prices map[string]*pricedb.Price
}

func (m *mockPriceProvider) GetPrice(sku string) (*pricedb.Price, bool) {
	p, ok := m.prices[sku]
	return p, ok
}

type mockBackpackProvider struct {
	assetIDs   map[string][]uint64
	items      map[uint64]*tf2.Item
	lockedIDs  []uint64
	schemaMock *schema.Schema
}

func (m *mockBackpackProvider) GetAssetIDs(sku string) []uint64 {
	return m.assetIDs[sku]
}

func (m *mockBackpackProvider) LockItems(ids []uint64) {
	m.lockedIDs = append(m.lockedIDs, ids...)
}

func (m *mockBackpackProvider) UnlockItems(ids []uint64) {
}

func (m *mockBackpackProvider) GetItem(id uint64) (*tf2.Item, bool) {
	it, ok := m.items[id]
	return it, ok
}

type dummySchemaProvider struct {
	sch *schema.Schema
}

func (d dummySchemaProvider) Get() *schema.Schema {
	return d.sch
}

func (m *mockBackpackProvider) Schema() backpack.SchemaProvider {
	return dummySchemaProvider{sch: m.schemaMock}
}

type mockConfigProvider struct {
	cfg tf2trading.Config
}

func (m *mockConfigProvider) GetConfig() tf2trading.Config {
	return m.cfg
}

type mockTradeProvider struct {
	sentOffers chan trading.OfferParams
}

func (m *mockTradeProvider) SendOffer(ctx context.Context, p trading.OfferParams) (uint64, error) {
	m.sentOffers <- p
	return 12345, nil
}

func TestCritEventListener_Run_HeartbeatEvent_SendsDeadMansRequest(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	eventsChan := make(chan crit.SSEEvent, 10)
	refreshChan := make(chan bool, 10)

	client := &mockCritClient{
		token:         "test_token",
		events:        eventsChan,
		refreshCalled: refreshChan,
	}

	priceProvider := &mockPriceProvider{}
	bpProvider := &mockBackpackProvider{}
	cfgProvider := &mockConfigProvider{}
	tradeProvider := &mockTradeProvider{}

	listener := critlistener.New(
		client,
		priceProvider,
		bpProvider,
		cfgProvider,
		tradeProvider,
		"https://events.pricedb.io/event-stream",
		eventBus,
		logger,
	)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	go func() {
		_ = listener.Run(ctx)
	}()

	eventsChan <- crit.SSEEvent{
		Event: "heartbeat",
		Data:  "",
	}

	select {
	case refreshCalled := <-refreshChan:
		assert.True(t, refreshCalled)
	case <-time.After(2 * time.Second):
		t.Fatal("Heartbeat did not trigger RefreshInventory (Dead Man's Request)")
	}
}

func TestCritEventListener_Run_TradeRequestEvent_SendsTradeOffer(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	eventsChan := make(chan crit.SSEEvent, 10)
	sentOffers := make(chan trading.OfferParams, 10)

	client := &mockCritClient{
		token:  "test_token",
		events: eventsChan,
	}

	priceProvider := &mockPriceProvider{
		prices: map[string]*pricedb.Price{
			"123;6": {
				Buy:  pricedb.Currencies{Keys: 0, Metal: 10.0},
				Sell: pricedb.Currencies{Keys: 0, Metal: 12.0},
			},
		},
	}

	bpProvider := &mockBackpackProvider{
		assetIDs: map[string][]uint64{
			"123;6": {456},
		},
		items: map[uint64]*tf2.Item{
			456: {
				ID:       456,
				DefIndex: 123,
				Quality:  6,
			},
		},
		schemaMock: &schema.Schema{},
	}

	cfgProvider := &mockConfigProvider{
		cfg: tf2trading.Config{
			Items: map[string]tf2trading.ItemConfig{
				"123;6": {
					SKU:        "123;6",
					EnableBuy:  true,
					EnableSell: true,
				},
			},
		},
	}

	tradeProvider := &mockTradeProvider{
		sentOffers: sentOffers,
	}

	listener := critlistener.New(
		client,
		priceProvider,
		bpProvider,
		cfgProvider,
		tradeProvider,
		"https://events.pricedb.io/event-stream",
		eventBus,
		logger,
	)

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	go func() {
		_ = listener.Run(ctx)
	}()

	payload := crit.TradeRequestEventEnvelope{
		Kind: "trade_request",
		TradeRequest: &crit.TradeRequestPayload{
			TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123456&token=ABCDEF",
			ItemsToGive: []crit.TradeRequestItem{
				{
					Kind:   "sku",
					SKU:    "123;6",
					Amount: 1,
				},
			},
			ItemsToReceive: []crit.TradeRequestItem{
				{
					Kind:   "sku",
					SKU:    "123;6",
					Amount: 1,
				},
			},
		},
	}

	dataBytes, _ := json.Marshal(payload)

	eventsChan <- crit.SSEEvent{
		Event: "trade_request",
		Data:  string(dataBytes),
	}

	select {
	case offer := <-sentOffers:
		assert.Equal(t, id.ID(76561197960389184), offer.PartnerID)
		assert.Equal(t, "ABCDEF", offer.Token)
		assert.Len(t, offer.ItemsToGive, 1)
		assert.Len(t, offer.ItemsToReceive, 1)
		assert.Equal(t, "123;6", offer.ItemsToReceive[0].SKU)
		assert.Equal(t, uint64(456), offer.ItemsToGive[0].AssetID)

	case <-time.After(2 * time.Second):
		t.Fatal("Trade request did not trigger trade offer transmission")
	}
}
