// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tests_test

import (
	stdjson "encoding/json"
	"os"
	"strconv"
	"testing"

	gojson "github.com/goccy/go-json"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

func BenchmarkSKU_FromString_Simple(b *testing.B) {
	s := "5021;6"

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_, _ = sku.FromString(s)
	}
}

func BenchmarkSKU_FromString_Complex(b *testing.B) {
	s := "15000;11;u4;australium;festive;kt-3;pk12;w1;p5801378;s-1004-1;sp380"

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_, _ = sku.FromString(s)
	}
}

func BenchmarkSKU_FromObject_Complex(b *testing.B) {
	item := &sku.Item{
		Defindex:   15000,
		Quality:    11,
		Effect:     4,
		Australium: true,
		Festivized: true,
		Killstreak: 3,
		Paintkit:   12,
		Wear:       1,
		Paint:      5801378,
		Spells:     []sku.Spell{{Attribute: 1004, Value: 1}},
		Parts:      []int{380},
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = sku.FromObject(item)
	}
}

func BenchmarkCurrency_Parse(b *testing.B) {
	input := "2 keys, 15.33 ref"

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_, _ = currency.Parse(input)
	}
}

func BenchmarkSOCache_ForEachItem_2000Items(b *testing.B) {
	cache := tf2.NewSOCache(nil)

	for i := uint64(1); i <= 2000; i++ {
		defindex := uint32(5000 + (i % 50))
		item := &tf2.Item{
			ID:         i,
			DefIndex:   defindex,
			IsTradable: true,
			SKU:        strconv.FormatUint(uint64(defindex), 10) + ";6",
		}
		_ = item
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		count := 0
		cache.ForEachItem(func(item *tf2.Item) bool {
			if item.DefIndex == 5021 {
				count++
			}

			return true
		})
	}
}

func BenchmarkSchema_Unmarshal_StandardJSON(b *testing.B) {
	data, err := os.ReadFile("testdata/schema.json")
	if err != nil {
		b.Skip("schema.json not found in testdata, skipping")
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		var raw schema.Raw
		if err := stdjson.Unmarshal(data, &raw); err != nil {
			b.Fatal(err)
		}

		_ = schema.New(&raw)
	}
}

// 2. Быстрый goccy/go-json
func BenchmarkSchema_Unmarshal_GoccyJSON(b *testing.B) {
	data, err := os.ReadFile("testdata/schema.json")
	if err != nil {
		b.Skip("schema.json not found in testdata, skipping")
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		var raw schema.Raw
		if err := gojson.Unmarshal(data, &raw); err != nil {
			b.Fatal(err)
		}

		_ = schema.New(&raw)
	}
}
