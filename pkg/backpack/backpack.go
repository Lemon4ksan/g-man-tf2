// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package backpack provides the TF2 backpack module.
package backpack

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/miyako/bus"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/lemon4ksan/miyako/log"
	"github.com/lemon4ksan/miyako/sync/keylock"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// ModuleName is the name of the backpack module.
const ModuleName = "tf2_backpack"

// WithModule returns a [steam.Option] that registers the [Backpack] module with the client.
func WithModule() steam.Option {
	return steam.WithModule(New())
}

// From returns the [Backpack] module instance retrieved from the [steam.Client].
func From(c *steam.Client) *Backpack {
	return steam.GetModule[*Backpack](c)
}

const (
	// ItemsPerPage defines the number of items contained in a single backpack page.
	ItemsPerPage = 50
	// SlotsPerRow defines the number of items displayed in a single slot row.
	SlotsPerRow = 10
)

// TradingProvider defines the interface for retrieving active sent trade offers.
type TradingProvider interface {
	GetActiveSentOffers(ctx context.Context) ([]trading.TradeOffer, error)
}

// SchemaProvider defines the interface for accessing the current TF2 item schema.
type SchemaProvider interface {
	Get() *schema.Schema
}

// ItemCache defines the interface for accessing the underlying TF2 item cache.
type ItemCache interface {
	GetItems() []*tf2.Item
	GetItem(id uint64) (*tf2.Item, bool)
	GetMaxSlots() int
}

// PositionOf calculates the Game Coordinator inventory index from page and slot numbers.
func PositionOf(page, slot int) uint32 {
	if page < 1 {
		page = 1
	}

	if slot < 1 {
		slot = 1
	}

	return uint32((page-1)*ItemsPerPage + slot)
}

// Backpack manages the Team Fortress 2 local inventory.
type Backpack struct {
	module.Base

	tf2     *tf2.TF2
	cache   ItemCache
	soCache *tf2.SOCache // Cached concrete pointer for zero-allocation stack closures
	manager SchemaProvider
	trading TradingProvider

	mu        sync.RWMutex
	itemLocks *keylock.KeyMutex[uint64]
	locked    generic.Set[uint64]
}

// New constructs a new [Backpack] instance with empty lock states and pre-declared dependencies.
func New() *Backpack {
	return &Backpack{
		Base:      module.New(ModuleName).WithDeps(tf2.ModuleName, schema.ModuleName, "trading"),
		itemLocks: keylock.New[uint64](),
		locked:    make(generic.Set[uint64]),
	}
}

// NewWithDeps constructs a lightweight [Backpack] instance using the specified cache, manager and locked map dependencies.
func NewWithDeps(cache ItemCache, manager SchemaProvider, locked generic.Set[uint64]) *Backpack {
	b := &Backpack{
		cache:     cache,
		manager:   manager,
		itemLocks: keylock.New[uint64](),
		locked:    locked,
	}

	if so, ok := cache.(*tf2.SOCache); ok {
		b.soCache = so
	}

	return b
}

// Init initializes the [Backpack] module by resolving its required dependencies.
func (m *Backpack) Init(init module.InitContext) error {
	if err := m.Base.Init(init); err != nil {
		return err
	}

	tf2Mod, err := module.Get[*tf2.TF2](init, tf2.ModuleName)
	if err != nil {
		return err
	}

	m.tf2 = tf2Mod
	m.cache = tf2Mod.Cache()
	m.soCache = tf2Mod.Cache()

	managerMod, err := module.Get[*schema.Manager](init, schema.ModuleName)
	if err != nil {
		return err
	}

	m.manager = managerMod

	tradingMod, err := module.Get[TradingProvider](init, "trading")
	if err == nil {
		m.trading = tradingMod
	}

	return nil
}

// StartAuthed starts the asynchronous event loops and background stale lock cleanup routines.
func (m *Backpack) StartAuthed(ctx context.Context, authCtx module.AuthContext) error {
	m.Go(m.eventLoop)

	if m.trading != nil {
		m.Go(func(ctx context.Context) {
			m.cleanupStaleLocks(ctx, m.trading)

			ticker := time.NewTicker(15 * time.Minute)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					m.cleanupStaleLocks(ctx, m.trading)
				}
			}
		})
	}

	return nil
}

// LockItems locks the specified item IDs to prevent them from being selected for other active trades.
func (m *Backpack) LockItems(ids []uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range ids {
		m.locked.Add(id)
	}
}

// UnlockItems releases the locks on the specified item IDs.
func (m *Backpack) UnlockItems(ids []uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range ids {
		delete(m.locked, id)
	}
}

// Cache returns the underlying [ItemCache] interface.
func (m *Backpack) Cache() ItemCache {
	return m.cache
}

// Schema returns the configured [SchemaProvider] interface.
func (m *Backpack) Schema() SchemaProvider {
	return m.manager
}

// GetItem searches the [ItemCache] and returns the [tf2.Item] matching the specified ID.
func (m *Backpack) GetItem(id uint64) (*tf2.Item, bool) {
	return m.cache.GetItem(id)
}

// DeleteItem requests the Game Coordinator to permanently delete the specified item.
func (m *Backpack) DeleteItem(ctx context.Context, itemID uint64) error {
	return m.tf2.DeleteItem(ctx, itemID)
}

// GetItemsBySKU returns all item IDs matching the specified target SKU with zero closure allocation.
func (m *Backpack) GetItemsBySKU(targetSKU string) []uint64 {
	if m.soCache != nil {
		return m.soCache.GetAssetIDsDirect(targetSKU, m.locked)
	}

	s := m.manager.Get()
	if s == nil {
		return nil
	}

	var result []uint64
	for _, item := range m.cache.GetItems() {
		if item.GetSKU(s) == targetSKU {
			result = append(result, item.ID)
		}
	}

	return result
}

// GetPureStock calculates and returns the current tradable keys and metal balances without allocations.
func (m *Backpack) GetPureStock() currency.PureStock {
	stock := currency.PureStock{}

	var (
		totalRef, totalRec, totalScrap                int
		untradableRef, untradableRec, untradableScrap int
	)

	processItem := func(item *tf2.Item) bool {
		def := schema.NormalizeDefindex(int(item.DefIndex))

		switch def {
		case schema.DefRefined:
			totalRef++

			if !item.IsTradable {
				untradableRef++
			}

		case schema.DefReclaimed:
			totalRec++

			if !item.IsTradable {
				untradableRec++
			}

		case schema.DefScrap:
			totalScrap++

			if !item.IsTradable {
				untradableScrap++
			}
		}

		if !item.IsTradable {
			return true
		}

		switch def {
		case schema.DefKey:
			stock.Keys++
		case schema.DefRefined:
			stock.Refined++
		case schema.DefReclaimed:
			stock.Reclaimed++
		case schema.DefScrap:
			stock.Scrap++
		}

		return true
	}

	if m.soCache != nil {
		m.soCache.ForEachItem(processItem)
	} else {
		for _, item := range m.cache.GetItems() {
			processItem(item)
		}
	}

	if m.Logger != nil && (totalRef > 0 || totalRec > 0 || totalScrap > 0) {
		m.Logger.Debug(
			"Pure stock metal count statistics",
			log.Int("total_ref", totalRef),
			log.Int("tradable_ref", int(stock.Refined)),
			log.Int("untradable_ref", untradableRef),
			log.Int("total_rec", totalRec),
			log.Int("tradable_rec", int(stock.Reclaimed)),
			log.Int("untradable_rec", untradableRec),
			log.Int("total_scrap", totalScrap),
			log.Int("tradable_scrap", int(stock.Scrap)),
			log.Int("untradable_scrap", untradableScrap),
		)

		if totalRef > 0 && untradableRef > 0 {
			var sample *tf2.Item

			findSample := func(item *tf2.Item) bool {
				if schema.NormalizeDefindex(int(item.DefIndex)) == schema.DefRefined && !item.IsTradable {
					sample = item
					return false
				}

				return true
			}

			if m.soCache != nil {
				m.soCache.ForEachItem(findSample)
			} else {
				for _, item := range m.cache.GetItems() {
					if !findSample(item) {
						break
					}
				}
			}

			if sample != nil {
				m.Logger.Debug("Untradable refined sample details",
					log.Uint64("id", sample.ID),
					log.Uint32("origin", sample.Origin),
					log.Uint32("flags", uint32(sample.Flags)),
					log.Uint32("quality", sample.Quality),
				)
			}
		}
	}

	return stock
}

// FindCraftableItems returns a list of tradable item IDs matching the specified defIndex without closure allocations.
func (m *Backpack) FindCraftableItems(defIndex uint32, count int) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.soCache != nil {
		return m.soCache.FindCraftableItemsDirect(defIndex, count, m.locked)
	}

	var result []uint64
	for _, item := range m.cache.GetItems() {
		if item.DefIndex == defIndex && item.IsCraftable && !m.locked.Has(item.ID) {
			result = append(result, item.ID)
			if count > 0 && len(result) == count {
				break
			}
		}
	}

	return result
}

// GetTotalCount returns the total number of items stored in the [ItemCache].
func (m *Backpack) GetTotalCount() int {
	return len(m.cache.GetItems())
}

// GetStock returns the current stock count for the specified SKU without closure allocations.
func (m *Backpack) GetStock(sku string) int {
	if m.soCache != nil {
		return m.soCache.GetStockDirect(sku)
	}

	s := m.manager.Get()
	if s == nil {
		return 0
	}

	count := 0
	for _, item := range m.cache.GetItems() {
		if item.GetSKU(s) == sku {
			count++
		}
	}

	return count
}

// FindWeaponsByClass returns all craftable, tradable, and unlocked weapons usable by the specified class name.
func (m *Backpack) FindWeaponsByClass(class string) []*tf2.Item {
	s := m.manager.Get()
	if s == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*tf2.Item

	processItem := func(item *tf2.Item) bool {
		if !item.IsCraftable || !item.IsTradable || m.locked.Has(item.ID) {
			return true
		}

		sch := s.ItemByDef(int(item.DefIndex))
		if sch == nil || sch.CraftClass != "weapon" {
			return true
		}

		if slices.Contains(sch.UsedByClasses, class) {
			result = append(result, item)
		}

		return true
	}

	if m.soCache != nil {
		m.soCache.ForEachItem(processItem)
		return result
	}

	for _, item := range m.cache.GetItems() {
		processItem(item)
	}

	return result
}

// FindWeaponsByClassForSmelting returns a slice of duplicate unique weapons eligible for smelting.
func (m *Backpack) FindWeaponsByClassForSmelting(class string) []*tf2.Item {
	s := m.manager.Get()
	if s == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var candidates []*tf2.Item

	processItem := func(item *tf2.Item) bool {
		if !item.IsCraftable || !item.IsTradable || m.locked.Has(item.ID) {
			return true
		}

		sch := s.ItemByDef(int(item.DefIndex))
		if sch == nil || sch.CraftClass != "weapon" {
			return true
		}

		if !slices.Contains(sch.UsedByClasses, class) {
			return true
		}

		if item.Quality != uint32(schema.QualityUnique) {
			return true
		}

		if item.IsElevated || item.KillstreakTier != 0 ||
			item.PaintPrimary != 0 || item.PaintSecondary != 0 ||
			item.Festivized || item.CustomName != "" || item.CustomDesc != "" ||
			len(item.Spells) > 0 || len(item.Parts) > 0 || item.Australium ||
			item.Paintkit != 0 || item.Wear != 0 || item.CraftNumber != 0 ||
			item.HasCustomDecal || s.IsPromoItem(sch) {
			return true
		}

		rareDefindexes := []int{160, 294, 161, 258, 298, 423, 727, 933, 947}
		if slices.Contains(rareDefindexes, int(item.DefIndex)) {
			return true
		}

		candidates = append(candidates, item)

		return true
	}

	if m.soCache != nil {
		m.soCache.ForEachItem(processItem)
	} else {
		for _, item := range m.cache.GetItems() {
			processItem(item)
		}
	}

	slices.SortFunc(candidates, func(a, b *tf2.Item) int {
		if a.ID < b.ID {
			return -1
		}

		return 1
	})

	byDef := make(map[uint32][]*tf2.Item)
	for _, item := range candidates {
		byDef[item.DefIndex] = append(byDef[item.DefIndex], item)
	}

	var duplicates []*tf2.Item

	baseCopies := make(map[uint32]*tf2.Item)

	for defIndex, items := range byDef {
		if len(items) >= 2 {
			baseCopies[defIndex] = items[0]
			duplicates = append(duplicates, items[1:]...)
		}
	}

	slices.SortFunc(duplicates, func(a, b *tf2.Item) int {
		if a.ID < b.ID {
			return -1
		}

		return 1
	})

	var result []*tf2.Item

	for len(duplicates) >= 2 {
		result = append(result, duplicates[0], duplicates[1])
		duplicates = duplicates[2:]
	}

	if len(duplicates) == 1 {
		unpaired := duplicates[0]

		base := baseCopies[unpaired.DefIndex]
		if base != nil {
			result = append(result, unpaired, base)
		}
	}

	return result
}

// GetMetalCount returns the total count of metal items matching the specified DefIndex without closure allocations.
func (m *Backpack) GetMetalCount(defIndex uint32) int {
	if m.soCache != nil {
		return m.soCache.GetMetalCountDirect(defIndex)
	}

	count := 0
	for _, item := range m.cache.GetItems() {
		if item.DefIndex == defIndex {
			count++
		}
	}

	return count
}

// GetAssetIDs returns available tradable and unlocked item IDs matching the target SKU without closure allocations.
func (m *Backpack) GetAssetIDs(targetSKU string) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.soCache != nil {
		return m.soCache.GetAssetIDsDirect(targetSKU, m.locked)
	}

	s := m.manager.Get()
	if s == nil {
		return nil
	}

	var result []uint64
	for _, item := range m.cache.GetItems() {
		if !m.locked.Has(item.ID) && item.IsTradable && item.GetSKU(s) == targetSKU {
			result = append(result, item.ID)
		}
	}

	return result
}

// GetLockedAssetIDs returns a slice of all item IDs currently locked in the backpack.
func (m *Backpack) GetLockedAssetIDs() []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]uint64, 0, len(m.locked))
	for id := range m.locked {
		result = append(result, id)
	}

	return result
}

// ApplyLayout analyzes the current inventory and moves items according to the rules.
func (m *Backpack) ApplyLayout(ctx context.Context, layout Layout) error {
	s := m.manager.Get()
	if s == nil {
		return errors.New("schema not ready")
	}

	m.mu.RLock()

	lockedSnapshot := make(generic.Set[uint64], len(m.locked))
	for id := range m.locked {
		lockedSnapshot[id] = struct{}{}
	}

	m.mu.RUnlock()

	plannedIDs := make(generic.Set[uint64])

	var moves []tf2.ItemPos

	allItems := m.cache.GetItems()

	currentPage := 1
	currentSlot := 1

	for _, section := range layout.Sections {
		if section.StartPage > 0 {
			if section.StartPage < currentPage || (section.StartPage == currentPage && currentSlot > 1) {
				if currentSlot > 1 {
					currentPage++
				}

				currentSlot = 1
			} else {
				currentPage = section.StartPage
				currentSlot = 1
			}
		}

		var matchedItems []*tf2.Item
		for _, item := range allItems {
			if plannedIDs.Has(item.ID) || lockedSnapshot.Has(item.ID) {
				continue
			}

			matches := false
			for _, f := range section.Filters {
				if f(item, s) {
					matches = true
					break
				}
			}

			if matches {
				matchedItems = append(matchedItems, item)
			}
		}

		if section.OrderBy != nil {
			slices.SortFunc(matchedItems, func(a, b *tf2.Item) int {
				return section.OrderBy(a, b, s)
			})
		}

		for _, item := range matchedItems {
			for {
				if section.EndPage > 0 && currentPage > section.EndPage {
					return fmt.Errorf("backpack: section %q overflowed its allocated page range (%d-%d)",
						section.Name, section.StartPage, section.EndPage)
				}

				targetPos := PositionOf(currentPage, currentSlot)

				if !isSlotOccupiedByLockedItem(targetPos, allItems, lockedSnapshot) {
					plannedIDs.Add(item.ID)

					if item.Position() != targetPos {
						moves = append(moves, tf2.ItemPos{
							ID:       item.ID,
							Position: targetPos,
						})
					}

					currentSlot++
					if currentSlot > ItemsPerPage {
						currentSlot = 1
						currentPage++
					}

					break
				}

				currentSlot++
				if currentSlot > ItemsPerPage {
					currentSlot = 1
					currentPage++
				}
			}
		}
	}

	if len(moves) == 0 {
		m.Logger.InfoContext(ctx, "Inventory is already sorted according to the layout")
		return nil
	}

	m.Logger.InfoContext(ctx, "Applying inventory layout", log.Int("moves_count", len(moves)))

	return m.tf2.MoveItems(ctx, moves)
}

func isSlotOccupiedByLockedItem(pos uint32, allItems []*tf2.Item, lockedSet generic.Set[uint64]) bool {
	for _, item := range allItems {
		if item.Position() == pos && lockedSet.Has(item.ID) {
			return true
		}
	}

	return false
}

func (m *Backpack) eventLoop(ctx context.Context) {
	sub := m.Bus.Subscribe(
		&tf2.BackpackLoadedEvent{},
		&tf2.ItemAcquiredEvent{},
		&tf2.ItemRemovedEvent{},
		&tf2.ItemUpdatedEvent{},
		&schema.UpdatedEvent{},
	)
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-sub.C():
			events := m.handleEvent(ctx, ev)
			for _, e := range events {
				m.Bus.Publish(e)
			}
		}
	}
}

func (m *Backpack) handleEvent(ctx context.Context, ev bus.Event) []bus.Event {
	var events []bus.Event

	if _, ok := ev.(*tf2.ItemAcquiredEvent); ok {
		count := len(m.cache.GetItems())
		slots := m.cache.GetMaxSlots()

		if slots > 0 && count >= slots {
			m.Logger.WarnContext(ctx, "Backpack is FULL!", log.Int("count", count), log.Int("max", slots))
			events = append(events, &FullEvent{Count: count, Max: slots})
		}
	}

	return events
}

func (m *Backpack) cleanupStaleLocks(ctx context.Context, tradingModule TradingProvider) {
	activeOffers, err := tradingModule.GetActiveSentOffers(ctx)
	if err != nil {
		m.Logger.ErrorContext(ctx, "Failed to get active offers for stale lock cleanup", log.Err(err))
		return
	}

	activeItems := generic.NewSet[uint64]()
	for _, off := range activeOffers {
		for _, it := range off.ItemsToGive {
			activeItems.Add(it.AssetID)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cleanedCount := 0
	for lockedID := range m.locked {
		if !activeItems.Has(lockedID) {
			delete(m.locked, lockedID)

			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		m.Logger.InfoContext(ctx, "Cleaned up stale item locks", log.Int("count", cleanedCount))
	}
}
