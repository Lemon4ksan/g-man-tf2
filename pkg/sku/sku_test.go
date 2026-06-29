// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sku_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man-tf2/pkg/sku"
)

func TestIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"363;6", true},
		{"655;5;u14;australium;kt-3;strange", true},
		{"363", true},
		{"invalid", false},
		{"363;abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := sku.IsValid(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFromString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    *sku.Item
		wantErr bool
	}{
		{
			name:  "basic_unique_item",
			input: "363;6",
			want: &sku.Item{
				Defindex: 363, Quality: 6, Craftable: true, Tradable: true,
			},
		},
		{
			name:  "unusual_professional_killstreak_australium",
			input: "655;5;u14;australium;kt-3;strange",
			want: &sku.Item{
				Defindex:   655,
				Quality:    5,
				Effect:     14,
				Australium: true,
				Killstreak: 3,
				Quality2:   11,
				Craftable:  true,
				Tradable:   true,
			},
		},
		{
			name:  "uncraftable_and_untradeable",
			input: "300;6;uncraftable;untradeable",
			want: &sku.Item{
				Defindex: 300, Quality: 6, Craftable: false, Tradable: false,
			},
		},
		{
			name:  "untradable_variation",
			input: "300;6;untradable",
			want: &sku.Item{
				Defindex: 300, Quality: 6, Craftable: true, Tradable: false,
			},
		},
		{
			name:  "skins_and_wear",
			input: "15000;15;pk1;w3",
			want: &sku.Item{
				Defindex: 15000, Quality: 15, Paintkit: 1, Wear: 3, Craftable: true, Tradable: true,
			},
		},
		{
			name:  "craft_number_and_series",
			input: "444;6;n100;c50",
			want: &sku.Item{
				Defindex: 444, Quality: 6, Craftnumber: 100, Crateseries: 50, Craftable: true, Tradable: true,
			},
		},
		{
			name:  "spells",
			input: "363;6;s-1009-1;s-1004-3",
			want: &sku.Item{
				Defindex:  363,
				Quality:   6,
				Spells:    []sku.Spell{{Attribute: 1009, Value: 1}, {Attribute: 1004, Value: 3}},
				Craftable: true,
				Tradable:  true,
			},
		},
		{
			name:  "single_value_spell",
			input: "363;6;s1006",
			want: &sku.Item{
				Defindex:  363,
				Quality:   6,
				Spells:    []sku.Spell{{Attribute: 1006, Value: 1}},
				Craftable: true,
				Tradable:  true,
			},
		},
		{
			name:  "strange_parts",
			input: "363;11;sp17;sp20",
			want: &sku.Item{
				Defindex: 363, Quality: 11, Parts: []int{17, 20}, Craftable: true, Tradable: true,
			},
		},
		{
			name:  "targets_outputs_and_paints",
			input: "20000;6;td-363;od-6522;oq-14;p5;sd123",
			want: &sku.Item{
				Defindex:      20000,
				Quality:       6,
				Target:        363,
				Output:        6522,
				OutputQuality: 14,
				Paint:         5,
				Seed:          123,
				Craftable:     true,
				Tradable:      true,
			},
		},
		{
			name:  "invalid_numeric_attrs_ignored_cleanly",
			input: "363;6;kt-abc;pk-abc;td-abc;n-abc;c-abc;od-abc;oq-abc;p-abc;sp-abc;s-abc;sd-abc",
			want: &sku.Item{
				Defindex:  363,
				Quality:   6,
				Craftable: true,
				Tradable:  true,
			},
		},
		{
			name:    "invalid_sku_too_short",
			input:   "363",
			wantErr: true,
		},
		{
			name:    "invalid_defindex",
			input:   "abc;6",
			wantErr: true,
		},
		{
			name:    "invalid_quality",
			input:   "363;abc",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := sku.FromString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FromString() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFromObject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		item *sku.Item
		want string
	}{
		{
			name: "basic",
			item: &sku.Item{Defindex: 363, Quality: 6, Craftable: true, Tradable: true},
			want: "363;6",
		},
		{
			name: "killstreak_with_dash",
			item: &sku.Item{Defindex: 200, Quality: 6, Killstreak: 2, Craftable: true, Tradable: true},
			want: "200;6;kt-2",
		},
		{
			name: "full_attributes_with_seed",
			item: &sku.Item{
				Defindex: 100, Quality: 5, Effect: 15, Australium: true, Craftable: false, Tradable: false,
				Wear: 3, Paintkit: 102, Quality2: 11, Killstreak: 3, Target: 400, Festivized: true,
				Craftnumber: 1, Crateseries: 2, Output: 3, OutputQuality: 4, Paint: 5, Seed: 123,
			},
			want: "100;5;u15;australium;uncraftable;untradable;w3;pk102;strange;kt-3;td-400;festive;n1;c2;od-3;oq-4;p5;sd123",
		},
		{
			name: "spells_and_strange_parts",
			item: &sku.Item{
				Defindex: 363, Quality: 6, Craftable: true, Tradable: true,
				Spells: []sku.Spell{{Attribute: 1009, Value: 1}, {Attribute: 0, Value: 5}}, // Пропускается атрибут 0
				Parts:  []int{17, 20},
			},
			want: "363;6;s-1009-1;sp17;sp20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := sku.FromObject(tt.item)
			if got != tt.want {
				t.Errorf("FromObject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	skus := []string{
		"363;6",
		"655;5;u14;australium;strange;kt-3",
		"15000;15;w3;pk1;strange;kt-3;festive",
		"300;6;uncraftable;untradable",
		"363;6;s-1009-1;s-1004-3",
		"363;11;sp17;sp20",
		"363;6;s-1006-1",
	}

	for _, s := range skus {
		t.Run(s, func(t *testing.T) {
			t.Parallel()

			item, err := sku.FromString(s)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			out := sku.FromObject(item)
			if s != out {
				t.Errorf("Round-trip failed!\nInput:  %s\nOutput: %s", s, out)
			}
		})
	}
}

func TestToPricingSKU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{
			input: "363;6",
			want:  "363;6",
		},
		{
			input: "363;6;festive",
			want:  "363;6",
		},
		{
			input: "363;6;s-1009-1;s-1004-3",
			want:  "363;6",
		},
		{
			input: "363;11;sp17;sp20",
			want:  "363;11",
		},
		{
			input: "200;6;p5;festive;s-1009-1;sp17",
			want:  "200;6",
		},
		{
			input: "15000;15;w3;pk1;strange;kt-3;festive",
			want:  "15000;15;w3;pk1;strange;kt-3",
		},
		{
			input: "invalid_sku",
			want:  "invalid_sku",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := sku.ToPricingSKU(tt.input)
			if got != tt.want {
				t.Errorf("ToPricingSKU(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
