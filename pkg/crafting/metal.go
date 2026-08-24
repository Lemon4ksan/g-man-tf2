// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crafting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lemon4ksan/foundation/async/log"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

var (
	ErrNotEnoughChange    = errors.New("tf2econ: not enough pure metal to make exact change")
	ErrNotEnoughKeys      = errors.New("tf2econ: not enough keys in inventory")
	ErrNoDuplicateWeapons = errors.New("crafting: no duplicate weapons found to smelt")
)

type AssetFetcher interface {
	GetAssetIDs(sku string) []uint64
	GetPureStock() currency.PureStock
	FindWeaponsByClassForSmelting(class string) []*tf2.Item
	GetMetalCount(defIndex uint32) int
}

type MetalManager struct {
	fetcher AssetFetcher
	logger  log.Logger
	craft   *Manager
}

func NewMetalManager(fetcher AssetFetcher, craft *Manager, logger log.Logger) *MetalManager {
	return &MetalManager{fetcher: fetcher, craft: craft, logger: logger}
}

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

func (m *MetalManager) SelectChange(amount currency.Scrap) ([]uint64, error) {
	selected, remaining := m.greedySelect(int(amount))
	if remaining > 0 {
		return nil, ErrNotEnoughChange
	}

	return selected, nil
}

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

func (m *MetalManager) TryToSmeltForChange(ctx context.Context, needed currency.Scrap) error {
	stock := m.fetcher.GetPureStock()
	if stock.TotalScrap() < needed {
		return fmt.Errorf("tf2econ: insufficient total metal value (have %d, need %d)", stock.TotalScrap(), needed)
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
		m.logger.Info("Checking duplicate weapons for change...", log.Int("remaining", finalRemaining))

		if err := m.SmeltDuplicates(ctx, currency.Scrap(finalRemaining)); err == nil {
			if _, afterWeapons := m.greedySelect(int(needed)); afterWeapons == 0 {
				return nil
			}
		}

		return fmt.Errorf("tf2econ: smelting didn't resolve the change problem, missing %d scrap", finalRemaining)
	}

	return nil
}

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
