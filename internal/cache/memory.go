package cache

import (
	"sync"
	"time"
)

type item struct {
	value      interface{}
	expiration int64
}

type MemoryCache struct {
	items map[string]item
	mu    sync.RWMutex
}

func NewMemoryCache() *MemoryCache {
	cache := &MemoryCache{
		items: make(map[string]item),
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

func (c *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiration := time.Now().Add(ttl).UnixNano()
	c.items[key] = item{
		value:      value,
		expiration: expiration,
	}
}

func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().UnixNano() > item.expiration {
		return nil, false
	}

	return item.value, true
}

func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now().UnixNano()

		for key, item := range c.items {
			if now > item.expiration {
				delete(c.items, key)
			}
		}

		c.mu.Unlock()
	}
}
