// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"context"
	"testing"
	"time"

	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/community/inventory"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/test/module"
	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

type mockCache struct {
	items    []*tf2.Item
	maxSlots int
}

func (m *mockCache) GetItems() []*tf2.Item {
	return m.items
}

func (m *mockCache) GetItem(id uint64) (*tf2.Item, bool) {
	for _, it := range m.items {
		if it.ID == id {
			return it, true
		}
	}

	return nil, false
}

func (m *mockCache) GetMaxSlots() int {
	return m.maxSlots
}

type mockSchemaProvider struct {
	s *schema.Schema
}

func (m *mockSchemaProvider) Get() *schema.Schema {
	return m.s
}

type mockTradingProvider struct {
	offers []trading.TradeOffer
}

func (m *mockTradingProvider) GetActiveSentOffers(ctx context.Context) ([]trading.TradeOffer, error) {
	return m.offers, nil
}

func mockSchemaForSmelting() *schema.Schema {
	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{
			Defindex:      1,
			ItemQuality:   6,
			CraftClass:    "weapon",
			UsedByClasses: []string{"Scout"},
		},
		{
			Defindex:      3,
			ItemQuality:   6,
			CraftClass:    "weapon",
			UsedByClasses: []string{"Soldier"},
		},
		{
			Defindex:      160,
			ItemQuality:   6,
			CraftClass:    "weapon",
			UsedByClasses: []string{"Scout"},
		},
	}

	return schema.New(raw)
}

func TestBackpack_LockItems_ValidIDs_LocksState(t *testing.T) {
	t.Parallel()

	bp := New()

	assert.Empty(t, bp.GetLockedAssetIDs())

	bp.LockItems([]uint64{100, 200})
	locked := bp.GetLockedAssetIDs()
	assert.ElementsMatch(t, []uint64{100, 200}, locked)

	bp.UnlockItems([]uint64{100})
	locked = bp.GetLockedAssetIDs()
	assert.ElementsMatch(t, []uint64{200}, locked)
}

func TestBackpack_GetPureStock_ValidCache_ReturnsCorrectStock(t *testing.T) {
	t.Parallel()

	mock := &mockCache{
		items: []*tf2.Item{
			{ID: 1, DefIndex: 5021, IsTradable: true},
			{ID: 2, DefIndex: 5021, IsTradable: false},
			{ID: 3, DefIndex: 5002, IsTradable: true},
			{ID: 4, DefIndex: 5002, IsTradable: true},
			{ID: 5, DefIndex: 5001, IsTradable: true},
			{ID: 6, DefIndex: 5000, IsTradable: true},
			{ID: 7, DefIndex: 123, IsTradable: true},
		},
	}

	bp := New()
	bp.cache = mock

	stock := bp.GetPureStock()

	expected := currency.PureStock{
		Keys:      1,
		Refined:   2,
		Reclaimed: 1,
		Scrap:     1,
	}

	assert.Equal(t, expected, stock)
}

func TestBackpack_GetAssetIDs_FilteringAndLocking_ReturnsAvailableIDs(t *testing.T) {
	t.Parallel()

	mock := &mockCache{
		items: []*tf2.Item{
			{ID: 1, IsTradable: true, SKU: "target_sku"},
			{ID: 2, IsTradable: true, SKU: "target_sku"},
			{ID: 3, IsTradable: false, SKU: "target_sku"},
			{ID: 4, IsTradable: true, SKU: "other_sku"},
		},
	}

	bp := New()
	bp.cache = mock
	bp.manager = &mockSchemaProvider{s: &schema.Schema{}}

	bp.LockItems([]uint64{2})

	ids := bp.GetAssetIDs("target_sku")
	assert.ElementsMatch(t, []uint64{1}, ids)
}

func TestBackpack_HandleEvent_FullBackpack_PublishesFullEvent(t *testing.T) {
	t.Parallel()

	mock := &mockCache{maxSlots: 10}
	bp := New()
	bp.cache = mock
	bp.Logger = log.Discard
	bp.Bus = bus.New()

	for i := range 10 {
		mock.items = append(mock.items, &tf2.Item{ID: uint64(i)})
	}

	events := bp.handleEvent(t.Context(), &tf2.ItemAcquiredEvent{Item: &tf2.Item{ID: 999}})
	assert.Len(t, events, 1)
	assert.IsType(t, &FullEvent{}, events[0])
}

func TestBackpack_HandleEvent_OtherEvents_DoesNotPanic(t *testing.T) {
	t.Parallel()

	bp := New()
	bp.Logger = log.Discard

	events := bp.handleEvent(t.Context(), &tf2.ItemRemovedEvent{ItemID: 123})
	assert.Empty(t, events)
}

func TestBackpack_ApplyLayout_SchemaNotReady_ReturnsError(t *testing.T) {
	t.Parallel()

	bp := New()
	bp.manager = schema.NewManager(schema.Config{})

	err := bp.ApplyLayout(t.Context(), Layout{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "schema not ready")
}

func TestBackpack_ApplyLayout_SuccessfulEmptyMoves_ReturnsNil(t *testing.T) {
	t.Parallel()

	s := mockSchema()
	mock := &mockCache{
		items: []*tf2.Item{
			{ID: 10, DefIndex: 1, Quality: 6, Inventory: 1},
			{ID: 20, DefIndex: 5000, Quality: 6, Inventory: 2},
		},
	}

	bp := New()
	bp.cache = mock
	bp.manager = &mockSchemaProvider{s: s}
	bp.Logger = log.Discard

	layout := Layout{
		Pages: map[int]PageLayout{
			1: {
				Filters: []Filter{
					BySKU("1;6"),
					BySKU("5000;6"),
				},
			},
		},
	}

	err := bp.ApplyLayout(t.Context(), layout)
	assert.NoError(t, err)
}

func TestBackpack_GetItemsBySKU_ValidSKU_ReturnsMatchingIDs(t *testing.T) {
	t.Parallel()

	mock := &mockCache{
		items: []*tf2.Item{
			{ID: 1, SKU: "target_sku"},
			{ID: 2, SKU: "other_sku"},
		},
	}

	bp := New()
	bp.cache = mock
	bp.manager = &mockSchemaProvider{s: &schema.Schema{}}

	ids := bp.GetItemsBySKU("target_sku")
	assert.ElementsMatch(t, []uint64{1}, ids)
}

func TestBackpack_CleanupStaleLocks_StaleLocks_RemovesObsoleteLocks(t *testing.T) {
	t.Parallel()

	bp := New()
	bp.LockItems([]uint64{1, 2, 3})

	mockTrading := &mockTradingProvider{
		offers: []trading.TradeOffer{
			{
				ItemsToGive: []*trading.Item{
					{AssetID: 1},
				},
			},
		},
	}

	bp.cleanupStaleLocks(t.Context(), mockTrading)

	locked := bp.GetLockedAssetIDs()
	assert.Contains(t, locked, uint64(1))
	assert.NotContains(t, locked, uint64(2))
	assert.NotContains(t, locked, uint64(3))
}

func TestBackpack_FindWeaponsByClass_ValidWeapons_ReturnsMatching(t *testing.T) {
	t.Parallel()

	s := mockSchemaForSmelting()
	mock := &mockCache{
		items: []*tf2.Item{
			{ID: 1, DefIndex: 1, IsCraftable: true, IsTradable: true},
			{ID: 2, DefIndex: 3, IsCraftable: true, IsTradable: true},
		},
	}

	bp := New()
	bp.cache = mock
	bp.manager = &mockSchemaProvider{s: s}

	weapons := bp.FindWeaponsByClass("Scout")
	assert.Len(t, weapons, 1)
	assert.Equal(t, uint64(1), weapons[0].ID)
}

func TestBackpack_FindCraftableItems_CountAndLockScenarios_ExpectedItemIDs(t *testing.T) {
	t.Parallel()

	mock := &mockCache{
		items: []*tf2.Item{
			{ID: 1, DefIndex: 1, IsCraftable: true},
			{ID: 2, DefIndex: 1, IsCraftable: true},
			{ID: 3, DefIndex: 1, IsCraftable: false},
			{ID: 4, DefIndex: 1, IsCraftable: true},
		},
	}

	bp := New()
	bp.cache = mock
	bp.LockItems([]uint64{2})

	res1 := bp.FindCraftableItems(1, 1)
	assert.ElementsMatch(t, []uint64{1}, res1)

	res0 := bp.FindCraftableItems(1, 0)
	assert.ElementsMatch(t, []uint64{1, 4}, res0)
}

func TestBackpack_GetMetalCount_ValidDefIndex_ReturnsCount(t *testing.T) {
	t.Parallel()

	mock := &mockCache{
		items: []*tf2.Item{
			{ID: 1, DefIndex: 5000},
			{ID: 2, DefIndex: 5000},
			{ID: 3, DefIndex: 5001},
		},
	}

	bp := New()
	bp.cache = mock

	count := bp.GetMetalCount(5000)
	assert.Equal(t, 2, count)
}

func TestBackpack_FindWeaponsByClassForSmelting_VariousDuplicates_ReturnsExpectedSmeltingPairs(t *testing.T) {
	t.Parallel()

	s := mockSchemaForSmelting()
	mock := &mockCache{
		items: []*tf2.Item{
			{ID: 10, DefIndex: 1, Quality: 6, IsCraftable: true, IsTradable: true},
			{ID: 11, DefIndex: 1, Quality: 6, IsCraftable: true, IsTradable: true},
			{ID: 12, DefIndex: 1, Quality: 6, IsCraftable: true, IsTradable: true},

			{ID: 20, DefIndex: 3, Quality: 6, IsCraftable: true, IsTradable: true},
			{ID: 21, DefIndex: 3, Quality: 6, IsCraftable: true, IsTradable: true},

			{ID: 30, DefIndex: 1, Quality: 11, IsCraftable: true, IsTradable: true},
			{ID: 40, DefIndex: 1, Quality: 6, IsCraftable: false, IsTradable: true},
			{ID: 50, DefIndex: 160, Quality: 6, IsCraftable: true, IsTradable: true},
		},
	}

	bp := New()
	bp.cache = mock
	bp.manager = &mockSchemaProvider{s: s}

	scoutSmelting := bp.FindWeaponsByClassForSmelting("Scout")
	assert.Len(t, scoutSmelting, 2)
	scoutIDs := []uint64{scoutSmelting[0].ID, scoutSmelting[1].ID}
	assert.ElementsMatch(t, []uint64{11, 12}, scoutIDs)

	soldierSmelting := bp.FindWeaponsByClassForSmelting("Soldier")
	assert.Len(t, soldierSmelting, 2)
	soldierIDs := []uint64{soldierSmelting[0].ID, soldierSmelting[1].ID}
	assert.ElementsMatch(t, []uint64{20, 21}, soldierIDs)
}

func TestPositionOf_Calculations_ReturnsCorrectIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		page     int
		slot     int
		expected uint32
	}{
		{"first_page_first_slot", 1, 1, 1},
		{"first_page_last_slot", 1, 50, 50},
		{"second_page_first_slot", 2, 1, 51},
		{"third_page_tenth_slot", 3, 10, 110},
		{"zero_bounds_correction", 0, 0, 1},
		{"negative_bounds_correction", -1, -5, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, PositionOf(tt.page, tt.slot))
		})
	}
}

func mockSchemaForCoverage() *schema.Schema {
	raw := &schema.Raw{}
	raw.Schema.Items = []*schema.Item{
		{
			Defindex:      13, // Australium Scattergun
			ItemQuality:   6,
			ItemName:      "Paint Can",
			UsedByClasses: []string{"Scout"},
			Attributes: []schema.ItemAttribute{
				{Name: "set_item_tint_value", Value: 8421376},
			},
		},
	}
	raw.Schema.AttributeControlledAttachedParticles = []*schema.ParticleEffect{
		{ID: 17, Name: "Sunbeams"},
	}
	raw.Schema.PaintKits = map[string]string{
		"10": "Sunbeams Skin",
	}

	return schema.New(raw)
}

func TestCoverage_MapCEconToTF2_Extra(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()

	// 1. desc == nil
	econNilDesc := inventory.CEconItem{
		Asset:       inventory.Asset{AssetID: "100", Amount: "abc"},
		Description: nil,
	}
	itemNilDesc := mapCEconToTF2(econNilDesc, s)
	assert.Equal(t, uint64(100), itemNilDesc.ID)

	// 2. defindex == 0 or quality == 0 tags matching Category "Quality"
	econTags := inventory.CEconItem{
		Asset: inventory.Asset{AssetID: "101", Amount: "2"},
		Description: &inventory.Description{
			Tags: []inventory.Tag{
				{Category: "Quality", LocalizedTagName: "Unique"},
			},
		},
	}
	itemTags := mapCEconToTF2(econTags, s)
	assert.Equal(t, 2, itemTags.Quantity)

	// 3. Various descriptions: Exterior, Effect, Killstreak, Paint, Crate, Strange Parts, Spells, Festive
	econDesc := inventory.CEconItem{
		Asset: inventory.Asset{AssetID: "102"},
		Description: &inventory.Description{
			Name:           "Festivized Sunbeams Rocket Launcher",
			MarketHashName: "Australium Rocket Launcher",
			Descriptions: []struct {
				Value string `json:"value"`
				Color string `json:"color,omitempty"`
			}{
				{Value: "Exterior: Field-Tested"},
				{Value: "★ Unusual Effect: Sunbeams"},
				{Value: "Killstreak Active: Specialized"},
				{Value: "Killstreak Active: Professional"},
				{Value: "Killstreak Active: Killstreak"},
				{Value: "Paint Color: Distinctive Lack of Hue"},
				{Value: "Crate Series #85"},
				{Value: "Strange Stat: Kills: 0"},
				{Value: "Strange Part: Robots Destroyed: 0", Color: "756b5e"},
				{Value: "Exorcism", Color: "7ea9d1"},
			},
			AppData: map[string]any{
				"def_index": "13",
				"quality":   "11", // Strange
			},
		},
	}

	itemDesc := mapCEconToTF2(econDesc, s)
	assert.Equal(t, uint64(102), itemDesc.ID)

	// 4. Quality Decorated (15) for Paint Kits, Native Festive (654), and AppData attributes with Australium
	econExtra := inventory.CEconItem{
		Asset: inventory.Asset{AssetID: "103"},
		Description: &inventory.Description{
			Name:           "Festivized Sunbeams Skin",
			MarketHashName: "Sunbeams Skin",
			AppData: map[string]any{
				"def_index": "654", // Native Festive
				"quality":   "15",  // Decorated
				"attributes": map[string]any{
					"2027": map[string]any{}, // Australium attribute
				},
			},
		},
	}

	itemExtra := mapCEconToTF2(econExtra, s)
	assert.Equal(t, uint64(103), itemExtra.ID)
}

func TestCoverage_TrivialGettersAndMethods(t *testing.T) {
	t.Parallel()

	mockC := &mockCache{}
	mockS := &mockSchemaProvider{}

	bp := New()
	bp.cache = mockC
	bp.manager = mockS

	// 1. Getters
	assert.Equal(t, mockC, bp.Cache())
	assert.Equal(t, mockS, bp.Schema())

	item, exists := bp.GetItem(123)
	assert.False(t, exists)
	assert.Nil(t, item)

	assert.Equal(t, 0, bp.GetTotalCount())

	assert.Equal(t, 0, bp.GetStock("5021;6"))
}

func TestCoverage_GetStock_Valid(t *testing.T) {
	t.Parallel()

	s := mockSchemaForCoverage()
	mockC := &mockCache{
		items: []*tf2.Item{
			{ID: 1, DefIndex: 13, Quality: 6, SKU: "13;6"},
		},
	}
	mockS := &mockSchemaProvider{s: s}

	bp := New()
	bp.cache = mockC
	bp.manager = mockS

	// Non-empty manager, matching SKU
	assert.Equal(t, 1, bp.GetStock("13;6"))
}

func TestCoverage_RemoteOptions(t *testing.T) {
	t.Parallel()

	inv := NewRemote(12345, nil, WithLogger(log.Discard), WithCommunityBackoff(nil))
	assert.NotNil(t, inv)
}

func TestCoverage_ModuleWithAndFrom(t *testing.T) {
	cfg := steam.DefaultConfig()

	client, err := steam.NewClient(cfg, WithModule())
	if err != nil {
		t.Skipf("steam.NewClient failed, skipping: %v", err)
		return
	}

	bp := From(client)
	assert.NotNil(t, bp)
}

func TestCoverage_Init_Errors(t *testing.T) {
	t.Parallel()

	// Missing tf2
	{
		ictx := module.NewInitContext()
		schemaMod := schema.NewManager(schema.DefaultConfig())
		ictx.SetModule(schema.ModuleName, schemaMod)

		bp := New()
		err := bp.Init(ictx)
		assert.Error(t, err)
	}

	// Missing schema
	{
		ictx := module.NewInitContext()
		tf2Mod := tf2.New()
		ictx.SetModule(tf2.ModuleName, tf2Mod)

		bp := New()
		err := bp.Init(ictx)
		assert.Error(t, err)
	}
}

func TestCoverage_InitAndStartAuthed(t *testing.T) {
	ictx := module.NewInitContext()

	tf2Mod := tf2.New()
	schemaMod := schema.NewManager(schema.DefaultConfig())

	ictx.SetModule(tf2.ModuleName, tf2Mod)
	ictx.SetModule(schema.ModuleName, schemaMod)

	bp := New()
	err := bp.Init(ictx)
	assert.NoError(t, err)
	assert.Equal(t, tf2Mod, bp.tf2)
	assert.Equal(t, schemaMod, bp.manager)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	authCtx := module.NewAuthContext(7656119)

	err = bp.StartAuthed(ctx, authCtx)
	assert.NoError(t, err)

	// Wait briefly for go routines to spin and shut down on context timeout
	time.Sleep(20 * time.Millisecond)
}

func TestCoverage_StartAuthed_WithTrading(t *testing.T) {
	ictx := module.NewInitContext()

	tf2Mod := tf2.New()
	schemaMod := schema.NewManager(schema.DefaultConfig())

	ictx.SetModule(tf2.ModuleName, tf2Mod)
	ictx.SetModule(schema.ModuleName, schemaMod)

	bp := New()
	err := bp.Init(ictx)
	assert.NoError(t, err)

	bp.trading = &mockTradingProvider{}

	ctx, cancel := context.WithCancel(context.Background())
	authCtx := module.NewAuthContext(7656119)

	err = bp.StartAuthed(ctx, authCtx)
	assert.NoError(t, err)

	// Wait a tiny bit then cancel context to cover the <-ctx.Done() case
	time.Sleep(5 * time.Millisecond)
	cancel()
	time.Sleep(10 * time.Millisecond)
}
