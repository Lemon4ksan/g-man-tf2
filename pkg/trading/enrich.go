// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"
	"fmt"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

// EnrichItem populates the SKU and Attributes fields of a generic trading.Item
// using the TF2 schema mapping.
func EnrichItem(item *trading.Item, s *schema.Schema) {
	if item == nil || s == nil {
		return
	}

	skuItem := s.ItemFromEconItem(item)
	if skuItem == nil {
		return
	}

	// 1. Generate and set standard SKU string
	item.SKU = s.SKUFromItem(skuItem)

	// 2. Populate attributes based on skuItem properties
	attrs := make([]trading.Attribute, 0)

	addAttr := func(defindex int, val float64) {
		attrs = append(attrs, trading.Attribute{
			Defindex:   defindex,
			Value:      fmt.Sprintf("%g", val),
			FloatValue: val,
		})
	}

	if skuItem.Effect != 0 {
		addAttr(schema.AttrUnusualEffect, float64(skuItem.Effect))
	}
	if skuItem.Wear != 0 {
		addAttr(schema.AttrWear, float64(skuItem.Wear)/5.0)
	}
	if skuItem.Australium {
		addAttr(schema.AttrAustralium, 1.0)
	}
	if skuItem.Paintkit != 0 {
		addAttr(schema.AttrPaintkit, float64(skuItem.Paintkit))
	}
	if skuItem.Killstreak != 0 {
		addAttr(schema.AttrKillstreak, float64(skuItem.Killstreak))
	}
	if skuItem.Festivized {
		addAttr(schema.AttrFestivized, 1.0)
	}
	if skuItem.Paint != 0 {
		addAttr(schema.AttrPaintColor, float64(skuItem.Paint))
	}
	if skuItem.Crateseries != 0 {
		addAttr(schema.AttrCrateSeries, float64(skuItem.Crateseries))
	}
	if skuItem.Quality2 == schema.QualityStrange {
		addAttr(schema.AttrStrangeScore, 1.0)
	}

	for _, spell := range skuItem.Spells {
		addAttr(spell.Attribute, float64(spell.Value))
	}

	for idx, part := range skuItem.Parts {
		addAttr(schema.DefPartsProxy+idx, float64(part))
	}

	item.Attributes = attrs
}

// ItemEnrichmentMiddleware enriches all items in the trade offer (both give and receive)
// with TF2 SKU and Attributes parsed using the schema.
func ItemEnrichmentMiddleware(schemaProvider func() *schema.Schema, logger log.Logger) engine.Middleware {
	return func(next engine.Handler) engine.Handler {
		return func(ctx *engine.TradeContext) error {
			s := schemaProvider()
			if s == nil {
				logger.Warn("Schema is not ready, skipping trade offer item enrichment")
				return next(ctx)
			}

			for _, it := range ctx.Offer.ItemsToGive {
				EnrichItem(it, s)
			}
			for _, it := range ctx.Offer.ItemsToReceive {
				EnrichItem(it, s)
			}

			return next(ctx)
		}
	}
}

// TF2PartnerInventoryProvider wraps a trading.PartnerInventoryProvider and enriches
// the fetched items with TF2 SKU and Attributes.
type TF2PartnerInventoryProvider struct {
	provider       trading.PartnerInventoryProvider
	schemaProvider func() *schema.Schema
}

// NewPartnerInventoryProvider creates a new TF2PartnerInventoryProvider wrapper.
func NewPartnerInventoryProvider(provider trading.PartnerInventoryProvider, schemaProvider func() *schema.Schema) trading.PartnerInventoryProvider {
	return &TF2PartnerInventoryProvider{
		provider:       provider,
		schemaProvider: schemaProvider,
	}
}

// GetPartnerInventory fetches partner inventory and enriches its items.
func (p *TF2PartnerInventoryProvider) GetPartnerInventory(ctx context.Context, partnerID id.ID) ([]*trading.Item, error) {
	items, err := p.provider.GetPartnerInventory(ctx, partnerID)
	if err != nil {
		return nil, err
	}
	s := p.schemaProvider()
	if s != nil {
		for _, it := range items {
			EnrichItem(it, s)
		}
	}
	return items, nil
}
