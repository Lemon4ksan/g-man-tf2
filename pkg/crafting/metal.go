// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// ErrNotEnoughChange is returned when the inventory lacks the metal units required to make exact change.
var ErrNotEnoughChange = errors.New("tf2econ: not enough pure metal to make exact change")

// AssetFetcher defines the inventory retrieval methods needed for trade metal selection.
type AssetFetcher interface {
	// GetAssetIDs returns available item IDs matching the target SKU.
	GetAssetIDs(sku string) []uint64
	// GetPureStock returns the current total pure currency balances.
	GetPureStock() currency.PureStock
	// FindWeaponsByClassForSmelting returns duplicate weapons eligible for smelting.
	FindWeaponsByClassForSmelting(class string) []*tf2.Item
	// GetMetalCount returns the total count of metal items matching the specified DefIndex.
	GetMetalCount(defIndex uint32) int
}

// MetalManager manages greedy metal selection, change calculations, and manual change smelting.
type MetalManager struct {
	fetcher AssetFetcher
	logger  log.Logger
	craft   *Manager
}

// NewMetalManager constructs a new [MetalManager] instance.
func NewMetalManager(fetcher AssetFetcher, craft *Manager, logger log.Logger) *MetalManager {
	return &MetalManager{fetcher: fetcher, craft: craft, logger: logger}
}

// SelectMetal selects metal IDs from the inventory to match the specified scrap value.
// If exact change cannot be collected, it executes a smelting cycle before retrying selection.
// Returns the selected item IDs, or an error if the overall balance is insufficient.
func (m *MetalManager) SelectMetal(ctx context.Context, needed currency.Scrap) ([]uint64, error) {
	if needed <= 0 {
		return nil, nil
	}

	selected, remaining := m.greedySelect(int(needed))
	if remaining > 0 {
		if err := m.craft.MakeChange(ctx, DefIndexScrap, remaining); err != nil {
			return nil, err
		}

		selected, remaining = m.greedySelect(int(needed))
	}

	if remaining > 0 {
		return nil, fmt.Errorf("not enough metal: missing %d scrap", remaining)
	}

	return selected, nil
}

// SelectChange collects an array of metal item IDs whose total value is exactly equal to the specified scrap value.
// Returns [ErrNotEnoughChange] if exact metal-change combinations cannot be formed.
func (m *MetalManager) SelectChange(amount currency.Scrap) ([]uint64, error) {
	selected, remaining := m.greedySelect(int(amount))
	if remaining > 0 {
		return nil, ErrNotEnoughChange
	}

	return selected, nil
}

// SelectKeysAndMetal selects keys and exact metal change item IDs to fulfill the specified offer requirement.
// Returns an error if there are insufficient keys or if exact metal change cannot be formed.
func (m *MetalManager) SelectKeysAndMetal(keys int, metal currency.Scrap) ([]uint64, error) {
	var selected []uint64

	if keys > 0 {
		availableKeys := m.fetcher.GetAssetIDs(currency.SKUKey)
		if len(availableKeys) < keys {
			return nil, errors.New("tf2econ: not enough keys in inventory")
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

func (m *MetalManager) greedySelect(needed int) (selected []uint64, remaining int) {
	ref := m.fetcher.GetAssetIDs(currency.SKURefined)
	rec := m.fetcher.GetAssetIDs(currency.SKUReclaimed)
	scrap := m.fetcher.GetAssetIDs(currency.SKUScrap)

	current := needed

	pick := func(items []uint64, value int) {
		for current >= value && len(items) > 0 {
			selected = append(selected, items[0])
			items = items[1:]
			current -= value
		}
	}

	pick(ref, 9)
	pick(rec, 3)
	pick(scrap, 1)

	return selected, current
}

// TryToSmeltForChange verifies if the target scrap value is obtainable and smelts metal units to make change.
// If metal smelting does not resolve the change requirement, it attempts to smelt duplicate weapons as a fallback.
// Returns an error if the total balance is insufficient or if smelting fails to form exact change.
func (m *MetalManager) TryToSmeltForChange(ctx context.Context, needed currency.Scrap) error {
	stock := m.fetcher.GetPureStock()
	totalValue := stock.TotalScrap()

	if totalValue < needed {
		return fmt.Errorf("tf2econ: insufficient total metal value (have %d, need %d)", totalValue, needed)
	}

	_, remaining := m.greedySelect(int(needed))
	if remaining == 0 {
		return nil
	}

	m.logger.Info("Attempting to break metal for exact change",
		log.Int("needed_scrap", remaining),
		log.Int("total_requested", int(needed)),
	)

	if err := m.craft.MakeChange(ctx, DefIndexScrap, remaining); err != nil {
		return fmt.Errorf("tf2econ: smelting failed: %w", err)
	}

	_, finalRemaining := m.greedySelect(int(needed))
	if finalRemaining > 0 {
		m.logger.Info(
			"Still need change after smelting metal, checking for duplicate weapons...",
			log.Int("remaining", finalRemaining),
		)

		if err := m.SmeltDuplicates(ctx, currency.Scrap(finalRemaining)); err == nil {
			_, afterWeapons := m.greedySelect(int(needed))
			if afterWeapons == 0 {
				return nil
			}
		}

		return fmt.Errorf("tf2econ: smelting didn't resolve the change problem, still need %d scrap", finalRemaining)
	}

	return nil
}

// SmeltDuplicates identifies duplicate weapons across classes and smelts them until the needed scrap value is covered.
// Returns an error if no duplicate weapons are available or if non-tradable weapons are encountered.
func (m *MetalManager) SmeltDuplicates(ctx context.Context, needed currency.Scrap) error {
	classes := []string{"Scout", "Soldier", "Pyro", "Demoman", "Heavy", "Engineer", "Medic", "Sniper", "Spy"}
	smelted := 0

	for _, class := range classes {
		for {
			weapons := m.fetcher.FindWeaponsByClassForSmelting(class)
			if len(weapons) < 2 {
				break
			}

			if !weapons[0].IsTradable || !weapons[1].IsTradable {
				return fmt.Errorf(
					"refusing to smelt: weapons must be tradable (IDs: %d, %d)",
					weapons[0].ID,
					weapons[1].ID,
				)
			}

			m.logger.Info("Smelting duplicate weapons for change", log.String("class", class))

			if _, err := m.craft.SmeltWeapons(ctx, weapons[0].ID, weapons[1].ID); err != nil {
				return err
			}

			smelted++
			if currency.Scrap(smelted) >= needed {
				return nil
			}

			time.Sleep(500 * time.Millisecond)
		}
	}

	if smelted == 0 {
		return errors.New("no duplicate weapons found to smelt")
	}

	return nil
}
