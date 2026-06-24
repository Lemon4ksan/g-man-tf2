// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mannco

import (
	"encoding/json"
	"errors"
)

// BaseResponse represents the standard Mannco.store API response envelope.
// If Err is true or Success is false, Content holds the error details.
// Otherwise, Content holds the actual response data.
type BaseResponse struct {
	Err     bool            `json:"err"`
	Success bool            `json:"success"`
	Content json.RawMessage `json:"content"`
	target  any
}

// IsSuccess checks if the response indicates a successful operation.
func (b *BaseResponse) IsSuccess() bool {
	return b.Success
}

// Error formats and returns the error details from the Content payload.
func (b *BaseResponse) Error() error {
	errMsg, err := json.Marshal(b.Content)
	if err != nil {
		return errors.New("unknown error")
	}

	return errors.New(string(errMsg))
}

// SetData binds the output destination struct target for automatic unmarshaling.
func (b *BaseResponse) SetData(data any) {
	b.target = data
}

// UnmarshalJSON handles decoding the outer envelope and automatically unmarshals
// the Content field into the configured target destination.
func (b *BaseResponse) UnmarshalJSON(data []byte) error {
	type Alias BaseResponse

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	b.Err = aux.Err
	b.Success = aux.Success

	b.Content = aux.Content
	if b.target != nil {
		if err := json.Unmarshal(b.Content, b.target); err != nil {
			return err
		}
	}

	return nil
}

// Craftable represents the craftability state of a TF2 item (1 = craftable, 0 = uncraftable).
// For games other than Team Fortress 2 (game 440), the API returns an empty string (""),
// which this custom type automatically parses as 0.
type Craftable int

// UnmarshalJSON parses both integer representations and fallback string values ("").
func (c *Craftable) UnmarshalJSON(data []byte) error {
	var val any
	if err := json.Unmarshal(data, &val); err != nil {
		return err
	}

	switch v := val.(type) {
	case float64:
		*c = Craftable(int(v))
	case int:
		*c = Craftable(v)
	case string:
		*c = 0
	default:
		*c = 0
	}

	return nil
}
