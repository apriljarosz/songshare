package search

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"songshare/internal/repositories"
	"songshare/internal/search/cache"
	"songshare/internal/services"
)

// Engine is the main search orchestrator
type Engine struct {
	cacheManager      *cache.Manager
	songRepository    repositories.SongRepository
	resolutionService *services.SongResolutionService
	sources           map[string]SearchSource
	ranker            *Ranker
	config           CacheConfig
}

// SearchSource defines the interface for search sources
type SearchSource interface {
	Name() string
	Search(ctx context.Context, req SearchRequest) ([]SearchResult, error)
	IsEnabled() bool
}

// NewEngine creates a new search engine with all components
func NewEngine(
	cacheManager *cache.Manager,
	songRepository repositories.SongRepository,
	resolutionService *services.SongResolutionService,
) *Engine {
	engine := &Engine{
		cacheManager:      cacheManager,
		songRepository:    songRepository,
		resolutionService: resolutionService,
		sources:           make(map[string]SearchSource),
		ranker:            NewRanker(),
		config:           DefaultCacheConfig(),
	}

	// Initialize search sources
	engine.initializeSources()

	return engine
}

// initializeSources sets up all available search sources
func (e *Engine) initializeSources() {
	// Local database source
	localSource := NewLocalSource(e.songRepository)
	e.sources[localSource.Name()] = localSource

	// Platform sources
	if spotifyService := e.resolutionService.GetPlatformService("spotify"); spotifyService != nil {
		spotifySource := NewSpotifySource(spotifyService)
		e.sources[spotifySource.Name()] = spotifySource
	}

	if appleService := e.resolutionService.GetPlatformService("apple_music"); appleService != nil {
		appleSource := NewAppleSource(appleService)
		e.sources[appleSource.Name()] = appleSource
	}

	if tidalService := e.resolutionService.GetPlatformService("tidal"); tidalService != nil {
		tidalSource := NewTidalSource(tidalService)
		e.sources[tidalSource.Name()] = tidalSource
	}
}

// Search performs a comprehensive search across all sources with smart caching
func (e *Engine) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	startTime := time.Now()
	
	// Validate request
	if req.IsEmpty() {
		return &SearchResponse{
			Results:   []SearchResult{},
			Query:     req,
			Total:     0,
			FromCache: false,
			Duration:  time.Since(startTime).String(),
		}, nil
	}

	// Check cache first
	cacheReq := e.searchToCacheRequest(req)
	if cacheResults, found := e.cacheManager.GetSearchResults(ctx, cacheReq); found {
		results := e.cacheToSearchResults(cacheResults)
		slog.Debug("Search cache hit", "query", req.Query, "results", len(results))
		
		// Rank cached results (they might have been cached with different ranking)
		rankedResults := e.ranker.RankResults(results, req.Query)
		
		return &SearchResponse{
			Results:   rankedResults,
			Query:     req,
			Total:     len(rankedResults),
			FromCache: true,
			Duration:  time.Since(startTime).String(),
		}, nil
	}

	// Cache miss - perform live search
	slog.Debug("Search cache miss, performing live search", "query", req.Query)
	
	results, err := e.performLiveSearch(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("live search failed: %w", err)
	}

	// Rank results by relevance
	rankedResults := e.ranker.RankResults(results, req.Query)

	// Apply limit
	limit := req.GetEffectiveLimit()
	if len(rankedResults) > limit {
		rankedResults = rankedResults[:limit]
	}

	// Cache the results for future queries
	cacheReq2 := e.searchToCacheRequest(req)
	if len(rankedResults) > 0 {
		cacheResults := e.searchToCacheResults(rankedResults)
		if err := e.cacheManager.StoreSearchResults(ctx, cacheReq2, cacheResults); err != nil {
			slog.Warn("Failed to cache search results", "error", err, "query", req.Query)
		}
	} else {
		// Cache negative result
		if err := e.cacheManager.StoreNegativeResult(ctx, cacheReq2); err != nil {
			slog.Warn("Failed to cache negative result", "error", err, "query", req.Query)
		}
	}

	return &SearchResponse{
		Results:   rankedResults,
		Query:     req,
		Total:     len(rankedResults),
		FromCache: false,
		Duration:  time.Since(startTime).String(),
	}, nil
}

// performLiveSearch searches all relevant sources and combines results
func (e *Engine) performLiveSearch(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	var allResults []SearchResult
	sources := e.getRelevantSources(req)
	
	// Search each source concurrently
	resultsChan := make(chan sourceResult, len(sources))
	
	for _, source := range sources {
		go func(src SearchSource) {
			results, err := src.Search(ctx, req)
			resultsChan <- sourceResult{
				source:  src.Name(),
				results: results,
				err:     err,
			}
		}(source)
	}

	// Collect results from all sources
	for i := 0; i < len(sources); i++ {
		result := <-resultsChan
		
		if result.err != nil {
			slog.Warn("Source search failed", "source", result.source, "error", result.err, "query", req.Query)
			continue
		}
		
		slog.Debug("Source search completed", "source", result.source, "results", len(result.results), "query", req.Query)
		
		// Add source information and timestamp
		for _, searchResult := range result.results {
			searchResult.CachedAt = time.Now()
			allResults = append(allResults, searchResult)
		}
	}

	return allResults, nil
}

// getRelevantSources returns the sources that should be searched based on the request
func (e *Engine) getRelevantSources(req SearchRequest) []SearchSource {
	var sources []SearchSource

	// Always search local database first
	if localSource, exists := e.sources["local"]; exists && localSource.IsEnabled() {
		sources = append(sources, localSource)
	}

	// If specific platform requested, only search that platform
	if req.Platform != "" {
		if source, exists := e.sources[req.Platform]; exists && source.IsEnabled() {
			sources = append(sources, source)
		}
		return sources
	}

	// Search all available platform sources
	platformOrder := []string{"spotify", "apple_music", "tidal"} // Priority order
	for _, platform := range platformOrder {
		if source, exists := e.sources[platform]; exists && source.IsEnabled() {
			sources = append(sources, source)
		}
	}

	return sources
}

// InvalidateCache removes cached results for a query
func (e *Engine) InvalidateCache(ctx context.Context, req SearchRequest) error {
	cacheReq := e.searchToCacheRequest(req)
	return e.cacheManager.InvalidateQuery(ctx, cacheReq)
}

// GetCacheStats returns comprehensive cache statistics
func (e *Engine) GetCacheStats(ctx context.Context) map[string]interface{} {
	return e.cacheManager.GetCacheStats(ctx)
}

// GetPopularQueries returns frequently searched queries for cache warming
func (e *Engine) GetPopularQueries(ctx context.Context, limit int) ([]SearchRequest, error) {
	// This would need access to the persistent cache
	// For now, return empty slice
	return []SearchRequest{}, nil
}

// WarmCache pre-loads popular searches
func (e *Engine) WarmCache(ctx context.Context) error {
	popularQueries, err := e.GetPopularQueries(ctx, 50)
	if err != nil {
		return fmt.Errorf("failed to get popular queries: %w", err)
	}

	// Convert search requests to cache requests
	cacheQueries := make([]cache.SearchRequest, len(popularQueries))
	for i, query := range popularQueries {
		cacheQueries[i] = e.searchToCacheRequest(query)
	}

	return e.cacheManager.WarmCache(ctx, cacheQueries)
}

// GetEnabledSources returns a list of currently enabled search sources
func (e *Engine) GetEnabledSources() []string {
	var enabled []string
	for name, source := range e.sources {
		if source.IsEnabled() {
			enabled = append(enabled, name)
		}
	}
	sort.Strings(enabled)
	return enabled
}

// sourceResult holds the result of searching a single source
type sourceResult struct {
	source  string
	results []SearchResult
	err     error
}

// Close closes all resources used by the search engine
func (e *Engine) Close() error {
	if err := e.cacheManager.Close(); err != nil {
		return fmt.Errorf("failed to close cache manager: %w", err)
	}
	return nil
}

// Type conversion functions to bridge search and cache packages

// searchToCacheRequest converts search.SearchRequest to cache.SearchRequest
func (e *Engine) searchToCacheRequest(req SearchRequest) cache.SearchRequest {
	return cache.SearchRequest{
		Query:    req.Query,
		Title:    req.Title,
		Artist:   req.Artist,
		Album:    req.Album,
		Platform: req.Platform,
		Limit:    req.Limit,
	}
}

// cacheToSearchResults converts cache.SearchResult slice to search.SearchResult slice
func (e *Engine) cacheToSearchResults(cacheResults []cache.SearchResult) []SearchResult {
	results := make([]SearchResult, len(cacheResults))
	for i, cr := range cacheResults {
		results[i] = SearchResult{
			ID:             cr.ID,
			Title:          cr.Title,
			Artists:        cr.Artists,
			Album:          cr.Album,
			Platform:       cr.Platform,
			URL:            cr.URL,
			ImageURL:       cr.ImageURL,
			Popularity:     cr.Popularity,
			DurationMs:     cr.DurationMs,
			ReleaseDate:    cr.ReleaseDate,
			ISRC:           cr.ISRC,
			Explicit:       cr.Explicit,
			Available:      cr.Available,
			Source:         cr.Source,
			RelevanceScore: cr.RelevanceScore,
			CachedAt:       cr.CachedAt,
		}
	}
	return results
}

// searchToCacheResults converts search.SearchResult slice to cache.SearchResult slice
func (e *Engine) searchToCacheResults(searchResults []SearchResult) []cache.SearchResult {
	results := make([]cache.SearchResult, len(searchResults))
	for i, sr := range searchResults {
		results[i] = cache.SearchResult{
			ID:             sr.ID,
			Title:          sr.Title,
			Artists:        sr.Artists,
			Album:          sr.Album,
			Platform:       sr.Platform,
			URL:            sr.URL,
			ImageURL:       sr.ImageURL,
			Popularity:     sr.Popularity,
			DurationMs:     sr.DurationMs,
			ReleaseDate:    sr.ReleaseDate,
			ISRC:           sr.ISRC,
			Explicit:       sr.Explicit,
			Available:      sr.Available,
			Source:         sr.Source,
			RelevanceScore: sr.RelevanceScore,
			CachedAt:       sr.CachedAt,
		}
	}
	return results
}