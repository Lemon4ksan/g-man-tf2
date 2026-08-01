// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

import (
	"context"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/fast"
	"github.com/lemon4ksan/aoni/mod"
	"github.com/lemon4ksan/aoni/option"
	"github.com/lemon4ksan/aoni/request"
	"github.com/lemon4ksan/miyako/yumi"
)

const (
	// BaseURL is the base URL for the PriceDB API.
	BaseURL = "https://pricedb.io/api/"
	// SKUURL is the base URL for the PriceDB SKU API.
	SKUURL = "https://sku.pricedb.io/api/"
	// SpellURL is the base URL for the PriceDB Spell API.
	SpellURL = "https://spell.pricedb.io/api/"
)

// Client is a thread-safe HTTP client for interacting with PriceDB.
type Client struct {
	r     request.Requester
	sku   request.Requester
	spell request.Requester
}

// NewClient creates a new PriceDB API client backed by fast.Client by default.
func NewClient(doer aoni.RequestDoer) *Client {
	if doer == nil {
		doer = fast.NewClient()
	}

	return &Client{
		r: request.AsRequester(
			aoni.Configure(doer, option.WithBaseURL(BaseURL), option.WithUserAgent("G-man Bot/1.0")),
		),
		sku: request.AsRequester(
			aoni.Configure(doer, option.WithBaseURL(SKUURL), option.WithUserAgent("G-man Bot/1.0")),
		),
		spell: request.AsRequester(
			aoni.Configure(doer, option.WithBaseURL(SpellURL), option.WithUserAgent("G-man Bot/1.0")),
		),
	}
}

// GetItem fetches the latest price for a specific item SKU.
func (c *Client) GetItem(ctx context.Context, sku string, mods ...aoni.RequestModifier) (*Price, error) {
	allMods := append([]aoni.RequestModifier{
		mod.WithVar("sku", sku),
	}, mods...)

	return request.GetTo[Price](ctx, c.r, "item/{sku}", allMods...)
}

// GetItemsBulk fetches the latest prices for an array of SKUs in a single request.
// It automatically filters out empty SKUs and splits the request into batches of up to 100 SKUs.
func (c *Client) GetItemsBulk(ctx context.Context, skus []string, mods ...aoni.RequestModifier) ([]*Price, error) {
	validSKUs := make([]string, 0, len(skus))
	for _, sku := range skus {
		if sku != "" {
			validSKUs = append(validSKUs, sku)
		}
	}

	if len(validSKUs) == 0 {
		return nil, nil
	}

	const batchSize = 100

	var batches [][]string
	for i := 0; i < len(validSKUs); i += batchSize {
		end := min(i+batchSize, len(validSKUs))
		batches = append(batches, validSKUs[i:end])
	}

	results, err := yumi.Map(ctx, yumi.PipelineConfig{
		Workers: 3,
		RPS:     5,
		Burst:   2,
	}, batches, func(chunkCtx context.Context, batch []string) ([]*Price, error) {
		req := bulkRequest{SKUs: batch}

		resp, err := request.PostTo[[]*Price](chunkCtx, c.r, "items-bulk", req)
		if err != nil {
			return nil, err
		}

		if resp != nil {
			return *resp, nil
		}

		return nil, nil
	})
	if err != nil {
		return nil, err
	}

	var allPrices []*Price
	for _, batch := range results {
		allPrices = append(allPrices, batch...)
	}

	return allPrices, nil
}

// Search performs a fuzzy search for items by name.
func (c *Client) Search(
	ctx context.Context,
	query string,
	limit int,
	mods ...aoni.RequestModifier,
) (*SearchResult, error) {
	req := struct {
		Q     string `url:"q"`
		Limit int    `url:"limit,omitempty"`
	}{query, limit}

	allMods := append([]aoni.RequestModifier{
		mod.WithQuery(req),
	}, mods...)

	return request.GetTo[SearchResult](ctx, c.r, "search", allMods...)
}

// GetHistory returns the price history for a specific SKU.
// start and end are optional Unix timestamps (use 0 to ignore).
func (c *Client) GetHistory(
	ctx context.Context,
	sku string,
	start, end int64,
	mods ...aoni.RequestModifier,
) ([]*Price, error) {
	req := struct {
		Start int64 `url:"start,omitempty"`
		End   int64 `url:"end,omitempty"`
	}{start, end}

	allMods := append([]aoni.RequestModifier{
		mod.WithQuery(req),
		mod.WithVar("sku", sku),
	}, mods...)

	resp, err := request.GetTo[[]*Price](
		ctx,
		c.r,
		"item-history/{sku}",
		allMods...,
	)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetStats returns statistics (min, max, avg) for an item's price history.
func (c *Client) GetStats(ctx context.Context, sku string, mods ...aoni.RequestModifier) (*ItemStats, error) {
	allMods := append([]aoni.RequestModifier{
		mod.WithVar("sku", sku),
	}, mods...)

	return request.GetTo[ItemStats](ctx, c.r, "item-stats/{sku}", allMods...)
}

// Compare compares two items side by side, returning the price differences.
func (c *Client) Compare(ctx context.Context, sku1, sku2 string, mods ...aoni.RequestModifier) (*CompareResult, error) {
	allMods := append([]aoni.RequestModifier{
		mod.WithVar("sku1", sku1),
		mod.WithVar("sku2", sku2),
	}, mods...)

	return request.GetTo[CompareResult](
		ctx, c.r, "compare/{sku1}/{sku2}",
		allMods...,
	)
}

// TriggerPriceCheck requests PriceDB to update the price for a specific SKU.
// This hits the Autobot integration endpoint.
func (c *Client) TriggerPriceCheck(ctx context.Context, sku string, mods ...aoni.RequestModifier) error {
	allMods := append([]aoni.RequestModifier{
		mod.WithVar("sku", sku),
	}, mods...)

	_, err := request.PostTo[request.NoResponse](ctx, c.r, "autob/items/{sku}", nil, allMods...)

	return err
}

// HealthCheck returns the current system statistics and health of the API.
func (c *Client) HealthCheck(ctx context.Context, mods ...aoni.RequestModifier) (*CacheStats, error) {
	return request.GetTo[CacheStats](ctx, c.r, "cache-stats", mods...)
}

// ResolveName looks up an item by name using the SKU Service.
func (c *Client) ResolveName(ctx context.Context, name string, mods ...aoni.RequestModifier) (map[string]any, error) {
	allMods := append([]aoni.RequestModifier{
		mod.WithVar("name", name),
	}, mods...)

	resp, err := request.GetTo[map[string]any](ctx, c.sku, "name/{name}", allMods...)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// ResolveSKU looks up item properties by its SKU using the SKU Service.
func (c *Client) ResolveSKU(ctx context.Context, sku string, mods ...aoni.RequestModifier) (map[string]any, error) {
	allMods := append([]aoni.RequestModifier{
		mod.WithVar("sku", sku),
	}, mods...)

	resp, err := request.GetTo[map[string]any](ctx, c.sku, "sku/{sku}", allMods...)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetSchema fetches the complete TF2 schema from PriceDB.
func (c *Client) GetSchema(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error) {
	resp, err := request.GetTo[map[string]any](ctx, c.sku, "schema", mods...)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetHealth returns the health status message from the PriceDB API.
func (c *Client) GetHealth(ctx context.Context, mods ...aoni.RequestModifier) (string, error) {
	resp, err := request.GetTo[string](ctx, c.r, "", mods...)
	if err != nil {
		return "", err
	}

	return *resp, nil
}

// GetItems returns a list of all unique items (name and SKU) in the database.
func (c *Client) GetItems(ctx context.Context, mods ...aoni.RequestModifier) ([]*ItemBrief, error) {
	resp, err := request.GetTo[[]*ItemBrief](ctx, c.r, "items", mods...)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetLatestPrices returns the 10 most recent price entries from the database.
func (c *Client) GetLatestPrices(ctx context.Context, mods ...aoni.RequestModifier) ([]*Price, error) {
	resp, err := request.GetTo[[]*Price](ctx, c.r, "latest-prices", mods...)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetPrices returns paginated price history.
func (c *Client) GetPrices(
	ctx context.Context,
	limit, offset int,
	mods ...aoni.RequestModifier,
) (*PriceHistoryResponse, error) {
	req := struct {
		Limit  int `url:"limit,omitempty"`
		Offset int `url:"offset,omitempty"`
	}{Limit: limit, Offset: offset}

	allMods := append([]aoni.RequestModifier{
		mod.WithQuery(req),
	}, mods...)

	return request.GetTo[PriceHistoryResponse](ctx, c.r, "prices", allMods...)
}

// GetSnapshot returns the most recent price for each SKU as of the given unix timestamp.
func (c *Client) GetSnapshot(ctx context.Context, timestamp int64, mods ...aoni.RequestModifier) ([]*Price, error) {
	allMods := append([]aoni.RequestModifier{
		mod.WithVar("timestamp", timestamp),
	}, mods...)

	resp, err := request.GetTo[[]*Price](
		ctx, c.r, "snapshot/{timestamp}",
		allMods...,
	)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetGraph returns the HTML page containing an interactive Chart.js graph.
func (c *Client) GetGraph(
	ctx context.Context,
	sku string,
	header bool,
	height int,
	width string,
	mods ...aoni.RequestModifier,
) (string, error) {
	req := struct {
		Header bool   `url:"header"`
		Height int    `url:"height,omitempty"`
		Width  string `url:"width,omitempty"`
	}{Header: header, Height: height, Width: width}

	allMods := append([]aoni.RequestModifier{
		mod.WithQuery(req),
		mod.WithVar("sku", sku),
	}, mods...)

	resp, err := request.GetTo[string](
		ctx,
		c.r,
		"graph/{sku}",
		allMods...,
	)
	if err != nil {
		return "", err
	}

	return *resp, nil
}

// GetAutobItems fetches the full pricelist in TF2Autobot-compatible format.
func (c *Client) GetAutobItems(ctx context.Context, mods ...aoni.RequestModifier) (*AutobItemsResponse, error) {
	return request.GetTo[AutobItemsResponse](ctx, c.r, "autob/items", mods...)
}

// GetAutobItem fetches the latest price for a single SKU in TF2Autobot-compatible format.
func (c *Client) GetAutobItem(ctx context.Context, sku string, mods ...aoni.RequestModifier) (*Price, error) {
	allMods := append([]aoni.RequestModifier{
		mod.WithVar("sku", sku),
	}, mods...)

	return request.GetTo[Price](ctx, c.r, "autob/items/{sku}", allMods...)
}

// GetImageBySKU returns the raw image data for a SKU from the SKU service.
func (c *Client) GetImageBySKU(ctx context.Context, sku string, mods ...aoni.RequestModifier) ([]byte, error) {
	allMods := append([]aoni.RequestModifier{
		mod.WithVar("sku", sku),
	}, mods...)

	resp, err := request.GetTo[[]byte](ctx, c.sku, "sku/{sku}/image", allMods...)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetImageByName returns the raw image data for an item by name from the SKU service.
func (c *Client) GetImageByName(ctx context.Context, name string, mods ...aoni.RequestModifier) ([]byte, error) {
	allMods := append([]aoni.RequestModifier{
		mod.WithVar("name", name),
	}, mods...)

	resp, err := request.GetTo[[]byte](ctx, c.sku, "name/{name}/image", allMods...)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// ListEffects lists all unusual particle effects known to the TF2 schema.
func (c *Client) ListEffects(ctx context.Context, mods ...aoni.RequestModifier) ([]*EffectInfo, error) {
	type response struct {
		Success bool          `json:"success"`
		Data    []*EffectInfo `json:"data"`
	}

	resp, err := request.GetTo[response](ctx, c.sku, "effect/list", mods...)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetEffectByID returns the name of a single unusual effect by its numeric ID.
func (c *Client) GetEffectByID(ctx context.Context, id int, mods ...aoni.RequestModifier) (*EffectInfo, error) {
	type response struct {
		Success bool        `json:"success"`
		Data    *EffectInfo `json:"data"`
	}

	allMods := append([]aoni.RequestModifier{
		mod.WithVar("id", id),
	}, mods...)

	resp, err := request.GetTo[response](ctx, c.sku, "effect/{id}", allMods...)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetEffectByName returns the ID of a single unusual effect by its display name.
func (c *Client) GetEffectByName(ctx context.Context, name string, mods ...aoni.RequestModifier) (*EffectInfo, error) {
	type response struct {
		Success bool        `json:"success"`
		Data    *EffectInfo `json:"data"`
	}

	allMods := append([]aoni.RequestModifier{
		mod.WithVar("name", name),
	}, mods...)

	resp, err := request.GetTo[response](ctx, c.sku, "effect/name/{name}", allMods...)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// ListPaints lists all paint colors.
func (c *Client) ListPaints(ctx context.Context, mods ...aoni.RequestModifier) ([]*PaintInfo, error) {
	type response struct {
		Success bool         `json:"success"`
		Data    []*PaintInfo `json:"data"`
	}

	resp, err := request.GetTo[response](ctx, c.sku, "paint/list", mods...)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetPaintByID returns the paint color by defindex.
func (c *Client) GetPaintByID(ctx context.Context, id int, mods ...aoni.RequestModifier) (*PaintInfo, error) {
	type response struct {
		Success bool       `json:"success"`
		Data    *PaintInfo `json:"data"`
	}

	allMods := append([]aoni.RequestModifier{
		mod.WithVar("id", id),
	}, mods...)

	resp, err := request.GetTo[response](ctx, c.sku, "paint/{id}", allMods...)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetPaintByName returns the paint color defindex by its display name.
func (c *Client) GetPaintByName(ctx context.Context, name string, mods ...aoni.RequestModifier) (*PaintInfo, error) {
	type response struct {
		Success bool       `json:"success"`
		Data    *PaintInfo `json:"data"`
	}

	allMods := append([]aoni.RequestModifier{
		mod.WithVar("name", name),
	}, mods...)

	resp, err := request.GetTo[response](ctx, c.sku, "paint/name/{name}", allMods...)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// ListWears lists the five wear levels.
func (c *Client) ListWears(ctx context.Context, mods ...aoni.RequestModifier) ([]*WearInfo, error) {
	type response struct {
		Success bool        `json:"success"`
		Data    []*WearInfo `json:"data"`
	}

	resp, err := request.GetTo[response](ctx, c.sku, "wear/list", mods...)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetWearByID returns the display name for a single wear level ID (1–5).
func (c *Client) GetWearByID(ctx context.Context, id int, mods ...aoni.RequestModifier) (*WearInfo, error) {
	type response struct {
		Success bool      `json:"success"`
		Data    *WearInfo `json:"data"`
	}

	allMods := append([]aoni.RequestModifier{
		mod.WithVar("id", id),
	}, mods...)

	resp, err := request.GetTo[response](ctx, c.sku, "wear/{id}", allMods...)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// DownloadSchema downloads the TF2 item schema as a map.
func (c *Client) DownloadSchema(ctx context.Context, mods ...aoni.RequestModifier) (map[string]any, error) {
	resp, err := request.GetTo[map[string]any](ctx, c.sku, "download", mods...)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// PredictSpellPrice estimates the premium values for given comma-separated spell names and item name.
func (c *Client) PredictSpellPrice(
	ctx context.Context,
	spells, item string,
	mods ...aoni.RequestModifier,
) (*SpellPredictionResponse, error) {
	req := struct {
		Spells string `url:"spells"`
		Item   string `url:"item"`
	}{Spells: spells, Item: item}

	allMods := append([]aoni.RequestModifier{
		mod.WithQuery(req),
	}, mods...)

	return request.GetTo[SpellPredictionResponse](ctx, c.spell, "spell/predict", allMods...)
}

// PredictSpellItem predicts spelled item price premium via POST.
func (c *Client) PredictSpellItem(
	ctx context.Context,
	itemName string,
	spellIDs []int,
	mods ...aoni.RequestModifier,
) (*PredictSpellItemResponse, error) {
	req := PredictSpellItemRequest{ItemName: itemName, SpellIDs: spellIDs}

	resp, err := request.PostTo[PredictSpellItemResponse](
		ctx, c.spell, "spell/predict-spell-item", req, mods...,
	)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetSpellValue returns the predicted premium for given comma-separated spell defindex IDs.
func (c *Client) GetSpellValue(
	ctx context.Context,
	ids string,
	mods ...aoni.RequestModifier,
) (*SpellValueResponse, error) {
	req := struct {
		IDs string `url:"ids"`
	}{IDs: ids}

	allMods := append([]aoni.RequestModifier{
		mod.WithQuery(req),
	}, mods...)

	return request.GetTo[SpellValueResponse](ctx, c.spell, "spell/spell-value", allMods...)
}

// GetSpellAnalytics returns comprehensive market analytics for all tracked spell combinations.
func (c *Client) GetSpellAnalytics(ctx context.Context, mods ...aoni.RequestModifier) ([]*SpellAnalyticsEntry, error) {
	resp, err := request.GetTo[[]*SpellAnalyticsEntry](ctx, c.spell, "spell/spell-analytics", mods...)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetItemSpellPremium returns the detailed spell premium breakdown for a specific item and spell combination.
func (c *Client) GetItemSpellPremium(
	ctx context.Context,
	item, ids string,
	mods ...aoni.RequestModifier,
) (*ItemSpellPremiumResponse, error) {
	req := struct {
		Item string `url:"item"`
		IDs  string `url:"ids"`
	}{Item: item, IDs: ids}

	allMods := append([]aoni.RequestModifier{
		mod.WithQuery(req),
	}, mods...)

	return request.GetTo[ItemSpellPremiumResponse](
		ctx, c.spell, "spell/item-spell-premium",
		allMods...,
	)
}

// GetSpellByID returns spell metadata for a given spell defindex ID.
func (c *Client) GetSpellByID(ctx context.Context, id int, mods ...aoni.RequestModifier) (*SpellMetadata, error) {
	req := struct {
		ID int `url:"id"`
	}{ID: id}

	allMods := append([]aoni.RequestModifier{
		mod.WithQuery(req),
	}, mods...)

	return request.GetTo[SpellMetadata](ctx, c.spell, "spell/spell-id-to-name", allMods...)
}

// GetSpellByName returns spell metadata for a given spell name.
func (c *Client) GetSpellByName(
	ctx context.Context,
	name string,
	mods ...aoni.RequestModifier,
) (*SpellMetadata, error) {
	req := struct {
		Name string `url:"name"`
	}{Name: name}

	allMods := append([]aoni.RequestModifier{
		mod.WithQuery(req),
	}, mods...)

	return request.GetTo[SpellMetadata](ctx, c.spell, "spell/spell-name-to-id", allMods...)
}

// ListSpells lists all available TF2 spell definitions.
func (c *Client) ListSpells(ctx context.Context, mods ...aoni.RequestModifier) ([]*SpellMetadata, error) {
	resp, err := request.GetTo[[]*SpellMetadata](ctx, c.spell, "spell/spells", mods...)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetSpellFetcherStatus returns the status and statistics of the spell data collection fetcher.
func (c *Client) GetSpellFetcherStatus(
	ctx context.Context,
	mods ...aoni.RequestModifier,
) (*FetcherStatusResponse, error) {
	return request.GetTo[FetcherStatusResponse](ctx, c.spell, "spell/fetcher-status", mods...)
}

// GetSpellHealth returns health status for the spell service.
func (c *Client) GetSpellHealth(ctx context.Context, mods ...aoni.RequestModifier) (*SpellHealthResponse, error) {
	return request.GetTo[SpellHealthResponse](ctx, c.spell, "spell/health", mods...)
}

// GetServiceStats returns comprehensive service statistics.
func (c *Client) GetServiceStats(ctx context.Context, mods ...aoni.RequestModifier) (*ServiceStatsResponse, error) {
	return request.GetTo[ServiceStatsResponse](ctx, c.spell, "stats", mods...)
}

// GetUnifiedStatus returns unified operational status across all services.
func (c *Client) GetUnifiedStatus(ctx context.Context, mods ...aoni.RequestModifier) (*UnifiedStatusResponse, error) {
	return request.GetTo[UnifiedStatusResponse](ctx, c.spell, "spell/status-proxy", mods...)
}
