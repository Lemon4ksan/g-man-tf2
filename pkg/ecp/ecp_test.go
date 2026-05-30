// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecp

import (
	"testing"
)

func TestEasyCopyPaste_Basic(t *testing.T) {
	e := New()
	e.SetUseBoldChars(false)
	e.SetUseWordSwap(false)

	originalName := "Mann Co. Supply Crate Key"

	// Test encoding
	ecpStr, err := e.ToEcpString(originalName, "sell")
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	expected := "buy_Mann_Co_Supply_Crate_Key"
	if ecpStr != expected {
		t.Errorf("Expected encoded string to be %q, got %q", expected, ecpStr)
	}

	// Test decoding
	decoded, err := e.ReverseEcpString(ecpStr)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if decoded.OriginalItemName != originalName {
		t.Errorf("Expected decoded name to be %q, got %q", originalName, decoded.OriginalItemName)
	}

	if decoded.DecodedIntent != "buy" {
		t.Errorf("Expected decoded intent to be 'buy', got %q", decoded.DecodedIntent)
	}
}

func TestEasyCopyPaste_WordSwapAndBold(t *testing.T) {
	e := New()
	e.SetUseBoldChars(true)
	e.SetUseWordSwap(true)

	originalName := "Specialized Killstreak Scattergun"

	// Test encoding with word swap and bold characters
	ecpStr, err := e.ToEcpString(originalName, "sell")
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	// Specialized -> Spec, Killstreak -> Ks
	// sell listing from bot -> buy from customer
	// "buy_Spec_Ks_Scattergun" in Unicode Bold:
	expectedBold := "𝗯𝘂𝘆_𝗦𝗽𝗲𝗰_𝗞𝘀_𝗦𝗰𝗮𝘁𝘁𝗲𝗿𝗴𝘂𝗻"
	if ecpStr != expectedBold {
		t.Errorf("Expected bold encoded string to be %q, got %q", expectedBold, ecpStr)
	}

	// Test decoding back to original
	decoded, err := e.ReverseEcpString(ecpStr)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if decoded.OriginalItemName != originalName {
		t.Errorf("Expected decoded name to be %q, got %q", originalName, decoded.OriginalItemName)
	}

	if decoded.DecodedIntent != "buy" {
		t.Errorf("Expected decoded intent to be 'buy', got %q", decoded.DecodedIntent)
	}
}

func TestEasyCopyPaste_Australium(t *testing.T) {
	e := New()
	e.SetUseBoldChars(false)
	e.SetUseWordSwap(true)

	originalName := "Strange Golden Frying Pan"

	ecpStr, err := e.ToEcpString(originalName, "buy")
	if err != nil {
		t.Fatalf("Failed to encode: %v", err)
	}

	expected := "sell_Strange_Golden_Frying_Pan"
	if ecpStr != expected {
		t.Errorf("Expected encoded string to be %q, got %q", expected, ecpStr)
	}

	decoded, err := e.ReverseEcpString(ecpStr)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if decoded.OriginalItemName != originalName {
		t.Errorf("Expected decoded name to be %q, got %q", originalName, decoded.OriginalItemName)
	}

	if decoded.DecodedIntent != "sell" {
		t.Errorf("Expected decoded intent to be 'sell', got %q", decoded.DecodedIntent)
	}
}
