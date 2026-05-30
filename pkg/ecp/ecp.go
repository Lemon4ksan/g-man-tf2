// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecp

import (
	"errors"
	"regexp"
	"strings"
	"sync"
)

var (
	nativeToBold = make(map[rune]rune)
	boldToNative = make(map[rune]rune)
)

func init() {
	nRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	bRunes := []rune("𝗮𝗯𝗰𝗱𝗲𝗳𝗴𝗵𝗶𝗷𝗸𝗹𝗺𝗻𝗼𝗽𝗾𝗿𝘀𝘁𝘂𝘃𝘄𝗫𝘆𝘇𝗔𝗕𝗖𝗗𝗘𝗙𝗚𝗛𝗜𝗝𝗞𝗟𝗠𝗡𝗢𝗣𝗤𝗥𝗦𝗧𝗨𝗩𝗪𝗫𝗬𝗭𝟬𝟭𝟮𝟯𝟰𝟱𝟲𝟳𝟴𝟵")

	for i := 0; i < len(nRunes) && i < len(bRunes); i++ {
		nativeToBold[nRunes[i]] = bRunes[i]
		boldToNative[bRunes[i]] = nRunes[i]
	}
}

var keyWordReplacements = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile("(?i)Australium"), "Aus"},
	{regexp.MustCompile("(?i)Killstreak"), "Ks"},
	{regexp.MustCompile("(?i)Specialized"), "Spec"},
	{regexp.MustCompile("(?i)Professional"), "Pro"},
	{regexp.MustCompile("(?i)'s"), "s"},
}

// MappedValue holds an original item name and its corresponding generated ECP string variants.
type MappedValue struct {
	Key   string
	Value []string
}

// DecodedECP holds the decoded original item name and customer intent parsed from an ECP string.
type DecodedECP struct {
	OriginalItemName string
	DecodedIntent    string
}

// EasyCopyPaste provides methods for encoding and decoding ECP strings
// with support for mathematical Unicode bold characters and keyword abbreviations.
type EasyCopyPaste struct {
	mu           sync.RWMutex
	useBoldChars bool
	useWordSwap  bool
	delimiters   []rune
	mappedItems  map[string][]string // Key: original name, Value: generated ECP strings
}

// New creates a new EasyCopyPaste instance.
func New() *EasyCopyPaste {
	return &EasyCopyPaste{
		useBoldChars: false,
		useWordSwap:  false,
		delimiters:   []rune{' ', '\'', '-', '/', '.', '#', '!', ':', '(', ')', ','},
		mappedItems:  make(map[string][]string),
	}
}

// SetUseBoldChars configures whether to stylize strings in Unicode bold.
func (e *EasyCopyPaste) SetUseBoldChars(val bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.useBoldChars = val
}

// SetUseWordSwap configures whether to use compressed keywords like Aus, Ks, Spec.
func (e *EasyCopyPaste) SetUseWordSwap(val bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.useWordSwap = val
}

// ToEcpString encodes a TF2 item name and intent to an ECP string.
func (e *EasyCopyPaste) ToEcpString(itemOriginalName, botSideIntent string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(itemOriginalName) == 0 {
		return "", errors.New("input could not be turned into ECP string because its length was 0")
	}

	customerSideIntent := "buy"
	if botSideIntent == "buy" {
		customerSideIntent = "sell"
	}

	mappedEcpEntry := e.mapString(itemOriginalName)
	if len(mappedEcpEntry.Value) == 0 {
		return "", errors.New("failed to generate ECP string")
	}

	finalEcpStr := mappedEcpEntry.Value[0]
	if e.useWordSwap {
		for _, ecpStrEntry := range mappedEcpEntry.Value {
			if len(ecpStrEntry) < len(finalEcpStr) {
				finalEcpStr = ecpStrEntry
			}
		}
	}

	nativeEcpString := customerSideIntent + "_" + finalEcpStr
	if e.useBoldChars {
		return e.swapToBoldChars(nativeEcpString), nil
	}

	return nativeEcpString, nil
}

// ReverseEcpString decodes an ECP string back to the original item name and intent.
func (e *EasyCopyPaste) ReverseEcpString(escaped string) (*DecodedECP, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(escaped) == 0 {
		return nil, errors.New("input ECP string's length is 0")
	}

	nativeStr := e.swapToNativeChars(escaped)

	customerIntent := ""
	if strings.HasPrefix(nativeStr, "sell_") {
		customerIntent = "sell"
	} else if strings.HasPrefix(nativeStr, "buy_") {
		customerIntent = "buy"
	}

	if customerIntent == "" {
		return nil, errors.New("could not decide customer intent from ECP string")
	}

	intentClearedEcpStr := strings.Replace(nativeStr, customerIntent+"_", "", 1)

	itemMappedOriginalName, ok := e.findMappedValue(intentClearedEcpStr)
	if !ok {
		return nil, errors.New("the item name was not found in the ECP map")
	}

	return &DecodedECP{
		OriginalItemName: itemMappedOriginalName.Key,
		DecodedIntent:    customerIntent,
	}, nil
}

func (e *EasyCopyPaste) swapToBoldChars(str string) string {
	var sb strings.Builder
	for _, r := range str {
		if b, ok := nativeToBold[r]; ok {
			sb.WriteRune(b)
		} else {
			sb.WriteRune(r)
		}
	}

	return sb.String()
}

func (e *EasyCopyPaste) swapToNativeChars(str string) string {
	var sb strings.Builder
	for _, r := range str {
		if n, ok := boldToNative[r]; ok {
			sb.WriteRune(n)
		} else {
			sb.WriteRune(r)
		}
	}

	return sb.String()
}

func (e *EasyCopyPaste) swapPreMappedKeywords(ecpString string) string {
	result := ecpString
	for _, rep := range keyWordReplacements {
		result = rep.pattern.ReplaceAllString(result, rep.replacement)
	}

	return result
}

func (e *EasyCopyPaste) constructEcpCharSequence(originalItemName string) string {
	runes := []rune(originalItemName)

	var result []rune
	for i := range runes {
		selectedChar := runes[i]
		if e.isDelimiter(selectedChar) {
			if i+1 < len(runes) && (runes[i+1] == ' ' || e.isDelimiter(runes[i+1])) || i+1 == len(runes) {
				continue
			} else {
				result = append(result, '_')
			}
		} else {
			result = append(result, selectedChar)
		}
	}

	return string(result)
}

func (e *EasyCopyPaste) isDelimiter(r rune) bool {
	for _, d := range e.delimiters {
		if d == r {
			return true
		}
	}

	return false
}

func (e *EasyCopyPaste) findMappedValue(str string) (MappedValue, bool) {
	lowerCaseStr := strings.ToLower(str)
	for key, value := range e.mappedItems {
		if strings.ToLower(key) == lowerCaseStr {
			return MappedValue{Key: key, Value: value}, true
		}

		for _, entry := range value {
			if strings.ToLower(entry) == lowerCaseStr {
				return MappedValue{Key: key, Value: value}, true
			}
		}
	}

	return MappedValue{}, false
}

func (e *EasyCopyPaste) mapString(itemName string) MappedValue {
	if found, ok := e.findMappedValue(itemName); ok {
		return found
	}

	ecpFormatSet := make(map[string]struct{})
	ecpFormatSet[e.constructEcpCharSequence(itemName)] = struct{}{}
	ecpFormatSet[e.constructEcpCharSequence(e.swapPreMappedKeywords(itemName))] = struct{}{}
	ecpFormatSet[e.swapPreMappedKeywords(e.constructEcpCharSequence(itemName))] = struct{}{}

	var ecpFormatDistinctArray []string
	for k := range ecpFormatSet {
		ecpFormatDistinctArray = append(ecpFormatDistinctArray, k)
	}

	e.mappedItems[itemName] = ecpFormatDistinctArray

	return MappedValue{Key: itemName, Value: ecpFormatDistinctArray}
}
