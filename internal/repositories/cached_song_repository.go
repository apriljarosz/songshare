package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"songshare/internal/cache"
	"songshare/internal/models"
)

// cachedSongRepository wraps a SongRepository with caching
type cachedSongRepository struct {
	repository SongRepository
	cache      cache.Cache
}

// NewCachedSongRepository creates a new cached song repository
func NewCachedSongRepository(repository SongRepository, cache cache.Cache) SongRepository {
	return &cachedSongRepository{
		repository: repository,
		cache:      cache,
	}
}

// Cache key generators
func songIDKey(id string) string                 { return "song:id:" + id }
func songISRCKey(isrc string) string             { return "song:isrc:" + isrc }
func songPlatformKey(platform, id string) string { return "song:platform:" + platform + ":" + id }
func songSearchKey(query string) string          { return "song:search:" + query }

// Cache TTL constants
const (
	songCacheTTL     = 1 * time.Hour
	searchCacheTTL   = 5 * time.Minute // Extended with smart invalidation
	negativeCacheTTL = 5 * time.Minute // For null results
)

// Save invalidates relevant cache entries and saves to repository
func (r *cachedSongRepository) Save(ctx context.Context, song *models.Song) error {
	err := r.repository.Save(ctx, song)
	if err != nil {
		return err
	}

	// Invalidate cache entries
	r.invalidateSongCache(ctx, song)

	return nil
}

// Update invalidates cache and updates in repository
func (r *cachedSongRepository) Update(ctx context.Context, song *models.Song) error {
	err := r.repository.Update(ctx, song)
	if err != nil {
		return err
	}

	// Invalidate cache entries
	r.invalidateSongCache(ctx, song)

	return nil
}

// FindByID checks cache first, then repository
func (r *cachedSongRepository) FindByID(ctx context.Context, id string) (*models.Song, error) {
	cacheKey := songIDKey(id)

	// Try cache first
	if cached, err := r.getFromCache(ctx, cacheKey); err == nil && cached != nil {
		return cached, nil
	}

	// Cache miss, query repository
	song, err := r.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cache the result (even if nil)
	r.cacheResult(ctx, cacheKey, song)

	return song, nil
}

// FindByISRC checks cache first, then repository
func (r *cachedSongRepository) FindByISRC(ctx context.Context, isrc string) (*models.Song, error) {
	cacheKey := songISRCKey(isrc)

	// Try cache first
	if cached, err := r.getFromCache(ctx, cacheKey); err == nil && cached != nil {
		return cached, nil
	}

	// Cache miss, query repository
	song, err := r.repository.FindByISRC(ctx, isrc)
	if err != nil {
		return nil, err
	}

	// Cache the result
	r.cacheResult(ctx, cacheKey, song)

	return song, nil
}

// FindByISRCBatch checks cache for each ISRC, fetches missing ones from repository
func (r *cachedSongRepository) FindByISRCBatch(ctx context.Context, isrcs []string) (map[string]*models.Song, error) {
	if len(isrcs) == 0 {
		return make(map[string]*models.Song), nil
	}

	result := make(map[string]*models.Song)
	var missingISRCs []string

	// Check cache for each ISRC
	for _, isrc := range isrcs {
		if isrc == "" {
			continue
		}
		
		cacheKey := songISRCKey(isrc)
		if cached, err := r.getFromCache(ctx, cacheKey); err == nil && cached != nil {
			result[isrc] = cached
		} else {
			missingISRCs = append(missingISRCs, isrc)
		}
	}

	// If all found in cache, return early
	if len(missingISRCs) == 0 {
		return result, nil
	}

	// Fetch missing ISRCs from repository
	dbResults, err := r.repository.FindByISRCBatch(ctx, missingISRCs)
	if err != nil {
		return nil, err
	}

	// Cache the results and add to return map
	for isrc, song := range dbResults {
		cacheKey := songISRCKey(isrc)
		r.cacheResult(ctx, cacheKey, song)
		result[isrc] = song
	}

	return result, nil
}

// FindByTitleArtist - not cached due to fuzzy nature
func (r *cachedSongRepository) FindByTitleArtist(ctx context.Context, title, artist string) ([]*models.Song, error) {
	return r.repository.FindByTitleArtist(ctx, title, artist)
}

// FindByPlatformID checks cache first, then repository
func (r *cachedSongRepository) FindByPlatformID(ctx context.Context, platform, externalID string) (*models.Song, error) {
	cacheKey := songPlatformKey(platform, externalID)

	// Try cache first
	if cached, err := r.getFromCache(ctx, cacheKey); err == nil && cached != nil {
		return cached, nil
	}

	// Cache miss, query repository
	song, err := r.repository.FindByPlatformID(ctx, platform, externalID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	r.cacheResult(ctx, cacheKey, song)

	return song, nil
}

// Search caches results for a short duration
func (r *cachedSongRepository) Search(ctx context.Context, query string, limit int) ([]*models.Song, error) {
	cacheKey := songSearchKey(fmt.Sprintf("%s:limit:%d", query, limit))

	// Try cache first
	if cached, err := r.getSearchFromCache(ctx, cacheKey); err == nil && cached != nil {
		return cached, nil
	}

	// Cache miss, query repository
	songs, err := r.repository.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	// Cache the search results
	r.cacheSearchResult(ctx, cacheKey, songs)

	return songs, nil
}

// FindSimilar - not cached due to complexity and changing nature
func (r *cachedSongRepository) FindSimilar(ctx context.Context, song *models.Song, limit int) ([]*models.Song, error) {
	return r.repository.FindSimilar(ctx, song, limit)
}

// FindMany - partially cached (individual songs may be cached)
func (r *cachedSongRepository) FindMany(ctx context.Context, ids []string) ([]*models.Song, error) {
	var songs []*models.Song
	var uncachedIDs []string

	// Check cache for each ID
	for _, id := range ids {
		cacheKey := songIDKey(id)
		if cached, err := r.getFromCache(ctx, cacheKey); err == nil && cached != nil {
			songs = append(songs, cached)
		} else {
			uncachedIDs = append(uncachedIDs, id)
		}
	}

	// Fetch uncached songs from repository
	if len(uncachedIDs) > 0 {
		uncachedSongs, err := r.repository.FindMany(ctx, uncachedIDs)
		if err != nil {
			return nil, err
		}

		// Cache individual results
		for _, song := range uncachedSongs {
			cacheKey := songIDKey(song.ID.Hex())
			r.cacheResult(ctx, cacheKey, song)
			songs = append(songs, song)
		}
	}

	return songs, nil
}

// SaveMany invalidates cache and saves to repository
func (r *cachedSongRepository) SaveMany(ctx context.Context, songs []*models.Song) error {
	err := r.repository.SaveMany(ctx, songs)
	if err != nil {
		return err
	}

	// Invalidate cache for all songs
	for _, song := range songs {
		r.invalidateSongCache(ctx, song)
	}

	return nil
}

// DeleteByID invalidates cache and deletes from repository
func (r *cachedSongRepository) DeleteByID(ctx context.Context, id string) error {
	// First get the song to invalidate related cache entries
	song, _ := r.repository.FindByID(ctx, id)

	err := r.repository.DeleteByID(ctx, id)
	if err != nil {
		return err
	}

	// Invalidate cache entries
	if song != nil {
		r.invalidateSongCache(ctx, song)
	}
	// Always invalidate ID-based cache
	r.cache.Delete(ctx, songIDKey(id))

	return nil
}

// Count - not cached as it changes frequently
func (r *cachedSongRepository) Count(ctx context.Context) (int64, error) {
	return r.repository.Count(ctx)
}

// FindByIDPrefix checks cache first, then repository
func (r *cachedSongRepository) FindByIDPrefix(ctx context.Context, prefix string) (*models.Song, error) {
	cacheKey := "song:prefix:" + prefix

	// Try cache first
	if cached, err := r.getFromCache(ctx, cacheKey); err == nil && cached != nil {
		return cached, nil
	}

	// Cache miss, query repository
	song, err := r.repository.FindByIDPrefix(ctx, prefix)
	if err != nil {
		return nil, err
	}

	// Cache the result
	r.cacheResult(ctx, cacheKey, song)

	return song, nil
}

// Helper methods for cache operations

// getFromCache retrieves a song from cache
func (r *cachedSongRepository) getFromCache(ctx context.Context, key string) (*models.Song, error) {
	data, err := r.cache.Get(ctx, key)
	if err != nil || data == nil {
		return nil, err
	}

	// Handle negative cache (null result marker)
	if string(data) == "null" {
		return nil, nil
	}

	var song models.Song
	if err := json.Unmarshal(data, &song); err != nil {
		slog.Error("Failed to unmarshal song from cache", "key", key, "error", err)
		// Delete corrupted cache entry
		r.cache.Delete(ctx, key)
		return nil, err
	}

	return &song, nil
}

// getSearchFromCache retrieves search results from cache
func (r *cachedSongRepository) getSearchFromCache(ctx context.Context, key string) ([]*models.Song, error) {
	data, err := r.cache.Get(ctx, key)
	if err != nil || data == nil {
		return nil, err
	}

	var songs []*models.Song
	if err := json.Unmarshal(data, &songs); err != nil {
		slog.Error("Failed to unmarshal search results from cache", "key", key, "error", err)
		r.cache.Delete(ctx, key)
		return nil, err
	}

	return songs, nil
}

// cacheResult caches a single song result
func (r *cachedSongRepository) cacheResult(ctx context.Context, key string, song *models.Song) {
	var data []byte
	var err error

	if song == nil {
		// Cache negative result
		data = []byte("null")
	} else {
		data, err = json.Marshal(song)
		if err != nil {
			slog.Error("Failed to marshal song for cache", "key", key, "error", err)
			return
		}
	}

	ttl := songCacheTTL
	if song == nil {
		ttl = negativeCacheTTL
	}

	if err := r.cache.Set(ctx, key, data, ttl); err != nil {
		slog.Error("Failed to cache song", "key", key, "error", err)
	}
}

// cacheSearchResult caches search results
func (r *cachedSongRepository) cacheSearchResult(ctx context.Context, key string, songs []*models.Song) {
	data, err := json.Marshal(songs)
	if err != nil {
		slog.Error("Failed to marshal search results for cache", "key", key, "error", err)
		return
	}

	if err := r.cache.Set(ctx, key, data, searchCacheTTL); err != nil {
		slog.Error("Failed to cache search results", "key", key, "error", err)
	}
}

// invalidateSongCache removes all cache entries for a song
func (r *cachedSongRepository) invalidateSongCache(ctx context.Context, song *models.Song) {
	// Delete primary cache keys
	if !song.ID.IsZero() {
		r.cache.Delete(ctx, songIDKey(song.ID.Hex()))
	}

	if song.ISRC != "" {
		r.cache.Delete(ctx, songISRCKey(song.ISRC))
	}

	// Delete platform-specific cache keys
	for _, link := range song.PlatformLinks {
		r.cache.Delete(ctx, songPlatformKey(link.Platform, link.ExternalID))
	}

	// Note: We don't invalidate search cache as it would be too expensive
	// Search cache has a shorter TTL to handle this
}

// InvalidateSearchCache invalidates search cache for a specific query
func (r *cachedSongRepository) InvalidateSearchCache(query string) {
	ctx := context.Background()
	
	// Delete the base query key (for backwards compatibility)
	baseKey := songSearchKey(query)
	r.cache.Delete(ctx, baseKey)
	
	// Also delete common query variations with different limits
	// Based on UI options: 10 (default), 25, 50, plus some common API limits
	commonLimits := []int{10, 25, 50, 100}
	for _, limit := range commonLimits {
		key := songSearchKey(fmt.Sprintf("%s:limit:%d", query, limit))
		r.cache.Delete(ctx, key)
	}
	
	slog.Debug("Invalidated search cache for query with all limit variations", "query", query)
}
