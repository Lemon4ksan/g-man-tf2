// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

import (
	"testing"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/g-man/pkg/test/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestClient(t *testing.T) (API, *mock.HTTPStub) {
	t.Helper()

	stub := mock.NewHTTPStub()
	client := NewClient(aoni.NewClient(stub))

	return client, stub
}

func setupTestSKUClient(t *testing.T) (SKUClient, *mock.HTTPStub) {
	t.Helper()

	stub := mock.NewHTTPStub()
	client := NewSKUClient(aoni.NewClient(stub))

	return client, stub
}

func setupTestSpellClient(t *testing.T) (SpellClient, *mock.HTTPStub) {
	t.Helper()

	stub := mock.NewHTTPStub()
	client := NewSpellClient(aoni.NewClient(stub))

	return client, stub
}

func TestClient(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("basic_price_lookups", func(t *testing.T) {
		t.Parallel()

		client, stub := setupTestClient(t)

		sku := "5021;6"
		stub.SetJSONResponse(
			"api/item/5021;6",
			200,
			Price{
				SKU:  sku,
				Name: "Mann Co. Supply Crate Key",
				Buy:  Currencies{Metal: 75},
				Sell: Currencies{Metal: 75.11},
			},
		)
		price, err := client.GetItem(ctx, sku)
		require.NoError(t, err)
		assert.Equal(t, sku, price.SKU)
		assert.Equal(t, 75.0, price.Buy.Metal)
		assert.Equal(t, 75.11, price.Sell.Metal)

		stub.SetJSONResponse("api/items-bulk", 200, []*Price{
			{SKU: "5021;6", Name: "Key", Buy: Currencies{Metal: 75}},
			{SKU: "5002;6", Name: "Refined", Buy: Currencies{Metal: 1}},
		})

		bulk, err := GetItemsBulk(ctx, client, []string{"5021;6", "5002;6"})
		require.NoError(t, err)
		assert.Len(t, bulk, 2)
		assert.Equal(t, "5021;6", bulk[0].SKU)

		bulkEmpty, err := GetItemsBulk(ctx, client, nil)
		require.NoError(t, err)
		assert.Empty(t, bulkEmpty)

		stub.SetRawResponse("api/autob/items/5021;6", 202, nil)

		err = client.TriggerPriceCheck(ctx, "5021;6")
		require.NoError(t, err)

		stub.SetJSONResponse(
			"api/autob/items",
			200,
			AutobItemsResponse{Success: true, Items: []*Price{{SKU: "5021;6"}}},
		)

		autob, err := client.GetAutobItems(ctx)
		require.NoError(t, err)
		assert.True(t, autob.Success)

		stub.SetJSONResponse("api/autob/items/5021;6", 200, Price{SKU: "5021;6"})

		autobItem, err := client.GetAutobItem(ctx, "5021;6")
		require.NoError(t, err)
		assert.Equal(t, "5021;6", autobItem.SKU)
	})

	t.Run("metadata_and_analytics", func(t *testing.T) {
		t.Parallel()

		client, stub := setupTestClient(t)

		stub.SetJSONResponse("api/search", 200, SearchResult{Total: 5})

		res, err := client.Search(ctx, "key", 10)
		require.NoError(t, err)
		assert.Equal(t, 5, res.Total)

		stub.SetJSONResponse("api/item-history/5021;6", 200, []*Price{{SKU: "5021;6"}})

		history, err := client.GetHistory(ctx, "5021;6", 1000, 2000)
		require.NoError(t, err)
		assert.Len(t, history, 1)

		stub.SetJSONResponse("api/item-stats/5021;6", 200, ItemStats{})

		stats, err := client.GetStats(ctx, "5021;6")
		require.NoError(t, err)
		assert.NotNil(t, stats)

		stub.SetJSONResponse("api/compare/1;6/2;6", 200, CompareResult{})

		comp, err := client.Compare(ctx, "1;6", "2;6")
		require.NoError(t, err)
		assert.NotNil(t, comp)

		stub.SetJSONResponse("api/cache-stats", 200, CacheStats{})

		health, err := client.HealthCheck(ctx)
		require.NoError(t, err)
		assert.NotNil(t, health)

		stub.SetJSONResponse("api/", 200, "healthy")

		statusStr, err := client.GetHealth(ctx)
		require.NoError(t, err)
		assert.Equal(t, "healthy", statusStr)

		stub.SetJSONResponse("api/items", 200, []*ItemBrief{{Name: "Key", SKU: "5021;6"}})

		itemsList, err := client.GetItems(ctx)
		require.NoError(t, err)
		assert.Len(t, itemsList, 1)

		stub.SetJSONResponse("api/latest-prices", 200, []*Price{{SKU: "5021;6"}})

		latest, err := client.GetLatestPrices(ctx)
		require.NoError(t, err)
		assert.Len(t, latest, 1)

		stub.SetJSONResponse("api/prices", 200, PriceHistoryResponse{Total: 10})

		pricesHist, err := client.GetPrices(ctx, 10, 0)
		require.NoError(t, err)
		assert.Equal(t, 10, pricesHist.Total)

		stub.SetJSONResponse("api/snapshot/1600000000", 200, []*Price{{SKU: "5021;6"}})

		snapshot, err := client.GetSnapshot(ctx, 1600000000)
		require.NoError(t, err)
		assert.Len(t, snapshot, 1)

		stub.SetJSONResponse("api/graph/5021;6", 200, "<html>graph</html>")

		graph, err := client.GetGraph(ctx, "5021;6", true, 400, "100%")
		require.NoError(t, err)
		assert.Equal(t, "<html>graph</html>", graph)
	})

	t.Run("sku_and_visual_assets", func(t *testing.T) {
		t.Parallel()

		skuClient, stub := setupTestSKUClient(t)

		stub.SetJSONResponse("api/name/Key", 200, ResolvedItem{DefIndex: 5021, Name: "Key"})

		resolvedName, err := skuClient.ResolveName(ctx, "Key")
		require.NoError(t, err)
		assert.Equal(t, 5021, resolvedName.DefIndex)

		stub.SetJSONResponse("api/sku/5021;6", 200, ResolvedItem{Name: "Key", SKU: "5021;6"})

		resolvedSKU, err := skuClient.ResolveSKU(ctx, "5021;6")
		require.NoError(t, err)
		assert.Equal(t, "Key", resolvedSKU.Name)

		stub.SetJSONResponse("api/schema", 200, map[string]any{"version": "1.0"})

		schemaMap, err := skuClient.GetSchema(ctx)
		require.NoError(t, err)
		assert.Equal(t, "1.0", schemaMap["version"])

		stub.SetRawResponse("api/sku/5021;6/image", 200, []byte{0x89, 0x50, 0x4E, 0x47})

		imgSKU, err := skuClient.GetImageBySKU(ctx, "5021;6")
		require.NoError(t, err)
		assert.Equal(t, []byte{0x89, 0x50, 0x4E, 0x47}, imgSKU)

		stub.SetRawResponse("api/name/Key/image", 200, []byte{0x89, 0x50, 0x4E, 0x47})

		imgName, err := skuClient.GetImageByName(ctx, "Key")
		require.NoError(t, err)
		assert.Equal(t, []byte{0x89, 0x50, 0x4E, 0x47}, imgName)
	})

	t.Run("visual_effects_paints_and_wears", func(t *testing.T) {
		t.Parallel()

		skuClient, stub := setupTestSKUClient(t)

		stub.SetJSONResponse("api/effect/list", 200, EffectsResponse{
			Success: true,
			Data:    []*EffectInfo{{ID: 17, Name: "Sunbeams"}},
		})

		effects, err := skuClient.ListEffects(ctx)
		require.NoError(t, err)
		assert.Len(t, effects.Data, 1)

		stub.SetJSONResponse("api/effect/17", 200, EffectResponse{
			Success: true,
			Data:    &EffectInfo{ID: 17, Name: "Sunbeams"},
		})

		effID, err := skuClient.GetEffectByID(ctx, 17)
		require.NoError(t, err)
		assert.Equal(t, "Sunbeams", effID.Data.Name)

		stub.SetJSONResponse("api/effect/name/Sunbeams", 200, EffectResponse{
			Success: true,
			Data:    &EffectInfo{ID: 17, Name: "Sunbeams"},
		})

		effName, err := skuClient.GetEffectByName(ctx, "Sunbeams")
		require.NoError(t, err)
		assert.Equal(t, 17, effName.Data.ID)

		stub.SetJSONResponse("api/paint/list", 200, PaintsResponse{
			Success: true,
			Data:    []*PaintInfo{{DefIndex: 5041, Name: "Gold"}},
		})

		paints, err := skuClient.ListPaints(ctx)
		require.NoError(t, err)
		assert.Len(t, paints.Data, 1)

		stub.SetJSONResponse("api/paint/5041", 200, PaintResponse{
			Success: true,
			Data:    &PaintInfo{DefIndex: 5041, Name: "Gold"},
		})

		paintID, err := skuClient.GetPaintByID(ctx, 5041)
		require.NoError(t, err)
		assert.Equal(t, "Gold", paintID.Data.Name)

		stub.SetJSONResponse("api/paint/name/Gold", 200, PaintResponse{
			Success: true,
			Data:    &PaintInfo{DefIndex: 5041, Name: "Gold"},
		})

		paintName, err := skuClient.GetPaintByName(ctx, "Gold")
		require.NoError(t, err)
		assert.Equal(t, 5041, paintName.Data.DefIndex)

		stub.SetJSONResponse("api/wear/list", 200, WearsResponse{
			Success: true,
			Data:    []*WearInfo{{ID: 1, Name: "Factory New"}},
		})

		wears, err := skuClient.ListWears(ctx)
		require.NoError(t, err)
		assert.Len(t, wears.Data, 1)

		stub.SetJSONResponse("api/wear/1", 200, WearResponse{
			Success: true,
			Data:    &WearInfo{ID: 1, Name: "Factory New"},
		})

		wearID, err := skuClient.GetWearByID(ctx, 1)
		require.NoError(t, err)
		assert.Equal(t, "Factory New", wearID.Data.Name)

		stub.SetJSONResponse("api/download", 200, map[string]any{"schema": "downloaded"})

		downloaded, err := skuClient.DownloadSchema(ctx)
		require.NoError(t, err)
		assert.Equal(t, "downloaded", downloaded["schema"])
	})

	t.Run("halloween_spell_features", func(t *testing.T) {
		t.Parallel()

		spellClient, stub := setupTestSpellClient(t)

		stub.SetJSONResponse("api/spell/predict", 200, SpellPredictionResponse{ItemName: "Rocket Launcher"})

		pred, err := spellClient.PredictSpellPrice(ctx, "Exorcism", "Rocket Launcher")
		require.NoError(t, err)
		assert.Equal(t, "Rocket Launcher", pred.ItemName)

		stub.SetJSONResponse("api/spell/predict-spell-item", 200, PredictSpellItemResponse{ItemName: "Rocket Launcher"})

		predItem, err := spellClient.PredictSpellItem(
			ctx,
			PredictSpellItemRequest{ItemName: "Rocket Launcher", SpellIDs: []int{1009}},
		)
		require.NoError(t, err)
		assert.Equal(t, "Rocket Launcher", predItem.ItemName)

		stub.SetJSONResponse("api/spell/spell-value", 200, SpellValueResponse{Count: 5})

		val, err := spellClient.GetSpellValue(ctx, "1009")
		require.NoError(t, err)
		assert.Equal(t, 5, val.Count)

		stub.SetJSONResponse("api/spell/spell-analytics", 200, []*SpellAnalyticsEntry{{Count: 10}})

		analytics, err := spellClient.GetSpellAnalytics(ctx)
		require.NoError(t, err)
		assert.Len(t, analytics, 1)

		stub.SetJSONResponse("api/spell/item-spell-premium", 200, ItemSpellPremiumResponse{Item: "Rocket Launcher"})

		premium, err := spellClient.GetItemSpellPremium(ctx, "Rocket Launcher", "1009")
		require.NoError(t, err)
		assert.Equal(t, "Rocket Launcher", premium.Item)

		stub.SetJSONResponse("api/spell/spell-id-to-name", 200, SpellMetadata{Name: "Exorcism"})

		spellID, err := spellClient.GetSpellByID(ctx, 1009)
		require.NoError(t, err)
		assert.Equal(t, "Exorcism", spellID.Name)

		stub.SetJSONResponse("api/spell/spell-name-to-id", 200, SpellMetadata{ID: 1009})

		spellName, err := spellClient.GetSpellByName(ctx, "Exorcism")
		require.NoError(t, err)
		assert.Equal(t, 1009, spellName.ID)

		stub.SetJSONResponse("api/spell/spells", 200, []*SpellMetadata{{ID: 1009}})

		spells, err := spellClient.ListSpells(ctx)
		require.NoError(t, err)
		assert.Len(t, spells, 1)

		stub.SetJSONResponse("api/spell/fetcher-status", 200, FetcherStatusResponse{Status: "active"})

		fetcher, err := spellClient.GetSpellFetcherStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, "active", fetcher.Status)

		stub.SetJSONResponse("api/spell/health", 200, SpellHealthResponse{Status: "healthy"})

		health, err := spellClient.GetSpellHealth(ctx)
		require.NoError(t, err)
		assert.Equal(t, "healthy", health.Status)

		stub.SetJSONResponse("api/stats", 200, ServiceStatsResponse{Status: "operational"})

		stats, err := spellClient.GetServiceStats(ctx)
		require.NoError(t, err)
		assert.Equal(t, "operational", stats.Status)

		stub.SetJSONResponse("api/spell/status-proxy", 200, UnifiedStatusResponse{Status: "all_green"})

		unified, err := spellClient.GetUnifiedStatus(ctx)
		require.NoError(t, err)
		assert.Equal(t, "all_green", unified.Status)
	})
}
