package neo4jgraph

import (
	"sync"
	"time"
)

type cacheEntry struct {
	until time.Time
	value any
}

type ttlCache struct {
	ttl time.Duration
	now func() time.Time
	mu  sync.Mutex
	m   map[string]cacheEntry
}

func newTTLCache(ttl time.Duration) *ttlCache {
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}
	return &ttlCache{ttl: ttl, now: time.Now, m: make(map[string]cacheEntry)}
}

func (c *ttlCache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || c.now().After(e.until) {
		delete(c.m, key)
		return nil, false
	}
	return e.value, true
}

func (c *ttlCache) set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cacheEntry{until: c.now().Add(c.ttl), value: value}
}
