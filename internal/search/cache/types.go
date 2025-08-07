package cache

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SearchRequest represents a search query (copied to avoid import cycle)
type SearchRequest struct {
	Query    string `json:"query"`    // Free-form search query
	Title    string `json:"title"`    // Specific title search
	Artist   string `json:"artist"`   // Specific artist search  
	Album    string `json:"album"`    // Specific album search
	Platform string `json:"platform"` // Filter: "spotify", "apple_music", "tidal", "" (all)
	Limit    int    `json:"limit"`    // Results per platform (default: 10, max: 50)
}

// SearchResult represents a single search result (copied to avoid import cycle)
type SearchResult struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Artists        []string  `json:"artists"`
	Album          string    `json:"album"`
	Platform       string    `json:"platform"`
	URL            string    `json:"url"`
	ImageURL       string    `json:"image_url"`
	Popularity     int       `json:"popularity"`     // 0-100
	DurationMs     int       `json:"duration_ms"`
	ReleaseDate    string    `json:"release_date"`
	ISRC           string    `json:"isrc"`
	Explicit       bool      `json:"explicit"`
	Available      bool      `json:"available"`
	Source         string    `json:"source"`         // "local" or "platform"
	RelevanceScore float64   `json:"relevance_score"`
	CachedAt       time.Time `json:"cached_at"`
}

// CachedSearchResult represents a persistent search cache entry in MongoDB
type CachedSearchResult struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	QueryHash   string            `bson:"query_hash"`     // Hash of search parameters
	Query       SearchRequest     `bson:"query"`          // Original query parameters
	Results     []SearchResult    `bson:"results"`        // Cached search results
	CreatedAt   time.Time         `bson:"created_at"`
	UpdatedAt   time.Time         `bson:"updated_at"`
	ExpiresAt   time.Time         `bson:"expires_at"`     // TTL index for auto-cleanup
	HitCount    int               `bson:"hit_count"`      // Usage analytics
	Platforms   []string          `bson:"platforms"`      // Which platforms were searched
	ResultCount int               `bson:"result_count"`   // Number of results
}

// CacheConfig holds cache TTL configurations
type CacheConfig struct {
	MemoryTTL    time.Duration
	RedisTTL     time.Duration
	MongoTTL     time.Duration
	NegativeTTL  time.Duration
}

// DefaultCacheConfig returns sensible default cache TTL values
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MemoryTTL:   5 * time.Minute,      // Hot data in memory
		RedisTTL:    1 * time.Hour,        // API responses in Redis
		MongoTTL:    24 * time.Hour,       // Search results in MongoDB
		NegativeTTL: 1 * time.Hour,        // Empty results
	}
}

// GenerateQueryHash creates a consistent hash for search query caching
func (sr *SearchRequest) GenerateQueryHash() string {
	// Simple hash generation - could be enhanced with actual hashing
	return sr.Query + "|" + sr.Title + "|" + sr.Artist + "|" + sr.Album + "|" + sr.Platform
}

// IsEmpty returns true if the search request has no search parameters
func (sr *SearchRequest) IsEmpty() bool {
	return sr.Query == "" && sr.Title == "" && sr.Artist == "" && sr.Album == ""
}

// GetEffectiveLimit returns the limit with defaults and bounds applied
func (sr *SearchRequest) GetEffectiveLimit() int {
	if sr.Limit <= 0 {
		return 10 // Default limit
	}
	if sr.Limit > 50 {
		return 50 // Maximum limit
	}
	return sr.Limit
}

