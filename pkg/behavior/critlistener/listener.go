// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package critlistener listens for crit.tf events and processes them.
package critlistener

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/trading"
	webtrading "github.com/lemon4ksan/g-man/pkg/trading/web"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/crit"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
	tf2trading "github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

// BehaviorName is the unique name of this behavior.
const BehaviorName = "crit_event_listener"

// CritClientProvider defines the subset of crit.Client methods we need.
type CritClientProvider interface {
	FetchAuthToken(ctx context.Context) (string, error)
	StreamEvents(ctx context.Context, streamURL, token string) (<-chan crit.SSEEvent, error)
	SendDeadMansRequest(ctx context.Context) (bool, error)
}

// PriceProvider defines pricedb methods.
type PriceProvider interface {
	GetPrice(sku string) (*pricedb.Price, bool)
}

// BackpackProvider defines backpack methods.
type BackpackProvider interface {
	GetAssetIDs(sku string) []uint64
	LockItems(ids []uint64)
	UnlockItems(ids []uint64)
	GetItem(id uint64) (*tf2.Item, bool)
	Schema() backpack.SchemaProvider
}

// ConfigProvider defines trading config manager.
type ConfigProvider interface {
	GetConfig() tf2trading.Config
}

// TradeProvider defines the trade manager.
type TradeProvider interface {
	SendOffer(ctx context.Context, p trading.OfferParams) (uint64, error)
}

// CritEventListener listens to the Crit.tf SSE stream for incoming trade requests and keepalives.
type CritEventListener struct {
	logger    log.Logger
	bus       *bus.Bus
	streamURL string

	client   CritClientProvider
	priceMgr PriceProvider
	bp       BackpackProvider
	cfgMgr   ConfigProvider
	tradeMgr TradeProvider
}

// Listen returns a behavior.Option to install CritEventListener.
func Listen(
	client CritClientProvider,
	priceMgr PriceProvider,
	bp BackpackProvider,
	cfgMgr ConfigProvider,
	tradeMgr TradeProvider,
	streamURL string,
) behavior.Option {
	return func(o *behavior.Orchestrator) {
		o.Register(New(client, priceMgr, bp, cfgMgr, tradeMgr, streamURL, o.Bus(), o.Logger()))
	}
}

// New constructs a new CritEventListener.
func New(
	client CritClientProvider,
	priceMgr PriceProvider,
	bp BackpackProvider,
	cfgMgr ConfigProvider,
	tradeMgr TradeProvider,
	streamURL string,
	b *bus.Bus,
	logger log.Logger,
) *CritEventListener {
	return &CritEventListener{
		logger:    logger.With(log.Module(BehaviorName)),
		bus:       b,
		streamURL: streamURL,
		client:    client,
		priceMgr:  priceMgr,
		bp:        bp,
		cfgMgr:    cfgMgr,
		tradeMgr:  tradeMgr,
	}
}

// Name returns the unique behavior name.
func (s *CritEventListener) Name() string {
	return BehaviorName
}

// Run starts the main loop of the SSE stream listener.
func (s *CritEventListener) Run(ctx context.Context) error {
	s.logger.Info("CritEventListener started")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 1. Fetch Auth Token
		token, err := s.client.FetchAuthToken(ctx)
		if err != nil {
			s.logger.Error("Failed to fetch Crit.tf auth token", log.Err(err))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(15 * time.Second):
			}

			continue
		}

		s.logger.Info("Successfully fetched Crit.tf auth token")

		// 2. Establish SSE connection
		streamURL := s.streamURL
		if streamURL == "" {
			streamURL = "https://events.pricedb.io/event-stream"
		}

		events, err := s.client.StreamEvents(ctx, streamURL, token)
		if err != nil {
			s.logger.Error("Failed to connect to Crit.tf event stream", log.Err(err))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(15 * time.Second):
			}

			continue
		}

		s.logger.Info("Crit.tf event stream connection established")

		// 3. Process events
		streamCtx, cancelStream := context.WithCancel(ctx)
		err = s.processEvents(streamCtx, events, cancelStream)
		cancelStream()

		if err != nil {
			s.logger.Error("Crit.tf event stream disconnected with error", log.Err(err))
		} else {
			s.logger.Info("Crit.tf event stream disconnected")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(15 * time.Second):
		}
	}
}

func (s *CritEventListener) processEvents(
	ctx context.Context,
	events <-chan crit.SSEEvent,
	cancelStream context.CancelFunc,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				return errors.New("event channel closed")
			}

			switch ev.Event {
			case "heartbeat":
				s.logger.Debug("Received heartbeat event")

				ok, err := s.client.SendDeadMansRequest(ctx)
				if err != nil || !ok {
					s.logger.Error("Dead Man's Request failed, restarting connection", log.Err(err))
					cancelStream()

					if err != nil {
						return fmt.Errorf("dead man's request failed: %w", err)
					}

					return errors.New("dead man's request returned false")
				}

				s.logger.Debug("Dead Man's Request successful, connection is okay")

			case "trade_request":
				s.logger.Info("Received trade_request event")

				var envelope crit.TradeRequestEventEnvelope
				if err := json.Unmarshal([]byte(ev.Data), &envelope); err != nil {
					s.logger.Error("Failed to parse trade request JSON", log.Err(err))
					continue
				}

				if envelope.TradeRequest == nil {
					s.logger.Warn("Trade request payload is missing")
					continue
				}

				go s.handleTradeRequest(ctx, envelope.TradeRequest)

			default:
				s.logger.Warn("Unknown event type encountered", log.String("event", ev.Event))
			}
		}
	}
}

func (s *CritEventListener) handleTradeRequest(ctx context.Context, payload *crit.TradeRequestPayload) {
	s.logger.Info("Processing trade request", log.String("url", payload.TradeOfferURL))

	// Step 5.1: Primary checks
	cfg := s.cfgMgr.GetConfig()
	if len(cfg.Items) == 0 {
		s.logger.Warn("Ignoring trade request: no items configured for trading")
		return
	}

	// Step 5.2: Parse trade URL
	partnerID, token, err := webtrading.ParseTradeURL(payload.TradeOfferURL)
	if err != nil {
		s.logger.Error("Failed to parse trade URL", log.Err(err))
		return
	}

	currencySKUs := map[string]bool{
		currency.SKUKey:       true,
		currency.SKURefined:   true,
		currency.SKUReclaimed: true,
		currency.SKUScrap:     true,
	}

	var (
		itemsToGive    []*trading.Item
		itemsToReceive []*trading.Item
		tempLocked     []uint64
	)

	schema := s.bp.Schema().Get()
	if schema == nil {
		s.logger.Error("Schema is not loaded, cannot process trade request")
		return
	}

	// Step 5.3: Items to Give
	for _, item := range payload.ItemsToGive {
		switch item.Kind {
		case "sku":
			isCurrency := currencySKUs[item.SKU]
			if !isCurrency {
				price, ok := s.priceMgr.GetPrice(item.SKU)
				if !ok || price.Sell.Metal <= 0 {
					s.logger.Warn("Ignoring trade request: item not in enabled pricelist", log.String("sku", item.SKU))
					return
				}

				if itemCfg, exists := cfg.Items[item.SKU]; exists {
					if !itemCfg.EnableSell {
						s.logger.Warn("Ignoring trade request: sell is disabled for SKU", log.String("sku", item.SKU))
						return
					}
				}
			}

			amount := item.Amount
			if amount <= 0 {
				amount = 1
			}

			availableIDs := s.bp.GetAssetIDs(item.SKU)

			var selectedIDs []uint64
			for _, id := range availableIDs {
				if !slices.Contains(tempLocked, id) {
					selectedIDs = append(selectedIDs, id)
					if len(selectedIDs) == amount {
						break
					}
				}
			}

			if len(selectedIDs) < amount {
				s.logger.Warn(
					"Ignoring trade request: not enough available items",
					log.String("sku", item.SKU),
					log.Int("needed", amount),
					log.Int("got", len(selectedIDs)),
				)

				return
			}

			for _, id := range selectedIDs {
				tempLocked = append(tempLocked, id)

				econItem, exists := s.bp.GetItem(id)
				if !exists {
					s.logger.Error("Item disappeared from backpack during mapping", log.Uint64("id", id))
					return
				}

				itemsToGive = append(itemsToGive, econItem.ToEconItem())
			}

		case "assetid":
			assetIDVal, err := strconv.ParseUint(item.AssetID, 10, 64)
			if err != nil {
				s.logger.Error("Failed to parse assetid", log.String("assetid", item.AssetID), log.Err(err))
				return
			}

			it, exists := s.bp.GetItem(assetIDVal)
			if !exists {
				s.logger.Warn(
					"Ignoring trade request: bot backpack does not contain asset ID",
					log.Uint64("assetid", assetIDVal),
				)

				return
			}

			sku := it.GetSKU(schema)

			isCurrency := currencySKUs[sku]
			if !isCurrency {
				price, ok := s.priceMgr.GetPrice(sku)
				if !ok || price.Sell.Metal <= 0 {
					s.logger.Warn("Ignoring trade request: item not in enabled pricelist", log.String("sku", sku))
					return
				}

				if itemCfg, exists := cfg.Items[sku]; exists {
					if !itemCfg.EnableSell {
						s.logger.Warn("Ignoring trade request: sell is disabled for SKU", log.String("sku", sku))
						return
					}
				}
			}

			tempLocked = append(tempLocked, assetIDVal)
			itemsToGive = append(itemsToGive, it.ToEconItem())

		default:
			s.logger.Warn("Unknown item kind on ItemsToGive", log.String("kind", item.Kind))
			return
		}
	}

	// Step 5.4: Items to Receive
	for _, item := range payload.ItemsToReceive {
		switch item.Kind {
		case "assetid":
			s.logger.Warn("Unsupported items_to_receive kind: assetid")
			return

		case "sku":
			isCurrency := currencySKUs[item.SKU]
			if !isCurrency {
				price, ok := s.priceMgr.GetPrice(item.SKU)
				if !ok || price.Buy.Metal <= 0 {
					s.logger.Warn("Ignoring trade request: item not in enabled pricelist", log.String("sku", item.SKU))
					return
				}

				if itemCfg, exists := cfg.Items[item.SKU]; exists {
					if !itemCfg.EnableBuy {
						s.logger.Warn("Ignoring trade request: buy is disabled for SKU", log.String("sku", item.SKU))
						return
					}
				}
			}

			amount := item.Amount
			if amount <= 0 {
				amount = 1
			}

			itemsToReceive = append(itemsToReceive, &trading.Item{
				AppID:     440,
				ContextID: 2,
				SKU:       item.SKU,
				Amount:    int64(amount),
			})

		default:
			s.logger.Warn("Unknown item kind on ItemsToReceive", log.String("kind", item.Kind))
			return
		}
	}

	// Step 5.5: Lock items and send the offer
	if len(tempLocked) > 0 {
		s.bp.LockItems(tempLocked)

		var sent bool
		defer func() {
			if !sent {
				s.bp.UnlockItems(tempLocked)
			}
		}()
	}

	params := trading.OfferParams{
		PartnerID:      partnerID,
		Token:          token,
		Message:        "Crit.tf Trade Request Auto-Offer",
		ItemsToGive:    itemsToGive,
		ItemsToReceive: itemsToReceive,
	}

	offerID, err := s.tradeMgr.SendOffer(ctx, params)
	if err != nil {
		s.logger.Error("Failed to send trade offer", log.Err(err))
		return
	}

	s.logger.Info("Successfully sent trade offer",
		log.Uint64("offer_id", offerID),
		log.String("partner", partnerID.String()),
		log.Int("items_to_give", len(itemsToGive)),
		log.Int("items_to_receive", len(itemsToReceive)),
	)
}
