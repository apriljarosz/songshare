package cache

import (
	"container/list"
	"sync"
	"time"
)

// MemoryCache implements an in-memory LRU cache for search results
type MemoryCache struct {
	maxItems int
	items    map[string]*list.Element
	lru      *list.List
	mu       sync.RWMutex
}

// cacheItem represents a cached search result with expiration
type cacheItem struct {
	key       string
	results   []SearchResult
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory LRU cache
func NewMemoryCache(maxItems int) *MemoryCache {
	return &MemoryCache{
		maxItems: maxItems,
		items:    make(map[string]*list.Element),
		lru:      list.New(),
	}
}

// Get retrieves search results from the cache
func (m *MemoryCache) Get(key string) ([]SearchResult, bool) {
	m.mu.RLock()
	elem, found := m.items[key]
	m.mu.RUnlock()
	
	if !found {
		return nil, false
	}
	
	item := elem.Value.(*cacheItem)
	
	// Check expiration
	if time.Now().After(item.expiresAt) {
		m.delete(key)
		return nil, false
	}
	
	// Move to front (most recently used)
	m.mu.Lock()
	m.lru.MoveToFront(elem)
	m.mu.Unlock()
	
	// Return a copy of the results to avoid mutation
	results := make([]SearchResult, len(item.results))
	copy(results, item.results)
	
	return results, true
}

// Set stores search results in the cache
func (m *MemoryCache) Set(key string, results []SearchResult, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	expiresAt := time.Now().Add(ttl)
	
	// Create a copy of results to store
	cachedResults := make([]SearchResult, len(results))
	copy(cachedResults, results)
	
	// If key already exists, update it
	if elem, found := m.items[key]; found {
		item := elem.Value.(*cacheItem)
		item.results = cachedResults
		item.expiresAt = expiresAt
		m.lru.MoveToFront(elem)
		return
	}
	
	// Add new item
	item := &cacheItem{
		key:       key,
		results:   cachedResults,
		expiresAt: expiresAt,
	}
	
	elem := m.lru.PushFront(item)
	m.items[key] = elem
	
	// Evict oldest items if over capacity
	for m.lru.Len() > m.maxItems {
		oldest := m.lru.Back()
		if oldest != nil {
			m.removeElement(oldest)
		}
	}
}

// Delete removes a key from the cache
func (m *MemoryCache) Delete(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delete(key)
}

// delete is the internal delete method (not thread-safe)
func (m *MemoryCache) delete(key string) {
	if elem, found := m.items[key]; found {
		m.removeElement(elem)
	}
}

// removeElement removes an element from both the map and list
func (m *MemoryCache) removeElement(elem *list.Element) {
	item := elem.Value.(*cacheItem)
	delete(m.items, item.key)
	m.lru.Remove(elem)
}

// Clear removes all items from the cache
func (m *MemoryCache) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.items = make(map[string]*list.Element)
	m.lru = list.New()
}

// Size returns the current number of items in the cache
func (m *MemoryCache) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// GetStats returns cache statistics
func (m *MemoryCache) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Count expired items
	expired := 0
	now := time.Now()
	for _, elem := range m.items {
		item := elem.Value.(*cacheItem)
		if now.After(item.expiresAt) {
			expired++
		}
	}
	
	return map[string]interface{}{
		"size":        len(m.items),
		"max_items":   m.maxItems,
		"expired":     expired,
		"utilization": float64(len(m.items)) / float64(m.maxItems),
	}
}

// CleanupExpired removes all expired items from the cache
func (m *MemoryCache) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	now := time.Now()
	removed := 0
	
	// Walk through all items and remove expired ones
	for _, elem := range m.items {
		item := elem.Value.(*cacheItem)
		if now.After(item.expiresAt) {
			m.removeElement(elem)
			removed++
		}
	}
	
	return removed
}