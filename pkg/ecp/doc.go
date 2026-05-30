// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package ecp implements the Easy-Copy-Paste (ECP) format for Team Fortress 2 items.

It provides utilities to encode complex item names and transaction intents into compact,
chat-friendly strings and decode them back to their original form. This is particularly
useful for steam trading bots that accept copy-pasted text commands from users.

The package supports mathematical Unicode bold styling and pre-mapped keyword abbreviations
(e.g., shortening "Australium" to "Aus" and "Killstreak" to "Ks") to minimize message lengths.

Key Types:
  - [EasyCopyPaste] is the central encoder and decoder that maintains state in memory.
  - [MappedValue] contains an item name along with its generated ECP abbreviations.
  - [DecodedECP] represents the output of a successfully parsed and decoded ECP string.

Basic Example:

	package main

	import (
		"fmt"
		"github.com/lemon4ksan/g-man-tf2/pkg/ecp"
	)

	func main() {
		// Initialize a new EasyCopyPaste instance
		e := ecp.New()
		e.SetUseWordSwap(true)
		e.SetUseBoldChars(true)

		// Encode an item name with a buy intent (resulting string will be styled for customer to sell)
		ecpStr, err := e.ToEcpString("Professional Killstreak Lugermorph", "buy")
		if err != nil {
			return
		}
		fmt.Println("ECP String:", ecpStr)

		// Decode the ECP string back to the original item name and intent
		decoded, err := e.ReverseEcpString(ecpStr)
		if err != nil {
			return
		}
		fmt.Printf("Item: %s, Intent: %s\n", decoded.OriginalItemName, decoded.DecodedIntent)
	}
*/
package ecp
