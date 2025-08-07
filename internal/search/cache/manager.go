package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"songshare/internal/cache"
)

// Manager orchestrates multi-layer caching for search results
type Manager struct {
	memory     *MemoryCache
	redis      cache.Cache
	persistent *PersistentCache
	config     CacheConfig
}

// NewManager creates a new cache manager with all cache layers
func NewManager(redis cache.Cache, persistent *PersistentCache, config CacheConfig) *Manager {
	return &Manager{
		memory:     NewMemoryCache(1000), // 1000 items LRU
		redis:      redis,
		persistent: persistent,
		config:     config,
	}
}

// GetSearchResults retrieves search results from cache hierarchy
// Returns results and a boolean indicating if found in cache
func (m *Manager) GetSearchResults(ctx context.Context, req SearchRequest) ([]SearchResult, bool) {
	queryHash := req.GenerateQueryHash()
	
	// Layer 1: Check in-memory cache (fastest)
	if results, found := m.memory.Get(queryHash); found {
		slog.Debug("Cache hit: memory", "query", req.Query, "results", len(results))
		return results, true
	}

	// Layer 2: Check MongoDB persistent cache (fast local lookup)
	if results, found := m.persistent.GetSearchResults(ctx, queryHash); found {
		// Store in memory cache for next time
		m.memory.Set(queryHash, results, m.config.MemoryTTL)
		slog.Debug("Cache hit: persistent", "query", req.Query, "results", len(results))
		return results, true
	}

	// Layer 3: Check individual result caches in Redis
	// This is for cases where we have some platform results cached but not a full search
	if results := m.getPartialResultsFromRedis(ctx, req); len(results) > 0 {
		slog.Debug("Partial cache hit: redis", "query", req.Query, "results", len(results))
		// Don't mark this as full cache hit since results may be incomplete
		return results, false
	}

	// Cache miss at all levels
	return nil, false
}

// StoreSearchResults stores search results in all appropriate cache layers
func (m *Manager) StoreSearchResults(ctx context.Context, req SearchRequest, results []SearchResult) error {
	queryHash := req.GenerateQueryHash()

	// Store in memory cache (immediate future requests)
	m.memory.Set(queryHash, results, m.config.MemoryTTL)

	// Store in persistent cache (long-term storage)
	if err := m.persistent.StoreSearchResults(ctx, queryHash, req, results); err != nil {
		slog.Warn("Failed to store results in persistent cache", "error", err, "query", req.Query)
	}

	// Store individual results in Redis for platform-level caching
	for _, result := range results {
		if err := m.storeIndividualResult(ctx, result); err != nil {
			slog.Warn("Failed to store individual result in Redis", "error", err, "title", result.Title)
		}
	}

	return nil
}

// StoreNegativeResult caches the fact that a search returned no results
func (m *Manager) StoreNegativeResult(ctx context.Context, req SearchRequest) error {
	queryHash := req.GenerateQueryHash()
	
	// Store empty result set with shorter TTL
	emptyResults := []SearchResult{}
	
	// Memory cache
	m.memory.Set(queryHash, emptyResults, m.config.NegativeTTL)
	
	// Persistent cache with shorter TTL
	if err := m.persistent.StoreSearchResults(ctx, queryHash, req, emptyResults); err != nil {
		return fmt.Errorf("failed to store negative result: %w", err)
	}

	return nil
}

// InvalidateQuery removes all cached results for a specific query
func (m *Manager) InvalidateQuery(ctx context.Context, req SearchRequest) error {
	queryHash := req.GenerateQueryHash()
	
	// Remove from memory
	m.memory.Delete(queryHash)
	
	// Remove from persistent cache
	if err := m.persistent.InvalidateQuery(ctx, queryHash); err != nil {
		return fmt.Errorf("failed to invalidate persistent cache: %w", err)
	}
	
	return nil
}

// WarmCache pre-loads popular searches into cache
func (m *Manager) WarmCache(ctx context.Context, popularQueries []SearchRequest) error {
	slog.Info("Starting cache warming", "queries", len(popularQueries))
	
	for _, query := range popularQueries {
		// Check if already cached
		if _, found := m.GetSearchResults(ctx, query); found {
			continue // Skip if already cached
		}
		
		// This would trigger a fresh search and cache the results
		// Implementation depends on having access to the search engine
		slog.Debug("Would warm cache for query", "query", query.Query)
	}
	
	return nil
}

// GetCacheStats returns statistics about cache usage
func (m *Manager) GetCacheStats(ctx context.Context) map[string]interface{} {
	stats := make(map[string]interface{})
	
	// Memory cache stats
	memStats := m.memory.GetStats()
	stats["memory"] = memStats
	
	// Persistent cache stats
	if persistentStats, err := m.persistent.GetStats(ctx); err == nil {
		stats["persistent"] = persistentStats
	}
	
	return stats
}

// getPartialResultsFromRedis checks for individually cached platform results
func (m *Manager) getPartialResultsFromRedis(ctx context.Context, req SearchRequest) []SearchResult {
	var results []SearchResult
	
	// Build cache keys for different platforms
	platforms := []string{"spotify", "apple_music", "tidal"}
	if req.Platform != "" {
		platforms = []string{req.Platform}
	}
	
	for _, platform := range platforms {
		cacheKey := fmt.Sprintf("search:%s:%s:%s", platform, req.Query, req.GetEffectiveLimit())
		if cached, err := m.redis.Get(ctx, cacheKey); err == nil && cached != nil {
			var platformResults []SearchResult
			if err := json.Unmarshal(cached, &platformResults); err == nil {
				results = append(results, platformResults...)
			}
		}
	}
	
	return results
}

// storeIndividualResult caches a single search result in Redis
func (m *Manager) storeIndividualResult(ctx context.Context, result SearchResult) error {
	cacheKey := fmt.Sprintf("result:%s:%s", result.Platform, result.ID)
	
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	
	return m.redis.Set(ctx, cacheKey, data, m.config.RedisTTL)
}

// Close closes all cache connections
func (m *Manager) Close() error {
	if err := m.redis.Close(); err != nil {
		return fmt.Errorf("failed to close Redis cache: %w", err)
	}
	
	if err := m.persistent.Close(); err != nil {
		return fmt.Errorf("failed to close persistent cache: %w", err)
	}
	
	return nil
}