// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pricing provides centralized domain types for TF2 item valuation source tracking.
package pricing

// Source represents the origin or subsystem responsible for a price update.
type Source string

const (
	// SourceManual represents a price set manually by an administrator.
	// It is a core domain constant that disables automatic pricing adjustments.
	SourceManual Source = "Manual"
)
