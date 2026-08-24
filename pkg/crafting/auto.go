// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"context"
	"time"

	"github.com/lemon4ksan/foundation/async/log"
	"github.com/lemon4ksan/foundation/generic"
	"github.com/lemon4ksan/g-man/pkg/behavior"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

const BehaviorName = "pure_liquidator"

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

func WithLogger(l log.Logger) Option {
	return func(a *Automator) { a.logger = l }
}

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

func (a *Automator) Name() string { return BehaviorName }

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

func (a *Automator) Tick(ctx context.Context) error {
	scrapCount := a.inv.GetMetalCount(DefIndexScrap)
	refCount := a.inv.GetMetalCount(DefIndexRefined)
	recCount := a.inv.GetMetalCount(DefIndexReclaimed)

	switch {
	case scrapCount < a.minScrap && recCount > 0:
		a.logger.Info("Scrap supply low, smelting Reclaimed")
		_, err := a.manager.SmeltMetal(ctx, DefIndexReclaimed)

		return err

	case recCount < a.minRec && refCount > 0:
		a.logger.Info("Reclaimed supply low, smelting Refined")
		_, err := a.manager.SmeltMetal(ctx, DefIndexRefined)

		return err

	case scrapCount > a.maxScrap:
		a.logger.Info("Too much Scrap, combining into Reclaimed")
		_, err := a.manager.CombineMetal(ctx, DefIndexScrap)

		return err

	case recCount > a.maxRec:
		a.logger.Info("Too much Reclaimed, combining into Refined")
		_, err := a.manager.CombineMetal(ctx, DefIndexReclaimed)

		return err
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
