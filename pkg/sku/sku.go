// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sku

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lemon4ksan/foundation/silicon/pool"
)

var ErrEmptySKU = errors.New("invalid SKU: empty")

// Item represents a parsed TF2 item specification containing all standard SKU attributes.
type Item struct {
	Defindex      int
	Quality       int
	Craftable     bool
	Tradable      bool
	Killstreak    int
	Australium    bool
	Effect        int
	Festivized    bool
	Paintkit      int
	Wear          int
	Quality2      int
	Craftnumber   int
	Crateseries   int
	Target        int
	Output        int
	OutputQuality int
	Paint         int
	Spells        []Spell
	Parts         []int
	PartValues    map[int]int
	Seed          int
}

// GetItem retrieves a zeroed Item instance from the memory pool.
func GetItem() *Item {
	item := itemPool.Get()
	item.Reset()

	return item
}

// IsValid reports whether the given string represents a valid, parseable TF2 SKU.
func IsValid(skuStr string) bool {
	if len(skuStr) == 0 {
		return false
	}

	item := GetItem()
	err := ParseInto(skuStr, item)
	ReleaseItem(item)

	if err == nil {
		return true
	}

	_, ok := parseFastInt(skuStr)

	return ok
}

// ToPricingSKU normalizes an SKU for base market pricing by stripping cosmetic modifiers
// (festivized, spells, strange parts, paint, war paint pattern seed) that do not affect the base price.
//
// Parity: matches tf2-sku and @tf2autobot/tf2-schema item pricing normalization.
func ToPricingSKU(skuStr string) string {
	if !hasPricingModifiers(skuStr) {
		return skuStr
	}

	item, err := FromString(skuStr)
	if err != nil {
		return skuStr
	}
	defer ReleaseItem(item)

	item.Festivized = false
	item.Spells = item.Spells[:0]
	item.Parts = item.Parts[:0]
	item.PartValues = nil
	item.Paint = 0
	item.Seed = 0

	return FromObject(item)
}

func hasPricingModifiers(s string) bool {
	for {
		idx := strings.IndexByte(s, ';')
		if idx == -1 || idx+1 >= len(s) {
			return false
		}

		s = s[idx+1:]

		switch s[0] {
		case 'f':
			if strings.HasPrefix(s, "festive") {
				return true
			}

		case 's':
			if len(s) >= 2 && (s[1] == '-' || s[1] == 'p' || s[1] == 'd') {
				return true
			}

		case 'p':
			if len(s) >= 1 && (len(s) == 1 || s[1] != 'k') {
				return true
			}
		}
	}
}

func (it *Item) Reset() {
	it.Defindex = 0
	it.Quality = 0
	it.Craftable = true
	it.Tradable = true
	it.Killstreak = 0
	it.Australium = false
	it.Effect = 0
	it.Festivized = false
	it.Paintkit = 0
	it.Wear = 0
	it.Quality2 = 0
	it.Craftnumber = 0
	it.Crateseries = 0
	it.Target = 0
	it.Output = 0
	it.OutputQuality = 0
	it.Paint = 0
	it.Seed = 0
	it.Spells = nil
	it.Parts = nil
	it.PartValues = nil
}

// Spell represents an applied Halloween spell attribute and value pair.
type Spell struct {
	Attribute int
	Value     int
}

var skuBufferPool = pool.NewPerPStorage(func() *bytes.Buffer {
	b := new(bytes.Buffer)
	b.Grow(64)

	return b
})

var itemPool = pool.NewPerPStorage(func() *Item {
	return &Item{
		Craftable: true,
		Tradable:  true,
	}
})

// ParseInto parses a semicolon-delimited SKU string directly into an existing Item instance.
func ParseInto(skuStr string, item *Item) error {
	if len(skuStr) == 0 {
		return ErrEmptySKU
	}

	item.Reset()

	start := 0
	partIdx := 0

	for start < len(skuStr) {
		end := strings.IndexByte(skuStr[start:], ';')

		var part string
		if end == -1 {
			part = skuStr[start:]
			start = len(skuStr)
		} else {
			part = skuStr[start : start+end]
			start += end + 1
		}

		if len(part) == 0 {
			continue
		}

		if partIdx == 0 {
			defindex, err := strconv.Atoi(part)
			if err != nil {
				return fmt.Errorf("invalid defindex: %s", part)
			}

			item.Defindex = defindex
			partIdx++

			continue
		}

		if partIdx == 1 {
			quality, err := strconv.Atoi(part)
			if err != nil {
				return fmt.Errorf("invalid quality: %s", part)
			}

			item.Quality = quality
			partIdx++

			continue
		}

		parseSKUAttribute(item, part)

		partIdx++
	}

	if partIdx < 2 {
		return fmt.Errorf("invalid SKU: %s", skuStr)
	}

	return nil
}

// FromString parses a canonical SKU string into a pooled Item instance.
func FromString(skuStr string) (*Item, error) {
	item := itemPool.Get()
	if err := ParseInto(skuStr, item); err != nil {
		itemPool.Put(item)
		return nil, err
	}

	return item, nil
}

// ReleaseItem returns a previously acquired Item instance back to the memory pool.
func ReleaseItem(item *Item) {
	if item != nil {
		itemPool.Put(item)
	}
}

// FromObject formats an Item instance into a canonical semicolon-delimited SKU string.
func FromObject(item *Item) string {
	buf := skuBufferPool.Get()

	buf.Reset()
	defer skuBufferPool.Put(buf)

	var numBuf [20]byte

	buf.Write(strconv.AppendInt(numBuf[:0], int64(item.Defindex), 10))
	buf.WriteByte(';')
	buf.Write(strconv.AppendInt(numBuf[:0], int64(item.Quality), 10))

	if item.Effect != 0 {
		buf.WriteString(";u")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(item.Effect), 10))
	}

	if item.Australium {
		buf.WriteString(";australium")
	}

	if !item.Craftable {
		buf.WriteString(";uncraftable")
	}

	if !item.Tradable {
		buf.WriteString(";untradable")
	}

	if item.Wear != 0 {
		buf.WriteString(";w")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(item.Wear), 10))
	}

	if item.Paintkit != 0 {
		buf.WriteString(";pk")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(item.Paintkit), 10))
	}

	if item.Quality2 == 11 {
		buf.WriteString(";strange")
	}

	if item.Killstreak != 0 {
		buf.WriteString(";kt-")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(item.Killstreak), 10))
	}

	if item.Target != 0 {
		buf.WriteString(";td-")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(item.Target), 10))
	}

	if item.Festivized {
		buf.WriteString(";festive")
	}

	if item.Craftnumber != 0 {
		buf.WriteString(";n")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(item.Craftnumber), 10))
	}

	if item.Crateseries != 0 {
		buf.WriteString(";c")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(item.Crateseries), 10))
	}

	if item.Output != 0 {
		buf.WriteString(";od-")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(item.Output), 10))
	}

	if item.OutputQuality != 0 {
		buf.WriteString(";oq-")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(item.OutputQuality), 10))
	}

	if item.Paint != 0 {
		buf.WriteString(";p")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(item.Paint), 10))
	}

	for _, spell := range item.Spells {
		if spell.Attribute == 0 {
			continue
		}

		buf.WriteString(";s-")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(spell.Attribute), 10))
		buf.WriteByte('-')
		buf.Write(strconv.AppendInt(numBuf[:0], int64(spell.Value), 10))
	}

	for _, partID := range item.Parts {
		buf.WriteString(";sp")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(partID), 10))
	}

	if item.Seed != 0 {
		buf.WriteString(";sd")
		buf.Write(strconv.AppendInt(numBuf[:0], int64(item.Seed), 10))
	}

	return buf.String()
}

func parseSKUAttribute(item *Item, part string) {
	if len(part) == 0 {
		return
	}

	switch part[0] {
	case 'a':
		if part == "australium" {
			item.Australium = true
		}

	case 'c':
		if len(part) > 1 && part[1] >= '0' && part[1] <= '9' {
			if val, err := strconv.Atoi(part[1:]); err == nil {
				item.Crateseries = val
			}
		}

	case 'f':
		if part == "festive" {
			item.Festivized = true
		}

	case 'k':
		if strings.HasPrefix(part, "kt-") && len(part) > 3 {
			if val, err := strconv.Atoi(part[3:]); err == nil {
				item.Killstreak = val
			}
		}

	case 'n':
		if len(part) > 1 && part[1] >= '0' && part[1] <= '9' {
			if val, err := strconv.Atoi(part[1:]); err == nil {
				item.Craftnumber = val
			}
		}

	case 'o':
		if strings.HasPrefix(part, "od-") && len(part) > 3 {
			if val, err := strconv.Atoi(part[3:]); err == nil {
				item.Output = val
			}
		} else if strings.HasPrefix(part, "oq-") && len(part) > 3 {
			if val, err := strconv.Atoi(part[3:]); err == nil {
				item.OutputQuality = val
			}
		}

	case 'p':
		parsePaintOrPaintkit(item, part)

	case 's':
		parseSpellOrStrangeAttr(item, part)

	case 't':
		if strings.HasPrefix(part, "td-") && len(part) > 3 {
			if val, err := strconv.Atoi(part[3:]); err == nil {
				item.Target = val
			}
		}

	case 'u':
		parseUnusualOrRestrictions(item, part)

	case 'w':
		if len(part) > 1 && part[1] >= '0' && part[1] <= '9' {
			if val, err := strconv.Atoi(part[1:]); err == nil {
				item.Wear = val
			}
		}
	}
}

func parsePaintOrPaintkit(item *Item, part string) {
	if strings.HasPrefix(part, "pk") && len(part) > 2 {
		if val, ok := parseFastInt(part[2:]); ok {
			item.Paintkit = val
		}

		return
	}

	if len(part) > 1 && part[1] >= '0' && part[1] <= '9' && !strings.Contains(part, "-") {
		if val, ok := parseFastInt(part[1:]); ok {
			item.Paint = val
		}
	}
}

func parseSpellOrStrangeAttr(item *Item, part string) {
	switch {
	case part == "strange":
		item.Quality2 = 11

	case strings.HasPrefix(part, "sd") && len(part) > 2:
		if val, ok := parseFastInt(part[2:]); ok {
			item.Seed = val
		}

	case strings.HasPrefix(part, "sp") && len(part) > 2:
		if val, ok := parseFastInt(part[2:]); ok {
			item.Parts = append(item.Parts, val)
		}

	case strings.HasPrefix(part, "s-") && len(part) > 2:
		if idx := strings.IndexByte(part[2:], '-'); idx != -1 {
			a, _ := parseFastInt(part[2 : 2+idx])
			v, _ := parseFastInt(part[2+idx+1:])
			item.Spells = append(item.Spells, Spell{Attribute: a, Value: v})
		}

	case len(part) > 1 && part[1] >= '0' && part[1] <= '9':
		if val, ok := parseFastInt(part[1:]); ok {
			item.Spells = append(item.Spells, Spell{Attribute: val, Value: 1})
		}
	}
}

func parseUnusualOrRestrictions(item *Item, part string) {
	switch {
	case part == "uncraftable":
		item.Craftable = false
	case part == "untradable" || part == "untradeable":
		item.Tradable = false
	case len(part) > 1 && part[1] >= '0' && part[1] <= '9':
		if val, ok := parseFastInt(part[1:]); ok {
			item.Effect = val
		}
	}
}

func parseFastInt(s string) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}

	var v int
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}

		v = v*10 + int(c-'0')
	}

	return v, true
}
