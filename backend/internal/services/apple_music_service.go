package services

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"songshare/internal/cache"

	"github.com/go-resty/resty/v2"
	"github.com/golang-jwt/jwt/v5"
)

// appleMusicService implements PlatformService for Apple Music
type appleMusicService struct {
	client      *resty.Client
	keyID       string
	teamID      string
	keyFile     string
	privateKey  *ecdsa.PrivateKey
	jwtToken    string
	tokenExpiry time.Time
	cache       cache.Cache
	mu          sync.RWMutex
}

// Apple Music API endpoints
const (
	appleMusicAPIURL = "https://api.music.apple.com/v1"
)

// Cache TTL constants for Apple Music API responses
const (
	appleMusicTrackCacheTTL  = 4 * time.Hour  // Individual track lookups
	appleMusicSearchCacheTTL = 2 * time.Hour  // Search results
	appleMusicISRCCacheTTL   = 24 * time.Hour // ISRC-based lookups (very stable)
)

// NewAppleMusicService creates a new Apple Music service
func NewAppleMusicService(keyID, teamID, keyFile string, cache cache.Cache) PlatformService {
	client := resty.New().
		SetTimeout(10 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second)

	service := &appleMusicService{
		client:  client,
		keyID:   keyID,
		teamID:  teamID,
		keyFile: keyFile,
		cache:   cache,
	}

	// Load private key
	if err := service.loadPrivateKey(); err != nil {
		slog.Error("Failed to load Apple Music private key", "error", err)
	}

	return service
}

// GetPlatformName returns the platform name
func (s *appleMusicService) GetPlatformName() string {
	return "apple_music"
}

// ParseURL extracts track ID from Apple Music URL
func (s *appleMusicService) ParseURL(url string) (*TrackInfo, error) {
	matches := AppleMusicURLPattern.Regex.FindStringSubmatch(url)
	if len(matches) <= AppleMusicURLPattern.TrackIDIndex {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "parse_url",
			Message:   "invalid Apple Music URL format",
			URL:       url,
		}
	}

	trackID := matches[AppleMusicURLPattern.TrackIDIndex]

	return &TrackInfo{
		Platform:   "apple_music",
		ExternalID: trackID,
		URL:        url, // Use original URL
		Available:  true,
	}, nil
}

// GetTrackByID fetches track details from Apple Music API
func (s *appleMusicService) GetTrackByID(ctx context.Context, trackID string) (*TrackInfo, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("api:apple_music:track:%s", trackID)
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
		var trackInfo TrackInfo
		if err := json.Unmarshal(cached, &trackInfo); err == nil {
			return &trackInfo, nil
		}
	}

	if err := s.ensureValidToken(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	token := s.jwtToken
	s.mu.RUnlock()

	var appleMusicTrack AppleMusicTrack
	resp, err := s.client.R().
		SetContext(ctx).
		SetAuthToken(token).
		SetResult(&appleMusicTrack).
		Get(fmt.Sprintf("%s/catalog/us/songs/%s", appleMusicAPIURL, trackID))

	if err != nil {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "get_track",
			Message:   "request failed",
			Err:       err,
		}
	}

	if resp.StatusCode() == 404 {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "get_track",
			Message:   "track not found",
		}
	}

	if resp.StatusCode() != 200 {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "get_track",
			Message:   fmt.Sprintf("API returned status %d", resp.StatusCode()),
		}
	}

	if len(appleMusicTrack.Data) == 0 {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "get_track",
			Message:   "no track data returned",
		}
	}

	trackInfo := s.convertAppleMusicTrack(&appleMusicTrack.Data[0], nil) // Pass nil for albums as it's not fetched here

	// Cache the result
	if data, err := json.Marshal(trackInfo); err == nil {
		if err := s.cache.Set(ctx, cacheKey, data, appleMusicTrackCacheTTL); err != nil {
			slog.Error("Failed to cache Apple Music track", "trackID", trackID, "error", err)
		}
	}

	return trackInfo, nil
}

// SearchTrack searches for tracks on Apple Music
func (s *appleMusicService) SearchTrack(ctx context.Context, query SearchQuery) ([]*TrackInfo, error) {
	searchQuery := s.buildSearchQuery(query)
	limit := query.Limit
	if limit == 0 {
		limit = 10
	}
	if limit > 25 {
		limit = 25 // Apple Music API limit
	}

	// Check cache first
	cacheKey := fmt.Sprintf("api:apple_music:search:%s:limit:%d", searchQuery, limit)
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
		var tracks []*TrackInfo
		if err := json.Unmarshal(cached, &tracks); err == nil {
			return tracks, nil
		}
	}

	if err := s.ensureValidToken(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	token := s.jwtToken
	s.mu.RUnlock()

	var searchResult AppleMusicSearchResult
	resp, err := s.client.R().
		SetContext(ctx).
		SetAuthToken(token).
		SetQueryParams(map[string]string{
			"term":    searchQuery,
			"types":   "songs",
			"limit":   fmt.Sprintf("%d", limit),
			"include": "albums", // Include album relationships to get isCompilation field
		}).
		SetResult(&searchResult).
		Get(fmt.Sprintf("%s/catalog/us/search", appleMusicAPIURL))

	if err != nil {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "search",
			Message:   "request failed",
			Err:       err,
		}
	}

	if resp.StatusCode() != 200 {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "search",
			Message:   fmt.Sprintf("API returned status %d", resp.StatusCode()),
		}
	}

	tracks := make([]*TrackInfo, 0, len(searchResult.Results.Songs.Data))
	for _, track := range searchResult.Results.Songs.Data {
		tracks = append(tracks, s.convertAppleMusicTrack(&track, searchResult.Included))
	}

	// Cache the results
	if data, err := json.Marshal(tracks); err == nil {
		// Use longer TTL for ISRC searches since they're more stable
		cacheTTL := appleMusicSearchCacheTTL
		if query.ISRC != "" {
			cacheTTL = appleMusicISRCCacheTTL
		}

		if err := s.cache.Set(ctx, cacheKey, data, cacheTTL); err != nil {
			slog.Error("Failed to cache Apple Music search results", "query", searchQuery, "error", err)
		}
	}

	return tracks, nil
}

// GetTrackByISRC finds track by ISRC code using the catalog endpoint
func (s *appleMusicService) GetTrackByISRC(ctx context.Context, isrc string) (*TrackInfo, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("api:apple_music:isrc:%s", isrc)
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
		var track TrackInfo
		if err := json.Unmarshal(cached, &track); err == nil {
			return &track, nil
		}
	}

	if err := s.ensureValidToken(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	token := s.jwtToken
	s.mu.RUnlock()

	var searchResult AppleMusicTrack
	resp, err := s.client.R().
		SetContext(ctx).
		SetAuthToken(token).
		SetQueryParams(map[string]string{
			"filter[isrc]": isrc,
		}).
		SetResult(&searchResult).
		Get(fmt.Sprintf("%s/catalog/us/songs", appleMusicAPIURL))

	if err != nil {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "get_by_isrc",
			Message:   "request failed",
			Err:       err,
		}
	}

	if resp.StatusCode() != 200 {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "get_by_isrc",
			Message:   fmt.Sprintf("API returned status %d", resp.StatusCode()),
		}
	}

	if len(searchResult.Data) == 0 {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "get_by_isrc",
			Message:   "no tracks found with ISRC " + isrc,
		}
	}

	// Convert the first result
	track := s.convertAppleMusicTrack(&searchResult.Data[0], nil) // Pass nil for albums as it's not fetched here

	// Cache the result
	if data, err := json.Marshal(track); err == nil {
		if err := s.cache.Set(ctx, cacheKey, data, appleMusicISRCCacheTTL); err != nil {
			slog.Error("Failed to cache Apple Music ISRC result", "isrc", isrc, "error", err)
		}
	}

	return track, nil
}

// GetAlbumByID fetches album details from Apple Music API
func (s *appleMusicService) GetAlbumByID(ctx context.Context, albumID string) (*AppleMusicAlbum, error) {
	if err := s.ensureValidToken(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	token := s.jwtToken
	s.mu.RUnlock()

	var albumResponse struct {
		Data []AppleMusicAlbum `json:"data"`
	}

	resp, err := s.client.R().
		SetContext(ctx).
		SetAuthToken(token).
		SetResult(&albumResponse).
		Get(fmt.Sprintf("%s/catalog/us/albums/%s", appleMusicAPIURL, albumID))

	if err != nil {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "get_album",
			Message:   "request failed",
			Err:       err,
		}
	}

	if resp.StatusCode() != 200 {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "get_album",
			Message:   fmt.Sprintf("API returned status %d", resp.StatusCode()),
		}
	}

	if len(albumResponse.Data) == 0 {
		return nil, &PlatformError{
			Platform:  "apple_music",
			Operation: "get_album",
			Message:   "album not found",
		}
	}

	return &albumResponse.Data[0], nil
}

// BuildURL constructs Apple Music URL from track ID
func (s *appleMusicService) BuildURL(trackID string) string {
	return fmt.Sprintf("https://music.apple.com/us/song/%s", trackID)
}

// Health checks Apple Music API health
func (s *appleMusicService) Health(ctx context.Context) error {
	if s.keyID == "" || s.teamID == "" {
		return &PlatformError{
			Platform:  "apple_music",
			Operation: "health",
			Message:   "missing Apple Music API credentials",
		}
	}

	if s.privateKey == nil {
		return &PlatformError{
			Platform:  "apple_music",
			Operation: "health",
			Message:   "private key not loaded",
		}
	}

	// Test token generation
	return s.ensureValidToken()
}

// loadPrivateKey loads the Apple Music private key from file
func (s *appleMusicService) loadPrivateKey() error {
	keyData, err := os.ReadFile(s.keyFile)
	if err != nil {
		return fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block from private key")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	ecdsaKey, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return fmt.Errorf("private key is not ECDSA")
	}

	s.privateKey = ecdsaKey
	return nil
}

// ensureValidToken ensures we have a valid JWT token
func (s *appleMusicService) ensureValidToken() error {
	s.mu.RLock()
	if s.jwtToken != "" && time.Now().Before(s.tokenExpiry) {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.jwtToken != "" && time.Now().Before(s.tokenExpiry) {
		return nil
	}

	if s.privateKey == nil {
		return &PlatformError{
			Platform:  "apple_music",
			Operation: "auth",
			Message:   "private key not loaded",
		}
	}

	// Generate new JWT token
	token, err := s.generateJWT()
	if err != nil {
		return &PlatformError{
			Platform:  "apple_music",
			Operation: "auth",
			Message:   "failed to generate JWT token",
			Err:       err,
		}
	}

	s.jwtToken = token
	s.tokenExpiry = time.Now().Add(55 * time.Minute) // JWT tokens last 60 minutes, refresh at 55

	slog.Info("Apple Music JWT token refreshed", "expires_at", s.tokenExpiry)

	return nil
}

// generateJWT creates a JWT token for Apple Music API authentication
func (s *appleMusicService) generateJWT() (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": s.teamID,
		"iat": now.Unix(),
		"exp": now.Add(60 * time.Minute).Unix(), // 60 minutes expiration
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = s.keyID

	return token.SignedString(s.privateKey)
}

// buildSearchQuery constructs a search query string for Apple Music
func (s *appleMusicService) buildSearchQuery(query SearchQuery) string {
	if query.ISRC != "" {
		return fmt.Sprintf("isrc:%s", query.ISRC)
	}

	if query.Query != "" {
		return query.Query
	}

	var parts []string
	if query.Title != "" {
		parts = append(parts, query.Title)
	}
	if query.Artist != "" {
		parts = append(parts, query.Artist)
	}
	if query.Album != "" {
		parts = append(parts, query.Album)
	}

	if len(parts) == 0 {
		return "music" // Default search term
	}

	return strings.Join(parts, " ") // Apple Music search handles multiple terms well in a single string
}

// convertAppleMusicTrack converts Apple Music API response to TrackInfo
func (s *appleMusicService) convertAppleMusicTrack(track *AppleMusicSong, albums []AppleMusicAlbum) *TrackInfo {
	artists := []string{}
	if track.Attributes.ArtistName != "" {
		artists = append(artists, track.Attributes.ArtistName)
	}

	// Get image URL
	var imageURL string
	if track.Attributes.Artwork.URL != "" {
		// Apple Music artwork URLs use template format
		imageURL = strings.ReplaceAll(track.Attributes.Artwork.URL, "{w}", "400")
		imageURL = strings.ReplaceAll(imageURL, "{h}", "400")
	}

	// Check if this track is from a compilation album
	isCompilation := false
	if track.Relationships != nil && track.Relationships.Albums != nil && len(track.Relationships.Albums.Data) > 0 {
		albumID := track.Relationships.Albums.Data[0].ID
		for _, album := range albums {
			if album.ID == albumID {
				isCompilation = album.Attributes.IsCompilation
				break
			}
		}
	}

	return &TrackInfo{
		Platform:    "apple_music",
		ExternalID:  track.ID,
		URL:         s.BuildURL(track.ID),
		Title:       track.Attributes.Name,
		Artists:     artists,
		Album:       track.Attributes.AlbumName,
		ISRC:        track.Attributes.ISRC,
		Duration:    track.Attributes.DurationInMillis,
		ReleaseDate: track.Attributes.ReleaseDate,
		Explicit:    track.Attributes.ContentRating == "explicit",
		ImageURL:    imageURL,
		Available:   true,
		IsCompilation: isCompilation,
	}
}

// Apple Music API response structures
type AppleMusicTrack struct {
	Data []AppleMusicSong `json:"data"`
}

type AppleMusicSearchResult struct {
	Results AppleMusicResults `json:"results"`
	Included []AppleMusicAlbum `json:"included,omitempty"`
}

type AppleMusicResults struct {
	Songs AppleMusicSongs `json:"songs"`
}

type AppleMusicSongs struct {
	Data []AppleMusicSong `json:"data"`
}

type AppleMusicSong struct {
	ID         string                   `json:"id"`
	Type       string                   `json:"type"`
	Attributes AppleMusicSongAttributes `json:"attributes"`
	Relationships *AppleMusicRelationships `json:"relationships,omitempty"`
}

type AppleMusicRelationships struct {
	Albums *AppleMusicAlbumsRelationship `json:"albums,omitempty"`
}

type AppleMusicAlbumsRelationship struct {
	Data []AppleMusicAlbumReference `json:"data"`
}

type AppleMusicAlbumReference struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type AppleMusicSongAttributes struct {
	Name             string            `json:"name"`
	ArtistName       string            `json:"artistName"`
	AlbumName        string            `json:"albumName"`
	ISRC             string            `json:"isrc"`
	DurationInMillis int               `json:"durationInMillis"`
	ReleaseDate      string            `json:"releaseDate"`
	ContentRating    string            `json:"contentRating,omitempty"`
	Artwork          AppleMusicArtwork `json:"artwork"`
}

// Album structures for relationships
type AppleMusicAlbum struct {
	ID         string                   `json:"id"`
	Type       string                   `json:"type"`
	Attributes AppleMusicAlbumAttributes `json:"attributes"`
}

type AppleMusicAlbumAttributes struct {
	Name          string `json:"name"`
	ArtistName    string `json:"artistName"`
	IsCompilation bool   `json:"isCompilation"`
	IsSingle      bool   `json:"isSingle"`
	IsComplete    bool   `json:"isComplete"`
	ReleaseDate   string `json:"releaseDate"`
}

type AppleMusicArtwork struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
