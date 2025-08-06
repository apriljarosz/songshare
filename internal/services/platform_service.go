package services

import (
	"context"
	"regexp"

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
	Regex     *regexp.Regexp
	Platform  string
	TrackIDIndex int // Index of the track ID capture group
}

// Common URL patterns for different platforms
var (
	SpotifyURLPattern = URLPattern{
		Regex:        regexp.MustCompile(`(?:https?://)?(?:open\.)?spotify\.com/track/([a-zA-Z0-9]+)`),
		Platform:     "spotify",
		TrackIDIndex: 1,
	}

	AppleMusicURLPattern = URLPattern{
		Regex:        regexp.MustCompile(`(?:https?://)?music\.apple\.com/[a-z]{2}/(?:album|song)/[^/]+/(\d+)`),
		Platform:     "apple_music",
		TrackIDIndex: 1,
	}
)

// ParsePlatformURL attempts to parse a URL and determine which platform it belongs to
func ParsePlatformURL(url string) (platform string, trackID string, err error) {
	patterns := []URLPattern{SpotifyURLPattern, AppleMusicURLPattern}

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