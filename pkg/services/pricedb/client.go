// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

import (
	"context"

	"github.com/lemon4ksan/g-man/pkg/rest"
)

const (
	// BaseURL is the base URL for the PriceDB API.
	BaseURL = "https://pricedb.io"
	// SKUURL is the base URL for the PriceDB SKU API.
	SKUURL = "https://sku.pricedb.io"
	// SpellURL is the base URL for the PriceDB Spell API.
	SpellURL = "https://spell.pricedb.io"
)

// Client is a thread-safe HTTP client for interacting with PriceDB.
type Client struct {
	restClient  *rest.Client
	skuClient   *rest.Client
	spellClient *rest.Client
}

// NewClient creates a new PriceDB API client.
// If httpClient is nil, a default robust client is created.
func NewClient(httpClient rest.HTTPDoer) *Client {
	return &Client{
		restClient:  rest.NewClient(httpClient).WithBaseURL(BaseURL).WithUserAgent("G-man Bot/1.0"),
		skuClient:   rest.NewClient(httpClient).WithBaseURL(SKUURL).WithUserAgent("G-man Bot/1.0"),
		spellClient: rest.NewClient(httpClient).WithBaseURL(SpellURL).WithUserAgent("G-man Bot/1.0"),
	}
}

// WithUserAgent returns a new Client configured with a custom User-Agent.
func (c *Client) WithUserAgent(ua string) *Client {
	return &Client{
		restClient:  c.restClient.WithUserAgent(ua),
		skuClient:   c.skuClient.WithUserAgent(ua),
		spellClient: c.spellClient.WithUserAgent(ua),
	}
}

// UserAgent returns the configured User-Agent for this client.
func (c *Client) UserAgent() string {
	return c.restClient.UserAgent()
}

// GetItem fetches the latest price for a specific item SKU.
func (c *Client) GetItem(ctx context.Context, sku string) (*Price, error) {
	return rest.GetJSON[Price](ctx, c.restClient, "/api/item/{sku}", nil, rest.WithVar("sku", sku))
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

	var allPrices []*Price

	for i := 0; i < len(validSKUs); i += batchSize {
		end := min(i+batchSize, len(validSKUs))
		batch := validSKUs[i:end]

		req := bulkRequest{SKUs: batch}

		resp, err := rest.PostJSON[bulkRequest, []*Price](ctx, c.restClient, "/api/items-bulk", req, nil)
		if err != nil {
			return nil, err
		}

		if resp != nil {
			allPrices = append(allPrices, *resp...)
		}
	}

	return allPrices, nil
}

// Search performs a fuzzy search for items by name.
func (c *Client) Search(ctx context.Context, query string, limit int) (*SearchResult, error) {
	req := struct {
		Q     string `url:"q"`
		Limit int    `url:"limit,omitempty"`
	}{query, limit}

	return rest.GetJSON[SearchResult](ctx, c.restClient, "/api/search", req)
}

// GetHistory returns the price history for a specific SKU.
// start and end are optional Unix timestamps (use 0 to ignore).
func (c *Client) GetHistory(ctx context.Context, sku string, start, end int64) ([]*Price, error) {
	req := struct {
		Start int64 `url:"start,omitempty"`
		End   int64 `url:"end,omitempty"`
	}{start, end}

	resp, err := rest.GetJSON[[]*Price](ctx, c.restClient, "/api/item-history/{sku}", req, rest.WithVar("sku", sku))
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetStats returns statistics (min, max, avg) for an item's price history.
func (c *Client) GetStats(ctx context.Context, sku string) (*ItemStats, error) {
	return rest.GetJSON[ItemStats](ctx, c.restClient, "/api/item-stats/{sku}", nil, rest.WithVar("sku", sku))
}

// Compare compares two items side by side, returning the price differences.
func (c *Client) Compare(ctx context.Context, sku1, sku2 string) (*CompareResult, error) {
	return rest.GetJSON[CompareResult](
		ctx,
		c.restClient,
		"/api/compare/{sku1}/{sku2}",
		nil,
		rest.WithVar("sku1", sku1),
		rest.WithVar("sku2", sku2),
	)
}

// TriggerPriceCheck requests PriceDB to update the price for a specific SKU.
// This hits the Autobot integration endpoint.
func (c *Client) TriggerPriceCheck(ctx context.Context, sku string) error {
	// We don't care about the response body, just the HTTP status code
	_, err := rest.PostJSON[any, any](ctx, c.restClient, "/api/autob/items/{sku}", nil, nil, rest.WithVar("sku", sku))

	return err
}

// HealthCheck returns the current system statistics and health of the API.
func (c *Client) HealthCheck(ctx context.Context) (*CacheStats, error) {
	return rest.GetJSON[CacheStats](ctx, c.restClient, "/api/cache-stats", nil)
}

// ResolveName looks up an item by name using the SKU Service.
func (c *Client) ResolveName(ctx context.Context, name string) (map[string]any, error) {
	resp, err := rest.GetJSON[map[string]any](ctx, c.skuClient, "/api/name/{name}", nil, rest.WithVar("name", name))
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// ResolveSKU looks up item properties by its SKU using the SKU Service.
func (c *Client) ResolveSKU(ctx context.Context, sku string) (map[string]any, error) {
	resp, err := rest.GetJSON[map[string]any](ctx, c.skuClient, "/api/sku/{sku}", nil, rest.WithVar("sku", sku))
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetSchema fetches the complete TF2 schema from PriceDB.
func (c *Client) GetSchema(ctx context.Context) (map[string]any, error) {
	resp, err := rest.GetJSON[map[string]any](ctx, c.skuClient, "/api/schema", nil)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetHealth returns the health status message from the PriceDB API.
func (c *Client) GetHealth(ctx context.Context) (string, error) {
	resp, err := rest.GetJSON[string](ctx, c.restClient, "/api/", nil)
	if err != nil {
		return "", err
	}

	return *resp, nil
}

// GetItems returns a list of all unique items (name and SKU) in the database.
func (c *Client) GetItems(ctx context.Context) ([]*ItemBrief, error) {
	resp, err := rest.GetJSON[[]*ItemBrief](ctx, c.restClient, "/api/items", nil)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetLatestPrices returns the 10 most recent price entries from the database.
func (c *Client) GetLatestPrices(ctx context.Context) ([]*Price, error) {
	resp, err := rest.GetJSON[[]*Price](ctx, c.restClient, "/api/latest-prices", nil)
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

	return rest.GetJSON[PriceHistoryResponse](ctx, c.restClient, "/api/prices", req)
}

// GetSnapshot returns the most recent price for each SKU as of the given unix timestamp.
func (c *Client) GetSnapshot(ctx context.Context, timestamp int64) ([]*Price, error) {
	resp, err := rest.GetJSON[[]*Price](
		ctx,
		c.restClient,
		"/api/snapshot/{timestamp}",
		nil,
		rest.WithVar("timestamp", timestamp),
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

	resp, err := rest.GetJSON[string](ctx, c.restClient, "/api/graph/{sku}", req, rest.WithVar("sku", sku))
	if err != nil {
		return "", err
	}

	return *resp, nil
}

// GetAutobItems fetches the full pricelist in TF2Autobot-compatible format.
func (c *Client) GetAutobItems(ctx context.Context) (*AutobItemsResponse, error) {
	return rest.GetJSON[AutobItemsResponse](ctx, c.restClient, "/api/autob/items", nil)
}

// GetAutobItem fetches the latest price for a single SKU in TF2Autobot-compatible format.
func (c *Client) GetAutobItem(ctx context.Context, sku string) (*Price, error) {
	return rest.GetJSON[Price](ctx, c.restClient, "/api/autob/items/{sku}", nil, rest.WithVar("sku", sku))
}

// GetImageBySKU returns the raw image data for a SKU from the SKU service.
func (c *Client) GetImageBySKU(ctx context.Context, sku string) ([]byte, error) {
	resp, err := rest.GetJSON[[]byte](ctx, c.skuClient, "/api/sku/{sku}/image", nil, rest.WithVar("sku", sku))
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetImageByName returns the raw image data for an item by name from the SKU service.
func (c *Client) GetImageByName(ctx context.Context, name string) ([]byte, error) {
	resp, err := rest.GetJSON[[]byte](ctx, c.skuClient, "/api/name/{name}/image", nil, rest.WithVar("name", name))
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

	resp, err := rest.GetJSON[response](ctx, c.skuClient, "/api/effect/list", nil)
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

	resp, err := rest.GetJSON[response](ctx, c.skuClient, "/api/effect/{id}", nil, rest.WithVar("id", id))
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

	resp, err := rest.GetJSON[response](ctx, c.skuClient, "/api/effect/name/{name}", nil, rest.WithVar("name", name))
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

	resp, err := rest.GetJSON[response](ctx, c.skuClient, "/api/paint/list", nil)
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

	resp, err := rest.GetJSON[response](ctx, c.skuClient, "/api/paint/{id}", nil, rest.WithVar("id", id))
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

	resp, err := rest.GetJSON[response](ctx, c.skuClient, "/api/paint/name/{name}", nil, rest.WithVar("name", name))
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

	resp, err := rest.GetJSON[response](ctx, c.skuClient, "/api/wear/list", nil)
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

	resp, err := rest.GetJSON[response](ctx, c.skuClient, "/api/wear/{id}", nil, rest.WithVar("id", id))
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

// DownloadSchema downloads the TF2 item schema as a map.
func (c *Client) DownloadSchema(ctx context.Context) (map[string]any, error) {
	resp, err := rest.GetJSON[map[string]any](ctx, c.skuClient, "/api/download", nil)
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

	return rest.GetJSON[SpellPredictionResponse](ctx, c.spellClient, "/api/spell/predict", req)
}

// PredictSpellItem predicts spelled item price premium via POST.
func (c *Client) PredictSpellItem(
	ctx context.Context,
	itemName string,
	spellIDs []int,
) (*PredictSpellItemResponse, error) {
	req := PredictSpellItemRequest{ItemName: itemName, SpellIDs: spellIDs}

	resp, err := rest.PostJSON[PredictSpellItemRequest, PredictSpellItemResponse](
		ctx,
		c.spellClient,
		"/api/spell/predict-spell-item",
		req,
		nil,
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

	return rest.GetJSON[SpellValueResponse](ctx, c.spellClient, "/api/spell/spell-value", req)
}

// GetSpellAnalytics returns comprehensive market analytics for all tracked spell combinations.
func (c *Client) GetSpellAnalytics(ctx context.Context) ([]*SpellAnalyticsEntry, error) {
	resp, err := rest.GetJSON[[]*SpellAnalyticsEntry](ctx, c.spellClient, "/api/spell/spell-analytics", nil)
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

	return rest.GetJSON[ItemSpellPremiumResponse](ctx, c.spellClient, "/api/spell/item-spell-premium", req)
}

// GetSpellByID returns spell metadata for a given spell defindex ID.
func (c *Client) GetSpellByID(ctx context.Context, id int) (*SpellMetadata, error) {
	req := struct {
		ID int `url:"id"`
	}{ID: id}

	return rest.GetJSON[SpellMetadata](ctx, c.spellClient, "/api/spell/spell-id-to-name", req)
}

// GetSpellByName returns spell metadata for a given spell name.
func (c *Client) GetSpellByName(ctx context.Context, name string) (*SpellMetadata, error) {
	req := struct {
		Name string `url:"name"`
	}{Name: name}

	return rest.GetJSON[SpellMetadata](ctx, c.spellClient, "/api/spell/spell-name-to-id", req)
}

// ListSpells lists all available TF2 spell definitions.
func (c *Client) ListSpells(ctx context.Context) ([]*SpellMetadata, error) {
	resp, err := rest.GetJSON[[]*SpellMetadata](ctx, c.spellClient, "/api/spell/spells", nil)
	if err != nil {
		return nil, err
	}

	return *resp, nil
}

// GetSpellFetcherStatus returns the status and statistics of the spell data collection fetcher.
func (c *Client) GetSpellFetcherStatus(ctx context.Context) (*FetcherStatusResponse, error) {
	return rest.GetJSON[FetcherStatusResponse](ctx, c.spellClient, "/api/spell/fetcher-status", nil)
}

// GetSpellHealth returns health status for the spell service.
func (c *Client) GetSpellHealth(ctx context.Context) (*SpellHealthResponse, error) {
	return rest.GetJSON[SpellHealthResponse](ctx, c.spellClient, "/api/spell/health", nil)
}

// GetServiceStats returns comprehensive service statistics.
func (c *Client) GetServiceStats(ctx context.Context) (*ServiceStatsResponse, error) {
	return rest.GetJSON[ServiceStatsResponse](ctx, c.spellClient, "/api/stats", nil)
}

// GetUnifiedStatus returns unified operational status across all services.
func (c *Client) GetUnifiedStatus(ctx context.Context) (*UnifiedStatusResponse, error) {
	return rest.GetJSON[UnifiedStatusResponse](ctx, c.spellClient, "/api/spell/status-proxy", nil)
}
