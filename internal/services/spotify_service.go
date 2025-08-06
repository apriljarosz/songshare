package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"golang.org/x/oauth2/clientcredentials"
)

// spotifyService implements PlatformService for Spotify
type spotifyService struct {
	client       *resty.Client
	clientID     string
	clientSecret string
	tokenSource  *clientcredentials.Config
	accessToken  string
	tokenExpiry  time.Time
	mu           sync.RWMutex
}

// Spotify API endpoints
const (
	spotifyTokenURL  = "https://accounts.spotify.com/api/token"
	spotifyAPIURL    = "https://api.spotify.com/v1"
)

// NewSpotifyService creates a new Spotify service
func NewSpotifyService(clientID, clientSecret string) PlatformService {
	tokenSource := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     spotifyTokenURL,
	}

	client := resty.New().
		SetTimeout(10*time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1*time.Second).
		SetRetryMaxWaitTime(5*time.Second)

	return &spotifyService{
		client:       client,
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenSource:  tokenSource,
	}
}

// GetPlatformName returns the platform name
func (s *spotifyService) GetPlatformName() string {
	return "spotify"
}

// ParseURL extracts track ID from Spotify URL
func (s *spotifyService) ParseURL(url string) (*TrackInfo, error) {
	matches := SpotifyURLPattern.Regex.FindStringSubmatch(url)
	if len(matches) <= SpotifyURLPattern.TrackIDIndex {
		return nil, &PlatformError{
			Platform:  "spotify",
			Operation: "parse_url",
			Message:   "invalid Spotify URL format",
			URL:       url,
		}
	}

	trackID := matches[SpotifyURLPattern.TrackIDIndex]
	
	// Basic track info without API call
	return &TrackInfo{
		Platform:   "spotify",
		ExternalID: trackID,
		URL:        s.BuildURL(trackID),
		Available:  true, // Assume available until proven otherwise
	}, nil
}

// GetTrackByID fetches track details from Spotify API
func (s *spotifyService) GetTrackByID(ctx context.Context, trackID string) (*TrackInfo, error) {
	if err := s.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	s.mu.RLock()
	token := s.accessToken
	s.mu.RUnlock()

	var spotifyTrack SpotifyTrack
	resp, err := s.client.R().
		SetContext(ctx).
		SetAuthToken(token).
		SetResult(&spotifyTrack).
		Get(fmt.Sprintf("%s/tracks/%s", spotifyAPIURL, trackID))

	if err != nil {
		return nil, &PlatformError{
			Platform:  "spotify",
			Operation: "get_track",
			Message:   "request failed",
			Err:       err,
		}
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, &PlatformError{
			Platform:  "spotify",
			Operation: "get_track",
			Message:   "track not found",
		}
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, &PlatformError{
			Platform:  "spotify",
			Operation: "get_track",
			Message:   fmt.Sprintf("API returned status %d", resp.StatusCode()),
		}
	}

	return s.convertSpotifyTrack(&spotifyTrack), nil
}

// SearchTrack searches for tracks on Spotify
func (s *spotifyService) SearchTrack(ctx context.Context, query SearchQuery) ([]*TrackInfo, error) {
	if err := s.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	searchQuery := s.buildSearchQuery(query)
	limit := query.Limit
	if limit == 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50 // Spotify API limit
	}

	s.mu.RLock()
	token := s.accessToken
	s.mu.RUnlock()

	var searchResult SpotifySearchResult
	resp, err := s.client.R().
		SetContext(ctx).
		SetAuthToken(token).
		SetQueryParams(map[string]string{
			"q":     searchQuery,
			"type":  "track",
			"limit": fmt.Sprintf("%d", limit),
		}).
		SetResult(&searchResult).
		Get(fmt.Sprintf("%s/search", spotifyAPIURL))

	if err != nil {
		return nil, &PlatformError{
			Platform:  "spotify",
			Operation: "search",
			Message:   "request failed",
			Err:       err,
		}
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, &PlatformError{
			Platform:  "spotify",
			Operation: "search",
			Message:   fmt.Sprintf("API returned status %d", resp.StatusCode()),
		}
	}

	tracks := make([]*TrackInfo, 0, len(searchResult.Tracks.Items))
	for _, track := range searchResult.Tracks.Items {
		tracks = append(tracks, s.convertSpotifyTrack(&track))
	}

	return tracks, nil
}

// GetTrackByISRC finds track by ISRC code
func (s *spotifyService) GetTrackByISRC(ctx context.Context, isrc string) (*TrackInfo, error) {
	query := SearchQuery{
		ISRC:  isrc,
		Limit: 1,
	}
	
	tracks, err := s.SearchTrack(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(tracks) == 0 {
		return nil, &PlatformError{
			Platform:  "spotify",
			Operation: "get_by_isrc",
			Message:   "no tracks found with ISRC " + isrc,
		}
	}

	return tracks[0], nil
}

// BuildURL constructs Spotify URL from track ID
func (s *spotifyService) BuildURL(trackID string) string {
	return fmt.Sprintf("https://open.spotify.com/track/%s", trackID)
}

// Health checks Spotify API health
func (s *spotifyService) Health(ctx context.Context) error {
	return s.ensureValidToken(ctx)
}

// ensureValidToken ensures we have a valid access token
func (s *spotifyService) ensureValidToken(ctx context.Context) error {
	s.mu.RLock()
	if s.accessToken != "" && time.Now().Before(s.tokenExpiry) {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.accessToken != "" && time.Now().Before(s.tokenExpiry) {
		return nil
	}

	// Get new token
	token, err := s.tokenSource.Token(ctx)
	if err != nil {
		return &PlatformError{
			Platform:  "spotify",
			Operation: "auth",
			Message:   "failed to get access token",
			Err:       err,
		}
	}

	s.accessToken = token.AccessToken
	s.tokenExpiry = token.Expiry
	
	slog.Info("Spotify access token refreshed", "expires_at", token.Expiry)
	
	return nil
}

// buildSearchQuery constructs a search query string for Spotify
func (s *spotifyService) buildSearchQuery(query SearchQuery) string {
	if query.ISRC != "" {
		return fmt.Sprintf("isrc:%s", query.ISRC)
	}

	if query.Query != "" {
		return query.Query
	}

	var parts []string
	if query.Title != "" {
		parts = append(parts, fmt.Sprintf("track:\"%s\"", query.Title))
	}
	if query.Artist != "" {
		parts = append(parts, fmt.Sprintf("artist:\"%s\"", query.Artist))
	}
	if query.Album != "" {
		parts = append(parts, fmt.Sprintf("album:\"%s\"", query.Album))
	}

	if len(parts) == 0 {
		return "*" // Return all tracks if no search criteria
	}

	return strings.Join(parts, " ")
}

// convertSpotifyTrack converts Spotify API response to TrackInfo
func (s *spotifyService) convertSpotifyTrack(track *SpotifyTrack) *TrackInfo {
	artists := make([]string, len(track.Artists))
	for i, artist := range track.Artists {
		artists[i] = artist.Name
	}

	// Get image URL (prefer medium size)
	var imageURL string
	if len(track.Album.Images) > 0 {
		imageURL = track.Album.Images[0].URL
		for _, img := range track.Album.Images {
			if img.Width >= 300 && img.Width <= 640 {
				imageURL = img.URL
				break
			}
		}
	}

	return &TrackInfo{
		Platform:    "spotify",
		ExternalID:  track.ID,
		URL:         s.BuildURL(track.ID),
		Title:       track.Name,
		Artists:     artists,
		Album:       track.Album.Name,
		ISRC:        track.ExternalIDs.ISRC,
		Duration:    track.DurationMs,
		ReleaseDate: track.Album.ReleaseDate,
		Explicit:    track.Explicit,
		Popularity:  track.Popularity,
		ImageURL:    imageURL,
		Available:   true,
	}
}

// Spotify API response structures
type SpotifyTrack struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Artists     []SpotifyArtist `json:"artists"`
	Album       SpotifyAlbum    `json:"album"`
	DurationMs  int             `json:"duration_ms"`
	Explicit    bool            `json:"explicit"`
	Popularity  int             `json:"popularity"`
	ExternalIDs SpotifyExternalIDs `json:"external_ids"`
}

type SpotifyArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SpotifyAlbum struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	ReleaseDate string         `json:"release_date"`
	Images      []SpotifyImage `json:"images"`
}

type SpotifyImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type SpotifyExternalIDs struct {
	ISRC string `json:"isrc"`
}

type SpotifySearchResult struct {
	Tracks SpotifyTracksPaging `json:"tracks"`
}

type SpotifyTracksPaging struct {
	Items []SpotifyTrack `json:"items"`
	Total int            `json:"total"`
}