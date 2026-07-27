// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backpack

import (
	"context"
	"testing"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
)

func setupBenchmarkRemote(itemCount int) *Remote {
	sch := schema.New(&schema.Raw{})
	r := NewRemote(76561198000000000, nil, nil, sch)

	items := make([]TF2Item, 0, itemCount)
	for i := 0; i < itemCount; i++ {
		assetID := uint64(2000000000 + i)

		var (
			defIndex int
			quality  = schema.QualityUnique
		)

		switch {
		case i < 300: // 300 Refined
			defIndex = schema.DefRefined
		case i < 450: // 150 Reclaimed
			defIndex = schema.DefReclaimed
		case i < 600: // 150 Scrap
			defIndex = schema.DefScrap
		case i < 800: // 200 Keys
			defIndex = schema.DefKey
		case i < 1600: // 800 Weapons
			defIndex = 200 + (i % 50)
		default: // 400 Cosmetics / Unusuals
			defIndex = 30000 + (i % 100)
			quality = schema.QualityUnusual
		}

		it := TF2Item{
			ID:         assetID,
			OriginalID: assetID,
			Defindex:   defIndex,
			Quality:    quality,
			Quantity:   1,
		}

		if quality == schema.QualityUnusual {
			it.Attributes = []TF2Attribute{
				{Defindex: int(schema.AttrUnusualEffect), Value: float64(13)},
			}
		}

		it.SKU = it.ToSKU() // Pre-cache SKU string
		items = append(items, it)
	}

	r.items = items
	r.fetched = true

	return r
}

func BenchmarkRemote_FindMetalInPartnerInventory_2000Items(b *testing.B) {
	r := setupBenchmarkRemote(2000)
	ctx := context.Background()
	amount := currency.Scrap(25) // e.g. 2.77 ref change required

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		items, err := r.FindMetalInPartnerInventory(ctx, amount)
		if err != nil {
			b.Fatal(err)
		}

		_ = items
	}
}

func BenchmarkRemote_GetItemsBySKU_2000Items(b *testing.B) {
	r := setupBenchmarkRemote(2000)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		items, err := r.GetItemsBySKU(ctx, currency.SKURefined)
		if err != nil {
			b.Fatal(err)
		}

		_ = items
	}
}

func BenchmarkRemote_GetItems_2000Items(b *testing.B) {
	r := setupBenchmarkRemote(2000)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		items, err := r.GetItems(ctx)
		if err != nil {
			b.Fatal(err)
		}

		_ = items
	}
}
