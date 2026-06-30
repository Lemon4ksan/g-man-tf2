// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/steam/community/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

type communityFixturePayload struct {
	Assets []struct {
		AssetID    string `json:"assetid"`
		ClassID    string `json:"classid"`
		InstanceID string `json:"instanceid"`
		Amount     string `json:"amount"`
	} `json:"assets"`
	Descriptions []struct {
		ClassID        string         `json:"classid"`
		InstanceID     string         `json:"instanceid"`
		MarketHashName string         `json:"market_hash_name"`
		Name           string         `json:"name"`
		Tradable       int            `json:"tradable"`
		AppData        map[string]any `json:"app_data"`
		Descriptions   []struct {
			Value string `json:"value"`
			Color string `json:"color,omitempty"`
		} `json:"descriptions"`
	} `json:"descriptions"`
}

func loadFixturePayload(t testing.TB) (communityFixturePayload, map[string]struct {
	ClassID        string         `json:"classid"`
	InstanceID     string         `json:"instanceid"`
	MarketHashName string         `json:"market_hash_name"`
	Name           string         `json:"name"`
	Tradable       int            `json:"tradable"`
	AppData        map[string]any `json:"app_data"`
	Descriptions   []struct {
		Value string `json:"value"`
		Color string `json:"color,omitempty"`
	} `json:"descriptions"`
}) {
	t.Helper()
	fixturePath := filepath.Join("testdata", "glitched_community_76561197991477148.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to load fixture: %v", err)
	}

	var payload communityFixturePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Failed to unmarshal fixture: %v", err)
	}

	descMap := make(map[string]struct {
		ClassID        string         `json:"classid"`
		InstanceID     string         `json:"instanceid"`
		MarketHashName string         `json:"market_hash_name"`
		Name           string         `json:"name"`
		Tradable       int            `json:"tradable"`
		AppData        map[string]any `json:"app_data"`
		Descriptions   []struct {
			Value string `json:"value"`
			Color string `json:"color,omitempty"`
		} `json:"descriptions"`
	})

	for _, d := range payload.Descriptions {
		key := d.ClassID + "_" + d.InstanceID
		descMap[key] = d
	}

	return payload, descMap
}

func TestFullGlitchedInventoryParsing_76561197991477148(t *testing.T) {
	t.Parallel()

	payload, descMap := loadFixturePayload(t)
	s := mockSchemaForCoverage()

	totalItems := len(payload.Assets)
	parsedItems := 0
	spellsDetected := 0
	killstreaksDetected := 0
	unusualsInDesc := 0
	paintsInDesc := 0
	complexItemsCount := 0

	for _, ast := range payload.Assets {
		key := ast.ClassID + "_" + ast.InstanceID
		desc, exists := descMap[key]
		if !exists {
			continue
		}

		econDescs := make([]struct {
			Value string `json:"value"`
			Color string `json:"color,omitempty"`
		}, len(desc.Descriptions))
		for i, d := range desc.Descriptions {
			econDescs[i] = struct {
				Value string `json:"value"`
				Color string `json:"color,omitempty"`
			}{Value: d.Value, Color: d.Color}
		}

		econ := inventory.CEconItem{
			Asset: inventory.Asset{
				AssetID: ast.AssetID,
				Amount:  ast.Amount,
			},
			Description: inventory.Description{
				ClassID:        desc.ClassID,
				InstanceID:     desc.InstanceID,
				Tradable:       desc.Tradable,
				MarketHashName: desc.MarketHashName,
				Name:           desc.Name,
				AppData:        desc.AppData,
				Descriptions:   econDescs,
			},
		}

		item := mapCEconToTF2(econ, s)
		parsedItems++

		// Verify SKU generation stability across all 2,406 items
		skuStr := item.ToSKU()
		assert.NotEmpty(t, skuStr, "SKU generation should never produce an empty string for item: %s", desc.MarketHashName)

		hasSpellInDesc := false
		hasKSInDesc := false

		isFabricatorTool := strings.Contains(desc.MarketHashName, "Fabricator") || strings.Contains(desc.MarketHashName, "Kit")

		for _, dLine := range desc.Descriptions {
			if dLine.Color == "7ea9d1" && strings.Contains(strings.ToLower(dLine.Value), "spell") {
				hasSpellInDesc = true
			}
			if !isFabricatorTool && (strings.Contains(dLine.Value, "Killstreak Active") || strings.Contains(dLine.Value, "Killstreaks Active")) {
				hasKSInDesc = true
			}
			if strings.HasPrefix(dLine.Value, "★ Unusual Effect:") {
				unusualsInDesc++
			}
			if strings.HasPrefix(dLine.Value, "Paint Color:") {
				paintsInDesc++
			}
		}

		if len(desc.Descriptions) >= 8 {
			complexItemsCount++
		}

		hasSpellAttr := false
		hasKSAttr := false

		for _, attr := range item.Attributes {
			if attr.Defindex >= schema.DefSpellProxy {
				hasSpellAttr = true
			}
			if attr.Defindex == schema.AttrKillstreak {
				hasKSAttr = true
			}
		}

		if hasSpellInDesc {
			spellsDetected++
			assert.True(t, hasSpellAttr, "Item with spell description should have spell proxy attribute parsed: %s", desc.MarketHashName)
		}
		if hasKSInDesc {
			killstreaksDetected++
			assert.True(t, hasKSAttr, "Item with killstreak description should have killstreak attribute parsed: %s", desc.MarketHashName)
		}
	}

	t.Logf("=== FULL GLITCHED INVENTORY UNIT TEST STATS (SteamID: 76561197991477148) ===")
	t.Logf("Total Inventory Assets Verified: %d", totalItems)
	t.Logf("Parsed Items (Zero Panics): %d", parsedItems)
	t.Logf("Complex Items Verified (>=8 descs): %d", complexItemsCount)
	t.Logf("Spells Correctly Parsed & Verified: %d", spellsDetected)
	t.Logf("Killstreaks Correctly Parsed & Verified: %d", killstreaksDetected)
	t.Logf("Unusual Effects Present in Descriptions: %d", unusualsInDesc)
	t.Logf("Paints Present in Descriptions: %d", paintsInDesc)
}

func TestSKURoundtripIdempotency_FullInventory(t *testing.T) {
	t.Parallel()

	payload, descMap := loadFixturePayload(t)
	s := mockSchemaForCoverage()

	validSKUs := 0
	roundtripSuccesses := 0

	for _, ast := range payload.Assets {
		key := ast.ClassID + "_" + ast.InstanceID
		desc, exists := descMap[key]
		if !exists {
			continue
		}

		econDescs := make([]struct {
			Value string `json:"value"`
			Color string `json:"color,omitempty"`
		}, len(desc.Descriptions))
		for i, d := range desc.Descriptions {
			econDescs[i] = struct {
				Value string `json:"value"`
				Color string `json:"color,omitempty"`
			}{Value: d.Value, Color: d.Color}
		}

		econ := inventory.CEconItem{
			Asset: inventory.Asset{AssetID: ast.AssetID, Amount: ast.Amount},
			Description: inventory.Description{
				ClassID: desc.ClassID, InstanceID: desc.InstanceID, Tradable: desc.Tradable,
				MarketHashName: desc.MarketHashName, Name: desc.Name, AppData: desc.AppData, Descriptions: econDescs,
			},
		}

		item := mapCEconToTF2(econ, s)
		origSKU := item.ToSKU()

		// 1. Validate SKU format
		if sku.IsValid(origSKU) {
			validSKUs++
		}

		// 2. Perform Roundtrip Deserialization and Reserialization
		parsedItem, err := sku.FromString(origSKU)
		require.NoError(t, err, "sku.FromString should successfully parse generated SKU: %s", origSKU)

		reserializedSKU := sku.FromObject(parsedItem)
		assert.Equal(t, origSKU, reserializedSKU, "Roundtrip SKU serialization must be 100%% idempotent for item: %s", desc.MarketHashName)
		roundtripSuccesses++
	}

	t.Logf("=== SKU ROUNDTRIP IDEMPOTENCY TEST STATS ===")
	t.Logf("Total Roundtrips Verified: %d / %d", roundtripSuccesses, len(payload.Assets))
	t.Logf("Format Validated SKUs: %d", validSKUs)
}

func TestUltraGlitchedTrophiesParsing(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("testdata", "glitched_raw_76561197991477148.json")
	data, err := os.ReadFile(fixturePath)
	require.NoError(t, err, "Should be able to load raw Web API fixture")

	var rawItems []TF2Item
	err = json.Unmarshal(data, &rawItems)
	require.NoError(t, err, "Should unmarshal raw items")

	trophiesFound := 0

	for _, item := range rawItems {
		skuStr := item.ToSKU()

		// Verify iconic trophies by quality and defindex
		switch {
		case item.Defindex == 5021 && item.Quality == 9:
			trophiesFound++
			t.Logf("🏆 Verified Self-Made Key -> SKU: %s", skuStr)
			assert.Equal(t, "5021;9", skuStr, "SKU for Self-Made Key must be 5021;9")

		case item.Defindex == 5000 && item.Quality == 3:
			trophiesFound++
			t.Logf("🏆 Verified Vintage Scrap Metal -> SKU: %s", skuStr)
			assert.Equal(t, "5000;3", skuStr, "SKU for Vintage Scrap must be 5000;3")

		case item.Defindex == 5001 && item.Quality == 3:
			trophiesFound++
			t.Logf("🏆 Verified Vintage Reclaimed Metal -> SKU: %s", skuStr)
			assert.Equal(t, "5001;3", skuStr, "SKU for Vintage Reclaimed must be 5001;3")

		case item.Defindex == 5002 && item.Quality == 3:
			trophiesFound++
			t.Logf("🏆 Verified Vintage Refined Metal -> SKU: %s", skuStr)
			assert.Equal(t, "5002;3", skuStr, "SKU for Vintage Refined must be 5002;3")

		case item.Quality == 14:
			// Check if it has Strange Score attribute (214)
			hasStrangeAttr := false
			for _, attr := range item.Attributes {
				if attr.Defindex == schema.AttrStrangeScore {
					hasStrangeAttr = true
					break
				}
			}
			if hasStrangeAttr {
				trophiesFound++
				t.Logf("🏆 Verified Strange Collector's Item (Defindex: %d) -> SKU: %s", item.Defindex, skuStr)
				assert.Contains(t, skuStr, ";14", "Strange Collector's SKU must have primary Quality 14")
				assert.Contains(t, skuStr, ";strange", "Strange Collector's SKU must contain ';strange' tag")
			}
		}
	}

	assert.Greater(t, trophiesFound, 0, "Should find and verify iconic glitched trophy items in raw Web API fixture")
}

func TestGoldenPanParsing(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("testdata", "glitched_raw_76561197991477148.json")
	data, err := os.ReadFile(fixturePath)
	require.NoError(t, err, "Should be able to load raw Web API fixture")

	var rawItems []TF2Item
	err = json.Unmarshal(data, &rawItems)
	require.NoError(t, err)

	var goldenPan *TF2Item
	for i := range rawItems {
		if rawItems[i].ID == 16486306785 {
			goldenPan = &rawItems[i]
			break
		}
	}

	require.NotNil(t, goldenPan, "Should find the iconic Golden Frying Pan asset 16486306785")

	skuStr := goldenPan.ToSKU()
	t.Logf("🍳 Golden Frying Pan Asset 16486306785 SKU -> %s", skuStr)

	assert.Equal(t, 1071, goldenPan.Defindex, "Golden Pan must have Defindex 1071")
	assert.Equal(t, 11, goldenPan.Quality, "Golden Pan must have Quality 11 (Strange)")
	assert.Contains(t, skuStr, "1071;11", "SKU must start with 1071;11")
	assert.Contains(t, skuStr, "festive", "Golden Pan SKU must contain 'festive'")

	// Verify Roundtrip idempotency on Golden Pan
	parsedPan, err := sku.FromString(skuStr)
	require.NoError(t, err)
	assert.Equal(t, skuStr, sku.FromObject(parsedPan), "Golden Pan SKU roundtrip must be 100% idempotent")
}

func TestSmeltingSimulation_GlitchedInventory(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("testdata", "glitched_raw_76561197991477148.json")
	data, err := os.ReadFile(fixturePath)
	assert.NoError(t, err, "Should be able to load raw Web API fixture")

	var rawItems []*tf2.Item
	err = json.Unmarshal(data, &rawItems)
	assert.NoError(t, err)

	bp := NewWithDeps(&mockCache{items: rawItems}, &mockSchemaProvider{s: mockSchemaForCoverage()}, nil)

	classes := []string{"Scout", "Soldier", "Pyro", "Demoman", "Heavy", "Engineer", "Medic", "Sniper", "Spy"}
	totalSmeltCandidates := 0

	t.Logf("=== RUNNING AUTOMATED SMELTING SIMULATION ON INVENTORY 76561197991477148 (%d ITEMS) ===", len(rawItems))

	for _, class := range classes {
		candidates := bp.FindWeaponsByClassForSmelting(class)
		if len(candidates) > 0 {
			t.Logf("⚔️ Class %s smelting candidates count: %d", class, len(candidates))
			for _, item := range candidates {
				totalSmeltCandidates++
				t.Logf("   [SMELT CANDIDATE] ID: %d, Defindex: %d, Quality: %d", item.ID, item.DefIndex, item.Quality)

				assert.Equal(t, uint32(schema.QualityUnique), item.Quality, "Only Unique weapons can be smelted")
				assert.Equal(t, uint32(0), item.KillstreakTier, "Killstreak weapons must NEVER be smelted")
				assert.False(t, item.Australium, "Australium weapons must NEVER be smelted")
				assert.False(t, item.Festivized, "Festivized weapons must NEVER be smelted")
				assert.Empty(t, item.Spells, "Spelled weapons must NEVER be smelted")
				assert.Empty(t, item.Parts, "Strange part weapons must NEVER be smelted")
			}
		}
	}

	t.Logf("🏆 Total Smelt Candidates Found Across 2,406 Items: %d", totalSmeltCandidates)
}

func TestSmeltingSimulation_GroundedInventory_76561198315558870(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("testdata", "raw_76561198315558870.json")
	data, err := os.ReadFile(fixturePath)
	assert.NoError(t, err, "Should be able to load grounded raw Web API fixture")

	var rawItems []*tf2.Item
	err = json.Unmarshal(data, &rawItems)
	assert.NoError(t, err)

	bp := NewWithDeps(&mockCache{items: rawItems}, &mockSchemaProvider{s: mockSchemaForCoverage()}, nil)

	classes := []string{"Scout", "Soldier", "Pyro", "Demoman", "Heavy", "Engineer", "Medic", "Sniper", "Spy"}
	totalSmeltCandidates := 0

	t.Logf("=== RUNNING AUTOMATED SMELTING SIMULATION ON GROUNDED INVENTORY 76561198315558870 (%d ITEMS) ===", len(rawItems))

	for _, class := range classes {
		candidates := bp.FindWeaponsByClassForSmelting(class)
		if len(candidates) > 0 {
			t.Logf("⚔️ Class %s smelting candidates count: %d", class, len(candidates))
			for _, item := range candidates {
				totalSmeltCandidates++
				t.Logf("   [SMELT CANDIDATE] ID: %d, Defindex: %d, Quality: %d", item.ID, item.DefIndex, item.Quality)

				assert.Equal(t, uint32(schema.QualityUnique), item.Quality, "Only Unique weapons can be smelted")
				assert.Equal(t, uint32(0), item.KillstreakTier, "Killstreak weapons must NEVER be smelted")
				assert.False(t, item.Australium, "Australium weapons must NEVER be smelted")
				assert.False(t, item.Festivized, "Festivized weapons must NEVER be smelted")
				assert.Empty(t, item.Spells, "Spelled weapons must NEVER be smelted")
				assert.Empty(t, item.Parts, "Strange part weapons must NEVER be smelted")
			}
		}
	}

	t.Logf("🏆 Total Smelt Candidates Found Across 2,205 Grounded Items: %d", totalSmeltCandidates)
}

func BenchmarkFullInventoryParsing_Parallel(b *testing.B) {
	payload, descMap := loadFixturePayload(b)
	s := mockSchemaForCoverage()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		idx := 0
		numAssets := len(payload.Assets)
		for pb.Next() {
			ast := payload.Assets[idx%numAssets]
			idx++

			key := ast.ClassID + "_" + ast.InstanceID
			desc, exists := descMap[key]
			if !exists {
				continue
			}

			econDescs := make([]struct {
				Value string `json:"value"`
				Color string `json:"color,omitempty"`
			}, len(desc.Descriptions))
			for i, d := range desc.Descriptions {
				econDescs[i] = struct {
					Value string `json:"value"`
					Color string `json:"color,omitempty"`
				}{Value: d.Value, Color: d.Color}
			}

			econ := inventory.CEconItem{
				Asset: inventory.Asset{AssetID: ast.AssetID, Amount: ast.Amount},
				Description: inventory.Description{
					ClassID: desc.ClassID, InstanceID: desc.InstanceID, Tradable: desc.Tradable,
					MarketHashName: desc.MarketHashName, Name: desc.Name, AppData: desc.AppData, Descriptions: econDescs,
				},
			}

			item := mapCEconToTF2(econ, s)
			_ = item.ToSKU()
		}
	})
}
