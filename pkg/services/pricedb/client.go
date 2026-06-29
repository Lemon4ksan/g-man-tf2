// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

import (
	"context"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/miyako/generic"
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
	rest  *aoni.Client
	sku   *aoni.Client
	spell *aoni.Client
}

// NewClient creates a new PriceDB API client.
// If httpClient is nil, a default robust client is created.
func NewClient(client *aoni.Client) *Client {
	client = generic.Coalesce(client, aoni.DefaultClient)

	return &Client{
		rest:  client.WithBaseURL(BaseURL).WithUserAgent("G-man Bot/1.0"),
		sku:   client.WithBaseURL(SKUURL).WithUserAgent("G-man Bot/1.0"),
		spell: client.WithBaseURL(SpellURL).WithUserAgent("G-man Bot/1.0"),
	}
}

// WithUserAgent returns a new Client configured with a custom User-Agent.
func (c *Client) WithUserAgent(ua string) *Client {
	return &Client{
		rest:  c.rest.WithUserAgent(ua),
		sku:   c.sku.WithUserAgent(ua),
		spell: c.spell.WithUserAgent(ua),
	}
}

// UserAgent returns the configured User-Agent for this client.
func (c *Client) UserAgent() string {
	return c.rest.UserAgent()
}

// GetItem fetches the latest price for a specific item SKU.
func (c *Client) GetItem(ctx context.Context, sku string) (*Price, error) {
	// Чистая сигнатура без nil!
	return aoni.GetJSON[Price](ctx, c.rest, "item/{sku}", aoni.WithVar("sku", sku))
}

// GetItemsBulk fetches the latest prices for an array of SKUs in a single request.
// It automatically filters out empty SKUs and splits the request into batches of up to 100 SKUs.
func (c *Client) GetItemsBulk(ctx context.Context, skus []string) ([]*Price, error) {
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

		resp, err := aoni.PostJSON[[]*Price](chunkCtx, c.rest, "items-bulk", req)
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
func (c *Client) Search(ctx context.Context, query string, limit int) (*SearchResult, error) {
	req := struct {
		Q     string `url:"q"`
		Limit int    `url:"limit,omitempty"`
	}{query, limit}

	return aoni.GetJSON[SearchResult](ctx, c.rest, "search", aoni.WithQuery(req))
}

// GetHistory returns the price history for a specific SKU.
// start and end are optional Unix timestamps (use 0 to ignore).
func (c *Client) GetHistory(ctx context.Context, sku string, start, end int64) ([]*Price, error) {
	req := struct {
		Start int64 `url:"start,omitempty"`
		End   int64 `url:"end,omitempty"`
	}{start, end}

	resp, err := aoni.GetJSON[[]*Price](
		ctx,
		c.rest,
		"item-history/{sku}",
		aoni.WithQuery(req),
		aoni.WithVar("sku", sku),
	)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetStats returns statistics (min, max, avg) for an item's price history.
func (c *Client) GetStats(ctx context.Context, sku string) (*ItemStats, error) {
	return aoni.GetJSON[ItemStats](ctx, c.rest, "item-stats/{sku}", aoni.WithVar("sku", sku))
}

// Compare compares two items side by side, returning the price differences.
func (c *Client) Compare(ctx context.Context, sku1, sku2 string) (*CompareResult, error) {
	return aoni.GetJSON[CompareResult](
		ctx, c.rest, "compare/{sku1}/{sku2}",
		aoni.WithVar("sku1", sku1), aoni.WithVar("sku2", sku2),
	)
}

// TriggerPriceCheck requests PriceDB to update the price for a specific SKU.
// This hits the Autobot integration endpoint.
func (c *Client) TriggerPriceCheck(ctx context.Context, sku string) error {
	_, err := aoni.PostJSON[any](ctx, c.rest, "autob/items/{sku}", nil, aoni.WithVar("sku", sku))
	return err
}

// HealthCheck returns the current system statistics and health of the API.
func (c *Client) HealthCheck(ctx context.Context) (*CacheStats, error) {
	return aoni.GetJSON[CacheStats](ctx, c.rest, "cache-stats")
}

// ResolveName looks up an item by name using the SKU Service.
func (c *Client) ResolveName(ctx context.Context, name string) (map[string]any, error) {
	resp, err := aoni.GetJSON[map[string]any](ctx, c.sku, "name/{name}", aoni.WithVar("name", name))
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// ResolveSKU looks up item properties by its SKU using the SKU Service.
func (c *Client) ResolveSKU(ctx context.Context, sku string) (map[string]any, error) {
	resp, err := aoni.GetJSON[map[string]any](ctx, c.sku, "sku/{sku}", aoni.WithVar("sku", sku))
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetSchema fetches the complete TF2 schema from PriceDB.
func (c *Client) GetSchema(ctx context.Context) (map[string]any, error) {
	resp, err := aoni.GetJSON[map[string]any](ctx, c.sku, "schema")
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetHealth returns the health status message from the PriceDB API.
func (c *Client) GetHealth(ctx context.Context) (string, error) {
	resp, err := aoni.GetJSON[string](ctx, c.rest, "")
	if err != nil {
		return "", err
	}

	return *resp, nil
}

// GetItems returns a list of all unique items (name and SKU) in the database.
func (c *Client) GetItems(ctx context.Context) ([]*ItemBrief, error) {
	resp, err := aoni.GetJSON[[]*ItemBrief](ctx, c.rest, "items")
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetLatestPrices returns the 10 most recent price entries from the database.
func (c *Client) GetLatestPrices(ctx context.Context) ([]*Price, error) {
	resp, err := aoni.GetJSON[[]*Price](ctx, c.rest, "latest-prices")
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetPrices returns paginated price history.
func (c *Client) GetPrices(ctx context.Context, limit, offset int) (*PriceHistoryResponse, error) {
	req := struct {
		Limit  int `url:"limit,omitempty"`
		Offset int `url:"offset,omitempty"`
	}{Limit: limit, Offset: offset}

	return aoni.GetJSON[PriceHistoryResponse](ctx, c.rest, "prices", aoni.WithQuery(req))
}

// GetSnapshot returns the most recent price for each SKU as of the given unix timestamp.
func (c *Client) GetSnapshot(ctx context.Context, timestamp int64) ([]*Price, error) {
	resp, err := aoni.GetJSON[[]*Price](
		ctx, c.rest, "snapshot/{timestamp}",
		aoni.WithVar("timestamp", timestamp),
	)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetGraph returns the HTML page containing an interactive Chart.js graph.
func (c *Client) GetGraph(ctx context.Context, sku string, header bool, height int, width string) (string, error) {
	req := struct {
		Header bool   `url:"header"`
		Height int    `url:"height,omitempty"`
		Width  string `url:"width,omitempty"`
	}{Header: header, Height: height, Width: width}

	resp, err := aoni.GetJSON[string](
		ctx,
		c.rest,
		"graph/{sku}",
		aoni.WithQuery(req),
		aoni.WithVar("sku", sku),
	)
	if err != nil {
		return "", err
	}

	return *resp, nil
}

// GetAutobItems fetches the full pricelist in TF2Autobot-compatible format.
func (c *Client) GetAutobItems(ctx context.Context) (*AutobItemsResponse, error) {
	return aoni.GetJSON[AutobItemsResponse](ctx, c.rest, "autob/items")
}

// GetAutobItem fetches the latest price for a single SKU in TF2Autobot-compatible format.
func (c *Client) GetAutobItem(ctx context.Context, sku string) (*Price, error) {
	return aoni.GetJSON[Price](ctx, c.rest, "autob/items/{sku}", aoni.WithVar("sku", sku))
}

// GetImageBySKU returns the raw image data for a SKU from the SKU service.
func (c *Client) GetImageBySKU(ctx context.Context, sku string) ([]byte, error) {
	resp, err := aoni.GetJSON[[]byte](ctx, c.sku, "sku/{sku}/image", aoni.WithVar("sku", sku))
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetImageByName returns the raw image data for an item by name from the SKU service.
func (c *Client) GetImageByName(ctx context.Context, name string) ([]byte, error) {
	resp, err := aoni.GetJSON[[]byte](ctx, c.sku, "name/{name}/image", aoni.WithVar("name", name))
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// ListEffects lists all unusual particle effects known to the TF2 schema.
func (c *Client) ListEffects(ctx context.Context) ([]*EffectInfo, error) {
	type response struct {
		Success bool          `json:"success"`
		Data    []*EffectInfo `json:"data"`
	}

	resp, err := aoni.GetJSON[response](ctx, c.sku, "effect/list")
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetEffectByID returns the name of a single unusual effect by its numeric ID.
func (c *Client) GetEffectByID(ctx context.Context, id int) (*EffectInfo, error) {
	type response struct {
		Success bool        `json:"success"`
		Data    *EffectInfo `json:"data"`
	}

	resp, err := aoni.GetJSON[response](ctx, c.sku, "effect/{id}", aoni.WithVar("id", id))
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetEffectByName returns the ID of a single unusual effect by its display name.
func (c *Client) GetEffectByName(ctx context.Context, name string) (*EffectInfo, error) {
	type response struct {
		Success bool        `json:"success"`
		Data    *EffectInfo `json:"data"`
	}

	resp, err := aoni.GetJSON[response](ctx, c.sku, "effect/name/{name}", aoni.WithVar("name", name))
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// ListPaints lists all paint colors.
func (c *Client) ListPaints(ctx context.Context) ([]*PaintInfo, error) {
	type response struct {
		Success bool         `json:"success"`
		Data    []*PaintInfo `json:"data"`
	}

	resp, err := aoni.GetJSON[response](ctx, c.sku, "paint/list")
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetPaintByID returns the paint color by defindex.
func (c *Client) GetPaintByID(ctx context.Context, id int) (*PaintInfo, error) {
	type response struct {
		Success bool       `json:"success"`
		Data    *PaintInfo `json:"data"`
	}

	resp, err := aoni.GetJSON[response](ctx, c.sku, "paint/{id}", aoni.WithVar("id", id))
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetPaintByName returns the paint color defindex by its display name.
func (c *Client) GetPaintByName(ctx context.Context, name string) (*PaintInfo, error) {
	type response struct {
		Success bool       `json:"success"`
		Data    *PaintInfo `json:"data"`
	}

	resp, err := aoni.GetJSON[response](ctx, c.sku, "paint/name/{name}", aoni.WithVar("name", name))
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// ListWears lists the five wear levels.
func (c *Client) ListWears(ctx context.Context) ([]*WearInfo, error) {
	type response struct {
		Success bool        `json:"success"`
		Data    []*WearInfo `json:"data"`
	}

	resp, err := aoni.GetJSON[response](ctx, c.sku, "wear/list")
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// GetWearByID returns the display name for a single wear level ID (1–5).
func (c *Client) GetWearByID(ctx context.Context, id int) (*WearInfo, error) {
	type response struct {
		Success bool      `json:"success"`
		Data    *WearInfo `json:"data"`
	}

	resp, err := aoni.GetJSON[response](ctx, c.sku, "wear/{id}", aoni.WithVar("id", id))
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// DownloadSchema downloads the TF2 item schema as a map.
func (c *Client) DownloadSchema(ctx context.Context) (map[string]any, error) {
	resp, err := aoni.GetJSON[map[string]any](ctx, c.sku, "download")
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// PredictSpellPrice estimates the premium values for given comma-separated spell names and item name.
func (c *Client) PredictSpellPrice(ctx context.Context, spells, item string) (*SpellPredictionResponse, error) {
	req := struct {
		Spells string `url:"spells"`
		Item   string `url:"item"`
	}{Spells: spells, Item: item}

	return aoni.GetJSON[SpellPredictionResponse](ctx, c.spell, "spell/predict", aoni.WithQuery(req))
}

// PredictSpellItem predicts spelled item price premium via POST.
func (c *Client) PredictSpellItem(
	ctx context.Context,
	itemName string,
	spellIDs []int,
) (*PredictSpellItemResponse, error) {
	req := PredictSpellItemRequest{ItemName: itemName, SpellIDs: spellIDs}

	resp, err := aoni.PostJSON[PredictSpellItemResponse](
		ctx, c.spell, "spell/predict-spell-item", req,
	)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetSpellValue returns the predicted premium for given comma-separated spell defindex IDs.
func (c *Client) GetSpellValue(ctx context.Context, ids string) (*SpellValueResponse, error) {
	req := struct {
		IDs string `url:"ids"`
	}{IDs: ids}

	return aoni.GetJSON[SpellValueResponse](ctx, c.spell, "spell/spell-value", aoni.WithQuery(req))
}

// GetSpellAnalytics returns comprehensive market analytics for all tracked spell combinations.
func (c *Client) GetSpellAnalytics(ctx context.Context) ([]*SpellAnalyticsEntry, error) {
	resp, err := aoni.GetJSON[[]*SpellAnalyticsEntry](ctx, c.spell, "spell/spell-analytics")
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetItemSpellPremium returns the detailed spell premium breakdown for a specific item and spell combination.
func (c *Client) GetItemSpellPremium(ctx context.Context, item, ids string) (*ItemSpellPremiumResponse, error) {
	req := struct {
		Item string `url:"item"`
		IDs  string `url:"ids"`
	}{Item: item, IDs: ids}

	return aoni.GetJSON[ItemSpellPremiumResponse](
		ctx, c.spell, "spell/item-spell-premium",
		aoni.WithQuery(req),
	)
}

// GetSpellByID returns spell metadata for a given spell defindex ID.
func (c *Client) GetSpellByID(ctx context.Context, id int) (*SpellMetadata, error) {
	req := struct {
		ID int `url:"id"`
	}{ID: id}

	return aoni.GetJSON[SpellMetadata](ctx, c.spell, "spell/spell-id-to-name", aoni.WithQuery(req))
}

// GetSpellByName returns spell metadata for a given spell name.
func (c *Client) GetSpellByName(ctx context.Context, name string) (*SpellMetadata, error) {
	req := struct {
		Name string `url:"name"`
	}{Name: name}

	return aoni.GetJSON[SpellMetadata](ctx, c.spell, "spell/spell-name-to-id", aoni.WithQuery(req))
}

// ListSpells lists all available TF2 spell definitions.
func (c *Client) ListSpells(ctx context.Context) ([]*SpellMetadata, error) {
	resp, err := aoni.GetJSON[[]*SpellMetadata](ctx, c.spell, "spell/spells")
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetSpellFetcherStatus returns the status and statistics of the spell data collection fetcher.
func (c *Client) GetSpellFetcherStatus(ctx context.Context) (*FetcherStatusResponse, error) {
	return aoni.GetJSON[FetcherStatusResponse](ctx, c.spell, "spell/fetcher-status")
}

// GetSpellHealth returns health status for the spell service.
func (c *Client) GetSpellHealth(ctx context.Context) (*SpellHealthResponse, error) {
	return aoni.GetJSON[SpellHealthResponse](ctx, c.spell, "spell/health")
}

// GetServiceStats returns comprehensive service statistics.
func (c *Client) GetServiceStats(ctx context.Context) (*ServiceStatsResponse, error) {
	return aoni.GetJSON[ServiceStatsResponse](ctx, c.spell, "stats")
}

// GetUnifiedStatus returns unified operational status across all services.
func (c *Client) GetUnifiedStatus(ctx context.Context) (*UnifiedStatusResponse, error) {
	return aoni.GetJSON[UnifiedStatusResponse](ctx, c.spell, "spell/status-proxy")
}
