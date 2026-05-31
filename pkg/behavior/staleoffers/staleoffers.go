// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package staleoffers provides automatic cancellation of outbound trade offers
// that have remained active/pending for longer than a specified duration.
package staleoffers

import (
	"context"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/trading"

	tf2trading "github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

// Config holds configuration parameters for the Stale Offers behavior.
type Config struct {
	// CheckInterval defines how frequently the behavior scans for stale sent offers.
	CheckInterval time.Duration `json:"check_interval"`
}

// DefaultConfig returns a [Config] containing production-ready default values.
func DefaultConfig() Config {
	return Config{
		CheckInterval: 2 * time.Minute,
	}
}

// TradingProvider defines the interface to fetch sent active offers and cancel them.
type TradingProvider interface {
	GetActiveSentOffers(ctx context.Context) ([]trading.TradeOffer, error)
	CancelOffer(ctx context.Context, offerID uint64) error
}

// ConfigProvider defines the interface to fetch the current trading configurations.
type ConfigProvider interface {
	GetConfig() tf2trading.Config
}

// Canceller periodically checks for and cancels sent trade offers that have been pending for too long.
type Canceller struct {
	config Config
	logger log.Logger
	bus    *bus.Bus

	trading TradingProvider
	cfgMgr  ConfigProvider
}

// Monitor returns a [behavior.Option] that registers the [Canceller] with the orchestrator.
func Monitor(
	trading TradingProvider,
	cfgMgr ConfigProvider,
	cfg Config,
) behavior.Option {
	return func(o *behavior.Orchestrator) {
		o.Register(New(trading, cfgMgr, o.Bus(), o.Logger(), cfg))
	}
}

// New creates a new [Canceller] behavior with the specified dependencies.
func New(
	trading TradingProvider,
	cfgMgr ConfigProvider,
	bus *bus.Bus,
	logger log.Logger,
	cfg Config,
) *Canceller {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = DefaultConfig().CheckInterval
	}

	return &Canceller{
		config:  cfg,
		logger:  logger.With(log.String("module", "stale_offers_canceller")),
		bus:     bus,
		trading: trading,
		cfgMgr:  cfgMgr,
	}
}

// Name returns the unique name of the behavior.
func (c *Canceller) Name() string {
	return "stale_offers"
}

// Run starts the background check loop.
// It blocks until the context is cancelled.
func (c *Canceller) Run(ctx context.Context) error {
	c.logger.Info("Stale offers canceller background loop started", log.Duration("interval", c.config.CheckInterval))

	ticker := time.NewTicker(c.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			c.checkStaleOffers(ctx)
		}
	}
}

func (c *Canceller) checkStaleOffers(ctx context.Context) {
	tradeCfg := c.cfgMgr.GetConfig()
	if !tradeCfg.EnableAutoCancelStaleOffers {
		return
	}

	limit, err := time.ParseDuration(tradeCfg.CancelStaleOffersAfter)
	if err != nil {
		c.logger.Error(
			"Failed to parse cancel_stale_offers_after duration, using fallback 15m",
			log.String("value", tradeCfg.CancelStaleOffersAfter),
			log.Err(err),
		)

		limit = 15 * time.Minute
	}

	c.logger.Debug("Checking for stale sent trade offers", log.Duration("limit", limit))

	offers, err := c.trading.GetActiveSentOffers(ctx)
	if err != nil {
		c.logger.Error("Failed to retrieve active sent offers", log.Err(err))
		return
	}

	now := time.Now()
	for _, off := range offers {
		if !off.IsOurOffer || !off.IsActive() {
			continue
		}

		created := off.CreatedAt()

		age := now.Sub(created)
		if age > limit {
			c.logger.Info(
				"Automatically cancelling stale sent trade offer",
				log.Uint64("offer_id", off.ID),
				log.Duration("pending_for", age),
				log.Duration("limit", limit),
			)

			if err := c.trading.CancelOffer(ctx, off.ID); err != nil {
				c.logger.Error("Failed to cancel stale sent trade offer", log.Uint64("offer_id", off.ID), log.Err(err))
			} else {
				c.logger.Info("Successfully cancelled stale sent trade offer", log.Uint64("offer_id", off.ID))
			}
		}
	}
}
