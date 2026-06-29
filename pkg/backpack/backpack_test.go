// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"context"
	"errors"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/test/mock"
	"github.com/lemon4ksan/miyako/bus"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	pb "github.com/lemon4ksan/g-man-tf2/pkg/protobuf/tf2"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// Shared mock definitions used across all backpack tests
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
	err    error
}

func (m *mockTradingProvider) GetActiveSentOffers(ctx context.Context) ([]trading.TradeOffer, error) {
	if m.err != nil {
		return nil, m.err
	}

	return m.offers, nil
}

type mockLogRecorder struct {
	messages []string
}

func (m *mockLogRecorder) Debug(msg string, fields ...log.Field) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogRecorder) DebugContext(ctx context.Context, msg string, fields ...log.Field) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogRecorder) Info(msg string, fields ...log.Field) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogRecorder) InfoContext(ctx context.Context, msg string, fields ...log.Field) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogRecorder) Warn(msg string, fields ...log.Field) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogRecorder) WarnContext(ctx context.Context, msg string, fields ...log.Field) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogRecorder) Error(msg string, fields ...log.Field) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogRecorder) ErrorContext(ctx context.Context, msg string, fields ...log.Field) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogRecorder) With(fields ...log.Field) log.Logger {
	return m
}

func (m *mockLogRecorder) Close() error {
	return nil
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

func TestPositionOf(t *testing.T) {
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

func TestBackpack_LockAndUnlock(t *testing.T) {
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

func TestBackpack_GetPureStock(t *testing.T) {
	t.Parallel()

	t.Run("without_logger", func(t *testing.T) {
		mockC := &mockCache{
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
		bp.cache = mockC

		stock := bp.GetPureStock()
		expected := currency.PureStock{
			Keys:      1,
			Refined:   2,
			Reclaimed: 1,
			Scrap:     1,
		}
		assert.Equal(t, expected, stock)
	})

	t.Run("with_logger_and_untradable_ref", func(t *testing.T) {
		mockC := &mockCache{
			items: []*tf2.Item{
				{ID: 1, DefIndex: 5002, IsTradable: true},
				{ID: 2, DefIndex: 5002, IsTradable: false},
			},
		}
		bp := New()
		bp.cache = mockC

		loggerRecorder := &mockLogRecorder{}
		bp.Logger = loggerRecorder

		stock := bp.GetPureStock()
		assert.Equal(t, int(1), int(stock.Refined))

		require.NotEmpty(t, loggerRecorder.messages)
		assert.Contains(t, loggerRecorder.messages[0], "Pure stock metal count statistics")
		assert.Contains(t, loggerRecorder.messages[1], "Untradable refined sample details")
	})
}

func TestBackpack_GetAssetIDs(t *testing.T) {
	t.Parallel()

	mockC := &mockCache{
		items: []*tf2.Item{
			{ID: 1, IsTradable: true, SKU: "target_sku"},
			{ID: 2, IsTradable: true, SKU: "target_sku"},
			{ID: 3, IsTradable: false, SKU: "target_sku"},
			{ID: 4, IsTradable: true, SKU: "other_sku"},
		},
	}

	bp := New()
	bp.cache = mockC
	bp.manager = &mockSchemaProvider{s: &schema.Schema{}}

	bp.LockItems([]uint64{2})

	ids := bp.GetAssetIDs("target_sku")
	assert.ElementsMatch(t, []uint64{1}, ids)
}

func TestBackpack_FindCraftableItems(t *testing.T) {
	t.Parallel()

	mockC := &mockCache{
		items: []*tf2.Item{
			{ID: 1, DefIndex: 1, IsCraftable: true},
			{ID: 2, DefIndex: 1, IsCraftable: true},
			{ID: 3, DefIndex: 1, IsCraftable: false},
			{ID: 4, DefIndex: 1, IsCraftable: true},
		},
	}

	bp := New()
	bp.cache = mockC
	bp.LockItems([]uint64{2})

	res1 := bp.FindCraftableItems(1, 1)
	assert.ElementsMatch(t, []uint64{1}, res1)

	res0 := bp.FindCraftableItems(1, 0)
	assert.ElementsMatch(t, []uint64{1, 4}, res0)
}

func TestBackpack_GetMetalCount(t *testing.T) {
	t.Parallel()

	mockC := &mockCache{
		items: []*tf2.Item{
			{ID: 1, DefIndex: 5000},
			{ID: 2, DefIndex: 5000},
			{ID: 3, DefIndex: 5001},
		},
	}

	bp := New()
	bp.cache = mockC

	count := bp.GetMetalCount(5000)
	assert.Equal(t, 2, count)
}

func TestBackpack_FindWeaponsByClass(t *testing.T) {
	t.Parallel()

	s := mockSchemaForSmelting()
	mockC := &mockCache{
		items: []*tf2.Item{
			{ID: 1, DefIndex: 1, IsCraftable: true, IsTradable: true},
			{ID: 2, DefIndex: 3, IsCraftable: true, IsTradable: true},
		},
	}

	bp := New()
	bp.cache = mockC
	bp.manager = &mockSchemaProvider{s: s}

	weapons := bp.FindWeaponsByClass("Scout")
	require.Len(t, weapons, 1)
	assert.Equal(t, uint64(1), weapons[0].ID)
}

func TestBackpack_FindWeaponsByClassForSmelting(t *testing.T) {
	t.Parallel()

	s := mockSchemaForSmelting()

	t.Run("schema_nil", func(t *testing.T) {
		bp := New()
		bp.manager = &mockSchemaProvider{s: nil}
		res := bp.FindWeaponsByClassForSmelting("Scout")
		assert.Nil(t, res)
	})

	t.Run("various_skips", func(t *testing.T) {
		mockC := &mockCache{
			items: []*tf2.Item{
				{ID: 1, DefIndex: 1, IsCraftable: false, IsTradable: true},
				{ID: 2, DefIndex: 1, IsCraftable: true, IsTradable: false},
				{ID: 3, DefIndex: 999, IsCraftable: true, IsTradable: true},
				{ID: 4, DefIndex: 1, Quality: 5, IsCraftable: true, IsTradable: true},
				{ID: 5, DefIndex: 1, Quality: 6, IsElevated: true, IsCraftable: true, IsTradable: true},
				{ID: 6, DefIndex: 1, Quality: 6, KillstreakTier: 1, IsCraftable: true, IsTradable: true},
				{ID: 7, DefIndex: 1, Quality: 6, Paint: 1, IsCraftable: true, IsTradable: true},
				{ID: 8, DefIndex: 1, Quality: 6, Festivized: true, IsCraftable: true, IsTradable: true},
				{ID: 9, DefIndex: 1, Quality: 6, CustomName: "custom", IsCraftable: true, IsTradable: true},
				{ID: 10, DefIndex: 1, Quality: 6, CustomDesc: "desc", IsCraftable: true, IsTradable: true},
				{ID: 11, DefIndex: 1, Quality: 6, Spells: []sku.Spell{{}}, IsCraftable: true, IsTradable: true},
				{ID: 12, DefIndex: 1, Quality: 6, Parts: []uint32{1}, IsCraftable: true, IsTradable: true},
				{ID: 13, DefIndex: 1, Quality: 6, Australium: true, IsCraftable: true, IsTradable: true},
				{ID: 14, DefIndex: 1, Quality: 6, Paintkit: 1, IsCraftable: true, IsTradable: true},
				{ID: 15, DefIndex: 1, Quality: 6, CraftNumber: 1, IsCraftable: true, IsTradable: true},
				{ID: 16, DefIndex: 1, Quality: 6, HasCustomDecal: true, IsCraftable: true, IsTradable: true},
			},
		}
		bp := New()
		bp.cache = mockC
		bp.manager = &mockSchemaProvider{s: s}
		res := bp.FindWeaponsByClassForSmelting("Scout")
		assert.Empty(t, res)
	})

	t.Run("unpaired_duplicate_scenarios", func(t *testing.T) {
		mockC := &mockCache{
			items: []*tf2.Item{
				{ID: 10, DefIndex: 1, Quality: 6, IsCraftable: true, IsTradable: true},
				{ID: 11, DefIndex: 1, Quality: 6, IsCraftable: true, IsTradable: true},
				{ID: 12, DefIndex: 1, Quality: 6, IsCraftable: true, IsTradable: true},
				{ID: 20, DefIndex: 3, Quality: 6, IsCraftable: true, IsTradable: true},
				{ID: 21, DefIndex: 3, Quality: 6, IsCraftable: true, IsTradable: true},
			},
		}
		bp := New()
		bp.cache = mockC
		bp.manager = &mockSchemaProvider{s: s}

		res := bp.FindWeaponsByClassForSmelting("Scout")
		require.Len(t, res, 2)
		assert.Equal(t, uint64(11), res[0].ID)
		assert.Equal(t, uint64(12), res[1].ID)

		raw := &schema.Raw{}
		raw.Schema.Items = []*schema.Item{
			{Defindex: 1, ItemQuality: 6, CraftClass: "weapon", UsedByClasses: []string{"Scout"}},
			{Defindex: 3, ItemQuality: 6, CraftClass: "weapon", UsedByClasses: []string{"Scout"}},
		}
		sCombined := schema.New(raw)
		bp.manager = &mockSchemaProvider{s: sCombined}

		resCombined := bp.FindWeaponsByClassForSmelting("Scout")
		require.Len(t, resCombined, 4)
		assert.Equal(t, uint64(11), resCombined[0].ID)
		assert.Equal(t, uint64(12), resCombined[1].ID)
		assert.Equal(t, uint64(21), resCombined[2].ID)
		assert.Equal(t, uint64(20), resCombined[3].ID)
	})
}

func TestBackpack_ApplyLayout(t *testing.T) {
	t.Parallel()

	t.Run("schema_not_ready", func(t *testing.T) {
		bp := New()
		bp.manager = &mockSchemaProvider{s: nil}
		err := bp.ApplyLayout(t.Context(), Layout{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "schema not ready")
	})

	t.Run("successful_empty_moves_returns_nil", func(t *testing.T) {
		s := mockSchema()
		mockC := &mockCache{
			items: []*tf2.Item{
				{ID: 10, DefIndex: 1, Quality: 6, Inventory: 1},
				{ID: 20, DefIndex: 5000, Quality: 6, Inventory: 2},
			},
		}

		bp := New()
		bp.cache = mockC
		bp.manager = &mockSchemaProvider{s: s}
		bp.Logger = log.Discard

		layout := Layout{
			Sections: []SectionLayout{
				{
					Name: "Test Section",
					Filters: []Filter{
						BySKU("1;6"),
						BySKU("5000;6"),
					},
				},
			},
		}

		err := bp.ApplyLayout(t.Context(), layout)
		assert.NoError(t, err)
	})

	t.Run("page_ranges", func(t *testing.T) {
		s := mockSchema()
		bp := New()
		bp.manager = &mockSchemaProvider{s: s}
		bp.Logger = log.Discard

		t.Run("successful_start_page_placement", func(t *testing.T) {
			mockC := &mockCache{
				items: []*tf2.Item{
					{ID: 10, DefIndex: 1, Quality: 6, SKU: "1;6", Inventory: 51},
					{ID: 20, DefIndex: 5000, Quality: 6, SKU: "5000;6", Inventory: 151},
				},
			}
			bp.cache = mockC

			layout := Layout{
				Sections: []SectionLayout{
					{
						Name:      "Section 1",
						Filters:   []Filter{BySKU("1;6")},
						StartPage: 2,
						EndPage:   2,
					},
					{
						Name:      "Section 2",
						Filters:   []Filter{BySKU("5000;6")},
						StartPage: 4,
					},
				},
			}

			err := bp.ApplyLayout(t.Context(), layout)
			assert.NoError(t, err)
		})

		t.Run("section_overflow_error", func(t *testing.T) {
			items := make([]*tf2.Item, 51)
			for i := range 51 {
				items[i] = &tf2.Item{ID: uint64(i + 1), DefIndex: 1, Quality: 6, SKU: "1;6", Inventory: uint32(i + 1)}
			}

			mockC := &mockCache{items: items}
			bp.cache = mockC

			layout := Layout{
				Sections: []SectionLayout{
					{
						Name:      "Section 1",
						Filters:   []Filter{BySKU("1;6")},
						StartPage: 1,
						EndPage:   1,
					},
				},
			}

			err := bp.ApplyLayout(t.Context(), layout)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "overflowed its allocated page range")
		})
	})
}

func TestBackpack_ApplyLayout_SortersAndUnmoved(t *testing.T) {
	t.Parallel()

	s := mockSchemaForLayout()
	bp := New()
	bp.manager = &mockSchemaProvider{s: s}
	bp.Logger = log.Discard

	mockC := &mockCache{
		items: []*tf2.Item{
			{ID: 10, DefIndex: 1, Quality: 6, SKU: "1;6", Inventory: 1},
		},
	}
	bp.cache = mockC

	layout := Layout{
		Sections: []SectionLayout{
			{
				Name: "Sorted Section",
				Filters: []Filter{
					BySKU("1;6"),
				},
				OrderBy: func(a, b *tf2.Item, s *schema.Schema) int {
					return int(a.ID) - int(b.ID)
				},
			},
		},
	}

	err := bp.ApplyLayout(t.Context(), layout)
	assert.NoError(t, err)
}

func TestBackpack_ApplyLayout_LockedSlotOccupation(t *testing.T) {
	t.Parallel()

	s := mockSchemaForLayout()
	bp := New()
	bp.manager = &mockSchemaProvider{s: s}
	bp.Logger = log.Discard

	mockC := &mockCache{
		items: []*tf2.Item{
			{ID: 10, DefIndex: 5000, Quality: 6, SKU: "5000;6", Inventory: 1},
			{ID: 20, DefIndex: 1, Quality: 6, SKU: "1;6", Inventory: 10},
		},
	}
	bp.cache = mockC
	bp.LockItems([]uint64{10})

	layout := Layout{
		Sections: []SectionLayout{
			{
				Name: "Moves Section",
				Filters: []Filter{
					BySKU("1;6"),
				},
			},
		},
	}

	ictx := mock.NewInitContext()
	appsMod := apps.New()
	tf2Mod := tf2.New()
	schemaMod := schema.NewManager(schema.Config{})

	gcMock := mock.NewGCMock()
	ictx.SetModule("gc", gcMock)
	ictx.SetModule(apps.ModuleName, appsMod)
	ictx.SetModule(schema.ModuleName, schemaMod)
	ictx.SetModule(tf2.ModuleName, tf2Mod)

	err := tf2Mod.Init(ictx)
	require.NoError(t, err)

	bp.tf2 = tf2Mod

	err = bp.ApplyLayout(t.Context(), layout)
	require.NoError(t, err)

	gcMock.AssertSent(t, uint32(pb.EGCItemMsg_k_EMsgGCSetItemPositions))
}

func TestBackpack_CleanupStaleLocks(t *testing.T) {
	t.Parallel()

	t.Run("trading_success", func(t *testing.T) {
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
		bp.Logger = log.Discard

		bp.cleanupStaleLocks(t.Context(), mockTrading)

		locked := bp.GetLockedAssetIDs()
		assert.Contains(t, locked, uint64(1))
		assert.NotContains(t, locked, uint64(2))
		assert.NotContains(t, locked, uint64(3))
	})

	t.Run("trading_error", func(t *testing.T) {
		bp := New()
		bp.LockItems([]uint64{1, 2})

		mockTrading := &mockTradingProvider{
			err: errors.New("provider error"),
		}
		loggerRecorder := &mockLogRecorder{}
		bp.Logger = loggerRecorder

		bp.cleanupStaleLocks(t.Context(), mockTrading)

		locked := bp.GetLockedAssetIDs()
		assert.Len(t, locked, 2)

		require.NotEmpty(t, loggerRecorder.messages)
		assert.Contains(t, loggerRecorder.messages[0], "Failed to get active offers for stale lock cleanup")
	})
}

func TestBackpack_HandleEvent(t *testing.T) {
	t.Parallel()

	t.Run("backpack_full", func(t *testing.T) {
		mockC := &mockCache{maxSlots: 10}
		bp := New()
		bp.cache = mockC
		bp.Logger = log.Discard

		for i := range 10 {
			mockC.items = append(mockC.items, &tf2.Item{ID: uint64(i)})
		}

		events := bp.handleEvent(t.Context(), &tf2.ItemAcquiredEvent{Item: &tf2.Item{ID: 999}})
		require.Len(t, events, 1)
		assert.IsType(t, &FullEvent{}, events[0])
	})

	t.Run("backpack_not_full", func(t *testing.T) {
		mockC := &mockCache{maxSlots: 10}
		bp := New()
		bp.cache = mockC

		events := bp.handleEvent(t.Context(), &tf2.ItemAcquiredEvent{Item: &tf2.Item{ID: 999}})
		assert.Empty(t, events)
	})

	t.Run("unrelated_event", func(t *testing.T) {
		bp := New()
		events := bp.handleEvent(t.Context(), &tf2.ItemRemovedEvent{ItemID: 123})
		assert.Empty(t, events)
	})
}

func TestBackpack_DeleteItem(t *testing.T) {
	t.Parallel()

	ictx := mock.NewInitContext()
	tf2Mod := tf2.New()
	schemaMod := schema.NewManager(schema.Config{})
	appsMod := apps.New()

	gcMock := mock.NewGCMock()
	ictx.SetModule(gc.ModuleName, gcMock)
	ictx.SetModule(apps.ModuleName, appsMod)
	ictx.SetModule(schema.ModuleName, schemaMod)
	ictx.SetModule(tf2.ModuleName, tf2Mod)

	err := tf2Mod.Init(ictx)
	require.NoError(t, err)

	bp := New()
	err = bp.Init(ictx)
	require.NoError(t, err)

	err = bp.DeleteItem(t.Context(), 100)
	require.NoError(t, err)

	gcMock.AssertSentRaw(t, uint32(pb.EGCItemMsg_k_EMsgGCDelete))
}

func TestBackpack_GetItemsBySKU_And_GetStock(t *testing.T) {
	t.Parallel()

	t.Run("s_nil", func(t *testing.T) {
		bp := New()
		bp.manager = &mockSchemaProvider{s: nil}
		assert.Nil(t, bp.GetItemsBySKU("1;6"))
		assert.Equal(t, 0, bp.GetStock("1;6"))
	})

	t.Run("s_not_nil", func(t *testing.T) {
		s := mockSchemaForLayout()
		mockC := &mockCache{
			items: []*tf2.Item{
				{ID: 100, DefIndex: 1, Quality: 6, SKU: "1;6"},
				{ID: 200, DefIndex: 1, Quality: 6, SKU: "1;6"},
				{ID: 300, DefIndex: 5000, Quality: 6, SKU: "5000;6"},
			},
		}
		bp := New()
		bp.cache = mockC
		bp.manager = &mockSchemaProvider{s: s}

		assert.ElementsMatch(t, []uint64{100, 200}, bp.GetItemsBySKU("1;6"))
		assert.Equal(t, 2, bp.GetStock("1;6"))
	})
}

func TestBackpack_WithModuleAndFrom(t *testing.T) {
	t.Parallel()

	opt := WithModule()
	assert.NotNil(t, opt)

	sc, err := steam.NewClient(steam.Config{})
	if err == nil && sc != nil {
		opt(sc)
		retrieved := From(sc)
		assert.NotNil(t, retrieved)
	}
}

func TestBackpack_NewWithDeps(t *testing.T) {
	t.Parallel()

	cache := &mockCache{}
	mgr := &mockSchemaProvider{}
	locked := make(generic.Set[uint64])

	bp := NewWithDeps(cache, mgr, locked)
	assert.NotNil(t, bp)
	assert.Equal(t, cache, bp.cache)
	assert.Equal(t, mgr, bp.manager)
}

func TestBackpack_CacheAndGetters(t *testing.T) {
	t.Parallel()

	cache := &mockCache{}
	mgr := &mockSchemaProvider{}
	bp := NewWithDeps(cache, mgr, nil)

	assert.Equal(t, cache, bp.Cache())
	assert.Equal(t, mgr, bp.Schema())
}

func TestBackpack_GetItem(t *testing.T) {
	t.Parallel()

	mockC := &mockCache{
		items: []*tf2.Item{
			{ID: 100},
		},
	}
	bp := New()
	bp.cache = mockC

	it, ok := bp.GetItem(100)
	assert.True(t, ok)
	assert.Equal(t, uint64(100), it.ID)

	_, ok = bp.GetItem(200)
	assert.False(t, ok)
}

func TestBackpack_GetTotalCount(t *testing.T) {
	t.Parallel()

	mockC := &mockCache{
		items: []*tf2.Item{
			{ID: 100},
			{ID: 200},
		},
	}
	bp := New()
	bp.cache = mockC

	assert.Equal(t, 2, bp.GetTotalCount())
}

func TestBackpack_EventLoopCancel(t *testing.T) {
	t.Parallel()

	bp := New()
	bp.Bus = bus.New()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	bp.eventLoop(ctx)
}

func TestBackpack_Lifecycle(t *testing.T) {
	t.Parallel()

	t.Run("init_missing_tf2", func(t *testing.T) {
		ictx := mock.NewInitContext()
		schemaMod := schema.NewManager(schema.Config{})
		ictx.SetModule(schema.ModuleName, schemaMod)

		bp := New()
		err := bp.Init(ictx)
		assert.Error(t, err)
	})

	t.Run("init_missing_schema", func(t *testing.T) {
		ictx := mock.NewInitContext()
		tf2Mod := tf2.New()
		ictx.SetModule(tf2.ModuleName, tf2Mod)

		bp := New()
		err := bp.Init(ictx)
		assert.Error(t, err)
	})

	t.Run("init_and_start_authed_success", func(t *testing.T) {
		ictx := mock.NewInitContext()
		tf2Mod := tf2.New()
		schemaMod := schema.NewManager(schema.Config{})

		ictx.SetModule(tf2.ModuleName, tf2Mod)
		ictx.SetModule(schema.ModuleName, schemaMod)

		bp := New()
		err := bp.Init(ictx)
		require.NoError(t, err)

		bp.trading = &mockTradingProvider{}

		ctx, cancel := context.WithCancel(t.Context())
		authCtx := mock.NewAuthContext(7656119)

		err = bp.StartAuthed(ctx, authCtx)
		require.NoError(t, err)

		cancel()
	})
}
