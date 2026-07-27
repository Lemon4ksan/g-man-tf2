// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bytesconv

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestB2S_S2B(t *testing.T) {
	t.Parallel()

	t.Run("b2s_empty_and_valid", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "", B2S(nil))
		assert.Equal(t, "", B2S([]byte{}))
		assert.Equal(t, "hello", B2S([]byte("hello")))
	})

	t.Run("s2b_empty_and_valid", func(t *testing.T) {
		t.Parallel()

		assert.Nil(t, S2B(""))
		assert.Equal(t, []byte("world"), S2B("world"))
	})
}

func TestLowercaseByte(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   byte
		want byte
	}{
		{'A', 'a'},
		{'Z', 'z'},
		{'a', 'a'},
		{'0', '0'},
	}

	for _, tt := range tests {
		t.Run(string(tt.in), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, LowercaseByte(tt.in))
		})
	}
}

func TestEqualFoldASCII(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b string
		want bool
	}{
		{"hello", "HELLO", true},
		{"", "", true},
		{"abc", "abcd", false},
		{"foo", "bar", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, EqualFoldASCII(tt.a, tt.b))
		})
	}
}

func TestAppendToLower(t *testing.T) {
	t.Parallel()

	dst := []byte("prefix_")
	res := AppendToLower(dst, []byte("HELLO"))
	assert.Equal(t, []byte("prefix_hello"), res)

	assert.Equal(t, dst, AppendToLower(dst, nil))
}

func TestAppendQueryEscaped(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	AppendQueryEscaped(buf, []byte("hello world & tf2"))
	assert.Equal(t, "hello+world+%26+tf2", buf.String())
}

func TestTrimQuotes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []byte("test"), TrimQuotes([]byte(`"test"`)))
	assert.Equal(t, []byte("test"), TrimQuotes([]byte(`test`)))
	assert.Equal(t, []byte(`"`), TrimQuotes([]byte(`"`)))
}

func TestParseUint64(t *testing.T) {
	t.Parallel()

	val, ok := ParseUint64([]byte("123456"))
	assert.True(t, ok)
	assert.Equal(t, uint64(123456), val)

	_, ok = ParseUint64([]byte(""))
	assert.False(t, ok)

	_, ok = ParseUint64([]byte("123a"))
	assert.False(t, ok)
}

func TestParseInt64(t *testing.T) {
	t.Parallel()

	val, ok := ParseInt64([]byte("-12345"))
	assert.True(t, ok)
	assert.Equal(t, int64(-12345), val)

	val, ok = ParseInt64([]byte("+54321"))
	assert.True(t, ok)
	assert.Equal(t, int64(54321), val)

	_, ok = ParseInt64([]byte("-"))
	assert.False(t, ok)

	_, ok = ParseInt64([]byte(""))
	assert.False(t, ok)

	_, ok = ParseInt64([]byte("-12a"))
	assert.False(t, ok)
}
