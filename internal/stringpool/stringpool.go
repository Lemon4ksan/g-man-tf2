// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package stringpool provides concurrent string interning to eliminate redundant
// string allocations across large inventories and schema structures.
package stringpool

import (
	"sync"
)

const shardCount = 64

type shard struct {
	mu   sync.RWMutex
	pool map[string]string
}

// Pool is a sharded thread-safe string interner.
type Pool struct {
	shards [shardCount]shard
}

var globalPool = NewPool()

// NewPool creates a new sharded string pool.
func NewPool() *Pool {
	p := &Pool{}
	for i := range shardCount {
		p.shards[i].pool = make(map[string]string, 256)
	}

	return p
}

func (p *Pool) getShard(s string) *shard {
	// Fast fnv-1a hash for sharding
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}

	return &p.shards[h%shardCount]
}

// Intern returns a canonical copy of the string s.
// If s is already in the pool, the existing instance is returned.
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
	if interned, ok := sh.pool[s]; ok {
		sh.mu.Unlock()
		return interned
	}

	sh.pool[s] = s
	sh.mu.Unlock()

	return s
}

// Intern returns a canonical copy of s using the global string pool.
func Intern(s string) string {
	return globalPool.Intern(s)
}
