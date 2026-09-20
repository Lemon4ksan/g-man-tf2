// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

import (
	"context"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/foundation/async/pipeline"
)

// Client is an alias for API.
type Client = API

// NewClient creates a new PriceDB API client (alias for New).
func NewClient(doer any, opts ...aoni.ClientOption) Client {
	return New(doer, opts...)
}

// GetItemsBulk fetches the latest prices for an array of SKUs in concurrent batches.
// It automatically filters out empty SKUs and splits the request into batches of up to 100 SKUs.
func GetItemsBulk(ctx context.Context, client API, skus []string, mods ...aoni.RequestModifier) ([]*Price, error) {
	validSKUs := make([]string, 0, len(skus))
	for _, sku := range skus {
		if sku != "" {
			validSKUs = append(validSKUs, sku)
		}
	}

	if len(validSKUs) == 0 {
		return nil, nil
	}

	const batchSize = 100

	var batches [][]string
	for i := 0; i < len(validSKUs); i += batchSize {
		end := min(i+batchSize, len(validSKUs))
		batches = append(batches, validSKUs[i:end])
	}

	results, err := pipeline.Map(ctx, pipeline.PipelineConfig{
		Workers: 3,
		RPS:     5,
		Burst:   2,
	}, batches, func(chunkCtx context.Context, batch []string) ([]*Price, error) {
		req := bulkRequest{SKUs: batch}
		return client.PostItemsBulk(chunkCtx, req, mods...)
	})
	if err != nil {
		return nil, err
	}

	var allPrices []*Price
	for _, batch := range results {
		allPrices = append(allPrices, batch...)
	}

	return allPrices, nil
}
