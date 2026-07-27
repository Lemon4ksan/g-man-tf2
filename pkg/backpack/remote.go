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
	"github.com/lemon4ksan/g-man/pkg/steam/community"
	"github.com/lemon4ksan/g-man/pkg/steam/community/inventory"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/lemon4ksan/g-man/pkg/steam/webapi"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/miyako/generic"
	"github.com/lemon4ksan/miyako/log"

	"github.com/lemon4ksan/g-man-tf2/internal/bytesconv"
	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

// Remote manages auditing and validation tasks for external player inventories.
// Uses streaming page-by-page fetching and compact PackedItem memory representation.
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

// Option defines configuration setter functions for initializing [Remote] instances.
type Option = generic.Option[*Remote]

// WithLogger configures a custom [log.Logger] for logging [Remote] operations.
func WithLogger(l log.Logger) Option {
	return func(inv *Remote) {
		inv.logger = l
	}
}

// WithDupeCheckers registers an array of historical verification engines with the [Remote] auditor.
func WithDupeCheckers(dc []DupeChecker) Option {
	return func(inv *Remote) {
		inv.dupeCheckers = dc
	}
}

// WithAssetClassCache configures a shared cache for asset class descriptions.
func WithAssetClassCache(cache *sync.Map) Option {
	return func(inv *Remote) {
		inv.descCache = cache
	}
}

// NewRemote constructs a new configured [Remote] instance for auditing external profiles.
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

// GetItems retrieves all items in the external inventory.
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

// GetItemsBySKU retrieves all items in the external inventory matching the specified SKU.
// Performs zero-allocation search over the flat packedItems slice.
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

// CanTradeWithoutHold queries Steam API trade escrow times for the external account.
func (r *Remote) CanTradeWithoutHold(ctx context.Context, token string) (bool, error) {
	req := &webapi.IEconService_GetTradeHoldDurations_v1_Request{
		SteamIDTarget: r.steamID, TradeOfferAccessToken: token,
	}

	type respType struct {
		TheirHold int `json:"their_escrow"`
		MyHold    int `json:"my_escrow"`
	}

	resp, err := webapi.IEconService_GetTradeHoldDurations_v1[respType](ctx, r.client, req)
	if err != nil {
		return false, err
	}

	return resp.TheirHold == 0, nil
}

// IsDuped queries history checking engines to verify if the specified asset is a duplicate.
func (r *Remote) IsDuped(ctx context.Context, assetID uint64) (*bool, error) {
	duped, recorded, err := r.checkWithServices(ctx, assetID)
	if err != nil {
		return nil, err
	}

	if recorded {
		return &duped, nil
	}

	r.mu.Lock()
	if !r.fetched {
		if err := r.fetch(ctx); err != nil {
			r.mu.Unlock()
			return nil, err
		}
	}

	var targetItem *TF2Item
	for i := range r.items {
		if r.items[i].ID == assetID {
			targetItem = &r.items[i]
			break
		}
	}

	r.mu.Unlock()

	if targetItem == nil {
		return nil, ErrItemNotFound
	}

	duped, recorded, err = r.checkWithServices(ctx, targetItem.OriginalID)
	if err != nil {
		return nil, err
	}

	if recorded {
		return &duped, nil
	}

	return nil, nil
}

// FindMetalInPartnerInventory searches the partner's inventory in a single pass over packed items.
func (r *Remote) FindMetalInPartnerInventory(ctx context.Context, amount currency.Scrap) ([]*trading.Item, error) {
	r.mu.Lock()
	if !r.fetched {
		if err := r.fetch(ctx); err != nil {
			r.mu.Unlock()
			return nil, err
		}
	}

	var (
		refinedItems   []TF2Item
		reclaimedItems []TF2Item
		scrapItems     []TF2Item
	)

	for i := range r.items {
		skuStr := r.items[i].ToSKU()
		switch skuStr {
		case currency.SKURefined:
			refinedItems = append(refinedItems, r.items[i])
		case currency.SKUReclaimed:
			reclaimedItems = append(reclaimedItems, r.items[i])
		case currency.SKUScrap:
			scrapItems = append(scrapItems, r.items[i])
		}
	}

	r.mu.Unlock()

	bestVal := -1
	bestRef, bestRec, bestScr := 0, 0, 0

	lenRef := len(refinedItems)
	lenRec := len(reclaimedItems)
	lenScr := len(scrapItems)

	limitRef := min((int(amount)+8)/9, lenRef)

	for ref := 0; ref <= limitRef; ref++ {
		rem1 := int(amount) - 9*ref
		if rem1 <= 0 {
			val := 9 * ref
			if bestVal == -1 || val < bestVal {
				bestVal = val
				bestRef, bestRec, bestScr = ref, 0, 0
			}

			continue
		}

		limitRec := min((rem1+2)/3, lenRec)

		for rec := 0; rec <= limitRec; rec++ {
			rem2 := rem1 - 3*rec
			if rem2 <= 0 {
				val := 9*ref + 3*rec
				if bestVal == -1 || val < bestVal {
					bestVal = val
					bestRef, bestRec, bestScr = ref, rec, 0
				}

				continue
			}

			s := rem2
			if s > lenScr {
				s = lenScr
			}

			val := 9*ref + 3*rec + s
			if val >= int(amount) {
				if bestVal == -1 || val < bestVal {
					bestVal = val
					bestRef, bestRec, bestScr = ref, rec, s
				}
			}
		}
	}

	if bestVal == -1 {
		return nil, fmt.Errorf("partner is missing %d scrap for counter-offer", amount)
	}

	var selected []*trading.Item
	for i := 0; i < bestRef; i++ {
		selected = append(selected, refinedItems[i].ToEconItem())
	}

	for i := 0; i < bestRec; i++ {
		selected = append(selected, reclaimedItems[i].ToEconItem())
	}

	for i := 0; i < bestScr; i++ {
		selected = append(selected, scrapItems[i].ToEconItem())
	}

	return selected, nil
}

func (r *Remote) checkWithServices(
	ctx context.Context,
	assetID uint64,
) (isDuped, isRecorded bool, err error) {
	for _, checker := range r.dupeCheckers {
		status, checkErr := checker.CheckHistory(ctx, assetID)
		if checkErr != nil {
			r.logger.Warn("Dupe checker failed",
				log.String("service", reflect.TypeOf(checker).Name()),
				log.Err(checkErr),
			)

			continue
		}

		if !status.Recorded {
			continue
		}

		isRecorded = true

		if status.IsDuped {
			isDuped = true
			break
		}
	}

	return isDuped, isRecorded, err
}

func (r *Remote) fetch(ctx context.Context) error {
	if r.community == nil || r.community.SessionID(community.BaseURL) == "" {
		return errors.New("cannot fetch remote inventory: no community web session available")
	}

	if r.schema == nil {
		return errors.New("cannot fetch remote inventory: no schema available")
	}

	return r.fetchCommunityStreaming(ctx)
}

// fetchCommunityStreaming processes the remote inventory page-by-page.
// Temporary JSON pages and descriptions are garbage-collected immediately
// after conversion into 32-byte PackedItem value structs.
func (r *Remote) fetchCommunityStreaming(ctx context.Context) error {
	var (
		unifiedItems = make([]TF2Item, 0, 500)
		packedList   = make([]tf2.PackedItem, 0, 500)
	)

	totalCount, err := inventory.StreamUserInventoryContents(
		ctx,
		r.community,
		r.steamID,
		440,
		2,
		false,
		"english",
		func(econItem *inventory.CEconItem, isCurrency bool) bool {
			tfItem := MapCEconToTF2(*econItem, r.schema)
			unifiedItems = append(unifiedItems, tfItem)

			packed := PackTF2Item(&tfItem)
			packedList = append(packedList, packed)

			return true
		},
	)
	if err != nil {
		return fmt.Errorf("community streaming inventory fetch failed: %w", err)
	}

	r.items = unifiedItems
	r.packedItems = packedList
	r.slots = totalCount
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

	var missingKeys []uintDescKey

	seenKeys := make(map[uintDescKey]bool)

	for _, it := range items {
		desc := it.Description
		if desc.AppData == nil && len(desc.Tags) == 0 && len(desc.Descriptions) == 0 && desc.Name == "" {
			continue
		}

		hasDefIndex := desc.AppData != nil && desc.AppData.DefIndex > 0

		if !hasDefIndex && r.schema != nil {
			nameToParse := desc.MarketHashName
			if nameToParse == "" {
				nameToParse = desc.Name
			}

			if nameToParse != "" {
				if parsed := r.schema.ItemFromName(nameToParse); parsed != nil && parsed.Defindex > 0 {
					hasDefIndex = true
				}
			}
		}

		if !hasDefIndex {
			cID, _ := bytesconv.ParseUint64(bytesconv.S2B(desc.ClassID))
			instID, _ := bytesconv.ParseUint64(bytesconv.S2B(desc.InstanceID))
			k := uintDescKey{ClassID: cID, InstanceID: instID}

			if _, cached := r.descCache.Load(k); !cached {
				if !seenKeys[k] {
					seenKeys[k] = true
					missingKeys = append(missingKeys, k)
				}
			}
		}
	}

	if len(missingKeys) == 0 {
		r.applyCachedDescriptions(items)
		return nil
	}

	type GetAssetClassInfoResponse struct {
		Result map[string]json.RawMessage `json:"result"`
	}

	chunkSize := 50
	for i := 0; i < len(missingKeys); i += chunkSize {
		end := min(i+chunkSize, len(missingKeys))
		chunk := missingKeys[i:end]

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
					dk := uintDescKey{ClassID: cID, InstanceID: instID}

					r.descCache.Store(dk, desc)
				}
			}
		}
	}

	r.applyCachedDescriptions(items)

	return nil
}

func (r *Remote) applyCachedDescriptions(items []inventory.CEconItem) {
	for i := range items {
		if items[i].Description.ClassID == "" {
			continue
		}

		hasDefIndex := items[i].Description.AppData != nil && items[i].Description.AppData.DefIndex > 0

		if !hasDefIndex {
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
					if val, ok := di.(float64); ok {
						items[i].Description.AppData.DefIndex = int(val)
					}
				}
			}
		}
	}
}
