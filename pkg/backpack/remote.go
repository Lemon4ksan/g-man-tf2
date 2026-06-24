// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"sync"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/community"
	"github.com/lemon4ksan/g-man/pkg/steam/community/inventory"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/lemon4ksan/g-man/pkg/steam/webapi"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/miyako/generic"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

// Remote manages auditing and validation tasks for external player inventories.
// It loads inventory data from the Steam community web interface and enriches
// missing item metadata via ISteamEconomy/GetAssetClassInfo.
// Use [NewRemote] to instantiate and configure remote profile sessions.
type Remote struct {
	steamID   uint64
	client    service.Doer
	community community.Requester
	schema    *schema.Schema
	logger    log.Logger

	dupeCheckers []DupeChecker

	mu      sync.Mutex
	items   []TF2Item
	slots   int
	fetched bool
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

// NewRemote constructs a new configured [Remote] instance for auditing external profiles.
// The community requester is required for inventory fetching.
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
		schema:       schema,
	}

	generic.ApplyOptions(p, opts...)

	return p
}

// GetItems retrieves all items in the external inventory.
// Returns an error if the inventory fails to load.
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
// Returns an error if the inventory fails to load due to privacy settings or API rate limits.
func (r *Remote) GetItemsBySKU(ctx context.Context, targetSKU string) ([]TF2Item, error) {
	r.mu.Lock()
	if !r.fetched {
		if err := r.fetch(ctx); err != nil {
			r.mu.Unlock()
			return nil, err
		}
	}

	items := r.items
	r.mu.Unlock()

	var result []TF2Item

	for _, it := range items {
		if it.ToSKU() == targetSKU {
			result = append(result, it)
		}
	}

	return result, nil
}

// CanTradeWithoutHold queries Steam API trade escrow times for the external account.
// Returns true if the trade can be completed instantly, or an error if the request fails.
func (r *Remote) CanTradeWithoutHold(ctx context.Context, token string) (bool, error) {
	req := &webapi.IEconService_GetTradeHoldDurations_v1_Request{
		SteamIDTarget: r.steamID, TradeOfferAccessToken: token,
	}

	type resp struct {
		TheirHold int `json:"their_escrow"`
		MyHold    int `json:"my_escrow"`
	}

	res, err := webapi.IEconService_GetTradeHoldDurations_v1[resp](ctx, r.client, req)
	if err != nil {
		return false, err
	}

	return res.TheirHold == 0, nil
}

// IsDuped queries history checking engines to verify if the specified asset is a duplicate.
// It verifies the current asset ID first, then falls back to verifying the item's original ID.
// Returns a boolean flag pointer, or an error if the asset is not found.
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

	items := r.items
	r.mu.Unlock()

	var targetItem *TF2Item

	for _, item := range items {
		if item.ID == assetID {
			targetItem = &item
			break
		}
	}

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

// FindMetalInPartnerInventory searches the partner's inventory to match the required scrap value.
// It returns a slice of matching [trading.Item] currencies, or an error if the partner lacks change.
func (r *Remote) FindMetalInPartnerInventory(ctx context.Context, amount currency.Scrap) ([]*trading.Item, error) {
	refinedItems, err := r.GetItemsBySKU(ctx, currency.SKURefined)
	if err != nil {
		refinedItems = nil
	}

	reclaimedItems, err := r.GetItemsBySKU(ctx, currency.SKUReclaimed)
	if err != nil {
		reclaimedItems = nil
	}

	scrapItems, err := r.GetItemsBySKU(ctx, currency.SKUScrap)
	if err != nil {
		scrapItems = nil
	}

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

		limitRec := (rem1 + 2) / 3
		if limitRec > lenRec {
			limitRec = lenRec
		}

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

	return r.fetchCommunity(ctx)
}

func (r *Remote) fetchCommunity(ctx context.Context) error {
	items, currencies, total, err := inventory.GetUserInventoryContents(
		ctx, r.community, r.steamID, 440, 2, false, "english",
	)
	if err != nil {
		return fmt.Errorf("community inventory fetch failed: %w", err)
	}

	allCEconItems := make([]inventory.CEconItem, 0, len(items)+len(currencies))
	allCEconItems = append(allCEconItems, items...)
	allCEconItems = append(allCEconItems, currencies...)

	if err := r.enrichCommunityItems(ctx, allCEconItems); err != nil {
		r.logger.Warn("Failed to enrich community items descriptions", log.Err(err))
	}

	unifiedItems := make([]TF2Item, 0, len(allCEconItems))
	for _, it := range allCEconItems {
		unifiedItems = append(unifiedItems, mapCEconToTF2(it, r.schema))
	}

	r.items = unifiedItems
	r.slots = total
	r.fetched = true

	return nil
}

// enrichCommunityItems performs a batch query to ISteamEconomy/GetAssetClassInfo
// to fill in missing AppData for items obtained from the community web interface.
func (r *Remote) enrichCommunityItems(ctx context.Context, items []inventory.CEconItem) error {
	if r.client == nil {
		r.logger.Warn("No WebAPI client available, skipping community items enrichment")
		return nil
	}

	type descKey struct {
		ClassID    string
		InstanceID string
	}

	var missingKeys []descKey

	seenKeys := make(map[descKey]bool)

	for _, it := range items {
		desc := it.Description
		if len(desc.AppData) == 0 && len(desc.Tags) == 0 && len(desc.Descriptions) == 0 && desc.Name == "" {
			continue
		}

		hasDefIndex := false
		if desc.AppData != nil {
			_, hasDefIndex = desc.AppData["def_index"]
		}

		if !hasDefIndex {
			k := descKey{ClassID: desc.ClassID, InstanceID: desc.InstanceID}
			if !seenKeys[k] {
				seenKeys[k] = true
				missingKeys = append(missingKeys, k)
			}
		}
	}

	if len(missingKeys) == 0 {
		return nil
	}

	type GetAssetClassInfoResponse struct {
		Result map[string]json.RawMessage `json:"result"`
	}

	type rawAssetClassDescription struct {
		ClassID    string         `json:"classid"`
		InstanceID string         `json:"instanceid"`
		AppData    map[string]any `json:"app_data,omitempty"`
	}

	resolvedDescs := make(map[descKey]rawAssetClassDescription)

	chunkSize := 50
	for i := 0; i < len(missingKeys); i += chunkSize {
		end := min(i+chunkSize, len(missingKeys))
		chunk := missingKeys[i:end]

		params := make(url.Values)
		params.Set("appid", "440")
		params.Set("language", "english")
		params.Set("class_count", strconv.Itoa(len(chunk)))

		for idx, k := range chunk {
			params.Set(fmt.Sprintf("classid%d", idx), k.ClassID)

			if k.InstanceID != "0" && k.InstanceID != "" {
				params.Set(fmt.Sprintf("instanceid%d", idx), k.InstanceID)
			}
		}

		apiResp, err := service.WebAPI[GetAssetClassInfoResponse](
			ctx,
			r.client,
			"GET",
			"ISteamEconomy",
			"GetAssetClassInfo",
			1,
			params,
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
					cID := desc.ClassID
					if cID == "" {
						cID = key
					}

					resolvedDescs[descKey{ClassID: cID, InstanceID: desc.InstanceID}] = desc
				}
			}
		}
	}

	for i := range items {
		if items[i].Description.ClassID == "" {
			continue
		}

		hasDefIndex := false
		if items[i].Description.AppData != nil {
			_, hasDefIndex = items[i].Description.AppData["def_index"]
		}

		if !hasDefIndex {
			k := descKey{ClassID: items[i].Description.ClassID, InstanceID: items[i].Description.InstanceID}

			var resolved rawAssetClassDescription

			found := false
			if resolved, found = resolvedDescs[k]; !found {
				resolved, found = resolvedDescs[descKey{ClassID: items[i].Description.ClassID, InstanceID: "0"}]
			}

			if found && resolved.AppData != nil {
				items[i].Description.AppData = resolved.AppData
			}
		}
	}

	return nil
}
