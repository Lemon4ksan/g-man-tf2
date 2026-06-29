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

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/miyako/bus"
	"github.com/lemon4ksan/miyako/generic"
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
// Index values are 1-based. If page or slot is less than 1, they default to 1.
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
// It acts as a lightweight view over the [ItemCache] and coordinates with the Game Coordinator.
// Use [New] to create a new instance and register it using [WithModule].
type Backpack struct {
	module.Base

	tf2     *tf2.TF2
	cache   ItemCache
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
// This constructor is intended for external sidecar setups or tests that need to reuse backpack methods.
func NewWithDeps(cache ItemCache, manager SchemaProvider, locked generic.Set[uint64]) *Backpack {
	return &Backpack{
		cache:     cache,
		manager:   manager,
		itemLocks: keylock.New[uint64](),
		locked:    locked,
	}
}

// Init initializes the [Backpack] module by resolving its required dependencies.
// Returns an error if any of the mandatory dependency modules are missing.
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
// Returns an error if the context is cancelled during startup.
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
// Nil or empty slices are ignored.
func (m *Backpack) LockItems(ids []uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range ids {
		m.locked.Add(id)
	}
}

// UnlockItems releases the locks on the specified item IDs.
// Nil or empty slices are ignored.
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
// Returns false as the second value if the item does not exist.
func (m *Backpack) GetItem(id uint64) (*tf2.Item, bool) {
	return m.cache.GetItem(id)
}

// DeleteItem requests the Game Coordinator to permanently delete the specified item.
// Returns an error if the network packet cannot be sent to the Game Coordinator.
func (m *Backpack) DeleteItem(ctx context.Context, itemID uint64) error {
	return m.tf2.DeleteItem(ctx, itemID)
}

// GetItemsBySKU returns all item IDs matching the specified target SKU.
// Returns nil if targetSKU is empty or the schema is not loaded.
func (m *Backpack) GetItemsBySKU(targetSKU string) []uint64 {
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

// GetPureStock calculates and returns the current tradable keys and metal balances.
// This value is used by the metal manager to calculate change.
func (m *Backpack) GetPureStock() currency.PureStock {
	stock := currency.PureStock{}

	var (
		totalRef, totalRec, totalScrap                int
		untradableRef, untradableRec, untradableScrap int
	)

	for _, item := range m.cache.GetItems() {
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
			continue
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
			for _, item := range m.cache.GetItems() {
				if schema.NormalizeDefindex(int(item.DefIndex)) == schema.DefRefined && !item.IsTradable {
					sample = item
					break
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

// FindCraftableItems returns a list of tradable item IDs matching the specified defIndex.
// It filters out locked items and stops once the specified count limit is reached.
// If count is less than or equal to 0, no limit is applied and all matching items are returned.
func (m *Backpack) FindCraftableItems(defIndex uint32, count int) []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []uint64
	for _, item := range m.cache.GetItems() {
		if item.DefIndex == defIndex && item.IsCraftable && !m.locked.Has(item.ID) {
			result = append(result, item.ID)
			if len(result) == count {
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

// GetStock returns the current stock count for the specified SKU.
// Returns 0 if the SKU is empty or the schema is not loaded.
func (m *Backpack) GetStock(sku string) int {
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
// Returns nil if the class is invalid or the schema is not loaded.
func (m *Backpack) FindWeaponsByClass(class string) []*tf2.Item {
	s := m.manager.Get()
	if s == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*tf2.Item
	for _, item := range m.cache.GetItems() {
		if !item.IsCraftable || !item.IsTradable || m.locked.Has(item.ID) {
			continue
		}

		sch := s.ItemByDef(int(item.DefIndex))
		if sch == nil || sch.CraftClass != "weapon" {
			continue
		}

		if slices.Contains(sch.UsedByClasses, class) {
			result = append(result, item)
		}
	}

	return result
}

// FindWeaponsByClassForSmelting returns a slice of duplicate unique weapons eligible for smelting.
// It filters out special items such as Australiums, professional killstreaks, painted or custom items.
// It preserves at least one base copy of each weapon type in the inventory.
// Returns nil if the class name is invalid or the schema is not loaded.
func (m *Backpack) FindWeaponsByClassForSmelting(class string) []*tf2.Item {
	s := m.manager.Get()
	if s == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var candidates []*tf2.Item
	for _, item := range m.cache.GetItems() {
		if !item.IsCraftable || !item.IsTradable || m.locked.Has(item.ID) {
			continue
		}

		sch := s.ItemByDef(int(item.DefIndex))
		if sch == nil || sch.CraftClass != "weapon" {
			continue
		}

		if !slices.Contains(sch.UsedByClasses, class) {
			continue
		}

		if item.Quality != uint32(schema.QualityUnique) {
			continue
		}

		if item.IsElevated {
			continue
		}

		if item.KillstreakTier != 0 {
			continue
		}

		if item.Paint != 0 {
			continue
		}

		if item.Festivized {
			continue
		}

		if item.CustomName != "" || item.CustomDesc != "" {
			continue
		}

		if len(item.Spells) > 0 {
			continue
		}

		if len(item.Parts) > 0 {
			continue
		}

		if item.Australium {
			continue
		}

		if item.Paintkit != 0 || item.Wear != 0 {
			continue
		}

		if item.CraftNumber != 0 {
			continue
		}

		if item.HasCustomDecal {
			continue
		}

		if s.IsPromoItem(sch) {
			continue
		}

		rareDefindexes := []int{160, 294, 161, 258, 298, 423, 727, 933, 947}
		if slices.Contains(rareDefindexes, int(item.DefIndex)) {
			continue
		}

		candidates = append(candidates, item)
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

// GetMetalCount returns the total count of metal items matching the specified DefIndex.
func (m *Backpack) GetMetalCount(defIndex uint32) int {
	count := 0
	for _, item := range m.cache.GetItems() {
		if item.DefIndex == defIndex {
			count++
		}
	}

	return count
}

// GetAssetIDs returns available tradable and unlocked item IDs matching the target SKU.
// It automatically filters out items currently locked in other active trade offers.
func (m *Backpack) GetAssetIDs(targetSKU string) []uint64 {
	s := m.manager.Get()
	if s == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

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
// It packs items tightly in a continuous sequence, advancing pages only when a page is full.
// Returns an error if the schema is not ready, if item moves fail, or if the context is cancelled.
func (m *Backpack) ApplyLayout(ctx context.Context, layout Layout) error {
	s := m.manager.Get()
	if s == nil {
		return errors.New("schema not ready")
	}

	// Snapshot locked items under read lock
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
