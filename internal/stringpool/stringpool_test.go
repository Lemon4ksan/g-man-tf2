// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stringpool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPool_Intern(t *testing.T) {
	t.Parallel()

	p := NewPool()

	t.Run("empty_string", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "", p.Intern(""))
	})

	t.Run("intern_deduplication", func(t *testing.T) {
		t.Parallel()

		s1 := string([]byte{'s', 'c', 'a', 't', 't', 'e', 'r', 'g', 'u', 'n'})
		s2 := string([]byte{'s', 'c', 'a', 't', 't', 'e', 'r', 'g', 'u', 'n'})

		interned1 := p.Intern(s1)
		interned2 := p.Intern(s2)

		assert.Equal(t, "scattergun", interned1)
		assert.Equal(t, "scattergun", interned2)
	})

	t.Run("concurrent_intern", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		for range 50 {
			wg.Add(1)

			go func() {
				defer wg.Done()

				_ = p.Intern("Rocket Launcher")
			}()
		}

		wg.Wait()

		assert.Equal(t, "Rocket Launcher", p.Intern("Rocket Launcher"))
	})

	t.Run("global_intern_helper", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "Global String", Intern("Global String"))
	})
}
