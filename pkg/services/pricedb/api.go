// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

import (
	"context"

	"github.com/lemon4ksan/aoni"
)

// API is the interface for the PriceDB API.
//
// @aoni:service casing=snake_case
// @base_url "https://pricedb.io/api/"
// @version "v1.0.0"
type API interface {
	// GetItem fetches the latest price for a specific item SKU.
	// @get "item/{sku}"
	GetItem(ctx context.Context, sku string, mods ...aoni.RequestModifier) (*Price, error)

	// PostItemsBulk fetches the latest prices for multiple SKUs in a single request.
	// @post "items-bulk"
	PostItemsBulk(ctx context.Context, req bulkRequest, mods ...aoni.RequestModifier) ([]*Price, error)

	// Search performs a fuzzy search for items by name.
	// @get "search"
	Search(ctx context.Context, q string, limit int, mods ...aoni.RequestModifier) (*SearchResult, error)

	// GetHistory returns the price history for a specific SKU.
	// @get "item-history/{sku}"
	GetHistory(ctx context.Context, sku string, start int64, end int64, mods ...aoni.RequestModifier) ([]*Price, error)

	// GetStats returns statistics (min, max, avg) for an item's price history.
	// @get "item-stats/{sku}"
	GetStats(ctx context.Context, sku string, mods ...aoni.RequestModifier) (*ItemStats, error)

	// Compare compares two items side by side, returning the price differences.
	// @get "compare/{sku1}/{sku2}"
	Compare(ctx context.Context, sku1 string, sku2 string, mods ...aoni.RequestModifier) (*CompareResult, error)

	// TriggerPriceCheck requests PriceDB to update the price for a specific SKU.
	// @post "autob/items/{sku}"
	TriggerPriceCheck(ctx context.Context, sku string, mods ...aoni.RequestModifier) error

	// HealthCheck returns the current system statistics and health of the API.
	// @get "cache-stats"
	HealthCheck(ctx context.Context, mods ...aoni.RequestModifier) (*CacheStats, error)

	// GetHealth returns the health status message from the PriceDB API.
	// @get ""
	GetHealth(ctx context.Context, mods ...aoni.RequestModifier) (string, error)

	// GetItems returns a list of all unique items (name and SKU) in the database.
	// @get "items"
	GetItems(ctx context.Context, mods ...aoni.RequestModifier) ([]*ItemBrief, error)

	// GetLatestPrices returns the 10 most recent price entries from the database.
	// @get "latest-prices"
	GetLatestPrices(ctx context.Context, mods ...aoni.RequestModifier) ([]*Price, error)

	// GetPrices returns paginated price history.
	// @get "prices"
	GetPrices(ctx context.Context, limit int, offset int, mods ...aoni.RequestModifier) (*PriceHistoryResponse, error)

	// GetSnapshot returns the most recent price for each SKU as of the given unix timestamp.
	// @get "snapshot/{timestamp}"
	GetSnapshot(ctx context.Context, timestamp int64, mods ...aoni.RequestModifier) ([]*Price, error)

	// GetGraph returns the HTML page containing an interactive Chart.js graph.
	// @get "graph/{sku}"
	GetGraph(ctx context.Context, sku string, header bool, height int, width string, mods ...aoni.RequestModifier) (string, error)

	// GetAutobItems fetches the full pricelist in TF2Autobot-compatible format.
	// @get "autob/items"
	GetAutobItems(ctx context.Context, mods ...aoni.RequestModifier) (*AutobItemsResponse, error)

	// GetAutobItem fetches the latest price for a single SKU in TF2Autobot-compatible format.
	// @get "autob/items/{sku}"
	GetAutobItem(ctx context.Context, sku string, mods ...aoni.RequestModifier) (*Price, error)
}

// SKUClient is the interface for the PriceDB SKU Service.
//
// @aoni:service casing=snake_case
// @base_url "https://sku.pricedb.io/api/"
// @version "v1.0.0"
// @source "pricedb_sku.json"
type SKUClient interface {
	// ResolveName looks up an item by name using the SKU Service.
	// @get "name/{name}"
	ResolveName(ctx context.Context, name string, mods ...aoni.RequestModifier) (*ResolvedItem, error)

	// ResolveSKU looks up item properties by its SKU using the SKU Service.
	// @get "sku/{sku}"
	ResolveSKU(ctx context.Context, sku string, mods ...aoni.RequestModifier) (*ResolvedItem, error)

	// GetSchema fetches the complete TF2 schema from PriceDB.
	// @get "schema"
	GetSchema(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error)

	// GetImageBySKU returns the raw image data for a SKU from the SKU service.
	// @get "sku/{sku}/image"
	GetImageBySKU(ctx context.Context, sku string, mods ...aoni.RequestModifier) ([]byte, error)

	// GetImageByName returns the raw image data for an item by name from the SKU service.
	// @get "name/{name}/image"
	GetImageByName(ctx context.Context, name string, mods ...aoni.RequestModifier) ([]byte, error)

	// ListEffects lists all unusual particle effects known to the TF2 schema.
	// @get "effect/list"
	ListEffects(ctx context.Context, mods ...aoni.RequestModifier) (*EffectsResponse, error)

	// GetEffectByID returns the name of a single unusual effect by its numeric ID.
	// @get "effect/{id}"
	GetEffectByID(ctx context.Context, id int, mods ...aoni.RequestModifier) (*EffectResponse, error)

	// GetEffectByName returns the ID of a single unusual effect by its display name.
	// @get "effect/name/{name}"
	GetEffectByName(ctx context.Context, name string, mods ...aoni.RequestModifier) (*EffectResponse, error)

	// ListPaints lists all paint colors.
	// @get "paint/list"
	ListPaints(ctx context.Context, mods ...aoni.RequestModifier) (*PaintsResponse, error)

	// GetPaintByID returns the paint color by defindex.
	// @get "paint/{id}"
	GetPaintByID(ctx context.Context, id int, mods ...aoni.RequestModifier) (*PaintResponse, error)

	// GetPaintByName returns the paint color defindex by its display name.
	// @get "paint/name/{name}"
	GetPaintByName(ctx context.Context, name string, mods ...aoni.RequestModifier) (*PaintResponse, error)

	// ListWears lists the five wear levels.
	// @get "wear/list"
	ListWears(ctx context.Context, mods ...aoni.RequestModifier) (*WearsResponse, error)

	// GetWearByID returns the display name for a single wear level ID (1–5).
	// @get "wear/{id}"
	GetWearByID(ctx context.Context, id int, mods ...aoni.RequestModifier) (*WearResponse, error)

	// ListPaintKits lists all War Paint kits known to the TF2 schema.
	// @get "paintkit/list"
	ListPaintKits(ctx context.Context, mods ...aoni.RequestModifier) (*PaintKitsResponse, error)

	// GetPaintKitByID returns War Paint kit details by ID.
	// @get "paintkit/{id}"
	GetPaintKitByID(ctx context.Context, id int, mods ...aoni.RequestModifier) (*PaintKitResponse, error)

	// GetPaintKitByName returns War Paint kit details by name.
	// @get "paintkit/name/{name}"
	GetPaintKitByName(ctx context.Context, name string, mods ...aoni.RequestModifier) (*PaintKitResponse, error)

	// ListStrangeParts lists all Strange Parts.
	// @get "strangepart/list"
	ListStrangeParts(ctx context.Context, mods ...aoni.RequestModifier) (*StrangePartsResponse, error)

	// ListCrateSeries lists all crate series numbers.
	// @get "crateseries/list"
	ListCrateSeries(ctx context.Context, mods ...aoni.RequestModifier) (*CrateSeriesResponse, error)

	// ListCraftWeapons lists all craftable weapons.
	// @get "craftweapon/list"
	ListCraftWeapons(ctx context.Context, mods ...aoni.RequestModifier) (*WeaponsResponse, error)

	// ListUncraftWeapons lists all uncraftable weapons.
	// @get "uncraftweapon/list"
	ListUncraftWeapons(ctx context.Context, mods ...aoni.RequestModifier) (*WeaponsResponse, error)

	// ListQualities lists all item qualities.
	// @get "quality/list"
	ListQualities(ctx context.Context, mods ...aoni.RequestModifier) (*QualitiesResponse, error)

	// DownloadSchema downloads the TF2 item schema as a map.
	// @get "download"
	DownloadSchema(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error)
}

// SpellClient is the interface for the PriceDB Spell Service.
//
// @aoni:service casing=snake_case
// @base_url "https://spell.pricedb.io/api/"
// @version "v1.0.0"
// @source "pricedb_spells.json"
type SpellClient interface {
	// PredictSpellPrice estimates the premium values for given comma-separated spell names and item name.
	// @get "spell/predict"
	PredictSpellPrice(ctx context.Context, spells string, item string, mods ...aoni.RequestModifier) (*SpellPredictionResponse, error)

	// PredictSpellItem predicts spelled item price premium via POST.
	// @post "spell/predict-spell-item"
	PredictSpellItem(ctx context.Context, req PredictSpellItemRequest, mods ...aoni.RequestModifier) (*PredictSpellItemResponse, error)

	// GetSpellValue returns the predicted premium for given comma-separated spell defindex IDs.
	// @get "spell/spell-value"
	GetSpellValue(ctx context.Context, ids string, mods ...aoni.RequestModifier) (*SpellValueResponse, error)

	// GetSpellAnalytics returns comprehensive market analytics for all tracked spell combinations.
	// @get "spell/spell-analytics"
	GetSpellAnalytics(ctx context.Context, mods ...aoni.RequestModifier) ([]*SpellAnalyticsEntry, error)

	// GetItemSpellPremium returns the detailed spell premium breakdown for a specific item and spell combination.
	// @get "spell/item-spell-premium"
	GetItemSpellPremium(ctx context.Context, item string, ids string, mods ...aoni.RequestModifier) (*ItemSpellPremiumResponse, error)

	// GetSpellByID returns spell metadata for a given spell defindex ID.
	// @get "spell/spell-id-to-name"
	GetSpellByID(ctx context.Context, id int, mods ...aoni.RequestModifier) (*SpellMetadata, error)

	// GetSpellByName returns spell metadata for a given spell name.
	// @get "spell/spell-name-to-id"
	GetSpellByName(ctx context.Context, name string, mods ...aoni.RequestModifier) (*SpellMetadata, error)

	// ListSpells lists all available TF2 spell definitions.
	// @get "spell/spells"
	ListSpells(ctx context.Context, mods ...aoni.RequestModifier) ([]*SpellMetadata, error)

	// GetSpellFetcherStatus returns the status and statistics of the spell data collection fetcher.
	// @get "spell/fetcher-status"
	GetSpellFetcherStatus(ctx context.Context, mods ...aoni.RequestModifier) (*FetcherStatusResponse, error)

	// GetSpellHealth returns health status for the spell service.
	// @get "spell/health"
	GetSpellHealth(ctx context.Context, mods ...aoni.RequestModifier) (*SpellHealthResponse, error)

	// GetServiceStats returns comprehensive service statistics.
	// @get "stats"
	GetServiceStats(ctx context.Context, mods ...aoni.RequestModifier) (*ServiceStatsResponse, error)

	// GetUnifiedStatus returns unified operational status across all services.
	// @get "spell/status-proxy"
	GetUnifiedStatus(ctx context.Context, mods ...aoni.RequestModifier) (*UnifiedStatusResponse, error)
}
