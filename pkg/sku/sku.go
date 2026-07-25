// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sku implements the TF2 Stock Keeping Unit format.
package sku

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var rxPriceKey = regexp.MustCompile(
	`^(\d+);([0-9]|[1][0-5])(;((uncraftable)|(untrad(e)?able)|(australium)|(festive)|(strange)|((u|pk|td-|c|od-|oq-|p|sd)\d+)|(w[1-5])|(kt-[1-3])|(n((100)|[1-9]\d?))))*?$|^\d+$`,
)

// IsValid tests if a string matches the standard TF2 SKU format.
func IsValid(sku string) bool {
	return rxPriceKey.MatchString(sku)
}

// Item represents a TF2 item with all possible SKU attributes.
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
	Quality2      int // 11 for strange
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

// Spell represents a Halloween spell attached to an item.
type Spell struct {
	Attribute int
	Value     int
}

var skuBufferPool = sync.Pool{
	New: func() any {
		b := new(bytes.Buffer)
		b.Grow(64)
		return b
	},
}

// FromString parses a SKU string into an Item.
// The expected format is "defindex;quality[;attribute]*".
// Attributes may include dashes (e.g., "kt-2") which are ignored during parsing.
func FromString(sku string) (*Item, error) {
	if len(sku) == 0 {
		return nil, errors.New("invalid SKU: empty")
	}

	item := &Item{
		Craftable: true,
		Tradable:  true,
	}

	start := 0
	partIdx := 0

	for start < len(sku) {
		end := strings.IndexByte(sku[start:], ';')

		var part string
		if end == -1 {
			part = sku[start:]
			start = len(sku)
		} else {
			part = sku[start : start+end]
			start += end + 1
		}

		if len(part) == 0 {
			continue
		}

		if partIdx == 0 {
			defindex, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid defindex: %s", part)
			}

			item.Defindex = defindex
			partIdx++

			continue
		}

		if partIdx == 1 {
			quality, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid quality: %s", part)
			}

			item.Quality = quality
			partIdx++

			continue
		}

		parseSKUAttribute(item, part)

		partIdx++
	}

	if partIdx < 2 {
		return nil, fmt.Errorf("invalid SKU: %s", sku)
	}

	return item, nil
}

// FromObject converts an Item into its SKU string representation.
// The output format follows the conventions used in the original JavaScript code.
func FromObject(item *Item) string {
	buf := skuBufferPool.Get().(*bytes.Buffer)

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

// ToPricingSKU normalizes the specified SKU string by stripping transient flags
// such as Festivized, Spells, Strange Parts, and Paint, which are typically
// not priced separately or ignored by the base price database.
// Returns the unmodified SKU string if parsing fails.
func ToPricingSKU(skuStr string) string {
	it, err := FromString(skuStr)
	if err != nil {
		return skuStr
	}

	it.Festivized = false
	it.Spells = nil
	it.Parts = nil
	it.PartValues = nil
	it.Paint = 0

	return FromObject(it)
}

func parseSKUAttribute(item *Item, part string) {
	switch {
	case part == "uncraftable":
		item.Craftable = false
	case part == "untradable" || part == "untradeable":
		item.Tradable = false
	case part == "australium":
		item.Australium = true
	case part == "festive":
		item.Festivized = true
	case part == "strange":
		item.Quality2 = 11
	case strings.HasPrefix(part, "kt-") && len(part) > 3:
		if val, err := strconv.Atoi(part[3:]); err == nil {
			item.Killstreak = val
		}
	case strings.HasPrefix(part, "u") && len(part) > 1:
		if val, err := strconv.Atoi(part[1:]); err == nil {
			item.Effect = val
		}
	case strings.HasPrefix(part, "pk") && len(part) > 2:
		if val, err := strconv.Atoi(part[2:]); err == nil {
			item.Paintkit = val
		}
	case strings.HasPrefix(part, "sd") && len(part) > 2:
		if val, err := strconv.Atoi(part[2:]); err == nil {
			item.Seed = val
		}
	case strings.HasPrefix(part, "w") && len(part) > 1:
		if val, err := strconv.Atoi(part[1:]); err == nil {
			item.Wear = val
		}
	case strings.HasPrefix(part, "td-") && len(part) > 3:
		if val, err := strconv.Atoi(part[3:]); err == nil {
			item.Target = val
		}
	case strings.HasPrefix(part, "n") && len(part) > 1:
		if val, err := strconv.Atoi(part[1:]); err == nil {
			item.Craftnumber = val
		}
	case strings.HasPrefix(part, "c") && len(part) > 1:
		if val, err := strconv.Atoi(part[1:]); err == nil {
			item.Crateseries = val
		}
	case strings.HasPrefix(part, "od-") && len(part) > 3:
		if val, err := strconv.Atoi(part[3:]); err == nil {
			item.Output = val
		}
	case strings.HasPrefix(part, "oq-") && len(part) > 3:
		if val, err := strconv.Atoi(part[3:]); err == nil {
			item.OutputQuality = val
		}
	case strings.HasPrefix(part, "p") && len(part) > 1 && !strings.Contains(part, "-"):
		if val, err := strconv.Atoi(part[1:]); err == nil {
			item.Paint = val
		}
	case strings.HasPrefix(part, "s-") && len(part) > 2:
		if idx := strings.IndexByte(part[2:], '-'); idx != -1 {
			a, _ := strconv.Atoi(part[2 : 2+idx])
			v, _ := strconv.Atoi(part[2+idx+1:])
			item.Spells = append(item.Spells, Spell{Attribute: a, Value: v})
		}

	case strings.HasPrefix(part, "sp") && len(part) > 2:
		if val, err := strconv.Atoi(part[2:]); err == nil {
			item.Parts = append(item.Parts, val)
		}
	case strings.HasPrefix(part, "s") && len(part) > 1:
		if val, err := strconv.Atoi(part[1:]); err == nil {
			item.Spells = append(item.Spells, Spell{Attribute: val, Value: 1})
		}
	}
}
