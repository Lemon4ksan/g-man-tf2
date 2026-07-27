// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package stringpool implements thread-safe string interning across 64 shards to eliminate duplicate string allocations.
package stringpool

import (
	"sync"
)

const shardCount = 64

type shard struct {
	mu   sync.RWMutex
	pool map[string]string
}

// Pool maintains 64 sharded maps to minimize mutex contention during concurrent schema string lookups.
type Pool struct {
	shards [shardCount]shard
}

var globalPool = NewPool()

// NewPool constructs a sharded string interner.
func NewPool() *Pool {
	p := &Pool{}
	for i := range shardCount {
		p.shards[i].pool = make(map[string]string, 256)
	}

	return p
}

func (p *Pool) getShard(s string) *shard {
	var h uint32 = 2166136261
	for i := range len(s) {
		h ^= uint32(s[i])
		h *= 16777619
	}

	return &p.shards[h%shardCount]
}

// Intern returns a canonical shared reference to string s.
//
// Thread Safety:
//   - Safe for concurrent use across multiple goroutines using double-checked RWMutex locking.
func (p *Pool) Intern(s string) string {
	if len(s) == 0 {
		return ""
	}

	sh := p.getShard(s)

	sh.mu.RLock()

	if interned, ok := sh.pool[s]; ok {
		sh.mu.RUnlock()
		return interned
	}

	sh.mu.RUnlock()

	sh.mu.Lock()
	defer sh.mu.Unlock()

	if interned, ok := sh.pool[s]; ok {
		return interned
	}

	sh.pool[s] = s

	return s
}

// Intern returns a canonical shared reference to s using the package-level global string pool.
func Intern(s string) string {
	return globalPool.Intern(s)
}
