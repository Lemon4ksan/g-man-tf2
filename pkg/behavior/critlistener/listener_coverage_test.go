// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package critlistener

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/crit"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	tf2trading "github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

type coverageMockCritClient struct {
	FetchAuthTokenErr error
	FetchAuthTokenRes string
	StreamEventsErr   error
	StreamEventsChan  chan crit.SSEEvent
	SendDeadMansErr   error
	SendDeadMansRes   bool
}

func (m *coverageMockCritClient) FetchAuthToken(ctx context.Context) (string, error) {
	return m.FetchAuthTokenRes, m.FetchAuthTokenErr
}

func (m *coverageMockCritClient) StreamEvents(
	ctx context.Context,
	streamURL, token string,
) (<-chan crit.SSEEvent, error) {
	return m.StreamEventsChan, m.StreamEventsErr
}

func (m *coverageMockCritClient) SendDeadMansRequest(ctx context.Context) (bool, error) {
	return m.SendDeadMansRes, m.SendDeadMansErr
}

type mockBackpack struct {
	schemaObj *schema.Schema
	items     map[uint64]*tf2.Item
	assetIDs  map[string][]uint64
}

func (m *mockBackpack) GetAssetIDs(sku string) []uint64 { return m.assetIDs[sku] }
func (m *mockBackpack) LockItems(ids []uint64)          {}
func (m *mockBackpack) UnlockItems(ids []uint64)        {}
func (m *mockBackpack) GetItem(id uint64) (*tf2.Item, bool) {
	it, ok := m.items[id]
	return it, ok
}

type dummySchemaProvider struct {
	s *schema.Schema
}

func (d dummySchemaProvider) Get() *schema.Schema { return d.s }

func (m *mockBackpack) Schema() backpack.SchemaProvider {
	return dummySchemaProvider{s: m.schemaObj}
}

type mockPrice struct {
	prices map[string]*pricedb.Price
}

func (m *mockPrice) GetPrice(sku string) (*pricedb.Price, bool) {
	p, ok := m.prices[sku]
	return p, ok
}

type mockConfig struct {
	cfg tf2trading.Config
}

func (m *mockConfig) GetConfig() tf2trading.Config { return m.cfg }

type mockTrade struct {
	err error
}

func (m *mockTrade) SendOffer(ctx context.Context, p trading.OfferParams) (uint64, error) {
	return 456, m.err
}

func TestCoverage_FetchAuthTokenError(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	mockClient := &coverageMockCritClient{
		FetchAuthTokenErr: errors.New("auth error"),
	}

	listener := New(mockClient, nil, nil, nil, nil, "stream_url", eventBus, logger)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := listener.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCoverage_StreamEventsError(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	mockClient := &coverageMockCritClient{
		FetchAuthTokenRes: "token",
		StreamEventsErr:   errors.New("stream error"),
	}

	listener := New(mockClient, nil, nil, nil, nil, "stream_url", eventBus, logger)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := listener.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestCoverage_Listen(t *testing.T) {
	t.Parallel()

	b := bus.New()
	logger := log.Discard
	opt := Listen(nil, nil, nil, nil, nil, "stream_url")
	orch := behavior.NewOrchestrator(logger, b)
	opt(orch)
}

func TestCoverage_Name(t *testing.T) {
	t.Parallel()

	b := bus.New()
	logger := log.Discard
	listener := New(nil, nil, nil, nil, nil, "stream_url", b, logger)
	assert.Equal(t, BehaviorName, listener.Name())
}

func TestCoverage_HandleTradeRequest_NoItemsConfigured(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpack{}
	price := &mockPrice{}
	cfg := &mockConfig{cfg: tf2trading.Config{}}
	trade := &mockTrade{}

	listener := New(nil, price, bp, cfg, trade, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_InvalidTradeURL(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpack{}
	price := &mockPrice{}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"5021;6": {},
		},
	}}
	trade := &mockTrade{}

	listener := New(nil, price, bp, cfg, trade, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "invalid_url",
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_NilSchema(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpack{schemaObj: nil}
	price := &mockPrice{}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"5021;6": {},
		},
	}}
	trade := &mockTrade{}

	listener := New(nil, price, bp, cfg, trade, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_UnknownKindItemsToGive(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	s := schema.New(&schema.Raw{})
	bp := &mockBackpack{schemaObj: s}
	price := &mockPrice{}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"5021;6": {},
		},
	}}
	trade := &mockTrade{}

	listener := New(nil, price, bp, cfg, trade, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
		ItemsToGive: []crit.TradeRequestItem{
			{Kind: "unknown"},
		},
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_ProcessEvents_SendDeadMansError(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	mockClient := &coverageMockCritClient{
		SendDeadMansErr: errors.New("refresh error"),
		SendDeadMansRes: false,
	}

	listener := New(mockClient, nil, nil, nil, nil, "stream_url", eventBus, logger)

	events := make(chan crit.SSEEvent, 1)
	events <- crit.SSEEvent{Event: "heartbeat"}

	close(events)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := listener.processEvents(ctx, events, cancel)
	assert.ErrorContains(t, err, "dead man's request failed")
}

func TestCoverage_ProcessEvents_SendDeadMansFalse(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	mockClient := &coverageMockCritClient{
		SendDeadMansRes: false,
	}

	listener := New(mockClient, nil, nil, nil, nil, "stream_url", eventBus, logger)

	events := make(chan crit.SSEEvent, 1)
	events <- crit.SSEEvent{Event: "heartbeat"}

	close(events)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := listener.processEvents(ctx, events, cancel)
	assert.ErrorContains(t, err, "dead man's request returned false")
}

func TestCoverage_ProcessEvents_InvalidTradeRequestJSON(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	listener := New(nil, nil, nil, nil, nil, "stream_url", eventBus, logger)

	events := make(chan crit.SSEEvent, 2)
	events <- crit.SSEEvent{Event: "trade_request", Data: "invalid{json"}

	events <- crit.SSEEvent{Event: "unknown"}

	close(events)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := listener.processEvents(ctx, events, cancel)
	assert.ErrorContains(t, err, "event channel closed")
}

func TestCoverage_ProcessEvents_NilTradeRequestPayload(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	listener := New(nil, nil, nil, nil, nil, "stream_url", eventBus, logger)

	events := make(chan crit.SSEEvent, 2)
	events <- crit.SSEEvent{Event: "trade_request", Data: `{"kind":"trade_request"}`}

	close(events)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := listener.processEvents(ctx, events, cancel)
	assert.ErrorContains(t, err, "event channel closed")
}

func TestCoverage_HandleTradeRequest_ItemNotInPricelist(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	s := schema.New(&schema.Raw{})
	bp := &mockBackpack{schemaObj: s}
	price := &mockPrice{prices: map[string]*pricedb.Price{}}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"123;6": {EnableSell: true},
		},
	}}

	listener := New(nil, price, bp, cfg, nil, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
		ItemsToGive: []crit.TradeRequestItem{
			{Kind: "sku", SKU: "123;6", Amount: 1},
		},
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_SellDisabled(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	s := schema.New(&schema.Raw{})
	bp := &mockBackpack{schemaObj: s}
	price := &mockPrice{prices: map[string]*pricedb.Price{
		"123;6": {Sell: pricedb.Currencies{Metal: 10.0}},
	}}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"123;6": {EnableSell: false},
		},
	}}

	listener := New(nil, price, bp, cfg, nil, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
		ItemsToGive: []crit.TradeRequestItem{
			{Kind: "sku", SKU: "123;6", Amount: 1},
		},
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_NotEnoughAvailableItems(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	s := schema.New(&schema.Raw{})
	bp := &mockBackpack{
		schemaObj: s,
		assetIDs: map[string][]uint64{
			"123;6": {},
		},
	}
	price := &mockPrice{prices: map[string]*pricedb.Price{
		"123;6": {Sell: pricedb.Currencies{Metal: 10.0}},
	}}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"123;6": {EnableSell: true},
		},
	}}

	listener := New(nil, price, bp, cfg, nil, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
		ItemsToGive: []crit.TradeRequestItem{
			{Kind: "sku", SKU: "123;6", Amount: 1},
		},
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_EconItemDisappeared(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	s := schema.New(&schema.Raw{})
	bp := &mockBackpack{
		schemaObj: s,
		assetIDs: map[string][]uint64{
			"123;6": {10001},
		},
		items: map[uint64]*tf2.Item{},
	}
	price := &mockPrice{prices: map[string]*pricedb.Price{
		"123;6": {Sell: pricedb.Currencies{Metal: 10.0}},
	}}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"123;6": {EnableSell: true},
		},
	}}

	listener := New(nil, price, bp, cfg, nil, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
		ItemsToGive: []crit.TradeRequestItem{
			{Kind: "sku", SKU: "123;6", Amount: 1},
		},
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_UnsupportedReceiveAssetID(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpack{}
	price := &mockPrice{}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"123;6": {},
		},
	}}

	listener := New(nil, price, bp, cfg, nil, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
		ItemsToReceive: []crit.TradeRequestItem{
			{Kind: "assetid", AssetID: "10001"},
		},
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_ReceiveItemNotInPricelist(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpack{}
	price := &mockPrice{prices: map[string]*pricedb.Price{}}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"123;6": {EnableBuy: true},
		},
	}}

	listener := New(nil, price, bp, cfg, nil, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
		ItemsToReceive: []crit.TradeRequestItem{
			{Kind: "sku", SKU: "123;6", Amount: 1},
		},
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_ReceiveBuyDisabled(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpack{}
	price := &mockPrice{prices: map[string]*pricedb.Price{
		"123;6": {Buy: pricedb.Currencies{Metal: 10.0}},
	}}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"123;6": {EnableBuy: false},
		},
	}}

	listener := New(nil, price, bp, cfg, nil, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
		ItemsToReceive: []crit.TradeRequestItem{
			{Kind: "sku", SKU: "123;6", Amount: 1},
		},
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_ReceiveUnknownKind(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpack{}
	price := &mockPrice{}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"123;6": {},
		},
	}}

	listener := New(nil, price, bp, cfg, nil, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
		ItemsToReceive: []crit.TradeRequestItem{
			{Kind: "unknown"},
		},
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_GiveInvalidAssetID(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpack{}
	price := &mockPrice{}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"123;6": {},
		},
	}}

	listener := New(nil, price, bp, cfg, nil, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
		ItemsToGive: []crit.TradeRequestItem{
			{Kind: "assetid", AssetID: "abc"},
		},
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_GiveAssetIDNotFound(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	bp := &mockBackpack{
		items: map[uint64]*tf2.Item{},
	}
	price := &mockPrice{}
	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"123;6": {},
		},
	}}

	listener := New(nil, price, bp, cfg, nil, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
		ItemsToGive: []crit.TradeRequestItem{
			{Kind: "assetid", AssetID: "10001"},
		},
	}

	listener.handleTradeRequest(t.Context(), payload)
}

func TestCoverage_HandleTradeRequest_Success(t *testing.T) {
	t.Parallel()

	logger := log.Discard
	eventBus := bus.New()

	s := schema.New(&schema.Raw{})
	bp := &mockBackpack{
		schemaObj: s,
		assetIDs: map[string][]uint64{
			"123;6":  {10001},
			"5021;6": {10003},
		},
		items: map[uint64]*tf2.Item{
			10001: {
				ID:       10001,
				DefIndex: 123,
				Quality:  6,
				SKU:      "123;6",
			},
			10002: {
				ID:       10002,
				DefIndex: 789,
				Quality:  6,
				SKU:      "789;6",
			},
			10003: {
				ID:       10003,
				DefIndex: 5021,
				Quality:  6,
				SKU:      "5021;6",
			},
			10004: {
				ID:       10004,
				DefIndex: 5002,
				Quality:  6,
				SKU:      "5002;6",
			},
		},
	}

	price := &mockPrice{prices: map[string]*pricedb.Price{
		"123;6": {Sell: pricedb.Currencies{Metal: 10.0}},
		"789;6": {Sell: pricedb.Currencies{Metal: 15.0}},
		"456;6": {Buy: pricedb.Currencies{Metal: 20.0}},
	}}

	cfg := &mockConfig{cfg: tf2trading.Config{
		Items: map[string]tf2trading.ItemConfig{
			"123;6": {EnableSell: true},
			"789;6": {EnableSell: true},
			"456;6": {EnableBuy: true},
		},
	}}

	trade := &mockTrade{}

	listener := New(nil, price, bp, cfg, trade, "", eventBus, logger)

	payload := &crit.TradeRequestPayload{
		TradeOfferURL: "https://steamcommunity.com/tradeoffer/new/?partner=123&token=ABC",
		ItemsToGive: []crit.TradeRequestItem{
			{Kind: "sku", SKU: "123;6", Amount: 1},
			{Kind: "assetid", AssetID: "10002"},
			{Kind: "sku", SKU: "5021;6", Amount: 0},
			{Kind: "assetid", AssetID: "10004"},
		},
		ItemsToReceive: []crit.TradeRequestItem{
			{Kind: "sku", SKU: "456;6", Amount: 1},
			{Kind: "sku", SKU: "5002;6", Amount: 0},
		},
	}

	listener.handleTradeRequest(t.Context(), payload)
}
