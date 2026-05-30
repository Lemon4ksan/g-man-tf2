// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// DefIndexScrap represents the item definition index for Scrap Metal (5000).
const DefIndexScrap uint32 = 5000

// DefIndexReclaimed represents the item definition index for Reclaimed Metal (5001).
const DefIndexReclaimed uint32 = 5001

// DefIndexRefined represents the item definition index for Refined Metal (5002).
const DefIndexRefined uint32 = 5002

// RecipeSmeltWeapons is the blueprint ID to craft 2 weapons of the same class into 1 Scrap.
const RecipeSmeltWeapons int16 = 3

// RecipeCombineScrap is the blueprint ID to combine 3 Scrap into 1 Reclaimed.
const RecipeCombineScrap int16 = 4

// RecipeCombineReclaimed is the blueprint ID to combine 3 Reclaimed into 1 Refined.
const RecipeCombineReclaimed int16 = 5

// RecipeSmeltReclaimed is the blueprint ID to smelt 1 Reclaimed into 3 Scrap.
const RecipeSmeltReclaimed int16 = 22

// RecipeSmeltRefined is the blueprint ID to smelt 1 Refined into 3 Reclaimed.
const RecipeSmeltRefined int16 = 23

// RecipeRebuildHeadgear is the blueprint ID to craft 2 hats into 1 random hat.
const RecipeRebuildHeadgear int16 = 8

// RecipeFabricateToken is the blueprint ID to craft 3 weapons into 1 class token.
const RecipeFabricateToken int16 = 6

// RecipeFabricateSlotToken is the blueprint ID to craft 3 weapons into 1 slot token.
const RecipeFabricateSlotToken int16 = 7

// RecipeCustomDynamic is the blueprint ID used for Killstreak Fabricators and Chemistry Sets.
const RecipeCustomDynamic int16 = 200

// InventoryProvider defines the inventory queries required for craft decision making.
type InventoryProvider interface {
	// FindCraftableItems returns available item IDs matching the defindex up to the specified count.
	FindCraftableItems(defIndex uint32, count int) []uint64
	// FindWeaponsByClassForSmelting returns duplicate weapons eligible for smelting.
	FindWeaponsByClassForSmelting(class string) []*tf2.Item
	// GetMetalCount returns the total count of metal items matching the specified DefIndex.
	GetMetalCount(defIndex uint32) int
}

// GCProvider defines the interface to interact with the Game Coordinator craft command.
type GCProvider interface {
	// Craft sends a craft instruction to the Game Coordinator and returns the created item IDs.
	Craft(ctx context.Context, items []uint64, recipe int16) ([]uint64, error)
}

// Manager orchestrates standard Team Fortress 2 crafting recipe requests.
type Manager struct {
	inv InventoryProvider
	gc  GCProvider
}

// NewManager constructs a new [Manager] instance.
func NewManager(inv InventoryProvider, gc GCProvider) *Manager {
	return &Manager{
		inv: inv,
		gc:  gc,
	}
}

// CombineMetal converts 3 units of low-grade metal into 1 high-grade metal.
// Supported defindexes: [DefIndexScrap] (converts to Reclaimed) and [DefIndexReclaimed] (converts to Refined).
// Returns the newly created metal ID, or an error if there is insufficient metal or if the recipe is invalid.
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

// SmeltMetal splits 1 unit of high-grade metal into 3 units of low-grade metal.
// Supported defindexes: [DefIndexReclaimed] (smelts to Scrap) and [DefIndexRefined] (smelts to Reclaimed).
// Returns the newly created metal IDs, or an error if the source metal is missing or if the recipe is invalid.
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

// SmeltWeapons crafts two weapons of the same class into one Scrap Metal.
// Returns the newly created scrap metal ID, or an error if the craft request fails.
func (cm *Manager) SmeltWeapons(ctx context.Context, weaponID1, weaponID2 uint64) ([]uint64, error) {
	return cm.gc.Craft(ctx, []uint64{weaponID1, weaponID2}, RecipeSmeltWeapons)
}

// CondenseMetal compresses all available scrap and reclaimed metal into higher-grade counterparts.
// It repeatedly combines units in batches of 3.
// Returns the total number of successful craft operations executed, or an error if an intermediate craft fails.
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

// MakeChange smelts higher-grade metal into lower-grade units until the target metal count is reached.
// If [DefIndexReclaimed] is requested, it smelts Refined metal.
// If [DefIndexScrap] is requested, it recursively smelts Reclaimed metal (and Refined if needed).
// Returns an error if there is no high-grade metal left to smelt.
func (cm *Manager) MakeChange(ctx context.Context, targetDefIndex uint32, targetCount int) error {
	for cm.inv.GetMetalCount(targetDefIndex) < targetCount {
		switch targetDefIndex {
		case DefIndexScrap:
			if cm.inv.GetMetalCount(DefIndexReclaimed) > 0 {
				if _, err := cm.SmeltMetal(ctx, DefIndexReclaimed); err != nil {
					return err
				}
			} else {
				if err := cm.MakeChange(ctx, DefIndexReclaimed, 1); err != nil {
					return err
				}
			}

		case DefIndexReclaimed:
			if cm.inv.GetMetalCount(DefIndexRefined) > 0 {
				if _, err := cm.SmeltMetal(ctx, DefIndexRefined); err != nil {
					return err
				}
			} else {
				return errors.New("make_change: no refined metal left to smelt")
			}

		default:
			return errors.New("make_change: cannot smelt this item type")
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// SmeltClassWeapons finds two unique duplicate weapons for the specified class and smelts them into scrap metal.
// Returns the newly created scrap ID, or an error if weapons are missing or if non-tradable items are selected.
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

	itemsToCraft := []uint64{weapons[0].ID, weapons[1].ID}

	return cm.gc.Craft(ctx, itemsToCraft, RecipeSmeltWeapons)
}
