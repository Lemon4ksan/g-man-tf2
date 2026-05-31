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

	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"google.golang.org/protobuf/proto"

	pb "github.com/lemon4ksan/g-man-tf2/pkg/protobuf/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

// EconItemFlag represents bitmask flags for Steam Econ items.
type EconItemFlag uint32

const (
	// EconItemFlagCannotTrade indicates the item cannot be traded.
	EconItemFlagCannotTrade EconItemFlag = 1 << iota
	// EconItemFlagCannotBeUsedInCrafting indicates the item cannot be used in crafting.
	EconItemFlagCannotBeUsedInCrafting
	// EconItemFlagCanBeTradedByFreeAccounts indicates the item can be traded even if the account is not premium.
	EconItemFlagCanBeTradedByFreeAccounts
	// EconItemFlagNonEconomy indicates the item is a non-economy item.
	EconItemFlagNonEconomy
	// EconItemFlagPurchasedAfterStoreCraftabilityChanges2012 relates to store items bought after the 2012 craftability update.
	EconItemFlagPurchasedAfterStoreCraftabilityChanges2012
	// EconItemFlagForceBlueTeam indicates the item is forced to blue team.
	EconItemFlagForceBlueTeam
	// EconItemFlagStoreItem indicates the item was bought from the Mann Co. Store.
	EconItemFlagStoreItem
	// EconItemFlagPreview indicates the item is a preview item from the store.
	EconItemFlagPreview
)

// HasFlag checks if the provided flag is set in the bitmask.
func (f EconItemFlag) HasFlag(flag EconItemFlag) bool {
	return (f & flag) != 0
}

// Attributes list configuration from items_game.txt.
const (
	// AttrCustomName represents a Name Tag customization.
	AttrCustomName uint32 = 111
	// AttrCustomDesc represents a Description Tag customization.
	AttrCustomDesc uint32 = 112
	// AttrMedalNumber represents a registration number on tournament medals.
	AttrMedalNumber uint32 = 133
	// AttrUnusualEffect represents the Unusual particle effect identifier.
	AttrUnusualEffect uint32 = 134
	// AttrPaintPrimary represents the primary team paint color decimal value.
	AttrPaintPrimary uint32 = 142
	// AttrCannotTrade represents the non-tradable restriction.
	AttrCannotTrade uint32 = 153
	// AttrCannotCraft represents the non-craftable restriction.
	AttrCannotCraft uint32 = 186
	// AttrCrateSeries represents the supply crate series number attribute.
	AttrCrateSeries uint32 = 187
	// AttrAlwaysTradable represents the attribute overriding standard tradable constraints.
	AttrAlwaysTradable uint32 = 195
	// AttrTradableAfter represents the unix timestamp limitation for trades.
	AttrTradableAfter uint32 = 211
	// AttrKillEater represents a Strange part counter tracking presence.
	AttrKillEater uint32 = 214
	// AttrCraftNumber represents the craft index numbering attribute.
	AttrCraftNumber uint32 = 229
	// AttrPaintSecondary represents the secondary team paint color decimal value.
	AttrPaintSecondary uint32 = 261
	// AttrStrangePart1 represents the first applied Strange Part slot.
	AttrStrangePart1 uint32 = 380
	// AttrStrangePart2 represents the second applied Strange Part slot.
	AttrStrangePart2 uint32 = 382
	// AttrStrangePart3 represents the third applied Strange Part slot.
	AttrStrangePart3 uint32 = 384
	// AttrCannotCraftVariant represents the crafting-restriction attribute variation.
	AttrCannotCraftVariant uint32 = 449
	// AttrEOTLEarlySupporter represents the End of the Line supporter tag.
	AttrEOTLEarlySupporter uint32 = 703
	// AttrWear represents the skin pattern wear float value.
	AttrWear uint32 = 725
	// AttrPaintkit represents the skin pattern index identifier.
	AttrPaintkit uint32 = 834
	// AttrSpell1 represents the first applied Halloween spell effect slot.
	AttrSpell1 uint32 = 1004
	// AttrSpell2 represents the second applied Halloween spell effect slot.
	AttrSpell2 uint32 = 1005
	// AttrSpell3 represents the third applied Halloween spell effect slot.
	AttrSpell3 uint32 = 1006
	// AttrSpell4 represents the fourth applied Halloween spell effect slot.
	AttrSpell4 uint32 = 1007
	// AttrSpell5 represents the fifth applied Halloween spell effect slot.
	AttrSpell5 uint32 = 1008
	// AttrSpell6 represents the sixth applied Halloween spell effect slot.
	AttrSpell6 uint32 = 1009
	// AttrTarget represents the target item defindex for tools.
	AttrTarget uint32 = 2012
	// AttrKillstreaker represents the professional killstreak eye effect index.
	AttrKillstreaker uint32 = 2013
	// AttrSheen represents the specialized killstreak weapon sheen effect index.
	AttrSheen uint32 = 2014
	// AttrKillstreakTier represents the active killstreak tier level.
	AttrKillstreakTier uint32 = 2025
	// AttrAustralium represents the Australium weapon quality attribute flag.
	AttrAustralium uint32 = 2027
	// AttrSeries represents the secondary series counter (e.g. Duck Journal).
	AttrSeries uint32 = 2031
	// AttrTauntUnusualEffect represents the Unusual particle effect for taunts.
	AttrTauntUnusualEffect uint32 = 2041
	// AttrQuestLoanerIDLow represents the low 32-bit of the linked contract ID.
	AttrQuestLoanerIDLow uint32 = 2051
	// AttrQuestLoanerIDHigh represents the high 32-bit of the linked contract ID.
	AttrQuestLoanerIDHigh uint32 = 2052
	// AttrFestivized represents the festivizer modification attribute flag.
	AttrFestivized uint32 = 2053
)

// Killstreak tiers configuration.
const (
	// KillstreakTierNone indicates no active killstreak.
	KillstreakTierNone uint32 = iota
	// KillstreakTierBasic indicates standard killstreak kit.
	KillstreakTierBasic
	// KillstreakTierSpecialized indicates specialized killstreak kit with weapon sheen.
	KillstreakTierSpecialized
	// KillstreakTierProfessional indicates professional killstreak kit with weapon sheen and eye effects.
	KillstreakTierProfessional
)

// Item Origins configuration.
const (
	// OriginDrop represents a random item drop.
	OriginDrop uint32 = 0
	// OriginAchievement represents an achievement unlock award.
	OriginAchievement uint32 = 1
	// OriginPurchase represents an item purchased from store.
	OriginPurchase uint32 = 2
	// OriginStorePromo represents store promotion item.
	OriginStorePromo uint32 = 5
	// OriginSupport represents item granted by Steam Support.
	OriginSupport uint32 = 7
	// OriginHalloween represents a Halloween cauldron or drop reward.
	OriginHalloween uint32 = 12
	// OriginForeign represents an unverified source.
	OriginForeign uint32 = 14
	// OriginPreview represents a temporary store preview item.
	OriginPreview uint32 = 17
	// OriginWorkshop represents a Steam Workshop contribution award.
	OriginWorkshop uint32 = 18
	// OriginLoaner represents a temporary contract loaner item.
	OriginLoaner uint32 = 24
)

// Quality configuration values.
const (
	// QualityNormal represents Stock quality items.
	QualityNormal uint32 = 0
	// QualityGenuine represents Genuine quality promo items.
	QualityGenuine uint32 = 1
	// QualityVintage represents Vintage quality items.
	QualityVintage uint32 = 3
	// QualityUnusual represents Unusual quality cosmetic items.
	QualityUnusual uint32 = 5
	// QualityUnique represents Unique quality standard items.
	QualityUnique uint32 = 6
	// QualityCommunity represents Community quality contribution items.
	QualityCommunity uint32 = 7
	// QualityValve represents Valve developer quality items.
	QualityValve uint32 = 8
	// QualitySelfMade represents Self-Made creator quality items.
	QualitySelfMade uint32 = 9
	// QualityCustomized represents Customized quality items.
	QualityCustomized uint32 = 10
	// QualityStrange represents Strange quality counting items.
	QualityStrange uint32 = 11
	// QualityCompleted represents Completed quality items.
	QualityCompleted uint32 = 12
	// QualityHaunted represents Haunted quality Halloween items.
	QualityHaunted uint32 = 13
	// QualityCollectors represents Collector's quality compiled items.
	QualityCollectors uint32 = 14
	// QualityDecorated represents Decorated quality skins.
	QualityDecorated uint32 = 15
)

// Item represents a parsed and normalized TF2 item with its economic and technical metadata.
type Item struct {
	// ID represents the unique asset identifier of the item.
	ID uint64
	// OriginalID represents the permanent origin identifier assigned upon creation.
	OriginalID uint64
	// AccountID represents the Steam Account ID of the current owner.
	AccountID uint32
	// DefIndex represents the item definition index from the schema.
	DefIndex uint32
	// Level represents the cosmetic item level.
	Level uint32
	// Quality represents the quality ID of the item.
	Quality uint32
	// Inventory represents the raw inventory positioning bits.
	Inventory uint32
	// Quantity represents the stack size of the item.
	Quantity uint32
	// Origin represents the item drop, craft, or purchase source ID.
	Origin uint32
	// Flags represents the bitmask restrictions applied to the item.
	Flags EconItemFlag
	// Style represents the selected cosmetic style index.
	Style uint32
	// InUse indicates whether the item is currently equipped or in use.
	InUse bool

	// CustomName represents the custom text applied by a name tag.
	CustomName string
	// CustomDesc represents the custom text applied by a description tag.
	CustomDesc string
	// SKU represents the canonical SKU string for pricing.
	SKU string

	// IsTradable indicates whether the item can be traded.
	IsTradable bool
	// IsMarketable indicates whether the item can be listed on the Steam Market.
	IsMarketable bool
	// IsCraftable indicates whether the item can be used in crafting recipes.
	IsCraftable bool

	// Effect represents the Unusual particle effect ID.
	Effect uint32
	// KillstreakTier represents the active killstreak tier index.
	KillstreakTier uint32
	// Australium indicates whether the item is an Australium variant.
	Australium bool
	// Festivized indicates whether a festivizer has been applied.
	Festivized bool
	// Wear represents the skin wear fraction value.
	Wear float32
	// Paintkit represents the skin pattern ID.
	Paintkit uint32
	// CrateSeries represents the series number for supply crates.
	CrateSeries uint32
	// Paint represents the applied paint color ID.
	Paint uint32

	// Sheen represents the killstreak sheen effect index.
	Sheen uint32
	// Killstreaker represents the professional killstreak eye effect index.
	Killstreaker uint32
	// CraftNumber represents the limited edition craft number.
	CraftNumber uint32
	// Series represents the secondary series ID.
	Series uint32
	// MedalNumber represents the number assigned to tournament medals.
	MedalNumber uint32
	// Target represents the defindex of the target item.
	Target uint32
	// IsElevated indicates whether the item is an elevated strange cosmetic.
	IsElevated bool
	// EarlySupporter indicates whether the item has the early supporter tag.
	EarlySupporter bool
	// QuestID represents the 64-bit ID of the linked contract.
	QuestID uint64
	// IsBuggedLoaner indicates whether the loaner is exceptionally tradable.
	IsBuggedLoaner bool
	// Spells contains the list of applied Halloween spells.
	Spells []sku.Spell
	// Parts contains the list of Strange Part IDs.
	Parts []uint32
}

// Position returns the item's slot index in the backpack.
func (i *Item) Position() uint32 {
	return i.Inventory & 0xFFFF
}

// GetSchema returns the [schema.Item] metadata for the item.
func (i *Item) GetSchema(s *schema.Schema) *schema.Item {
	return s.ItemByDef(int(i.DefIndex))
}

// IsWeapon returns true if the item is classified as a weapon.
func (i *Item) IsWeapon(s *schema.Schema) bool {
	sch := i.GetSchema(s)
	return sch != nil && sch.CraftClass == "weapon"
}

// ToEconItem maps the item fields into a universal exchange [trading.Item] format.
func (i Item) ToEconItem() *trading.Item {
	return &trading.Item{
		AppID:          AppID,
		ContextID:      2,
		AssetID:        i.ID,
		Amount:         int64(i.Quantity),
		Name:           i.CustomName,
		MarketName:     i.CustomName,
		MarketHashName: i.CustomName,
		Tradable:       i.IsTradable,
		Marketable:     i.IsMarketable,
	}
}

// ToSKUObject converts the item parameters to a [sku.Item] representation.
func (i Item) ToSKUObject() *sku.Item {
	quality := int(i.Quality)
	quality2 := 0

	if i.IsElevated && i.Quality != QualityStrange {
		quality2 = 11
	}

	if i.Effect != 0 && i.Quality == 11 && i.Paintkit == 0 {
		quality = 5
		quality2 = 11
	}

	return &sku.Item{
		Defindex:    int(i.DefIndex),
		Quality:     quality,
		Quality2:    quality2,
		Tradable:    i.IsTradable,
		Craftable:   i.IsCraftable,
		Killstreak:  int(i.KillstreakTier),
		Australium:  i.Australium,
		Effect:      int(i.Effect),
		Festivized:  i.Festivized,
		Paintkit:    int(i.Paintkit),
		Wear:        int(i.Wear * 100),
		Craftnumber: int(i.CraftNumber),
		Crateseries: int(i.CrateSeries),
		Target:      int(i.Target),
		Paint:       int(i.Paint),
		Spells:      i.Spells,
		Parts: func() []int {
			p := make([]int, len(i.Parts))
			for idx, v := range i.Parts {
				p[idx] = int(v)
			}

			return p
		}(),
	}
}

// GetSKU retrieves or generates the standard SKU string for the item.
func (i *Item) GetSKU(s *schema.Schema) string {
	if i.SKU != "" {
		return i.SKU
	}

	return s.SKUFromItem(i.ToSKUObject())
}

// Fix applies schema-based normalizations and defindex overrides to the item.
func (i *Item) Fix(s *schema.Schema) {
	sch := i.GetSchema(s)
	if sch == nil {
		return
	}

	if (i.DefIndex >= 5726 && i.DefIndex <= 5733) ||
		(i.DefIndex >= 5743 && i.DefIndex <= 5751) ||
		(i.DefIndex >= 5793 && i.DefIndex <= 5801) {
		i.DefIndex = 6527
	}

	strangifiers := []uint32{
		5661, 5721, 5722, 5723, 5724, 5725, 5753, 5754, 5755, 5756, 5757, 5758, 5759, 5783, 5784, 5804,
	}
	if slices.Contains(strangifiers, i.DefIndex) {
		i.DefIndex = 6522
	}

	if i.DefIndex >= 20001 && i.DefIndex <= 20009 {
		i.DefIndex = 20000
	}

	if sch.ItemClass == "supply_crate" && i.CrateSeries == 0 {
		for _, attr := range sch.Attributes {
			if attr.Name == "set supply crate series" {
				i.CrateSeries = uint32(attr.Value)
				break
			}
		}
	}
}

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
type Option = bus.Option[*SOCache]

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

	items     map[uint64]*Item
	slots     uint32
	isPremium bool

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
		items:   make(map[uint64]*Item),
		ratings: make(map[int32]uint32),
		coord:   coord,
		logger:  log.Discard,
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.bus == nil {
		s.bus = bus.New()
	}

	return s
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
	for _, item := range c.items {
		list = append(list, item)
	}

	return list
}

// GetItem returns the [Item] matching the specified asset ID.
func (c *SOCache) GetItem(id uint64) (*Item, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[id]

	return item, ok
}

// GetMetal returns up to count tradable metal item IDs matching the specified defIndex.
func (c *SOCache) GetMetal(defIndex uint32, count int) []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var ids []uint64

	for _, item := range c.items {
		if item.DefIndex == defIndex && item.IsTradable {
			ids = append(ids, item.ID)
			if len(ids) == count {
				return ids
			}
		}
	}

	return ids
}

// FindCraftableItems returns up to count tradable and craftable item IDs matching the defIndex.
func (c *SOCache) FindCraftableItems(defIndex uint32, count int) []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var ids []uint64

	for id, item := range c.items {
		if item.DefIndex == defIndex && item.IsTradable {
			ids = append(ids, id)
			if count > 0 && len(ids) == count {
				return ids
			}
		}
	}

	return ids
}

// FindWeaponsByClass returns all tradable weapons usable by the specified character class.
func (c *SOCache) FindWeaponsByClass(class string) []*Item {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Item

	for _, item := range c.items {
		sch := item.GetSchema(c.schema)
		if sch != nil && sch.CraftClass == "weapon" && item.IsTradable {
			if slices.Contains(sch.UsedByClasses, class) {
				result = append(result, item)
			}
		}
	}

	return result
}

// GetMetalCount returns the count of tradable metal items matching the defIndex.
func (c *SOCache) GetMetalCount(defIndex uint32) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0

	for _, item := range c.items {
		if item.DefIndex == defIndex && item.IsTradable {
			count++
		}
	}

	return count
}

// GetAssetIDsBySKU returns up to limit item IDs matching the target SKU.
func (c *SOCache) GetAssetIDsBySKU(targetSKU string, limit int) []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []uint64

	for _, item := range c.items {
		if item.SKU == targetSKU && item.IsTradable {
			result = append(result, item.ID)
		}
	}

	return result
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

	if gcVersion != ourVersion {
		c.logger.Warn("SOCache desync detected. Requesting refresh...",
			log.Uint64("expected", gcVersion),
			log.Uint64("actual", ourVersion),
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
			item.SKU = item.GetSKU(c.schema)
		}

		_, exists := c.items[item.ID]
		c.items[item.ID] = item

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

	for _, attr := range p.GetAttribute() {
		def := attr.GetDefIndex()
		val := attr.GetValueBytes()

		switch def {
		case AttrCustomName:
			item.CustomName = string(val)
		case AttrCustomDesc:
			item.CustomDesc = string(val)
		case AttrMedalNumber:
			item.MedalNumber = getUint(val)
		case AttrUnusualEffect:
			item.Effect = uint32(getFloat(val))
		case AttrPaintPrimary, AttrPaintSecondary:
			item.Paint = uint32(getFloat(val))
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
			}

		case AttrKillEater:
			item.IsElevated = true
		case AttrCraftNumber:
			item.CraftNumber = getUint(val)
		case AttrStrangePart1, AttrStrangePart2, AttrStrangePart3:
			item.Parts = append(item.Parts, uint32(getFloat(val)))
		case AttrCannotCraftVariant:
			item.IsCraftable = false
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
		}
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
