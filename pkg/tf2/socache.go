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

	"github.com/lemon4ksan/foundation/async/event"
	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/foundation/generic"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"google.golang.org/protobuf/proto"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	pb "github.com/lemon4ksan/g-man-tf2/protobuf/tf2"
)

const (
	SOTypeEconItem              int32 = 1
	SOTypeEconGameAccountClient int32 = 7
	SOTypeTFRatingData          int32 = 2007
)

type Option = generic.Option[*SOCache]

func WithLogger(l log.Logger) Option {
	return func(s *SOCache) { s.logger = l.With(log.Component("so_cache")) }
}

func WithBus(b *event.Bus) Option {
	return func(s *SOCache) { s.bus = b }
}

func WithSchema(s *schema.Schema) Option {
	return func(sc *SOCache) { sc.schema = s }
}

type SOCache struct {
	mu sync.RWMutex

	bus    *event.Bus
	schema *schema.Schema
	logger log.Logger

	items     map[uint64]PackedItem
	fullItems map[uint64]*Item
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

func NewSOCache(coord CoordinatorProvider, opts ...Option) *SOCache {
	s := &SOCache{
		items:     make(map[uint64]PackedItem),
		fullItems: make(map[uint64]*Item),
		ratings:   make(map[int32]uint32),
		coord:     coord,
		logger:    log.Discard,
	}

	generic.ApplyOptions(s, opts...)

	if s.bus == nil {
		s.bus = event.New()
	}

	return s
}

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

func (c *SOCache) GetMaxSlots() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return int(c.slots)
}

func (c *SOCache) IsPremium() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.isPremium
}

func (c *SOCache) GetMMR(ratingType int32) uint32 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.ratings[ratingType]
}

func (c *SOCache) GetTradeBanExpiration() uint32 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.tradeBanExpiration
}

func (c *SOCache) HasCompetitiveAccess() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.compAccess
}

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

func (c *SOCache) GetItemByOriginalID(originalID uint64) (*Item, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, full := range c.fullItems {
		if full != nil && full.OriginalID == originalID {
			return full, true
		}
	}

	for id, packed := range c.items {
		if packed.OriginalID == originalID {
			if full, ok := c.fullItems[id]; ok {
				return full, true
			}

			item := packed.ToItem(c.schema)

			return item, true
		}
	}

	return nil, false
}

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

func (c *SOCache) GetAssetIDsDirect(targetSKU string, locked generic.Set[uint64]) []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]uint64, 0, len(c.items))
	for id, item := range c.items {
		if item.IsTradable() && item.ToSKU(c.schema) == targetSKU && (locked == nil || !locked.Has(id)) {
			result = append(result, id)
		}
	}

	return result
}

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

func (c *SOCache) IsLoaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.loaded
}

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

	c.logger.Info("TF2 SOCache loaded/resynced", log.Int("items", count), log.Uint64("version", msg.GetVersion()))

	c.bus.Publish(&BackpackLoadedEvent{Count: count})
}

func (c *SOCache) handleSOUpdate(pkt *protocol.GCPacket) {
	msgType := pb.ESOMsg(pkt.MsgType &^ protocol.ProtoMask)

	var (
		newVersion uint64
		events     []event.Event
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
		return
	}

	if msg.GetVersion() != c.version.Load() || !c.IsLoaded() {
		c.requestRefresh(ctx, msg.GetOwner(), c.logger)
	}
}

func (c *SOCache) handleUpToDate(pkt *protocol.GCPacket) {
	msg := &pb.CMsgSOCacheSubscribedUpToDate{}
	if err := proto.Unmarshal(pkt.Payload, msg); err == nil {
		c.version.Store(msg.GetVersion())
	}
}

func (c *SOCache) requestRefresh(ctx context.Context, owner uint64, logger log.Logger) {
	req := &pb.CMsgSOCacheSubscriptionRefresh{Owner: proto.Uint64(owner)}
	_ = c.coord.Send(ctx, AppID, uint32(pb.ESOMsg_k_ESOMsg_CacheSubscriptionRefresh), req)
}

func (c *SOCache) processObject(typeID int32, data []byte, isBulk bool, events *[]event.Event) {
	switch typeID {
	case SOTypeEconItem:
		econItem := &pb.CSOEconItem{}
		if err := proto.Unmarshal(data, econItem); err != nil {
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
			} else {
				*events = append(*events, &ItemAcquiredEvent{Item: item})
			}
		}

	case SOTypeEconGameAccountClient:
		acc := &pb.CSOEconGameAccountClient{}
		if err := proto.Unmarshal(data, acc); err == nil {
			c.isPremium = !acc.GetTrialAccount()

			baseSlots := uint32(50)
			if c.isPremium {
				baseSlots = 300
			}

			c.slots = baseSlots + acc.GetAdditionalBackpackSlots()
			c.tradeBanExpiration = acc.GetTradeBanExpiration()
			c.compAccess = acc.GetCompetitiveAccess()
			c.phoneVerified = acc.GetPhoneVerified()
		}

	case SOTypeTFRatingData:
		rating := &pb.CSOTFRatingData{}
		if err := proto.Unmarshal(data, rating); err == nil {
			c.ratings[rating.GetRatingType()] = rating.GetRatingPrimary()
		}
	}
}

func (c *SOCache) processDestroy(typeID int32, data []byte, events *[]event.Event) {
	if typeID != SOTypeEconItem {
		return
	}

	econItem := &pb.CSOEconItem{}
	if err := proto.Unmarshal(data, econItem); err != nil {
		return
	}

	itemID := econItem.GetId()
	delete(c.items, itemID)
	delete(c.fullItems, itemID)

	if events != nil {
		*events = append(*events, &ItemRemovedEvent{ItemID: itemID})
	}
}

func (c *SOCache) protoToItem(p *pb.CSOEconItem) *Item {
	item := &Item{
		ID:           p.GetId(),
		OriginalID:   p.GetOriginalId(),
		DefIndex:     p.GetDefIndex(),
		Level:        p.GetLevel(),
		Quality:      p.GetQuality(),
		Inventory:    p.GetInventory(),
		Quantity:     p.GetQuantity(),
		Origin:       p.GetOrigin(),
		Flags:        EconItemFlag(p.GetFlags()),
		Style:        p.GetStyle(),
		InUse:        p.GetInUse(),
		AccountID:    p.GetAccountId(),
		CustomName:   p.GetCustomName(),
		CustomDesc:   p.GetCustomDesc(),
		IsTradable:   !EconItemFlag(p.GetFlags()).HasFlag(EconItemFlagCannotTrade),
		IsMarketable: !EconItemFlag(p.GetFlags()).HasFlag(EconItemFlagNonEconomy),
		IsCraftable:  true,
	}

	if item.OriginalID == 0 {
		item.OriginalID = item.ID
	}

	c.applyGCOriginAndQualityRestrictions(item)
	c.parseGCAttributes(p.GetAttribute(), item)

	return item
}

func (c *SOCache) parseGCAttributes(attributes []*pb.CSOEconItemAttribute, item *Item) {
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
		decalLo, decalHi     uint32
		part1ID, part1Val    uint32
		part2ID, part2Val    uint32
		part3ID, part3Val    uint32
		seedLo, seedHi       uint32
		hasSeedLo, hasSeedHi bool
		hasAlwaysTradable    bool
	)

	for _, attr := range attributes {
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
			hasAlwaysTradable = true

		case AttrTradableAfter:
			ts := getUint(val)
			if ts == 0 {
				ts = uint32(getFloat(val))
			}

			item.TradableAfter = ts
			if ts > uint32(time.Now().Unix()) && !hasAlwaysTradable {
				item.IsTradable = false
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
			item.Spells = append(item.Spells, sku.Spell{Attribute: int(def), Value: int(getFloat(val))})

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

	if hasAlwaysTradable {
		item.IsTradable = true
	}

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
}

func (c *SOCache) applyGCOriginAndQualityRestrictions(item *Item) {
	untradableOrigins := []uint32{
		OriginAchievement, OriginSupport, OriginHalloween, OriginForeign, OriginPreview, OriginWorkshop, OriginLoaner,
	}

	if slices.Contains(untradableOrigins, item.Origin) {
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

	if item.Origin == OriginPurchase {
		if !item.Flags.HasFlag(EconItemFlagPurchasedAfterStoreCraftabilityChanges2012) {
			item.IsCraftable = false
		}
	}

	if item.Flags.HasFlag(EconItemFlagPreview) {
		item.IsTradable = false
		item.IsCraftable = false
	}
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
