package services

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"songshare/internal/models"
)

// PlatformService defines the interface for music platform integrations
type PlatformService interface {
	// GetPlatformName returns the name of this platform
	GetPlatformName() string

	// ParseURL extracts track information from a platform URL
	ParseURL(url string) (*TrackInfo, error)

	// GetTrackByID fetches track information using platform-specific ID
	GetTrackByID(ctx context.Context, trackID string) (*TrackInfo, error)

	// SearchTrack searches for tracks on the platform
	SearchTrack(ctx context.Context, query SearchQuery) ([]*TrackInfo, error)

	// GetTrackByISRC finds a track by its ISRC code
	GetTrackByISRC(ctx context.Context, isrc string) (*TrackInfo, error)

	// BuildURL constructs a platform URL from track ID
	BuildURL(trackID string) string

	// Health checks if the platform service is healthy
	Health(ctx context.Context) error
}

// TrackInfo represents track information from a platform
type TrackInfo struct {
	// Platform identifiers
	Platform   string `json:"platform"`
	ExternalID string `json:"external_id"`
	URL        string `json:"url"`

	// Core track metadata
	Title    string   `json:"title"`
	Artists  []string `json:"artists"`
	Album    string   `json:"album,omitempty"`
	ISRC     string   `json:"isrc,omitempty"`
	Duration int      `json:"duration_ms,omitempty"` // Duration in milliseconds

	// Additional metadata
	ReleaseDate string   `json:"release_date,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	Explicit    bool     `json:"explicit,omitempty"`
	Popularity  int      `json:"popularity,omitempty"`
	ImageURL    string   `json:"image_url,omitempty"`

	// Platform-specific data
	Available bool `json:"available"`
}

// SearchQuery represents a search query for tracks
type SearchQuery struct {
	Title  string `json:"title,omitempty"`
	Artist string `json:"artist,omitempty"`
	Album  string `json:"album,omitempty"`
	ISRC   string `json:"isrc,omitempty"`
	Query  string `json:"query,omitempty"` // Free-form search query
	Limit  int    `json:"limit,omitempty"`
}

// ToSong converts TrackInfo to a models.Song
func (t *TrackInfo) ToSong() *models.Song {
	song := models.NewSong(t.Title, joinArtists(t.Artists))
	song.Album = t.Album
	song.ISRC = t.ISRC

	// Add platform link
	song.AddPlatformLink(t.Platform, t.ExternalID, t.URL, 1.0)

	// Set metadata
	song.Metadata.Duration = t.Duration
	song.Metadata.Genre = t.Genres
	song.Metadata.Explicit = t.Explicit
	song.Metadata.Popularity = t.Popularity
	song.Metadata.ImageURL = t.ImageURL

	return song
}

// joinArtists joins multiple artists into a single string
func joinArtists(artists []string) string {
	if len(artists) == 0 {
		return ""
	}
	if len(artists) == 1 {
		return artists[0]
	}

	result := artists[0]
	for i := 1; i < len(artists); i++ {
		result += ", " + artists[i]
	}
	return result
}

// URLPattern represents a URL pattern for parsing platform URLs
type URLPattern struct {
	Regex        *regexp.Regexp
	Platform     string
	TrackIDIndex int      // Index of the track ID capture group
	Description  string   // Human-readable description of the pattern
	Examples     []string // Example URLs this pattern should match
}

// URLPatternRegistry manages URL patterns for all platforms
type URLPatternRegistry struct {
	patterns []URLPattern
	mu       sync.RWMutex
}

// Global pattern registry
var patternRegistry = &URLPatternRegistry{
	patterns: []URLPattern{
		{
			Regex:        regexp.MustCompile(`(?:https?://)?music\.apple\.com/[a-z]{2}/(?:album|song)/(?:[^/]+/)?(\d+)`),
			Platform:     "apple_music",
			TrackIDIndex: 1,
			Description:  "Apple Music track URLs",
			Examples: []string{
				"https://music.apple.com/us/album/bohemian-rhapsody/1440806041?i=1440806053",
				"music.apple.com/us/song/1440806053",
			},
		},
		{
			Regex:        regexp.MustCompile(`(?:https?://)?(?:open\.)?spotify\.com/track/([a-zA-Z0-9]+)`),
			Platform:     "spotify",
			TrackIDIndex: 1,
			Description:  "Spotify track URLs",
			Examples: []string{
				"https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
				"spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			},
		},
		{
			Regex:        regexp.MustCompile(`(?:https?://)?(?:www\.)?(?:listen\.)?tidal\.com/(?:browse/)?track/(\d+)`),
			Platform:     "tidal",
			TrackIDIndex: 1,
			Description:  "Tidal track URLs",
			Examples: []string{
				"https://tidal.com/browse/track/77646168",
				"https://tidal.com/track/77646168",
				"https://listen.tidal.com/track/77646168",
			},
		},
		{
			Regex:        regexp.MustCompile(`(?:https?://)?(?:www\.)?(?:listen\.)?tidal\.com/.*[?&]trackId=(\d+)`),
			Platform:     "tidal",
			TrackIDIndex: 1,
			Description:  "Tidal album URLs with track ID parameter",
			Examples: []string{
				"https://tidal.com/browse/album/77646164?play=true&trackId=77646168",
			},
		},
	},
}

// RegisterURLPattern adds a new URL pattern to the registry
func (r *URLPatternRegistry) RegisterURLPattern(pattern URLPattern) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate the pattern
	if pattern.Regex == nil {
		return fmt.Errorf("regex cannot be nil")
	}
	if pattern.Platform == "" {
		return fmt.Errorf("platform name cannot be empty")
	}
	if pattern.TrackIDIndex < 1 {
		return fmt.Errorf("trackIDIndex must be >= 1 (capture group index)")
	}

	// Check for duplicate platform
	for _, existing := range r.patterns {
		if existing.Platform == pattern.Platform {
			// Replace existing pattern for this platform
			for i, p := range r.patterns {
				if p.Platform == pattern.Platform {
					r.patterns[i] = pattern
					return nil
				}
			}
		}
	}

	// Add new pattern
	r.patterns = append(r.patterns, pattern)
	return nil
}

// GetPatterns returns a copy of all registered patterns
func (r *URLPatternRegistry) GetPatterns() []URLPattern {
	r.mu.RLock()
	defer r.mu.RUnlock()

	patterns := make([]URLPattern, len(r.patterns))
	copy(patterns, r.patterns)
	return patterns
}

// GetSupportedPlatforms returns list of platforms with URL pattern support
func (r *URLPatternRegistry) GetSupportedPlatforms() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	platforms := make([]string, 0, len(r.patterns))
	for _, pattern := range r.patterns {
		platforms = append(platforms, pattern.Platform)
	}
	return platforms
}

// ValidatePattern tests a URL pattern against its example URLs
func (r *URLPatternRegistry) ValidatePattern(pattern URLPattern) error {
	if len(pattern.Examples) == 0 {
		return nil // No examples to validate
	}

	for _, example := range pattern.Examples {
		matches := pattern.Regex.FindStringSubmatch(example)
		if len(matches) <= pattern.TrackIDIndex {
			return fmt.Errorf("pattern failed to match example URL: %s", example)
		}
		if matches[pattern.TrackIDIndex] == "" {
			return fmt.Errorf("pattern matched but captured empty track ID for URL: %s", example)
		}
	}

	return nil
}

// RegisterURLPattern is a convenience function to register a new URL pattern
func RegisterURLPattern(platform string, regexPattern string, trackIDIndex int, description string, examples []string) error {
	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}

	pattern := URLPattern{
		Regex:        regex,
		Platform:     platform,
		TrackIDIndex: trackIDIndex,
		Description:  description,
		Examples:     examples,
	}

	// Validate pattern against examples
	if err := patternRegistry.ValidatePattern(pattern); err != nil {
		return fmt.Errorf("pattern validation failed: %w", err)
	}

	return patternRegistry.RegisterURLPattern(pattern)
}

// ParsePlatformURL attempts to parse a URL and determine which platform it belongs to
func ParsePlatformURL(url string) (platform string, trackID string, err error) {
	patterns := patternRegistry.GetPatterns()

	for _, pattern := range patterns {
		matches := pattern.Regex.FindStringSubmatch(url)
		if len(matches) > pattern.TrackIDIndex {
			return pattern.Platform, matches[pattern.TrackIDIndex], nil
		}
	}

	return "", "", &PlatformError{
		Platform:  "unknown",
		Operation: "parse_url",
		Message:   "unsupported platform URL",
		URL:       url,
	}
}

// GetURLPatterns returns all registered URL patterns (for debugging/documentation)
func GetURLPatterns() []URLPattern {
	return patternRegistry.GetPatterns()
}

// Legacy pattern variables for backward compatibility
var (
	SpotifyURLPattern = URLPattern{
		Regex:        regexp.MustCompile(`(?:https?://)?(?:open\.)?spotify\.com/track/([a-zA-Z0-9]+)`),
		Platform:     "spotify",
		TrackIDIndex: 1,
	}

	AppleMusicURLPattern = URLPattern{
		Regex:        regexp.MustCompile(`(?:https?://)?music\.apple\.com/[a-z]{2}/(?:album|song)/(?:[^/]+/)?(\d+)`),
		Platform:     "apple_music",
		TrackIDIndex: 1,
	}
)

// PlatformError represents an error from a platform service
type PlatformError struct {
	Platform  string
	Operation string
	Message   string
	URL       string
	Err       error
}

func (e *PlatformError) Error() string {
	msg := e.Platform + " " + e.Operation + " failed"
	if e.Message != "" {
		msg += ": " + e.Message
	}
	if e.URL != "" {
		msg += " (URL: " + e.URL + ")"
	}
	if e.Err != nil {
		msg += " - " + e.Err.Error()
	}
	return msg
}

func (e *PlatformError) Unwrap() error {
	return e.Err
}
