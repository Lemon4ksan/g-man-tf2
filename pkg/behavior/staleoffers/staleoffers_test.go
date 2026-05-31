// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package staleoffers

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/stretchr/testify/assert"

	tf2trading "github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

type mockTrading struct {
	mu        sync.Mutex
	offers    []trading.TradeOffer
	cancelled []uint64
}

func (m *mockTrading) GetActiveSentOffers(ctx context.Context) ([]trading.TradeOffer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.offers, nil
}

func (m *mockTrading) CancelOffer(ctx context.Context, offerID uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cancelled = append(m.cancelled, offerID)

	return nil
}

type mockConfig struct {
	cfg tf2trading.Config
}

func (m *mockConfig) GetConfig() tf2trading.Config {
	return m.cfg
}

func TestCanceller_StaleOffersAutoCancel(t *testing.T) {
	t.Run("Does nothing if auto-cancel is disabled", func(t *testing.T) {
		t.Parallel()

		logger := log.New(log.DefaultConfig(log.LevelDebug))
		eventBus := bus.New()

		offers := []trading.TradeOffer{
			{
				ID:          1,
				IsOurOffer:  true,
				State:       trading.OfferStateActive,
				TimeCreated: time.Now().Add(-30 * time.Minute).Unix(),
			},
		}

		tr := &mockTrading{offers: offers}
		cfg := &mockConfig{
			cfg: tf2trading.Config{
				EnableAutoCancelStaleOffers: false,
				CancelStaleOffersAfter:      "15m",
			},
		}

		canceller := New(tr, cfg, eventBus, logger, DefaultConfig())
		canceller.checkStaleOffers(context.Background())

		assert.Empty(t, tr.cancelled)
	})

	t.Run("Cancels stale sent offers when enabled", func(t *testing.T) {
		t.Parallel()

		logger := log.New(log.DefaultConfig(log.LevelDebug))
		eventBus := bus.New()

		now := time.Now()
		offers := []trading.TradeOffer{
			// 1. Sent by us, active, pending for 30m (> 15m) -> should be cancelled
			{
				ID:          100,
				IsOurOffer:  true,
				State:       trading.OfferStateActive,
				TimeCreated: now.Add(-30 * time.Minute).Unix(),
			},
			// 2. Sent by us, active, pending for 5m (< 15m) -> should NOT be cancelled
			{
				ID:          101,
				IsOurOffer:  true,
				State:       trading.OfferStateActive,
				TimeCreated: now.Add(-5 * time.Minute).Unix(),
			},
			// 3. Sent by us, but NOT active (already accepted) -> should NOT be cancelled
			{
				ID:          102,
				IsOurOffer:  true,
				State:       trading.OfferStateAccepted,
				TimeCreated: now.Add(-30 * time.Minute).Unix(),
			},
			// 4. Sent by partner (IsOurOffer == false) -> should NOT be cancelled
			{
				ID:          103,
				IsOurOffer:  false,
				State:       trading.OfferStateActive,
				TimeCreated: now.Add(-30 * time.Minute).Unix(),
			},
		}

		tr := &mockTrading{offers: offers}
		cfg := &mockConfig{
			cfg: tf2trading.Config{
				EnableAutoCancelStaleOffers: true,
				CancelStaleOffersAfter:      "15m",
			},
		}

		canceller := New(tr, cfg, eventBus, logger, DefaultConfig())
		canceller.checkStaleOffers(context.Background())

		assert.ElementsMatch(t, []uint64{100}, tr.cancelled)
	})

	t.Run("Uses fallback 15m duration if parsing duration fails", func(t *testing.T) {
		t.Parallel()

		logger := log.New(log.DefaultConfig(log.LevelDebug))
		eventBus := bus.New()

		now := time.Now()
		offers := []trading.TradeOffer{
			{
				ID:          200,
				IsOurOffer:  true,
				State:       trading.OfferStateActive,
				TimeCreated: now.Add(-20 * time.Minute).Unix(), // > 15m fallback
			},
			{
				ID:          201,
				IsOurOffer:  true,
				State:       trading.OfferStateActive,
				TimeCreated: now.Add(-5 * time.Minute).Unix(), // < 15m fallback
			},
		}

		tr := &mockTrading{offers: offers}
		cfg := &mockConfig{
			cfg: tf2trading.Config{
				EnableAutoCancelStaleOffers: true,
				CancelStaleOffersAfter:      "invalid_duration",
			},
		}

		canceller := New(tr, cfg, eventBus, logger, DefaultConfig())
		canceller.checkStaleOffers(context.Background())

		assert.ElementsMatch(t, []uint64{200}, tr.cancelled)
	})
}
