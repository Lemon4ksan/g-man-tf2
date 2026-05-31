// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

// PricelistChangedSource specifies the origin of pricelist modifications.
type PricelistChangedSource string

const (
	// PricelistChangedSourceManual is the source for pricelist modifications made manually.
	PricelistChangedSourceManual PricelistChangedSource = "Manual"
	// PricelistChangedSourcePriceDB is the source for automatic pricing from the price database.
	PricelistChangedSourcePriceDB PricelistChangedSource = "PriceDB"
)

// Currencies represents the price in TF2 currency.
type Currencies struct {
	Keys  int     `json:"keys"`
	Metal float64 `json:"metal"`
}

// ToMetal converts the currency to a total metal value.
func (c Currencies) ToMetal(keyPrice float64) float64 {
	return float64(c.Keys)*keyPrice + c.Metal
}

// IsZero returns true if both keys and metal are zero.
func (c Currencies) IsZero() bool {
	return c.Keys == 0 && c.Metal == 0
}

// Valid returns true if the price components are non-negative.
func (c Currencies) Valid() bool {
	return c.Keys >= 0 && c.Metal >= 0
}

// Price represents a single price entry for an item.
type Price struct {
	Name   string     `json:"name"`
	SKU    string     `json:"sku"`
	Source string     `json:"source"`
	Time   int64      `json:"time"`
	Buy    Currencies `json:"buy"`
	Sell   Currencies `json:"sell"`
}

// Validate checks if the price data is sane.
func (p Price) Validate() bool {
	if p.SKU == "" {
		return false
	}

	if !p.Buy.Valid() || !p.Sell.Valid() {
		return false
	}

	// Typically buy price should not exceed sell price
	// but we don't enforce it here to allow for market volatility or errors in source.
	return true
}

// HasProfit returns true if selling results in more metal than buying.
func (p Price) HasProfit(keyPrice float64) bool {
	return p.Sell.ToMetal(keyPrice) > p.Buy.ToMetal(keyPrice)
}

// SearchResult represents the response from the fuzzy search endpoint.
type SearchResult struct {
	Query   string `json:"query"`
	Total   int    `json:"total"`
	Limit   int    `json:"limit"`
	Results []struct {
		Price
		Relevance int `json:"relevance"`
	} `json:"results"`
}

// ItemStats represents the aggregated statistics for an item's price history.
type ItemStats struct {
	Buy  StatDetails `json:"buy"`
	Sell StatDetails `json:"sell"`
}

// StatDetails represents the statistical details for price history.
type StatDetails struct {
	Count int `json:"count"`
	Keys  struct {
		Min int     `json:"min"`
		Max int     `json:"max"`
		Avg float64 `json:"avg"`
	} `json:"keys"`
	Metal struct {
		Min float64 `json:"min"`
		Max float64 `json:"max"`
		Avg float64 `json:"avg"`
	} `json:"metal"`
}

// CompareResult represents the side-by-side comparison of two items.
type CompareResult struct {
	Items map[string]struct {
		Name string     `json:"name"`
		SKU  string     `json:"sku"`
		Buy  Currencies `json:"buy"`
		Sell Currencies `json:"sell"`
	} `json:"items"`
	Comparison struct {
		BuyDifference  Currencies `json:"buyDifference"`
		SellDifference Currencies `json:"sellDifference"`
	} `json:"comparison"`
	Meta struct {
		Compared    string `json:"compared"`
		HistoryDays int    `json:"historyDays"`
	} `json:"meta"`
}

// CacheStats represents the internal health and stats of the PriceDB server.
type CacheStats struct {
	Status string `json:"status"` // From /api/ health check
	DB     string `json:"db"`     // From /api/ health check
	Cache  struct {
		Size         int `json:"size"`
		MaxSize      int `json:"maxSize"`
		ActiveTimers int `json:"activeTimers"`
	} `json:"cache"`
	Database struct {
		TotalPrices  int   `json:"totalPrices"`
		UniqueItems  int   `json:"uniqueItems"`
		LatestUpdate int64 `json:"latestUpdate"`
	} `json:"database"`
}

// bulkRequest is the internal payload for fetching multiple SKUs.
type bulkRequest struct {
	SKUs []string `json:"skus"`
}

// ItemBrief represents name and SKU of a TF2 item.
type ItemBrief struct {
	Name string `json:"name"`
	SKU  string `json:"sku"`
}

// PriceHistoryResponse represents paginated price history.
type PriceHistoryResponse struct {
	Total   int      `json:"total"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
	Results []*Price `json:"results"`
}

// AutobItemsResponse represents a collection of latest prices in bot format.
type AutobItemsResponse struct {
	Success  bool     `json:"success"`
	Currency string   `json:"currency"`
	Items    []*Price `json:"items"`
}

// EffectInfo represents unusual effect metadata.
type EffectInfo struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// PaintInfo represents paint can metadata.
type PaintInfo struct {
	DefIndex int    `json:"defindex"`
	Name     string `json:"name"`
}

// WearInfo represents wear level metadata.
type WearInfo struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// SpellPriceInfo represents pricing metadata in various formats inside Spell predictions.
type SpellPriceInfo struct {
	TotalRef  float64 `json:"total_ref"`
	Keys      int     `json:"keys"`
	Metal     float64 `json:"metal"`
	Formatted string  `json:"formatted"`
}

// SpellPredictionResponse represents spelled item price prediction response.
type SpellPredictionResponse struct {
	ItemName      string                    `json:"item_name"`
	Spells        []string                  `json:"spells"`
	SpellIDs      []int                     `json:"spell_ids"`
	BasePrice     SpellPriceInfo            `json:"base_price"`
	Predictions   map[string]SpellPriceInfo `json:"predictions"`
	PremiumRanges map[string]struct {
		Ref       float64 `json:"ref"`
		Formatted string  `json:"formatted"`
	} `json:"premium_ranges"`
	MarketData struct {
		AvgFlatPremium float64 `json:"avg_flat_premium"`
		SampleSize     int     `json:"sample_size"`
		Confidence     string  `json:"confidence"`
	} `json:"market_data"`
	Method      string             `json:"method"`
	KeyRate     float64            `json:"key_rate"`
	Multipliers map[string]float64 `json:"multipliers"`
}

// PredictSpellItemRequest represents request body for predicting spelled item price (POST).
type PredictSpellItemRequest struct {
	ItemName string `json:"item_name"`
	SpellIDs []int  `json:"spell_ids"`
}

// PredictSpellItemResponse represents prediction response body for predicting spelled item price (POST).
type PredictSpellItemResponse struct {
	ItemName     string         `json:"item_name"`
	BasePrice    SpellPriceInfo `json:"base_price"`
	SpellPremium SpellPriceInfo `json:"spell_premium"`
	TotalPrice   SpellPriceInfo `json:"total_price"`
	SpellData    struct {
		SpellIDs       []int    `json:"spell_ids"`
		SpellNames     []string `json:"spell_names"`
		AvgFlatPremium float64  `json:"avg_flat_premium"`
		SampleSize     int      `json:"sample_size"`
		Confidence     string   `json:"confidence"`
	} `json:"spell_data"`
	Method  string  `json:"method"`
	KeyRate float64 `json:"key_rate"`
}

// SpellValueResponse represents estimated spell premium values.
type SpellValueResponse struct {
	SpellIDs         []int   `json:"spell_ids"`
	PredictedFlat    float64 `json:"predicted_flat"`
	PredictedPercent float64 `json:"predicted_percent"`
	AvgFlat          float64 `json:"avg_flat"`
	AvgPercent       float64 `json:"avg_percent"`
	Count            int     `json:"count"`
	Confidence       string  `json:"confidence"`
}

// SpellAnalyticsEntry represents comprehensive market analytics for a spell combination.
type SpellAnalyticsEntry struct {
	SpellCombo  []int   `json:"spell_combo"`
	AvgFlat     float64 `json:"avg_flat"`
	AvgPercent  float64 `json:"avg_percent"`
	Count       int     `json:"count"`
	LastUpdated string  `json:"last_updated"`
}

// ItemSpellPremiumResponse represents spell premium breakdown for an item.
type ItemSpellPremiumResponse struct {
	Item           string         `json:"item"`
	SpellIDs       []int          `json:"spell_ids"`
	BasePrice      SpellPriceInfo `json:"base_price"`
	SpellPremium   SpellPriceInfo `json:"spell_premium"`
	TotalPrice     SpellPriceInfo `json:"total_price"`
	PremiumPercent float64        `json:"premium_percent"`
	MarketData     struct {
		SampleSize     int     `json:"sample_size"`
		Confidence     string  `json:"confidence"`
		AvgFlatPremium float64 `json:"avg_flat_premium"`
	} `json:"market_data"`
}

// SpellMetadata represents spell defindex and display name.
type SpellMetadata struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	AttributeID int    `json:"attributeId"`
}

// FetcherStatusResponse represents spell data collection fetcher status.
type FetcherStatusResponse struct {
	Status           string `json:"status"`
	IsRunning        bool   `json:"isRunning"`
	LastRunTime      string `json:"lastRunTime"`
	NextScheduledRun string `json:"nextScheduledRun"`
	Statistics       struct {
		TotalFetched int `json:"totalFetched"`
		TotalAdded   int `json:"totalAdded"`
		TotalUpdated int `json:"totalUpdated"`
		RateLimits   int `json:"rateLimits"`
		Errors       int `json:"errors"`
	} `json:"statistics"`
	Schedule            string `json:"schedule"`
	HistoricalDataRange string `json:"historicalDataRange"`
	LastRunDuration     string `json:"lastRunDuration"`
	Performance         struct {
		ItemsPerMinute  float64 `json:"itemsPerMinute"`
		AvgResponseTime string  `json:"avgResponseTime"`
	} `json:"performance"`
}

// SpellHealthResponse represents spell service health check status.
type SpellHealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Uptime    string `json:"uptime"`
	Version   string `json:"version"`
}

// ServiceStatsResponse represents spell service statistics dashboard metrics.
type ServiceStatsResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Database  struct {
		Connected         bool   `json:"connected"`
		TotalSpelledItems int    `json:"totalSpelledItems"`
		AnalyzedCombos    int    `json:"analyzedCombos"`
		LastCleanup       string `json:"lastCleanup"`
	} `json:"database"`
	Fetcher struct {
		Status     string `json:"status"`
		IsRunning  bool   `json:"isRunning"`
		LastRun    string `json:"lastRun"`
		NextRun    string `json:"nextRun"`
		Statistics struct {
			TotalFetched int `json:"totalFetched"`
			TotalAdded   int `json:"totalAdded"`
			TotalUpdated int `json:"totalUpdated"`
			RateLimits   int `json:"rateLimits"`
			Errors       int `json:"errors"`
		} `json:"statistics"`
	} `json:"fetcher"`
	KeyPrices struct {
		Ref         float64 `json:"ref"`
		USD         float64 `json:"usd"`
		LastUpdated struct {
			Ref string `json:"ref"`
			USD string `json:"usd"`
		} `json:"lastUpdated"`
	} `json:"keyPrices"`
	Performance struct {
		AvgResponseTime   string `json:"avgResponseTime"`
		RequestsPerMinute int    `json:"requestsPerMinute"`
		CacheHitRate      string `json:"cacheHitRate"`
	} `json:"performance"`
	Spells struct {
		TotalAvailable int    `json:"totalAvailable"`
		WithMarketData int    `json:"withMarketData"`
		AvgPremium     string `json:"avgPremium"`
	} `json:"spells"`
}

// UnifiedStatusResponse represents the service status proxy info.
type UnifiedStatusResponse struct {
	Status     string            `json:"status"`
	Services   map[string]string `json:"services"`
	LastUpdate string            `json:"lastUpdate"`
}
