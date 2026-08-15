// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"sync"

	json "github.com/goccy/go-json"
	"github.com/lemon4ksan/aoni/request"
	"github.com/lemon4ksan/g-man/pkg/steam/community"
	"github.com/lemon4ksan/g-man/pkg/steam/community/inventory"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/lemon4ksan/miyako/log"

	"github.com/lemon4ksan/g-man-tf2/internal/bytesconv"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

var (
	ErrNoWebSession = errors.New("backpack: no community web session available")
	ErrNoSchema     = errors.New("backpack: no schema available")
)

type Remote struct {
	steamID   uint64
	client    service.Doer
	community community.Requester
	schema    *schema.Schema
	logger    log.Logger

	dupeCheckers []DupeChecker

	mu          sync.Mutex
	items       []TF2Item
	packedItems []tf2.PackedItem
	slots       int
	fetched     bool
	descCache   *sync.Map
}

type Option = generic.Option[*Remote]

func WithLogger(l log.Logger) Option {
	return func(inv *Remote) {
		inv.logger = l
	}
}

func WithDupeCheckers(dc []DupeChecker) Option {
	return func(inv *Remote) {
		inv.dupeCheckers = dc
	}
}

func WithAssetClassCache(cache *sync.Map) Option {
	return func(inv *Remote) {
		inv.descCache = cache
	}
}

func NewRemote(
	steamID uint64,
	client service.Doer,
	community community.Requester,
	schema *schema.Schema,
	opts ...Option,
) *Remote {
	p := &Remote{
		steamID:      steamID,
		client:       client,
		community:    community,
		logger:       log.Discard,
		dupeCheckers: make([]DupeChecker, 0),
		items:        make([]TF2Item, 0),
		packedItems:  make([]tf2.PackedItem, 0),
		schema:       schema,
		descCache:    &sync.Map{},
	}

	generic.ApplyOptions(p, opts...)

	return p
}

func (r *Remote) GetItems(ctx context.Context) ([]TF2Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.fetched {
		if err := r.fetch(ctx); err != nil {
			return nil, err
		}
	}

	result := make([]TF2Item, len(r.items))
	copy(result, r.items)

	return result, nil
}

func (r *Remote) GetItemsBySKU(ctx context.Context, targetSKU string) ([]TF2Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.fetched {
		if err := r.fetch(ctx); err != nil {
			return nil, err
		}
	}

	var result []TF2Item

	for i := range r.packedItems {
		if r.packedItems[i].ToSKU(r.schema) == targetSKU {
			result = append(result, r.items[i])
		}
	}

	return result, nil
}

func (r *Remote) CanTradeWithoutHold(ctx context.Context, token string) (bool, error) {
	type respType struct {
		TheirHold int `json:"their_escrow"`
		MyHold    int `json:"my_escrow"`
	}

	query := fmt.Sprintf("steamid_target=%d&trade_offer_access_token=%s", r.steamID, url.QueryEscape(token))
	resp, err := request.GetTo[respType](ctx, request.AsRequester(r.client), "IEconService/GetTradeHoldDurations/v1?"+query)
	if err != nil {
		return false, err
	}

	return resp.TheirHold == 0, nil
}

func (r *Remote) IsDuped(ctx context.Context, assetID uint64) (*bool, error) {
	isDuped, isRecorded, err := r.checkWithServices(ctx, assetID)
	if err != nil {
		return nil, err
	}

	if isRecorded {
		res := isDuped
		return &res, nil
	}

	r.mu.Lock()
	if !r.fetched {
		if err := r.fetch(ctx); err != nil {
			r.mu.Unlock()
			return nil, err
		}
	}

	var originalID uint64

	found := false

	for i := range r.items {
		if r.items[i].ID == assetID {
			originalID = r.items[i].OriginalID
			found = true
			break
		}
	}

	r.mu.Unlock()

	if !found {
		return nil, ErrItemNotFound
	}

	if originalID == 0 {
		originalID = assetID
	}

	isDuped, isRecorded, err = r.checkWithServices(ctx, originalID)
	if err != nil {
		return nil, err
	}

	if isRecorded {
		res := isDuped
		return &res, nil
	}

	return nil, nil
}

func (r *Remote) FindMetalInPartnerInventory(ctx context.Context, amount currency.Scrap) ([]*trading.Item, error) {
	r.mu.Lock()
	if !r.fetched {
		if err := r.fetch(ctx); err != nil {
			r.mu.Unlock()

			return nil, err
		}
	}

	var refinedItems, reclaimedItems, scrapItems []TF2Item

	for i := range r.items {
		switch r.items[i].ToSKU() {
		case currency.SKURefined:
			refinedItems = append(refinedItems, r.items[i])
		case currency.SKUReclaimed:
			reclaimedItems = append(reclaimedItems, r.items[i])
		case currency.SKUScrap:
			scrapItems = append(scrapItems, r.items[i])
		}
	}

	r.mu.Unlock()

	bestVal, bestRef, bestRec, bestScr := calculateBestMetalCombination(
		refinedItems,
		reclaimedItems,
		scrapItems,
		amount,
	)
	if bestVal == -1 {
		return nil, fmt.Errorf("partner is missing %d scrap for counter-offer", amount)
	}

	var selected []*trading.Item

	for i := range bestRef {
		selected = append(selected, refinedItems[i].ToEconItem())
	}

	for i := range bestRec {
		selected = append(selected, reclaimedItems[i].ToEconItem())
	}

	for i := range bestScr {
		selected = append(selected, scrapItems[i].ToEconItem())
	}

	return selected, nil
}

func calculateBestMetalCombination(
	refined, reclaimed, scrap []TF2Item,
	amount currency.Scrap,
) (bestVal, bestRef, bestRec, bestScr int) {
	bestVal = -1
	limitRef := min((int(amount)+8)/9, len(refined))

	for ref := 0; ref <= limitRef; ref++ {
		rem1 := int(amount) - 9*ref
		if rem1 <= 0 {
			val := 9 * ref
			if bestVal == -1 || val < bestVal {
				bestVal, bestRef, bestRec, bestScr = val, ref, 0, 0
			}

			continue
		}

		limitRec := min((rem1+2)/3, len(reclaimed))

		for rec := 0; rec <= limitRec; rec++ {
			rem2 := rem1 - 3*rec
			if rem2 <= 0 {
				val := 9*ref + 3*rec
				if bestVal == -1 || val < bestVal {
					bestVal, bestRef, bestRec, bestScr = val, ref, rec, 0
				}

				continue
			}

			s := min(rem2, len(scrap))
			val := 9*ref + 3*rec + s

			if val >= int(amount) && (bestVal == -1 || val < bestVal) {
				bestVal, bestRef, bestRec, bestScr = val, ref, rec, s
			}
		}
	}

	return bestVal, bestRef, bestRec, bestScr
}

func (r *Remote) checkWithServices(ctx context.Context, assetID uint64) (isDuped, isRecorded bool, err error) {
	for _, checker := range r.dupeCheckers {
		status, checkErr := checker.CheckHistory(ctx, assetID)
		if checkErr != nil {
			r.logger.Warn("Dupe checker failed",
				log.String("service", reflect.TypeOf(checker).Name()),
				log.Err(checkErr),
			)

			continue
		}

		if status.Recorded {
			return status.IsDuped, true, nil
		}
	}

	return false, false, nil
}

func (r *Remote) fetch(ctx context.Context) error {
	if r.community == nil || r.community.SessionID(community.BaseURL) == "" {
		return ErrNoWebSession
	}

	if r.schema == nil {
		return ErrNoSchema
	}

	return r.fetchCommunityStreaming(ctx)
}

func (r *Remote) fetchCommunityStreaming(ctx context.Context) error {
	econItems, _, _, err := inventory.GetUserInventoryContents(
		ctx, r.community, r.steamID, 440, 2, false, "english",
	)
	if err != nil {
		return fmt.Errorf("community streaming inventory fetch failed: %w", err)
	}

	if err := r.enrichCommunityItems(ctx, econItems); err != nil {
		r.logger.Warn("Failed to enrich community items", log.Err(err))
	}

	unifiedItems := make([]TF2Item, 0, len(econItems))
	packedList := make([]tf2.PackedItem, 0, len(econItems))

	for i := range econItems {
		tfItem := MapCEconToTF2(econItems[i], r.schema)
		unifiedItems = append(unifiedItems, tfItem)
		packedList = append(packedList, PackTF2Item(&tfItem))
	}

	r.items = unifiedItems
	r.packedItems = packedList
	r.slots = len(econItems)
	r.fetched = true

	return nil
}

type uintDescKey struct {
	ClassID    uint64
	InstanceID uint64
}

type rawAssetClassDescription struct {
	ClassID    string         `json:"classid"`
	InstanceID string         `json:"instanceid"`
	AppData    map[string]any `json:"app_data,omitempty"`
}

func (r *Remote) enrichCommunityItems(ctx context.Context, items []inventory.CEconItem) error {
	if r.client == nil {
		r.logger.Warn("No WebAPI client available, skipping community items enrichment")
		return nil
	}

	missingKeys := r.collectMissingDescriptionKeys(items)
	if len(missingKeys) == 0 {
		r.applyCachedDescriptions(items)
		return nil
	}

	const chunkSize = 50
	for i := 0; i < len(missingKeys); i += chunkSize {
		end := min(i+chunkSize, len(missingKeys))
		if err := r.fetchAssetClassInfoChunk(ctx, missingKeys[i:end]); err != nil {
			return err
		}
	}

	r.applyCachedDescriptions(items)

	return nil
}

func (r *Remote) collectMissingDescriptionKeys(items []inventory.CEconItem) []uintDescKey {
	var missing []uintDescKey

	seen := make(map[uintDescKey]bool)

	for _, it := range items {
		desc := it.Description
		if desc.AppData == nil && len(desc.Tags) == 0 && len(desc.Descriptions) == 0 && desc.Name == "" {
			continue
		}

		if desc.AppData != nil && desc.AppData.DefIndex > 0 {
			continue
		}

		if r.schema != nil {
			nameToParse := desc.MarketHashName
			if nameToParse == "" {
				nameToParse = desc.Name
			}

			if nameToParse != "" {
				if parsed := r.schema.ItemFromName(nameToParse); parsed != nil && parsed.Defindex > 0 {
					continue
				}
			}
		}

		cID, _ := bytesconv.ParseUint64(bytesconv.S2B(desc.ClassID))
		instID, _ := bytesconv.ParseUint64(bytesconv.S2B(desc.InstanceID))
		k := uintDescKey{ClassID: cID, InstanceID: instID}

		if _, cached := r.descCache.Load(k); !cached && !seen[k] {
			seen[k] = true
			missing = append(missing, k)
		}
	}

	return missing
}

func (r *Remote) fetchAssetClassInfoChunk(ctx context.Context, chunk []uintDescKey) error {
	type GetAssetClassInfoResponse struct {
		Result map[string]json.RawMessage `json:"result"`
	}

	params := url.Values{
		"appid":       {"440"},
		"language":    {"english"},
		"class_count": {strconv.Itoa(len(chunk))},
	}

	for idx, k := range chunk {
		params.Set(fmt.Sprintf("classid%d", idx), strconv.FormatUint(k.ClassID, 10))

		if k.InstanceID != 0 {
			params.Set(fmt.Sprintf("instanceid%d", idx), strconv.FormatUint(k.InstanceID, 10))
		}
	}

	apiResp, err := service.WebAPI[GetAssetClassInfoResponse](
		ctx, r.client, "GET", "ISteamEconomy", "GetAssetClassInfo", 1, params,
	)
	if err != nil {
		return err
	}

	if apiResp != nil && apiResp.Result != nil {
		for key, rawVal := range apiResp.Result {
			if key == "success" {
				continue
			}

			var desc rawAssetClassDescription
			if err := json.Unmarshal(rawVal, &desc); err == nil {
				cIDStr := desc.ClassID
				if cIDStr == "" {
					cIDStr = key
				}

				cID, _ := bytesconv.ParseUint64(bytesconv.S2B(cIDStr))
				instID, _ := bytesconv.ParseUint64(bytesconv.S2B(desc.InstanceID))

				r.descCache.Store(uintDescKey{ClassID: cID, InstanceID: instID}, desc)
			}
		}
	}

	return nil
}

func (r *Remote) applyCachedDescriptions(items []inventory.CEconItem) {
	for i := range items {
		if items[i].Description.ClassID == "" {
			continue
		}

		if items[i].Description.AppData != nil && items[i].Description.AppData.DefIndex > 0 {
			continue
		}

		cID, _ := bytesconv.ParseUint64(bytesconv.S2B(items[i].Description.ClassID))
		instID, _ := bytesconv.ParseUint64(bytesconv.S2B(items[i].Description.InstanceID))

		k := uintDescKey{ClassID: cID, InstanceID: instID}

		var resolved rawAssetClassDescription

		found := false

		if cachedVal, ok := r.descCache.Load(k); ok {
			resolved = cachedVal.(rawAssetClassDescription)
			found = true
		} else if cachedVal, ok := r.descCache.Load(uintDescKey{ClassID: cID, InstanceID: 0}); ok {
			resolved = cachedVal.(rawAssetClassDescription)
			found = true
		}

		if found && resolved.AppData != nil {
			if items[i].Description.AppData == nil {
				items[i].Description.AppData = &inventory.AppData{}
			}

			if di, ok := resolved.AppData["def_index"]; ok {
				switch val := di.(type) {
				case float64:
					items[i].Description.AppData.DefIndex = int(val)
				case int:
					items[i].Description.AppData.DefIndex = val
				case string:
					parsed, _ := strconv.Atoi(val)
					items[i].Description.AppData.DefIndex = parsed
				}
			}

			if q, ok := resolved.AppData["quality"]; ok {
				switch val := q.(type) {
				case float64:
					items[i].Description.AppData.Quality = int(val)
				case int:
					items[i].Description.AppData.Quality = val
				case string:
					parsed, _ := strconv.Atoi(val)
					items[i].Description.AppData.Quality = parsed
				}
			}

			if oid, ok := resolved.AppData["original_id"]; ok {
				switch val := oid.(type) {
				case float64:
					items[i].Description.AppData.OriginalID = uint64(val)
				case uint64:
					items[i].Description.AppData.OriginalID = val
				case int:
					items[i].Description.AppData.OriginalID = uint64(val)
				case string:
					parsed, _ := strconv.ParseUint(val, 10, 64)
					items[i].Description.AppData.OriginalID = parsed
				}
			}
		}
	}
}
