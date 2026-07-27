// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tf2

import (
	"context"
	"encoding/binary"
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/miyako/bus"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/lemon4ksan/miyako/log"
	"google.golang.org/protobuf/proto"

	pb "github.com/lemon4ksan/g-man-tf2/pkg/protobuf/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

// Shared Object Type IDs.
const (
	// SOTypeEconItem represents the inventory item object payload type ID (1).
	SOTypeEconItem int32 = 1
	// SOTypeEconGameAccountClient represents the client account settings payload type ID (7).
	SOTypeEconGameAccountClient int32 = 7
	// SOTypeTFRatingData represents matchmaking rating rating payload type ID (2007).
	SOTypeTFRatingData int32 = 2007
)

// Option defines configuration setter functions for [SOCache] instances.
type Option = generic.Option[*SOCache]

// WithLogger configures a custom [log.Logger] for logging [SOCache] operations.
func WithLogger(l log.Logger) Option {
	return func(s *SOCache) {
		s.logger = l.With(log.Component("so_cache"))
	}
}

// WithBus sets a custom event bus for emitting events.
func WithBus(b *bus.Bus) Option {
	return func(s *SOCache) {
		s.bus = b
	}
}

// WithSchema allows filling out the item SKU's during processing.
func WithSchema(schema *schema.Schema) Option {
	return func(s *SOCache) {
		s.schema = schema
	}
}

// SOCache manages the real-time Team Fortress 2 inventory.
// It maps and processes incoming Shared Object updates from the Game Coordinator.
type SOCache struct {
	mu sync.RWMutex

	bus    *bus.Bus
	schema *schema.Schema
	logger log.Logger

	items     map[uint64]PackedItem // Value map (32 bytes per item, zero pointers)
	fullItems map[uint64]*Item      // Fallback for rare items with custom text/spells/parts
	slots     uint32
	isPremium bool
	loaded    bool

	tradeBanExpiration uint32
	compAccess         bool
	phoneVerified      bool
	ratings            map[int32]uint32

	version atomic.Uint64
	ownerID atomic.Uint64

	coord CoordinatorProvider
}

// NewSOCache creates a new empty Shared Object Cache.
func NewSOCache(coord CoordinatorProvider, opts ...Option) *SOCache {
	s := &SOCache{
		items:     make(map[uint64]PackedItem),
		fullItems: make(map[uint64]*Item),
		ratings:   make(map[int32]uint32),
		coord:     coord,
		logger:    log.Discard,
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.bus == nil {
		s.bus = bus.New()
	}

	return s
}

// UpdateSchema sets the active item schema and automatically applies all schema-based
// normalizations, overrides, and SKU strings to all currently cached items.
// UpdateSchema sets the active item schema and automatically updates cached fullItems.
func (c *SOCache) UpdateSchema(s *schema.Schema) {
	if s == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.schema = s
	for _, item := range c.fullItems {
		item.Fix(s)
		item.SKU = item.GetSKU(s)
	}
}

// GetMaxSlots returns the maximum slot capacity of the backpack.
func (c *SOCache) GetMaxSlots() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return int(c.slots)
}

// IsPremium returns true if the account is premium in TF2.
func (c *SOCache) IsPremium() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isPremium
}

// GetMMR returns the rating value for the specified matchmaking group.
func (c *SOCache) GetMMR(ratingType int32) uint32 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ratings[ratingType]
}

// GetTradeBanExpiration returns the unix timestamp when the trade ban expires.
func (c *SOCache) GetTradeBanExpiration() uint32 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tradeBanExpiration
}

// HasCompetitiveAccess returns true if the account is eligible for competitive matchmaking.
func (c *SOCache) HasCompetitiveAccess() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.compAccess
}

// GetItems returns a snapshot slice of all items stored in the cache.
func (c *SOCache) GetItems() []*Item {
	c.mu.RLock()
	defer c.mu.RUnlock()

	list := make([]*Item, 0, len(c.items))
	for id, packed := range c.items {
		if full, ok := c.fullItems[id]; ok {
			list = append(list, full)
		} else {
			list = append(list, packed.ToItem(c.schema))
		}
	}

	return list
}

// GetItem returns the [Item] matching the specified asset ID.
func (c *SOCache) GetItem(id uint64) (*Item, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if full, ok := c.fullItems[id]; ok {
		return full, true
	}

	packed, ok := c.items[id]
	if !ok {
		return nil, false
	}

	return packed.ToItem(c.schema), true
}

// GetItemByOriginalID returns the [Item] matching the specified original ID.
func (c *SOCache) GetItemByOriginalID(originalID uint64) (*Item, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for id, packed := range c.items {
		if packed.OriginalID == originalID {
			if full, ok := c.fullItems[id]; ok {
				return full, true
			}

			return packed.ToItem(c.schema), true
		}
	}

	return nil, false
}

// ForEachItem iterates over all items in the cache, calling the provided function for each item.
func (c *SOCache) ForEachItem(fn func(item *Item) bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for id, packed := range c.items {
		var item *Item
		if full, ok := c.fullItems[id]; ok {
			item = full
		} else {
			item = packed.ToItem(c.schema)
		}

		if !fn(item) {
			break
		}
	}
}

// GetStockDirect counts items matching targetSKU directly without closure allocations.
func (c *SOCache) GetStockDirect(targetSKU string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, item := range c.items {
		if item.ToSKU(c.schema) == targetSKU {
			count++
		}
	}

	return count
}

// GetAssetIDsDirect collects available asset IDs matching targetSKU directly with exact 1-time slice capacity allocation.
func (c *SOCache) GetAssetIDsDirect(targetSKU string, locked generic.Set[uint64]) []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []uint64
	for id, item := range c.items {
		if item.IsTradable() && item.ToSKU(c.schema) == targetSKU && (locked == nil || !locked.Has(id)) {
			result = append(result, id)
		}
	}

	return result
}

// FindCraftableItemsDirect finds craftable items directly without closure allocations.
func (c *SOCache) FindCraftableItemsDirect(defIndex uint32, count int, locked generic.Set[uint64]) []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	targetDef := uint16(defIndex)

	capacity := count
	if capacity <= 0 {
		capacity = 16
	}

	result := make([]uint64, 0, capacity)

	for id, item := range c.items {
		if item.DefIndex == targetDef && item.IsCraftable() && (locked == nil || !locked.Has(id)) {
			result = append(result, id)
			if count > 0 && len(result) == count {
				break
			}
		}
	}

	return result
}

// GetMetalCountDirect counts metal items directly without closure allocations.
func (c *SOCache) GetMetalCountDirect(defIndex uint32) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	targetDef := uint16(defIndex)

	count := 0
	for _, item := range c.items {
		if item.DefIndex == targetDef && item.IsTradable() {
			count++
		}
	}

	return count
}

// IsLoaded returns whether the cache is loaded.
func (c *SOCache) IsLoaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.loaded
}

// Unload clears the cache and marks it as unloaded.
// SOCache can be reloaded after this.
func (c *SOCache) Unload() {
	c.mu.Lock()
	defer c.mu.Unlock()

	clear(c.items)
	c.loaded = false
}

func (c *SOCache) handleSubscribed(pkt *protocol.GCPacket) {
	msg := &pb.CMsgSOCacheSubscribed{}
	if err := proto.Unmarshal(pkt.Payload, msg); err != nil {
		c.logger.Error("Failed to unmarshal SOCacheSubscribed", log.Err(err))
		return
	}

	c.version.Store(msg.GetVersion())
	c.ownerID.Store(msg.GetOwner())

	c.mu.Lock()
	clear(c.items)
	c.loaded = true

	for _, subType := range msg.GetObjects() {
		typeID := subType.GetTypeId()
		for _, objData := range subType.GetObjectData() {
			c.processObject(typeID, objData, true, nil)
		}
	}

	count := len(c.items)
	c.mu.Unlock()

	c.logger.Info("TF2 SOCache loaded/resynced",
		log.Int("items", count),
		log.Uint64("version", msg.GetVersion()),
	)

	c.bus.Publish(&BackpackLoadedEvent{
		Count: count,
	})
}

func (c *SOCache) handleSOUpdate(pkt *protocol.GCPacket) {
	msgType := pb.ESOMsg(pkt.MsgType &^ protocol.ProtoMask)

	var (
		newVersion uint64
		events     []bus.Event
	)

	c.mu.Lock()
	switch msgType {
	case pb.ESOMsg_k_ESOMsg_Create, pb.ESOMsg_k_ESOMsg_Update:
		msg := &pb.CMsgSOSingleObject{}
		if proto.Unmarshal(pkt.Payload, msg) == nil {
			newVersion = msg.GetVersion()
			c.processObject(msg.GetTypeId(), msg.GetObjectData(), false, &events)
		}

	case pb.ESOMsg_k_ESOMsg_Destroy:
		msg := &pb.CMsgSOSingleObject{}
		if proto.Unmarshal(pkt.Payload, msg) == nil {
			newVersion = msg.GetVersion()
			c.processDestroy(msg.GetTypeId(), msg.GetObjectData(), &events)
		}

	case pb.ESOMsg_k_ESOMsg_UpdateMultiple:
		msg := &pb.CMsgSOMultipleObjects{}
		if err := proto.Unmarshal(pkt.Payload, msg); err == nil {
			newVersion = msg.GetVersion()

			for _, obj := range msg.GetObjects() {
				c.processObject(obj.GetTypeId(), obj.GetObjectData(), false, &events)
			}
		} else {
			c.logger.Error("Failed to unmarshal SOMultipleObjects", log.Err(err))
		}
	}

	if newVersion > 0 {
		c.version.Store(newVersion)
	}

	c.mu.Unlock()

	for _, ev := range events {
		c.bus.Publish(ev)
	}
}

func (c *SOCache) handleSOCacheCheck(ctx context.Context, pkt *protocol.GCPacket) {
	msg := &pb.CMsgSOCacheSubscriptionCheck{}
	if err := proto.Unmarshal(pkt.Payload, msg); err != nil {
		c.logger.Error("Failed to unmarshal CacheSubscriptionCheck", log.Err(err))
		return
	}

	gcVersion := msg.GetVersion()
	ourVersion := c.version.Load()
	owner := msg.GetOwner()

	c.logger.Debug("Received SOCache Check",
		log.Uint64("gc_version", gcVersion),
		log.Uint64("our_version", ourVersion),
	)

	if gcVersion != ourVersion || !c.IsLoaded() {
		c.logger.Warn("SOCache desync detected. Requesting refresh...",
			log.Uint64("expected", gcVersion),
			log.Uint64("actual", ourVersion),
			log.Bool("loaded", c.IsLoaded()),
		)
		c.requestRefresh(ctx, owner, c.logger)
	}
}

func (c *SOCache) handleUpToDate(pkt *protocol.GCPacket) {
	msg := &pb.CMsgSOCacheSubscribedUpToDate{}
	if err := proto.Unmarshal(pkt.Payload, msg); err == nil {
		c.version.Store(msg.GetVersion())
		c.logger.Debug("SOCache is up-to-date", log.Uint64("version", msg.GetVersion()))
	}
}

func (c *SOCache) requestRefresh(ctx context.Context, owner uint64, logger log.Logger) {
	req := &pb.CMsgSOCacheSubscriptionRefresh{
		Owner: proto.Uint64(owner),
	}

	err := c.coord.Send(ctx, AppID, uint32(pb.ESOMsg_k_ESOMsg_CacheSubscriptionRefresh), req)
	if err != nil {
		logger.Error("Failed to send CacheSubscriptionRefresh", log.Err(err))
	}
}

func (c *SOCache) processObject(typeID int32, data []byte, isBulk bool, events *[]bus.Event) {
	switch typeID {
	case SOTypeEconItem:
		econItem := &pb.CSOEconItem{}
		if err := proto.Unmarshal(data, econItem); err != nil {
			c.logger.Error("Failed to unmarshal CSOEconItem", log.Err(err))
			return
		}

		item := c.protoToItem(econItem)
		if c.schema != nil {
			item.Fix(c.schema)
			item.SKU = item.GetSKU(c.schema)
		}

		packed := PackGCItem(item)

		_, exists := c.items[item.ID]
		c.items[item.ID] = packed

		if item.CustomName != "" || item.CustomDesc != "" || len(item.Spells) > 0 || len(item.Parts) > 0 {
			c.fullItems[item.ID] = item
		} else {
			delete(c.fullItems, item.ID)
		}

		if !isBulk && events != nil {
			if exists {
				*events = append(*events, &ItemUpdatedEvent{Item: item})
				c.logger.Debug("Item updated in GC", log.Uint64("id", item.ID))
			} else {
				*events = append(*events, &ItemAcquiredEvent{Item: item})
				c.logger.Debug("New item acquired from GC", log.Uint64("id", item.ID))
			}
		}

	case SOTypeEconGameAccountClient:
		acc := &pb.CSOEconGameAccountClient{}
		if err := proto.Unmarshal(data, acc); err == nil {
			baseSlots := uint32(50)

			if acc.GetTrialAccount() {
				c.isPremium = false
			} else {
				c.isPremium = true
				baseSlots = 300
			}

			c.slots = baseSlots + acc.GetAdditionalBackpackSlots()
			c.tradeBanExpiration = acc.GetTradeBanExpiration()
			c.compAccess = acc.GetCompetitiveAccess()
			c.phoneVerified = acc.GetPhoneVerified()

			c.logger.Debug("Account metadata updated",
				log.Bool("premium", c.isPremium),
				log.Uint32("slots", c.slots),
				log.Bool("comp_access", c.compAccess),
			)
		}

	case SOTypeTFRatingData:
		rating := &pb.CSOTFRatingData{}
		if err := proto.Unmarshal(data, rating); err == nil {
			c.ratings[rating.GetRatingType()] = rating.GetRatingPrimary()
			c.logger.Debug("MMR updated",
				log.Int32("type", rating.GetRatingType()),
				log.Uint32("mmr", rating.GetRatingPrimary()),
			)
		}
	}
}

func (c *SOCache) processDestroy(typeID int32, data []byte, events *[]bus.Event) {
	if typeID != SOTypeEconItem {
		return
	}

	econItem := &pb.CSOEconItem{}
	if err := proto.Unmarshal(data, econItem); err != nil {
		c.logger.Error("Failed to unmarshal CSOEconItem for destroy", log.Err(err))
		return
	}

	itemID := econItem.GetId()
	delete(c.items, itemID)
	delete(c.fullItems, itemID)

	if events != nil {
		*events = append(*events, &ItemRemovedEvent{ItemID: itemID})
	}

	c.logger.Debug("Item removed from GC", log.Uint64("id", itemID))
}

func (c *SOCache) protoToItem(p *pb.CSOEconItem) *Item {
	item := &Item{
		ID:         p.GetId(),
		OriginalID: p.GetOriginalId(),
		DefIndex:   p.GetDefIndex(),
		Level:      p.GetLevel(),
		Quality:    p.GetQuality(),
		Inventory:  p.GetInventory(),
		Quantity:   p.GetQuantity(),
		Origin:     p.GetOrigin(),
		Flags:      EconItemFlag(p.GetFlags()),
		Style:      p.GetStyle(),
		InUse:      p.GetInUse(),
		AccountID:  p.GetAccountId(),

		CustomName: p.GetCustomName(),
		CustomDesc: p.GetCustomDesc(),

		IsTradable:   !EconItemFlag(p.GetFlags()).HasFlag(EconItemFlagCannotTrade),
		IsMarketable: !EconItemFlag(p.GetFlags()).HasFlag(EconItemFlagNonEconomy),

		IsCraftable: true,
	}

	getFloat := func(b []byte) float32 {
		if len(b) < 4 {
			return 0
		}

		return math.Float32frombits(binary.LittleEndian.Uint32(b))
	}

	getUint := func(b []byte) uint32 {
		if len(b) < 4 {
			return 0
		}

		return binary.LittleEndian.Uint32(b)
	}

	var (
		decalLo uint32
		decalHi uint32

		part1ID, part1Val uint32
		part2ID, part2Val uint32
		part3ID, part3Val uint32

		seedLo, seedHi       uint32
		hasSeedLo, hasSeedHi bool
	)

	for _, attr := range p.GetAttribute() {
		def := attr.GetDefIndex()
		val := attr.GetValueBytes()

		switch def {
		case AttrCustomName:
			if name := cleanGCString(val); name != "" {
				item.CustomName = name
			}
		case AttrCustomDesc:
			if desc := cleanGCString(val); desc != "" {
				item.CustomDesc = desc
			}
		case AttrMedalNumber:
			item.MedalNumber = getUint(val)
		case AttrUnusualEffect:
			item.Effect = uint32(getFloat(val))
		case AttrPaintPrimary:
			item.PaintPrimary = uint32(getFloat(val))
		case AttrPaintSecondary:
			item.PaintSecondary = uint32(getFloat(val))
		case AttrCannotTrade:
			item.IsTradable = false
		case AttrCannotCraft:
			item.IsCraftable = false
		case AttrCrateSeries:
			item.CrateSeries = uint32(getFloat(val))
		case AttrAlwaysTradable:
			item.IsTradable = true
		case AttrTradableAfter:
			if getUint(val) > uint32(time.Now().Unix()) {
				item.IsTradable = false
				item.TradableAfter = getUint(val)
			}
		case AttrCrafterAccountID:
			item.CrafterAccountID = uint32(getFloat(val))
		case AttrGifterAccountID:
			item.GifterAccountID = uint32(getFloat(val))
		case AttrKillEater:
			item.IsElevated = item.Quality != schema.QualityStrange
		case AttrKillEaterScoreValue:
			item.ScoreCount = getUint(val)
		case AttrCraftNumber:
			item.CraftNumber = getUint(val)
		case AttrStrangePart1:
			part1ID = uint32(getFloat(val))
			item.Parts = append(item.Parts, part1ID)
		case AttrStrangePart2:
			part2ID = uint32(getFloat(val))
			item.Parts = append(item.Parts, part2ID)
		case AttrStrangePart3:
			part3ID = uint32(getFloat(val))
			item.Parts = append(item.Parts, part3ID)
		case AttrStrangePart1Val:
			part1Val = getUint(val)
		case AttrStrangePart2Val:
			part2Val = getUint(val)
		case AttrStrangePart3Val:
			part3Val = getUint(val)
		case AttrEOTLEarlySupporter:
			item.EarlySupporter = getFloat(val) != 0
		case AttrQuestLoanerIDLow:
			item.QuestID = (item.QuestID & 0xFFFFFFFF00000000) | uint64(getUint(val))
		case AttrQuestLoanerIDHigh:
			item.QuestID = (item.QuestID & 0x00000000FFFFFFFF) | (uint64(getUint(val)) << 32)
		case AttrWear:
			item.Wear = getFloat(val)
		case AttrPaintkit:
			item.Paintkit = getUint(val)
		case AttrPaintkitSeedLo:
			seedLo = getUint(val)
			hasSeedLo = true
		case AttrPaintkitSeedHi:
			seedHi = getUint(val)
			hasSeedHi = true
		case AttrSpell1, AttrSpell2, AttrSpell3, AttrSpell4, AttrSpell5, AttrSpell6:
			item.Spells = append(item.Spells, sku.Spell{
				Attribute: int(def),
				Value:     int(getFloat(val)),
			})
		case AttrTarget:
			item.Target = uint32(getFloat(val))
		case AttrKillstreaker:
			item.Killstreaker = uint32(getFloat(val))
		case AttrSheen:
			item.Sheen = uint32(getFloat(val))
		case AttrKillstreakTier:
			item.KillstreakTier = uint32(getFloat(val))
		case AttrSeries:
			item.Series = uint32(getFloat(val))
		case AttrTauntUnusualEffect:
			item.Effect = uint32(getFloat(val))
		case AttrAustralium:
			item.Australium = getFloat(val) != 0
		case AttrFestivized:
			item.Festivized = getFloat(val) != 0
		case AttrCustomTextureLow:
			decalLo = getUint(val)
			item.HasCustomDecal = true
		case AttrCustomTextureHigh:
			decalHi = getUint(val)
			item.HasCustomDecal = true
		}
	}

	// Lazy map allocation: only allocate PartValues map if Strange Parts are present
	if part1ID != 0 || part2ID != 0 || part3ID != 0 {
		item.PartValues = make(map[uint32]uint32, 3)
		if part1ID != 0 {
			item.PartValues[part1ID] = part1Val
		}

		if part2ID != 0 {
			item.PartValues[part2ID] = part2Val
		}

		if part3ID != 0 {
			item.PartValues[part3ID] = part3Val
		}
	}

	if item.HasCustomDecal {
		item.DecalUGCID = (uint64(decalHi) << 32) | uint64(decalLo)
	}

	if hasSeedLo || hasSeedHi {
		item.PaintkitSeed = (uint64(seedHi) << 32) | uint64(seedLo)
	} else if item.Paintkit != 0 {
		item.PaintkitSeed = item.OriginalID
	}

	if slices.Contains(
		[]uint32{
			OriginAchievement,
			OriginSupport,
			OriginHalloween,
			OriginForeign,
			OriginPreview,
			OriginWorkshop,
			OriginLoaner,
		},
		item.Origin,
	) {
		if item.Origin == OriginLoaner && item.IsTradable {
			item.IsBuggedLoaner = true
		} else {
			item.IsTradable = false
			item.IsMarketable = false
		}
	}

	if slices.Contains(
		[]uint32{
			OriginStorePromo,
			OriginSupport,
			OriginHalloween,
			OriginForeign,
			OriginPreview,
			OriginWorkshop,
			OriginLoaner,
		},
		item.Origin,
	) {
		item.IsCraftable = false
	}

	if slices.Contains([]uint32{QualitySelfMade, QualityValve, QualityCommunity}, item.Quality) {
		item.IsTradable = false
		item.IsCraftable = false
	}

	for _, attr := range p.GetAttribute() {
		if attr.GetDefIndex() == AttrAlwaysTradable {
			item.IsTradable = true
			break
		}
	}

	if item.Origin == OriginPurchase {
		if !item.Flags.HasFlag(EconItemFlagPurchasedAfterStoreCraftabilityChanges2012) {
			item.IsCraftable = false
		}
	}

	if item.Flags.HasFlag(EconItemFlagPreview) {
		item.IsTradable = false
		item.IsCraftable = false
	}

	return item
}

func cleanGCString(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	start := 0
	for start < len(b) && (b[start] < 32 || b[start] == 127) {
		start++
	}

	end := len(b)
	for end > start && (b[end-1] < 32 || b[end-1] == 127 || b[end-1] == 0) {
		end--
	}

	if start >= end {
		return ""
	}

	return string(b[start:end])
}
