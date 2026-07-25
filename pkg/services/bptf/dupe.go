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
	"github.com/lemon4ksan/aoni/codec/decode"
	"github.com/lemon4ksan/aoni/request"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
)

// BackpackTFChecker implements duplicate checking via the backpack.tf website.
// Even though this is scraping, we use the transport and settings from Client.
type BackpackTFChecker struct {
	bptfClient *Client
}

// NewBackpackTFChecker creates a new checker instance.
// It takes a Client, which already contains API tokens and logger settings.
func NewBackpackTFChecker(client *Client) *BackpackTFChecker {
	return &BackpackTFChecker{
		bptfClient: client,
	}
}

// CheckHistory checks the item's history on the backpack.tf website.
func (c *BackpackTFChecker) CheckHistory(ctx context.Context, assetID uint64) (backpack.HistoryStatus, error) {
	path := "https://backpack.tf/item/" + strconv.FormatUint(assetID, 10)

	resp, err := request.GetTo[[]byte](ctx, c.bptfClient.REST(), path, decode.WithRaw())
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
