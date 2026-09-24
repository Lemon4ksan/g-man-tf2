// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"context"
	"errors"
	"fmt"
	"time"

	log "github.com/lemon4ksan/foundation/async/logkit"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

var (
	ErrNotEnoughChange    = errors.New("tf2econ: not enough pure metal to make exact change")
	ErrNotEnoughKeys      = errors.New("tf2econ: not enough keys in inventory")
	ErrNoDuplicateWeapons = errors.New("crafting: no duplicate weapons found to smelt")
)

// AssetFetcher provides inventory queries for pure currencies and craftable weapons.
type AssetFetcher interface {
	GetAssetIDs(sku string) []uint64
	GetPureStock() currency.PureStock
	FindWeaponsByClassForSmelting(class string) []*tf2.Item
	GetMetalCount(defIndex uint32) int
}

// MetalManager manages metal currency selection, change calculation, and smelting.
type MetalManager struct {
	fetcher AssetFetcher
	logger  log.Logger
	craft   *Manager
}

// NewMetalManager constructs a new MetalManager instance.
func NewMetalManager(fetcher AssetFetcher, craft *Manager, logger log.Logger) *MetalManager {
	return &MetalManager{fetcher: fetcher, craft: craft, logger: logger}
}

// SelectMetal selects metal asset IDs satisfying `needed` scrap value.
// It uses bidirectional sweep to pick change. If change cannot be satisfied from stock,
// it breaks down higher metal denominations using MakeChange without over-smelting.
//
// Parity: matches @tf2autobot/tf2 (classes/Crafting.js: getRequired).
func (m *MetalManager) SelectMetal(ctx context.Context, needed currency.Scrap) ([]uint64, error) {
	if needed <= 0 {
		return nil, nil
	}

	selected, remaining := m.bidirectionalSelect(int(needed))
	if remaining == 0 {
		return selected, nil
	}

	if m.craft != nil {
		if err := m.craft.MakeChange(ctx, DefIndexScrap, remaining); err != nil {
			return nil, err
		}

		selected, remaining = m.bidirectionalSelect(int(needed))
		if remaining == 0 {
			return selected, nil
		}
	} else if err := m.TryToSmeltForChange(ctx, needed); err == nil {
		selected, remaining = m.bidirectionalSelect(int(needed))
		if remaining == 0 {
			return selected, nil
		}
	}

	if remaining != 0 {
		return nil, fmt.Errorf("not enough metal: missing %d scrap", remaining)
	}

	return selected, nil
}

// SelectChange selects exact change in metal without modifying inventory.
func (m *MetalManager) SelectChange(amount currency.Scrap) ([]uint64, error) {
	selected, remaining := m.bidirectionalSelect(int(amount))
	if remaining != 0 {
		return nil, ErrNotEnoughChange
	}

	return selected, nil
}

// SelectKeysAndMetal selects keys and exact metal change required for a trade offer.
func (m *MetalManager) SelectKeysAndMetal(keys int, metal currency.Scrap) ([]uint64, error) {
	var selected []uint64

	if keys > 0 {
		availableKeys := m.fetcher.GetAssetIDs(currency.SKUKey)
		if len(availableKeys) < keys {
			return nil, ErrNotEnoughKeys
		}

		selected = append(selected, availableKeys[:keys]...)
	}

	if metal > 0 {
		metalIDs, err := m.SelectChange(metal)
		if err != nil {
			return nil, err
		}

		selected = append(selected, metalIDs...)
	}

	return selected, nil
}

// bidirectionalSelect selects metal asset IDs satisfying `needed` scrap value
// using a 3-pass sweep matching @tf2autobot UserCart.ts (getRequired parity):
// Pass 1: Forward sweep (Ref -> Rec -> Scrap) picking floor(remaining / value).
// Pass 2: Reverse sweep (Scrap -> Rec -> Ref) picking ceil(remaining / value) on higher tier.
// Pass 3: Deduction pass (Ref -> Rec -> Scrap) pruning redundant smaller denominations when remaining < 0.
func (m *MetalManager) bidirectionalSelect(needed int) (selected []uint64, remaining int) {
	if needed <= 0 {
		return nil, 0
	}

	tiers := []struct {
		sku   string
		value int
		items []uint64
	}{
		{sku: currency.SKURefined, value: 9, items: m.fetcher.GetAssetIDs(currency.SKURefined)},
		{sku: currency.SKUReclaimed, value: 3, items: m.fetcher.GetAssetIDs(currency.SKUReclaimed)},
		{sku: currency.SKUScrap, value: 1, items: m.fetcher.GetAssetIDs(currency.SKUScrap)},
	}

	picked := make([]int, len(tiers))
	remaining = needed
	hasReversed := false
	reverse := false
	index := 0

	for {
		val := tiers[index].value
		avail := len(tiers[index].items)

		amount := min(remaining/val, avail)

		if index == len(tiers)-1 {
			if hasReversed {
				break
			}

			reverse = true
		}

		currAmount := picked[index]
		if reverse && amount > 0 {
			ceilAmount := (remaining + val - 1) / val
			if currAmount+ceilAmount > avail {
				amount = avail - currAmount
			} else {
				amount = ceilAmount
			}
		}

		if amount >= 1 {
			picked[index] = currAmount + amount
			remaining -= amount * val
		}

		if remaining == 0 || remaining < 0 {
			break
		}

		if index == 0 && reverse {
			hasReversed = true
			reverse = false
		}

		if reverse {
			index--
		} else {
			index++
		}
	}

	if remaining < 0 {
		for i := range tiers {
			val := tiers[i].value
			amount := min(picked[i], (-remaining)/val)

			if amount >= 1 {
				remaining += amount * val
				picked[i] -= amount
			}
		}
	}

	for i := range tiers {
		for k := 0; k < picked[i]; k++ {
			selected = append(selected, tiers[i].items[k])
		}
	}

	return selected, remaining
}

// TryToSmeltForChange breaks down metal or duplicate craftable weapons to resolve change problems.
// Parity: matches @tf2autobot/tf2 duplicate weapon smelting and change breakdown.
func (m *MetalManager) TryToSmeltForChange(ctx context.Context, needed currency.Scrap) error {
	stock := m.fetcher.GetPureStock()

	var remAfterMetal int
	if stock.TotalScrap() >= needed {
		_, remaining := m.bidirectionalSelect(int(needed))
		if remaining == 0 {
			return nil
		}

		m.logger.Info("Attempting to break metal for exact change",
			log.Int("needed_scrap", remaining),
			log.Int("total_requested", int(needed)),
		)

		if m.craft != nil {
			if err := m.craft.MakeChange(ctx, DefIndexScrap, remaining); err != nil {
				return fmt.Errorf("tf2econ: smelting failed: %w", err)
			}

			_, remAfterMetal = m.bidirectionalSelect(int(needed))
			if remAfterMetal == 0 {
				return nil
			}
		}
	}

	// Parity: evaluate duplicate weapons when pure metal is insufficient or cannot satisfy change
	missing := needed - stock.TotalScrap()
	if missing <= 0 {
		missing = 1
	}

	m.logger.Info("Checking duplicate weapons for change...", log.Int("needed_scrap", int(missing)))

	if err := m.SmeltDuplicates(ctx, missing); err == nil {
		if _, afterWeapons := m.bidirectionalSelect(int(needed)); afterWeapons == 0 {
			return nil
		}
	}

	if stock.TotalScrap() < needed {
		return fmt.Errorf("tf2econ: insufficient total metal value (have %d, need %d)", stock.TotalScrap(), needed)
	}

	finalRem := remAfterMetal
	if finalRem == 0 {
		finalRem = int(needed)
	}

	return fmt.Errorf("tf2econ: smelting didn't resolve the change problem, missing %d scrap", finalRem)
}

// SmeltDuplicates smelts duplicate craftable weapons across all TF2 character classes
// into scrap metal to satisfy a required scrap deficit.
//
// It iterates sequentially through standard TF2 classes (Scout through Spy), finding
// pairs of craftable, tradable duplicate weapons via the AssetFetcher and executing
// TF2 Recipe 3 (RecipeSmeltWeapons) via the Game Coordinator crafting client. Smelting
// terminates early once accumulated scrap reaches or exceeds the requested 'needed' amount.
//
// Parameters:
//   - ctx: Context for cancellation and deadline control across Game Coordinator craft calls.
//   - needed: Target quantity of Scrap metal to generate from duplicate weapons.
//
// Returns:
//   - nil if at least 'needed' scrap was successfully smelted.
//   - ErrNoDuplicateWeapons if no duplicate craftable weapon pairs were found across any class (smelted == 0).
//   - An error if weapon pair validation fails (e.g., untradable weapons), the crafting client
//     is unconfigured, or a Game Coordinator craft request fails.
//
// Invariants:
//   - Only tradable weapons are smelted to prevent generating untradable scrap metal.
//   - Introduces a 500ms rate-limit pause between consecutive class craft operations.
//
// Parity: matches @tf2autobot/tf2 duplicate weapon smelting for trade change resolution.
func (m *MetalManager) SmeltDuplicates(ctx context.Context, needed currency.Scrap) error {
	smelted := 0

	for _, class := range schema.Classes {
		count, err := m.smeltClassDuplicatesForChange(ctx, class, needed-currency.Scrap(smelted))
		smelted += count

		if err != nil {
			return err
		}

		if currency.Scrap(smelted) >= needed {
			return nil
		}
	}

	if smelted == 0 {
		return ErrNoDuplicateWeapons
	}

	return nil
}

func (m *MetalManager) smeltClassDuplicatesForChange(
	ctx context.Context,
	class string,
	needed currency.Scrap,
) (int, error) {
	smelted := 0

	for {
		weapons := m.fetcher.FindWeaponsByClassForSmelting(class)
		if len(weapons) < 2 {
			return smelted, nil
		}

		if !weapons[0].IsTradable || !weapons[1].IsTradable {
			return smelted, fmt.Errorf(
				"refusing to smelt: weapons must be tradable (IDs: %d, %d)",
				weapons[0].ID,
				weapons[1].ID,
			)
		}

		if m.craft == nil {
			return smelted, errors.New("tf2econ: crafting client not configured")
		}

		m.logger.Info("Smelting duplicate weapons for change", log.String("class", class))

		if _, err := m.craft.SmeltWeapons(ctx, weapons[0].ID, weapons[1].ID); err != nil {
			return smelted, err
		}

		smelted++
		if currency.Scrap(smelted) >= needed {
			return smelted, nil
		}

		time.Sleep(500 * time.Millisecond)
	}
}
