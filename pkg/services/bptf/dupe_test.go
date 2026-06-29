// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bptf

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/lemon4ksan/aoni"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
)

type mockRoundTripper struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

type MockDupeChecker struct {
	Responses map[uint64]backpack.HistoryStatus
	Err       error
}

func (m *MockDupeChecker) CheckHistory(ctx context.Context, id uint64) (backpack.HistoryStatus, error) {
	if m.Err != nil {
		return backpack.HistoryStatus{}, m.Err
	}

	return m.Responses[id], nil
}

func createBptfHTML(hasTable, isDuped bool) string {
	var sb strings.Builder
	sb.WriteString("<html><body>")

	if hasTable {
		sb.WriteString("<table><tr><td>History</td></tr></table>")

		if isDuped {
			sb.WriteString(`<button id="dupe-modal-btn">Duplicated</button>`)
		}
	}

	sb.WriteString("</body></html>")

	return sb.String()
}

func TestBackpackTFChecker_CheckHistory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
		transport  func(req *http.Request) (*http.Response, error)
		wantStatus backpack.HistoryStatus
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "not_found_api_error",
			statusCode: 404,
			transport: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(strings.NewReader(`{"success":false,"message":"not found"}`)),
				}, nil
			},
			wantStatus: backpack.HistoryStatus{Recorded: false},
			wantErr:    true,
		},
		{
			name: "connection_failed_error",
			transport: func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("network timeout")
			},
			wantStatus: backpack.HistoryStatus{Recorded: false},
			wantErr:    true,
			errMsg:     "bptf dupe check request failed",
		},
		{
			name:       "clean_item_history",
			statusCode: 200,
			body:       createBptfHTML(true, false),
			wantStatus: backpack.HistoryStatus{Recorded: true, IsDuped: false},
		},
		{
			name:       "duped_item_history",
			statusCode: 200,
			body:       createBptfHTML(true, true),
			wantStatus: backpack.HistoryStatus{Recorded: true, IsDuped: true},
		},
		{
			name:       "no_table_not_recorded",
			statusCode: 200,
			body:       createBptfHTML(false, false),
			wantStatus: backpack.HistoryStatus{Recorded: false},
		},
		{
			name:       "malformed_html_no_document",
			statusCode: 200,
			body:       "",
			wantStatus: backpack.HistoryStatus{Recorded: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rt http.RoundTripper
			if tt.transport != nil {
				rt = &mockRoundTripper{fn: tt.transport}
			} else {
				rt = &mockRoundTripper{
					fn: func(req *http.Request) (*http.Response, error) {
						return &http.Response{
							StatusCode: tt.statusCode,
							Body:       io.NopCloser(strings.NewReader(tt.body)),
						}, nil
					},
				}
			}

			httpClient := &http.Client{Transport: rt}
			checker := &BackpackTFChecker{
				bptfClient: New(aoni.NewClient(httpClient), "", ""),
			}

			got, err := checker.CheckHistory(t.Context(), 123)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil")
				}

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)

				if !reflect.DeepEqual(got, tt.wantStatus) {
					t.Errorf("CheckHistory() = %v, want %v", got, tt.wantStatus)
				}
			}
		})
	}
}
