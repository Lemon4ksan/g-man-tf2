// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mannco

import (
	"context"
	"net/url"
	"sync"
	"time"

	json "github.com/goccy/go-json"
	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/mod"
	"github.com/lemon4ksan/aoni/realtime/ws"
	"github.com/lemon4ksan/aoni/request"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/lemon4ksan/miyako/log"
)

// DefaultWSURL is the default WebSocket endpoint for the Mannco.store Market Stream.
const DefaultWSURL = "wss://ws.mannco.store/ws"

// Event names for Mannco.store WebSocket Market stream.
const (
	EventPriceChanged        = "price_changed"
	EventListingAdded        = "listing_added"
	EventListingRemoved      = "listing_removed"
	EventBuyOrderAdded       = "buyorder_added"
	EventBuyOrderRemoved     = "buyorder_removed"
	EventBuyOrderUpdated     = "buyorder_updated"
	EventBuyOrderActivated   = "buyorder_activated"
	EventBuyOrderDeactivated = "buyorder_deactivated"
)

// WSEvent represents the envelope for all raw WebSocket market stream events.
type WSEvent struct {
	Event string          `json:"event"`
	TS    int64           `json:"ts"`
	Data  json.RawMessage `json:"data"`
}

// PriceChangedData holds payload data for the "price_changed" event.
type PriceChangedData struct {
	ID       int64  `json:"id"`
	AssetID  string `json:"assetId"`
	ItemID   int64  `json:"itemId"`
	OldPrice int64  `json:"oldPrice"`
	NewPrice int64  `json:"newPrice"`
}

// PriceChangedEvent represents the complete "price_changed" event.
type PriceChangedEvent struct {
	Event string           `json:"event"`
	TS    int64            `json:"ts"`
	Data  PriceChangedData `json:"data"`
}

// ListingAddedData holds payload data for the "listing_added" event.
type ListingAddedData struct {
	ID      int64  `json:"id"`
	AssetID string `json:"assetId"`
	ItemID  int64  `json:"itemId"`
	Price   int64  `json:"price"`
}

// ListingAddedEvent represents the complete "listing_added" event.
type ListingAddedEvent struct {
	Event string           `json:"event"`
	TS    int64            `json:"ts"`
	Data  ListingAddedData `json:"data"`
}

// ListingRemovedData holds payload data for the "listing_removed" event.
type ListingRemovedData struct {
	ID int64 `json:"id"`
}

// ListingRemovedEvent represents the complete "listing_removed" event.
type ListingRemovedEvent struct {
	Event string             `json:"event"`
	TS    int64              `json:"ts"`
	Data  ListingRemovedData `json:"data"`
}

// BuyOrderAddedData holds payload data for the "buyorder_added" event.
type BuyOrderAddedData struct {
	ID     int64 `json:"id"`
	ItemID int64 `json:"itemId"`
	Price  int64 `json:"price"`
	Amount int   `json:"amount"`
}

// BuyOrderAddedEvent represents the complete "buyorder_added" event.
type BuyOrderAddedEvent struct {
	Event string            `json:"event"`
	TS    int64             `json:"ts"`
	Data  BuyOrderAddedData `json:"data"`
}

// BuyOrderRemovedData holds payload data for the "buyorder_removed" event.
type BuyOrderRemovedData struct {
	ID       int64 `json:"id"`
	ItemID   int64 `json:"itemId"`
	OldPrice int64 `json:"oldPrice"`
}

// BuyOrderRemovedEvent represents the complete "buyorder_removed" event.
type BuyOrderRemovedEvent struct {
	Event string              `json:"event"`
	TS    int64               `json:"ts"`
	Data  BuyOrderRemovedData `json:"data"`
}

// BuyOrderUpdatedData holds payload data for the "buyorder_updated" event.
type BuyOrderUpdatedData struct {
	ID        int64 `json:"id"`
	ItemID    int64 `json:"itemId"`
	OldPrice  int64 `json:"oldPrice"`
	NewPrice  int64 `json:"newPrice"`
	OldAmount int   `json:"oldAmount"`
	NewAmount int   `json:"newAmount"`
}

// BuyOrderUpdatedEvent represents the complete "buyorder_updated" event.
type BuyOrderUpdatedEvent struct {
	Event string              `json:"event"`
	TS    int64               `json:"ts"`
	Data  BuyOrderUpdatedData `json:"data"`
}

// BuyOrderActivatedData holds payload data for the "buyorder_activated" event.
type BuyOrderActivatedData struct {
	ID     int64 `json:"id"`
	ItemID int64 `json:"itemId"`
	Price  int64 `json:"price"`
	Amount int   `json:"amount"`
}

// BuyOrderActivatedEvent represents the complete "buyorder_activated" event.
type BuyOrderActivatedEvent struct {
	Event string                `json:"event"`
	TS    int64                 `json:"ts"`
	Data  BuyOrderActivatedData `json:"data"`
}

// BuyOrderDeactivatedData holds payload data for the "buyorder_deactivated" event.
type BuyOrderDeactivatedData struct {
	ID     int64 `json:"id"`
	ItemID int64 `json:"itemId"`
	Price  int64 `json:"price"`
}

// BuyOrderDeactivatedEvent represents the complete "buyorder_deactivated" event.
type BuyOrderDeactivatedEvent struct {
	Event string                  `json:"event"`
	TS    int64                   `json:"ts"`
	Data  BuyOrderDeactivatedData `json:"data"`
}

// SocketManager handles real-time market updates for Mannco.store via WebSockets.
type SocketManager struct {
	r         aoni.WebSocketDialer
	url       string
	logger    log.Logger
	userAgent string

	mu   sync.Mutex
	conn ws.Conn

	onEvent               func(event *WSEvent)
	onPriceChanged        func(event *PriceChangedEvent)
	onListingAdded        func(event *ListingAddedEvent)
	onListingRemoved      func(event *ListingRemovedEvent)
	onBuyOrderAdded       func(event *BuyOrderAddedEvent)
	onBuyOrderRemoved     func(event *BuyOrderRemovedEvent)
	onBuyOrderUpdated     func(event *BuyOrderUpdatedEvent)
	onBuyOrderActivated   func(event *BuyOrderActivatedEvent)
	onBuyOrderDeactivated func(event *BuyOrderDeactivatedEvent)
}

// NewSocketManager creates a new WebSocket client for Mannco.store Market Stream.
func NewSocketManager(rawURL string, r aoni.WebSocketDialer, logger log.Logger) *SocketManager {
	if r == nil {
		r = request.DefaultClient
	}

	if logger == nil {
		logger = log.Discard
	}

	return &SocketManager{
		r:      r,
		url:    generic.Coalesce(rawURL, DefaultWSURL),
		logger: logger.With(log.Module("mannco_socket")),
	}
}

// OnEvent sets a generic callback for all incoming WebSocket market stream events.
func (s *SocketManager) OnEvent(fn func(event *WSEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onEvent = fn
}

// OnPriceChanged sets the callback for when a listing's price is updated.
func (s *SocketManager) OnPriceChanged(fn func(event *PriceChangedEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onPriceChanged = fn
}

// OnListingAdded sets the callback for when a new listing is added to the market.
func (s *SocketManager) OnListingAdded(fn func(event *ListingAddedEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onListingAdded = fn
}

// OnListingRemoved sets the callback for when a listing is removed from the market.
func (s *SocketManager) OnListingRemoved(fn func(event *ListingRemovedEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onListingRemoved = fn
}

// OnBuyOrderAdded sets the callback for when a new buy order is created.
func (s *SocketManager) OnBuyOrderAdded(fn func(event *BuyOrderAddedEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onBuyOrderAdded = fn
}

// OnBuyOrderRemoved sets the callback for when a buy order is removed.
func (s *SocketManager) OnBuyOrderRemoved(fn func(event *BuyOrderRemovedEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onBuyOrderRemoved = fn
}

// OnBuyOrderUpdated sets the callback for when a buy order's price or amount changes.
func (s *SocketManager) OnBuyOrderUpdated(fn func(event *BuyOrderUpdatedEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onBuyOrderUpdated = fn
}

// OnBuyOrderActivated sets the callback for when a buy order becomes active.
func (s *SocketManager) OnBuyOrderActivated(fn func(event *BuyOrderActivatedEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onBuyOrderActivated = fn
}

// OnBuyOrderDeactivated sets the callback for when a buy order becomes inactive.
func (s *SocketManager) OnBuyOrderDeactivated(fn func(event *BuyOrderDeactivatedEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onBuyOrderDeactivated = fn
}

// Run starts the WebSocket listener and maintains the connection.
func (s *SocketManager) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := s.connectAndListen(ctx); err != nil {
				s.logger.Warn("WebSocket connection failed, retrying...", log.Err(err))

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
				}
			}
		}
	}
}

func (s *SocketManager) connectAndListen(ctx context.Context) error {
	u, err := url.Parse(s.url)
	if err != nil {
		return err
	}

	if u.Path == "" || u.Path == "/" {
		u.Path = "/ws"
	}

	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}

	s.logger.Debug("Connecting to Mannco WebSocket...", log.String("url", u.String()))

	var mods []aoni.RequestModifier
	if s.userAgent != "" {
		mods = append(mods, mod.WithUserAgent(s.userAgent))
	}

	wsConn, resp, err := ws.DialWebSocket(ctx, s.r, u.String(), mods...)
	if err != nil {
		return err
	}

	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	s.mu.Lock()
	s.conn = wsConn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if s.conn != nil {
			_ = s.conn.Close()
			s.conn = nil
		}
		s.mu.Unlock()
	}()

	for {
		_, p, err := wsConn.ReadMessage()
		if err != nil {
			return err
		}

		if len(p) == 0 {
			continue
		}

		s.handleMessage(p)
	}
}

func (s *SocketManager) handleMessage(payload []byte) {
	var ev WSEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		s.logger.Warn("Failed to unmarshal market stream event from socket", log.Err(err))
		return
	}

	s.mu.Lock()
	onEvent := s.onEvent
	onPriceChanged := s.onPriceChanged
	onListingAdded := s.onListingAdded
	onListingRemoved := s.onListingRemoved
	onBuyOrderAdded := s.onBuyOrderAdded
	onBuyOrderRemoved := s.onBuyOrderRemoved
	onBuyOrderUpdated := s.onBuyOrderUpdated
	onBuyOrderActivated := s.onBuyOrderActivated
	onBuyOrderDeactivated := s.onBuyOrderDeactivated
	s.mu.Unlock()

	if onEvent != nil {
		onEvent(&ev)
	}

	switch ev.Event {
	case EventPriceChanged:
		if onPriceChanged != nil {
			var data PriceChangedData
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				s.logger.Warn("Failed to unmarshal price_changed data", log.Err(err))
				return
			}

			onPriceChanged(&PriceChangedEvent{Event: ev.Event, TS: ev.TS, Data: data})
		}

	case EventListingAdded:
		if onListingAdded != nil {
			var data ListingAddedData
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				s.logger.Warn("Failed to unmarshal listing_added data", log.Err(err))
				return
			}

			onListingAdded(&ListingAddedEvent{Event: ev.Event, TS: ev.TS, Data: data})
		}

	case EventListingRemoved:
		if onListingRemoved != nil {
			var data ListingRemovedData
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				s.logger.Warn("Failed to unmarshal listing_removed data", log.Err(err))
				return
			}

			onListingRemoved(&ListingRemovedEvent{Event: ev.Event, TS: ev.TS, Data: data})
		}

	case EventBuyOrderAdded:
		if onBuyOrderAdded != nil {
			var data BuyOrderAddedData
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				s.logger.Warn("Failed to unmarshal buyorder_added data", log.Err(err))
				return
			}

			onBuyOrderAdded(&BuyOrderAddedEvent{Event: ev.Event, TS: ev.TS, Data: data})
		}

	case EventBuyOrderRemoved:
		if onBuyOrderRemoved != nil {
			var data BuyOrderRemovedData
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				s.logger.Warn("Failed to unmarshal buyorder_removed data", log.Err(err))
				return
			}

			onBuyOrderRemoved(&BuyOrderRemovedEvent{Event: ev.Event, TS: ev.TS, Data: data})
		}

	case EventBuyOrderUpdated:
		if onBuyOrderUpdated != nil {
			var data BuyOrderUpdatedData
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				s.logger.Warn("Failed to unmarshal buyorder_updated data", log.Err(err))
				return
			}

			onBuyOrderUpdated(&BuyOrderUpdatedEvent{Event: ev.Event, TS: ev.TS, Data: data})
		}

	case EventBuyOrderActivated:
		if onBuyOrderActivated != nil {
			var data BuyOrderActivatedData
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				s.logger.Warn("Failed to unmarshal buyorder_activated data", log.Err(err))
				return
			}

			onBuyOrderActivated(&BuyOrderActivatedEvent{Event: ev.Event, TS: ev.TS, Data: data})
		}

	case EventBuyOrderDeactivated:
		if onBuyOrderDeactivated != nil {
			var data BuyOrderDeactivatedData
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				s.logger.Warn("Failed to unmarshal buyorder_deactivated data", log.Err(err))
				return
			}

			onBuyOrderDeactivated(&BuyOrderDeactivatedEvent{Event: ev.Event, TS: ev.TS, Data: data})
		}
	}
}
