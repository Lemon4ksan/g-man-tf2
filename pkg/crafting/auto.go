// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"context"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
)

// BehaviorName is the unique identifier for the pure liquidator behavior.
const BehaviorName = "pure_liquidator"

// WithPureLiquidator returns an option that registers the pure liquidator behavior with the orchestrator.
func WithPureLiquidator(mgr *Manager, inv InventoryProvider) behavior.Option {
	return func(o *behavior.Orchestrator) {
		o.Register(NewAutomator(mgr, inv, WithLogger(o.Logger())))
	}
}

// Automator is a high-level background orchestrator that maintains metal reserves and handles duplicate weapon recrafting.
// It periodically checks metal counts and weapon duplicates to ensure slot optimization.
type Automator struct {
	manager *Manager
	inv     InventoryProvider
	logger  log.Logger

	minScrap int
	minRec   int
	maxScrap int
	maxRec   int

	checkInterval time.Duration
}

// Option defines functional configuration setters for the [Automator].
type Option = bus.Option[*Automator]

// WithLogger configures a custom [log.Logger] for the [Automator].
func WithLogger(l log.Logger) Option {
	return func(a *Automator) {
		a.logger = l
	}
}

// NewAutomator constructs a new [Automator] instance with default threshold values.
func NewAutomator(mgr *Manager, inv InventoryProvider, opts ...Option) *Automator {
	a := &Automator{
		manager:       mgr,
		inv:           inv,
		logger:        log.Discard,
		minScrap:      3,
		minRec:        3,
		maxScrap:      9,
		maxRec:        9,
		checkInterval: 30 * time.Minute,
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Name returns the unique behavior identifier for the [Automator].
func (a *Automator) Name() string {
	return "pure_liquidator"
}

// Run starts the background check loop, checking and cleaning the inventory at scheduled intervals.
// Returns an error if the context is cancelled.
func (a *Automator) Run(ctx context.Context) error {
	a.logger.Info("Pure Liquidator behavior started", log.Duration("interval", a.checkInterval))

	ticker := time.NewTicker(a.checkInterval)
	defer ticker.Stop()

	if err := a.Tick(ctx); err != nil {
		a.logger.Error("Initial tick failed", log.Err(err))
	}

	if err := a.CleanInventory(ctx); err != nil {
		a.logger.Error("Initial clean failed", log.Err(err))
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := a.Tick(ctx); err != nil {
				a.logger.Error("Tick failed", log.Err(err))
			}

			if err := a.CleanInventory(ctx); err != nil {
				a.logger.Error("Clean failed", log.Err(err))
			}
		}
	}
}

// Tick performs a single evaluation check of the current metal reserves.
// It smelts high-grade metal or combines low-grade metal if thresholds are exceeded.
// Returns an error if the Game Coordinator craft command fails.
func (a *Automator) Tick(ctx context.Context) error {
	scrapCount := a.inv.GetMetalCount(DefIndexScrap)
	refCount := a.inv.GetMetalCount(DefIndexRefined)
	recCount := a.inv.GetMetalCount(DefIndexReclaimed)

	if scrapCount < a.minScrap && recCount > 0 {
		a.logger.Info("Scrap supply low, smelting Reclaimed")
		_, err := a.manager.SmeltMetal(ctx, DefIndexReclaimed)

		return err
	}

	if recCount < a.minRec && refCount > 0 {
		a.logger.Info("Reclaimed supply low, smelting Refined")
		_, err := a.manager.SmeltMetal(ctx, DefIndexRefined)

		return err
	}

	if scrapCount > a.maxScrap {
		a.logger.Info("Too much Scrap, combining into Reclaimed")
		_, err := a.manager.CombineMetal(ctx, DefIndexScrap)

		return err
	}

	if recCount > a.maxRec {
		a.logger.Info("Too much Reclaimed, combining into Refined")
		_, err := a.manager.CombineMetal(ctx, DefIndexReclaimed)

		return err
	}

	return nil
}

// CleanInventory scans the backpack for duplicate class weapons and smelts them into scrap metal.
// It runs a condensing cycle after smelting is complete to compress newly generated scrap.
// Returns an error if weapon smelting or metal condensing fails.
func (a *Automator) CleanInventory(ctx context.Context) error {
	classes := []string{"Scout", "Soldier", "Pyro", "Demoman", "Heavy", "Engineer", "Medic", "Sniper", "Spy"}

	for _, class := range classes {
		for {
			weapons := a.inv.FindWeaponsByClassForSmelting(class)
			if len(weapons) < 2 {
				break
			}

			a.logger.Info("Cleaning inventory: smelting class weapons", log.String("class", class))

			_, err := a.manager.SmeltClassWeapons(ctx, class)
			if err != nil {
				a.logger.Error("Failed to smelt class weapons", log.Err(err))
				break
			}

			time.Sleep(500 * time.Millisecond)
		}
	}

	_, err := a.manager.CondenseMetal(ctx)

	return err
}
