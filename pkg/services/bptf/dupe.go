// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/x/codec/decode"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
)

// BackpackTFChecker implements duplicate checking via the backpack.tf website.
// Even though this is scraping, we use the transport and settings from the configured requester.
type BackpackTFChecker struct {
	r *aoni.Client
}

// NewBackpackTFChecker creates a new checker instance.
// It takes a *aoni.Client, which already contains HTTP transport and middleware settings.
func NewBackpackTFChecker(r *aoni.Client) *BackpackTFChecker {
	return &BackpackTFChecker{
		r: r,
	}
}

// CheckHistory checks the item's history on the backpack.tf website.
func (c *BackpackTFChecker) CheckHistory(
	ctx context.Context,
	assetID uint64,
	mods ...aoni.RequestModifier,
) (backpack.HistoryStatus, error) {
	path := "https://backpack.tf/item/" + strconv.FormatUint(assetID, 10)

	allMods := append([]aoni.RequestModifier{decode.WithRaw()}, mods...)

	resp, err := c.r.GetTo[[]byte](ctx, path, allMods...)
	if err != nil {
		var apiErr *aoni.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return backpack.HistoryStatus{Recorded: false}, nil
		}

		return backpack.HistoryStatus{}, fmt.Errorf("bptf dupe check request failed: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(*resp))
	if err != nil {
		return backpack.HistoryStatus{}, fmt.Errorf("failed to parse bptf HTML: %w", err)
	}

	if doc.Find("table").Length() != 1 {
		return backpack.HistoryStatus{Recorded: false}, nil
	}

	isDuped := doc.Find("#dupe-modal-btn").Length() > 0

	return backpack.HistoryStatus{
		Recorded: true,
		IsDuped:  isDuped,
	}, nil
}
