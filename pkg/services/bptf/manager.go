// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"context"
	"fmt"
	"sync"

	"github.com/lemon4ksan/miyako/log"
	"github.com/lemon4ksan/miyako/yumi"

	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

// ListingManager manages high-level backpack.tf listings.
type ListingManager struct {
	client API
	schema *schema.Manager
	logger log.Logger

	mu       sync.RWMutex
	listings map[string]*Listing
}

// NewListingManager creates a new high-level listing manager.
func NewListingManager(client API, sm *schema.Manager, logger log.Logger) *ListingManager {
	return &ListingManager{
		client:   client,
		schema:   sm,
		logger:   logger.With(log.Module("bptf_listings")),
		listings: make(map[string]*Listing),
	}
}

// Client returns the underlying backpack.tf client.
func (m *ListingManager) Client() API {
	return m.client
}

// Sync fetches all current listings from backpack.tf and updates internal state.
func (m *ListingManager) Sync(ctx context.Context) error {
	m.logger.Info("syncing listings from backpack.tf")

	var allListings []Listing

	skip := 0
	limit := 500

	for {
		resp, err := m.client.GetV2ClassifiedsListings(ctx, skip, limit, 0, 0)
		if err != nil {
			return fmt.Errorf("failed to fetch listings at skip %d: %w", skip, err)
		}

		allListings = append(allListings, resp.Results...)

		if len(allListings) >= resp.Cursor.Total || len(resp.Results) == 0 {
			break
		}

		skip += len(resp.Results)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.listings = make(map[string]*Listing)
	for i := range allListings {
		m.listings[allListings[i].ID] = &allListings[i]
	}

	m.logger.Info("synced listings", log.Int("count", len(m.listings)))

	return nil
}

// Upsert creates or updates a listing.
func (m *ListingManager) Upsert(ctx context.Context, listing ListingResolvable) (*Listing, error) {
	resp, err := m.client.PostV2ClassifiedsListings(ctx, listing)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.listings[resp.ID] = resp
	m.mu.Unlock()

	return resp, nil
}

// Delete removes a listing.
func (m *ListingManager) Delete(ctx context.Context, id string) error {
	if _, err := m.client.DeleteV2ClassifiedsListingsByListingID(ctx, id); err != nil {
		return err
	}

	m.mu.Lock()
	delete(m.listings, id)
	m.mu.Unlock()

	return nil
}

// DeleteAll removes all listings managed by this manager in parallel batches.
func (m *ListingManager) DeleteAll(ctx context.Context) error {
	m.mu.RLock()

	ids := make([]string, 0, len(m.listings))
	for id := range m.listings {
		ids = append(ids, id)
	}

	m.mu.RUnlock()

	if len(ids) == 0 {
		return nil
	}

	const batchSize = 100

	var batches [][]string
	for i := 0; i < len(ids); i += batchSize {
		end := min(i+batchSize, len(ids))
		batches = append(batches, ids[i:end])
	}

	err := yumi.ForEach(ctx, yumi.PipelineConfig{
		Workers: 3,
		RPS:     5,
		Burst:   2,
	}, batches, func(chunkCtx context.Context, batch []string) error {
		_, err := m.client.DeleteV2ClassifiedsListingsBatch(chunkCtx)
		return err
	})
	if err != nil {
		return fmt.Errorf("batch delete failed: %w", err)
	}

	m.mu.Lock()
	m.listings = make(map[string]*Listing)
	m.mu.Unlock()

	return nil
}

// FindListingBySKU looks for a listing matching the given SKU and intent.
func (m *ListingManager) FindListingBySKU(sku string, intent ListingIntent) *Listing {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, l := range m.listings {
		if l.Intent == intent && m.matchesSKU(l, sku) {
			return l
		}
	}

	return nil
}

// ItemToSKU converts a backpack.tf ItemDocument to a standard TF2 SKU.
func (m *ListingManager) ItemToSKU(doc *ItemDocument) string {
	if doc == nil || m.schema == nil {
		return ""
	}

	s := m.schema.Get()
	if s == nil {
		return ""
	}

	item := s.ItemFromName(doc.Name)
	if item == nil {
		itemSchema := s.ItemByName(doc.BaseName)
		if itemSchema == nil {
			return ""
		}

		item = &sku.Item{
			Defindex: itemSchema.Defindex,
			Quality:  doc.Quality.ID,
		}

		if doc.Particle.ID != 0 {
			item.Effect = doc.Particle.ID
		}

		if doc.Paint.ID != 0 {
			item.Paint = doc.Paint.ID
		}

		if doc.ElevatedQuality.ID == 11 {
			item.Quality2 = 11
		}
	}

	return sku.FromObject(item)
}

func (m *ListingManager) matchesSKU(l *Listing, sku string) bool {
	if l.Details != "" && (l.Details == sku || l.Details == "SKU: "+sku) {
		return true
	}

	if m.schema == nil {
		return false
	}

	return m.ItemToSKU(l.Item) == sku
}

// AddMockListing inserts a listing directly into internal cache for testing.
func (m *ListingManager) AddMockListing(l *Listing) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.listings[l.ID] = l
}
