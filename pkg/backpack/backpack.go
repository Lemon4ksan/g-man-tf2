// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/lemon4ksan/foundation/async/event"
	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/foundation/generic"
	"github.com/lemon4ksan/foundation/sync/keylock"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/trading"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

const ModuleName = "tf2_backpack"

const (
	ItemsPerPage = 50
	SlotsPerRow  = 10
)

var ErrSchemaNotReady = errors.New("backpack: schema not ready")

// WithModule registers the Backpack module on a steam.Client instance.
func WithModule() steam.Option {
	return steam.WithModule(New())
}

// From extracts the registered Backpack module from a steam.Client instance.
func From(c *steam.Client) *Backpack {
	return steam.GetModule[*Backpack](c)
}

// TradingProvider defines the contract to query active outgoing trade offers.
type TradingProvider interface {
	GetActiveSentOffers(ctx context.Context) ([]trading.TradeOffer, error)
}

// SchemaProvider defines the contract to obtain the current TF2 schema snapshot.
type SchemaProvider interface {
	Get() *schema.Schema
}

// ItemCache defines the item querying interface provided by the underlying TF2 GC SOCache.
type ItemCache interface {
	GetItems() []*tf2.Item
	GetItem(id uint64) (*tf2.Item, bool)
	GetMaxSlots() int
}

// PositionOf calculates the 1-based linear backpack position from 1-based page and slot numbers.
func PositionOf(page, slot int) uint32 {
	if page < 1 {
		page = 1
	}

	if slot < 1 {
		slot = 1
	}

	return uint32((page-1)*ItemsPerPage + slot)
}

// Backpack manages TF2 backpack items, slot locks, layout rules, and GC inventory interactions.
type Backpack struct {
	module.Base

	tf2     *tf2.TF2
	cache   ItemCache
	soCache *tf2.SOCache
	manager SchemaProvider
	trading TradingProvider

	mu        sync.RWMutex
	itemLocks *keylock.KeyMutex[uint64]
	locked    generic.Set[uint64]
}

// New constructs a new Backpack module with standard dependencies.
func New() *Backpack {
	return &Backpack{
		Base:      module.New(ModuleName).WithDeps(tf2.ModuleName, schema.ModuleName, "trading"),
		itemLocks: keylock.New[uint64](),
		locked:    make(generic.Set[uint64]),
	}
}

// NewWithDeps constructs a Backpack instance with explicitly injected cache, schema, and lock dependencies.
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

	if tradingMod, err := module.Get[TradingProvider](init, "trading"); err == nil {
		m.trading = tradingMod
	}

	return nil
}

func (m *Backpack) StartAuthed(ctx context.Context, _ module.AuthContext) error {
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

// LockItems reserves the specified asset IDs to prevent them from being spent in concurrent actions.
func (m *Backpack) LockItems(ids []uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range ids {
		m.locked.Add(id)
	}
}

// UnlockItems releases reservations for the specified asset IDs.
func (m *Backpack) UnlockItems(ids []uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range ids {
		delete(m.locked, id)
	}
}

// Cache returns the underlying ItemCache used by the backpack.
func (m *Backpack) Cache() ItemCache { return m.cache }

// Schema returns the SchemaProvider associated with this backpack.
func (m *Backpack) Schema() SchemaProvider { return m.manager }

// GetItem retrieves an item from the cache by its 64-bit asset ID.
func (m *Backpack) GetItem(id uint64) (*tf2.Item, bool) {
	return m.cache.GetItem(id)
}

// DeleteItem requests the Game Coordinator to permanently delete the specified item.
func (m *Backpack) DeleteItem(ctx context.Context, itemID uint64) error {
	return m.tf2.DeleteItem(ctx, itemID)
}

// GetItemsBySKU returns all unlocked item asset IDs that match the target SKU.
func (m *Backpack) GetItemsBySKU(targetSKU string) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.soCache != nil {
		return m.soCache.GetAssetIDsDirect(targetSKU, m.locked)
	}

	s := m.manager.Get()
	if s == nil {
		return nil
	}

	items := m.cache.GetItems()

	result := make([]uint64, 0, len(items))
	for _, item := range items {
		if item.GetSKU(s) == targetSKU {
			result = append(result, item.ID)
		}
	}

	return result
}

// GetPureStock scans the backpack to tally available keys, refined, reclaimed, and scrap metal.
func (m *Backpack) GetPureStock() currency.PureStock {
	var (
		stock             currency.PureStock
		untradableRefined int
	)

	processItem := func(item *tf2.Item) bool {
		normDef := schema.NormalizeDefindex(int(item.DefIndex))

		if !item.IsTradable {
			if normDef == schema.DefRefined {
				untradableRefined++
			}

			return true
		}

		switch normDef {
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

	if m.Logger != nil {
		m.Logger.Debug("Pure stock metal count statistics", log.Any("stock", stock))

		if untradableRefined > 0 {
			m.Logger.Debug("Untradable refined sample details", log.Int("untradable_refined", untradableRefined))
		}
	}

	return stock
}

// FindCraftableItems finds unlocked, craftable items matching defIndex up to the specified count limit.
func (m *Backpack) FindCraftableItems(defIndex uint32, count int) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.soCache != nil {
		return m.soCache.FindCraftableItemsDirect(defIndex, count, m.locked)
	}

	items := m.cache.GetItems()

	result := make([]uint64, 0, len(items))
	for _, item := range items {
		if item.DefIndex == defIndex && item.IsCraftable && !m.locked.Has(item.ID) {
			result = append(result, item.ID)
			if count > 0 && len(result) == count {
				break
			}
		}
	}

	return result
}

// GetTotalCount returns the total number of items in the backpack cache.
func (m *Backpack) GetTotalCount() int {
	return len(m.cache.GetItems())
}

// GetStock returns the current quantity of items matching the given SKU in the backpack.
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

// FindWeaponsByClass returns all craftable, tradable weapons usable by the specified class name.
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
		if sch != nil && sch.CraftClass == "weapon" && slices.Contains(sch.UsedByClasses, class) {
			result = append(result, item)
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

	return result
}

// FindWeaponsByClassForSmelting locates duplicate weapon pairs for the given class eligible for smelting.
func (m *Backpack) FindWeaponsByClassForSmelting(class string) []*tf2.Item {
	s := m.manager.Get()
	if s == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var candidates []*tf2.Item

	processItem := func(item *tf2.Item) bool {
		sch := s.ItemByDef(int(item.DefIndex))
		if isEligibleForSmelting(item, sch, class, m.locked, s) {
			candidates = append(candidates, item)
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

	return groupSmeltingPairs(candidates)
}

func isEligibleForSmelting(
	item *tf2.Item,
	sch *schema.Item,
	class string,
	locked generic.Set[uint64],
	s *schema.Schema,
) bool {
	if !item.IsCraftable || !item.IsTradable || locked.Has(item.ID) {
		return false
	}

	if sch == nil || sch.CraftClass != "weapon" || !slices.Contains(sch.UsedByClasses, class) {
		return false
	}

	if item.Quality != uint32(schema.QualityUnique) {
		return false
	}

	if item.IsElevated || item.KillstreakTier != 0 ||
		item.PaintPrimary != 0 || item.PaintSecondary != 0 ||
		item.Festivized || item.CustomName != "" || item.CustomDesc != "" ||
		len(item.Spells) > 0 || len(item.Parts) > 0 || item.Australium ||
		item.Paintkit != 0 || item.Wear != 0 || item.CraftNumber != 0 ||
		item.HasCustomDecal || s.IsPromoItem(sch) {
		return false
	}

	rareDefindexes := []int{160, 294, 161, 258, 298, 423, 727, 933, 947}

	return !slices.Contains(rareDefindexes, int(item.DefIndex))
}

func groupSmeltingPairs(candidates []*tf2.Item) []*tf2.Item {
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
		if base := baseCopies[unpaired.DefIndex]; base != nil {
			result = append(result, unpaired, base)
		}
	}

	return result
}

// GetMetalCount returns the number of metal items in the backpack matching defIndex.
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

// GetAssetIDs returns unlocked, tradable asset IDs matching the given target SKU.
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

	items := m.cache.GetItems()

	result := make([]uint64, 0, len(items))
	for _, item := range items {
		if !m.locked.Has(item.ID) && item.IsTradable && item.GetSKU(s) == targetSKU {
			result = append(result, item.ID)
		}
	}

	return result
}

// GetLockedAssetIDs returns a slice of all currently locked item asset IDs.
func (m *Backpack) GetLockedAssetIDs() []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]uint64, 0, len(m.locked))
	for id := range m.locked {
		result = append(result, id)
	}

	return result
}

// ApplyLayout reorganizes items in the backpack to match the defined Layout specification.
func (m *Backpack) ApplyLayout(ctx context.Context, layout Layout) error {
	s := m.manager.Get()
	if s == nil {
		return ErrSchemaNotReady
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
	currentPage, currentSlot := 1, 1

	for _, section := range layout.Sections {
		currentPage, currentSlot = advanceSectionStart(section, currentPage, currentSlot)
		matchedItems := filterSectionItems(allItems, section, s, plannedIDs, lockedSnapshot)

		for _, item := range matchedItems {
			targetPos, nextPage, nextSlot, err := findNextAvailableSlot(
				currentPage, currentSlot, section, allItems, lockedSnapshot,
			)
			if err != nil {
				return err
			}

			currentPage, currentSlot = nextPage, nextSlot

			plannedIDs.Add(item.ID)

			if item.Position() != targetPos {
				moves = append(moves, tf2.ItemPos{ID: item.ID, Position: targetPos})
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

func advanceSectionStart(section SectionLayout, currPage, currSlot int) (int, int) {
	if section.StartPage <= 0 {
		return currPage, currSlot
	}

	if section.StartPage < currPage || (section.StartPage == currPage && currSlot > 1) {
		if currSlot > 1 {
			currPage++
		}

		return currPage, 1
	}

	return section.StartPage, 1
}

func filterSectionItems(
	allItems []*tf2.Item,
	section SectionLayout,
	s *schema.Schema,
	plannedIDs, lockedSnapshot generic.Set[uint64],
) []*tf2.Item {
	var matched []*tf2.Item

	for _, item := range allItems {
		if plannedIDs.Has(item.ID) || lockedSnapshot.Has(item.ID) {
			continue
		}

		for _, f := range section.Filters {
			if f(item, s) {
				matched = append(matched, item)
				break
			}
		}
	}

	if section.OrderBy != nil {
		slices.SortFunc(matched, func(a, b *tf2.Item) int {
			return section.OrderBy(a, b, s)
		})
	}

	return matched
}

func findNextAvailableSlot(
	page, slot int,
	section SectionLayout,
	allItems []*tf2.Item,
	locked generic.Set[uint64],
) (pos uint32, nextPage, nextSlot int, err error) {
	for {
		if section.EndPage > 0 && page > section.EndPage {
			return 0, 0, 0, fmt.Errorf("backpack: section %q overflowed its allocated page range (%d-%d)",
				section.Name, section.StartPage, section.EndPage)
		}

		targetPos := PositionOf(page, slot)
		slot, page = advanceSlot(slot, page)

		if !isSlotOccupiedByLockedItem(targetPos, allItems, locked) {
			return targetPos, page, slot, nil
		}
	}
}

func advanceSlot(slot, page int) (int, int) {
	slot++
	if slot > ItemsPerPage {
		return 1, page + 1
	}

	return slot, page
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
		case ev, ok := <-sub.C():
			if !ok {
				return
			}

			for _, e := range m.handleEvent(ctx, ev) {
				m.Bus.Publish(e)
			}
		}
	}
}

func (m *Backpack) handleEvent(ctx context.Context, ev event.Event) []event.Event {
	var events []event.Event

	switch e := ev.(type) {
	case *tf2.ItemAcquiredEvent:
		count := len(m.cache.GetItems())
		slots := m.cache.GetMaxSlots()

		if slots > 0 && count >= slots {
			m.Logger.WarnContext(ctx, "Backpack is FULL!", log.Int("count", count), log.Int("max", slots))
			events = append(events, &FullEvent{Count: count, Max: slots})
		}

		// Parity: matches @tf2autobot/tf2 automated safe item acknowledgement.
		if m.tf2 != nil {
			if e.Item != nil && e.Item.ID != 0 {
				isNew := (e.Item.Inventory >> 30) & 1
				if e.Item.Position() == 0 || isNew == 1 {
					if err := m.tf2.AcknowledgeItem(ctx, e.Item.ID); err != nil {
						if m.Logger != nil {
							m.Logger.ErrorContext(ctx, "Failed to auto-acknowledge item",
								log.Uint64("item_id", e.Item.ID),
								log.Err(err),
							)
						}
					}
				}
			} else {
				if err := m.tf2.AcknowledgeAll(ctx); err != nil {
					if m.Logger != nil {
						m.Logger.ErrorContext(ctx, "Failed to auto-acknowledge all items", log.Err(err))
					}
				}
			}
		}

	case *tf2.BackpackLoadedEvent:
		if m.tf2 != nil {
			if err := m.tf2.AcknowledgeAll(ctx); err != nil {
				if m.Logger != nil {
					m.Logger.ErrorContext(ctx, "Failed to acknowledge items on backpack load", log.Err(err))
				}
			}
		}
	}

	return events
}

// AcknowledgeItem acknowledges a single item by moving it to an unoccupied backpack slot.
func (m *Backpack) AcknowledgeItem(ctx context.Context, itemID uint64) error {
	if m.tf2 == nil {
		return errors.New("backpack: tf2 module not initialized")
	}

	return m.tf2.AcknowledgeItem(ctx, itemID)
}

// AcknowledgeAll scans for all unplaced or newly acquired items and assigns them to unoccupied slots.
func (m *Backpack) AcknowledgeAll(ctx context.Context) error {
	if m.tf2 == nil {
		return errors.New("backpack: tf2 module not initialized")
	}

	return m.tf2.AcknowledgeAll(ctx)
}

// GetMaxSlots returns the maximum number of backpack slots supported by the cache.
func (m *Backpack) GetMaxSlots() int {
	if m.cache != nil {
		return m.cache.GetMaxSlots()
	}

	return 0
}

// FreeSlotsCount returns the number of currently unoccupied backpack slots.
func (m *Backpack) FreeSlotsCount() int {
	maxSlots := m.GetMaxSlots()
	if maxSlots <= 0 {
		return 0
	}

	used := m.GetTotalCount()
	if used >= maxSlots {
		return 0
	}

	return maxSlots - used
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
