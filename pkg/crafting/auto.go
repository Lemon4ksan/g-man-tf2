// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"context"
	"time"

	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/foundation/generic"
	"github.com/lemon4ksan/g-man/pkg/behavior"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

const BehaviorName = "pure_liquidator"

// WithPureLiquidator registers the pure supply balancing Automator into a behavior Orchestrator.
func WithPureLiquidator(orch *behavior.Orchestrator, mgr *Manager, inv InventoryProvider) {
	orch.Register(NewAutomator(mgr, inv, WithLogger(orch.Logger())))
}

// Automator executes automated background metal balancing and duplicate weapon smelting.
type Automator struct {
	manager *Manager
	inv     InventoryProvider
	logger  log.Logger

	minScrap      int
	minRec        int
	maxScrap      int
	maxRec        int
	checkInterval time.Duration
}

type Option = generic.Option[*Automator]

// WithLogger sets the logger instance on the Automator.
func WithLogger(l log.Logger) Option {
	return func(a *Automator) { a.logger = l }
}

// NewAutomator constructs a new Automator instance with default threshold settings.
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

	generic.ApplyOptions(a, opts...)

	return a
}

// Name returns the registered behavior name of the Automator.
func (a *Automator) Name() string { return BehaviorName }

// Run starts the periodic metal balancing and weapon cleanup loop.
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
			a.performRoutine(ctx)
		}
	}
}

func (a *Automator) performRoutine(ctx context.Context) {
	if err := a.Tick(ctx); err != nil {
		a.logger.Error("Tick failed", log.Err(err))
	}

	if err := a.CleanInventory(ctx); err != nil {
		a.logger.Error("Clean failed", log.Err(err))
	}
}

// Tick performs batch pure supply balancing matching @tf2autobot keepMetalSupply:
// Simultaneously computes combineScrap, combineReclaimed, smeltRefined, and smeltReclaimed,
// executing batch balancing within a single cycle.
//
// Parity: matches @tf2autobot/tf2 (keepMetalSupply.ts).
func (a *Automator) Tick(ctx context.Context) error {
	scrapCount := a.inv.GetMetalCount(DefIndexScrap)
	refCount := a.inv.GetMetalCount(DefIndexRefined)
	recCount := a.inv.GetMetalCount(DefIndexReclaimed)

	// Parity: @tf2autobot/keepMetalSupply.ts:10 - do not craft if pure metal is depleted
	if refCount <= 0 && recCount <= 3 && scrapCount <= 3 {
		return nil
	}

	var combineScrap, combineRec, smeltRef, smeltRec int

	if recCount > a.maxRec {
		combineRec = (recCount - a.maxRec + 2) / 3
	} else if recCount < a.minRec {
		smeltRef = (a.minRec - recCount + 2) / 3
	}

	if scrapCount > a.maxScrap {
		combineScrap = (scrapCount - a.maxScrap + 2) / 3
	} else if scrapCount < a.minScrap {
		smeltRec = (a.minScrap - scrapCount + 2) / 3
	}

	for i := 0; i < combineScrap; i++ {
		if a.inv.GetMetalCount(DefIndexScrap) < 3 {
			break
		}

		a.logger.Info("Combining excess Scrap into Reclaimed", log.Int("step", i+1), log.Int("total", combineScrap))

		if _, err := a.manager.CombineMetal(ctx, DefIndexScrap); err != nil {
			return err
		}
	}

	for i := 0; i < combineRec; i++ {
		if a.inv.GetMetalCount(DefIndexReclaimed) < 3 {
			break
		}

		a.logger.Info("Combining excess Reclaimed into Refined", log.Int("step", i+1), log.Int("total", combineRec))

		if _, err := a.manager.CombineMetal(ctx, DefIndexReclaimed); err != nil {
			return err
		}
	}

	for i := 0; i < smeltRef; i++ {
		if a.inv.GetMetalCount(DefIndexRefined) < 1 {
			break
		}

		a.logger.Info("Smelting Refined into Reclaimed", log.Int("step", i+1), log.Int("total", smeltRef))

		if _, err := a.manager.SmeltMetal(ctx, DefIndexRefined); err != nil {
			return err
		}
	}

	for i := 0; i < smeltRec; i++ {
		if a.inv.GetMetalCount(DefIndexReclaimed) < 1 {
			break
		}

		a.logger.Info("Smelting Reclaimed into Scrap", log.Int("step", i+1), log.Int("total", smeltRec))

		if _, err := a.manager.SmeltMetal(ctx, DefIndexReclaimed); err != nil {
			return err
		}
	}

	return nil
}

func (a *Automator) CleanInventory(ctx context.Context) error {
	for _, class := range schema.Classes {
		a.smeltAllClassDuplicates(ctx, class)
	}

	_, err := a.manager.CondenseMetal(ctx)

	return err
}

func (a *Automator) smeltAllClassDuplicates(ctx context.Context, class string) {
	for {
		weapons := a.inv.FindWeaponsByClassForSmelting(class)
		if len(weapons) < 2 {
			return
		}

		a.logger.Info("Cleaning inventory: smelting class weapons", log.String("class", class))

		if _, err := a.manager.SmeltClassWeapons(ctx, class); err != nil {
			a.logger.Error("Failed to smelt class weapons", log.Err(err))
			return
		}

		time.Sleep(500 * time.Millisecond)
	}
}
