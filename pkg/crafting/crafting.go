// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package crafting automates metal condensing, weapon smelting, and trade change balancing.
package crafting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

const (
	DefIndexScrap     uint32 = 5000
	DefIndexReclaimed uint32 = 5001
	DefIndexRefined   uint32 = 5002
)

const (
	RecipeSmeltWeapons       int16 = 3
	RecipeCombineScrap       int16 = 4
	RecipeCombineReclaimed   int16 = 5
	RecipeFabricateToken     int16 = 6
	RecipeFabricateSlotToken int16 = 7
	RecipeRebuildHeadgear    int16 = 8
	RecipeSmeltReclaimed     int16 = 22
	RecipeSmeltRefined       int16 = 23
	RecipeCustomDynamic      int16 = 200
)

var (
	// ErrNoRefinedToSmelt indicates no refined metal exists to perform lower-denomination change smelting.
	ErrNoRefinedToSmelt = errors.New("crafting: no refined metal left to smelt")
	// ErrInvalidSmeltType indicates a non-smeltable metal defindex was provided.
	ErrInvalidSmeltType = errors.New("crafting: cannot smelt this item type")
)

type InventoryProvider interface {
	FindCraftableItems(defIndex uint32, count int) []uint64
	FindWeaponsByClassForSmelting(class string) []*tf2.Item
	GetMetalCount(defIndex uint32) int
}

type GCProvider interface {
	Craft(ctx context.Context, items []uint64, recipe int16) ([]uint64, error)
}

// Manager executes TF2 Game Coordinator crafting recipes.
type Manager struct {
	inv InventoryProvider
	gc  GCProvider
}

func NewManager(inv InventoryProvider, gc GCProvider) *Manager {
	return &Manager{inv: inv, gc: gc}
}

func (cm *Manager) CombineMetal(ctx context.Context, metalDefIndex uint32) ([]uint64, error) {
	items := cm.inv.FindCraftableItems(metalDefIndex, 3)
	if len(items) < 3 {
		return nil, fmt.Errorf("craft: not enough metal with defindex %d (need 3, got %d)", metalDefIndex, len(items))
	}

	var recipe int16
	switch metalDefIndex {
	case DefIndexScrap:
		recipe = RecipeCombineScrap
	case DefIndexReclaimed:
		recipe = RecipeCombineReclaimed
	default:
		return nil, fmt.Errorf("craft: invalid metal defindex for combination: %d", metalDefIndex)
	}

	return cm.gc.Craft(ctx, items, recipe)
}

func (cm *Manager) SmeltMetal(ctx context.Context, metalDefIndex uint32) ([]uint64, error) {
	items := cm.inv.FindCraftableItems(metalDefIndex, 1)
	if len(items) == 0 {
		return nil, fmt.Errorf("craft: no metal found with defindex %d", metalDefIndex)
	}

	var recipe int16
	switch metalDefIndex {
	case DefIndexReclaimed:
		recipe = RecipeSmeltReclaimed
	case DefIndexRefined:
		recipe = RecipeSmeltRefined
	default:
		return nil, fmt.Errorf("craft: invalid metal defindex for smelting: %d", metalDefIndex)
	}

	return cm.gc.Craft(ctx, items, recipe)
}

func (cm *Manager) SmeltWeapons(ctx context.Context, weaponID1, weaponID2 uint64) ([]uint64, error) {
	return cm.gc.Craft(ctx, []uint64{weaponID1, weaponID2}, RecipeSmeltWeapons)
}

func (cm *Manager) CondenseMetal(ctx context.Context) (int, error) {
	crafts := 0

	for cm.inv.GetMetalCount(DefIndexScrap) >= 3 {
		if _, err := cm.CombineMetal(ctx, DefIndexScrap); err != nil {
			return crafts, fmt.Errorf("condense scrap failed after %d crafts: %w", crafts, err)
		}

		crafts++

		time.Sleep(300 * time.Millisecond)
	}

	for cm.inv.GetMetalCount(DefIndexReclaimed) >= 3 {
		if _, err := cm.CombineMetal(ctx, DefIndexReclaimed); err != nil {
			return crafts, fmt.Errorf("condense reclaimed failed after %d crafts: %w", crafts, err)
		}

		crafts++

		time.Sleep(300 * time.Millisecond)
	}

	return crafts, nil
}

func (cm *Manager) MakeChange(ctx context.Context, targetDefIndex uint32, targetCount int) error {
	for cm.inv.GetMetalCount(targetDefIndex) < targetCount {
		if err := cm.smeltSingleMetalStep(ctx, targetDefIndex); err != nil {
			return err
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

func (cm *Manager) smeltSingleMetalStep(ctx context.Context, targetDefIndex uint32) error {
	switch targetDefIndex {
	case DefIndexScrap:
		if cm.inv.GetMetalCount(DefIndexReclaimed) > 0 {
			_, err := cm.SmeltMetal(ctx, DefIndexReclaimed)
			return err
		}

		return cm.MakeChange(ctx, DefIndexReclaimed, 1)

	case DefIndexReclaimed:
		if cm.inv.GetMetalCount(DefIndexRefined) > 0 {
			_, err := cm.SmeltMetal(ctx, DefIndexRefined)
			return err
		}

		return ErrNoRefinedToSmelt

	default:
		return ErrInvalidSmeltType
	}
}

func (cm *Manager) SmeltClassWeapons(ctx context.Context, class string) ([]uint64, error) {
	weapons := cm.inv.FindWeaponsByClassForSmelting(class)
	if len(weapons) < 2 {
		return nil, fmt.Errorf("not enough weapons for class %s", class)
	}

	if !weapons[0].IsTradable || !weapons[1].IsTradable {
		return nil, fmt.Errorf(
			"refusing to smelt: weapons must be tradable (IDs: %d, %d)",
			weapons[0].ID,
			weapons[1].ID,
		)
	}

	return cm.gc.Craft(ctx, []uint64{weapons[0].ID, weapons[1].ID}, RecipeSmeltWeapons)
}
