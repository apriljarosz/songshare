package services

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/apriljarosz/songshare/internal/cache"
	"github.com/apriljarosz/songshare/internal/models"
)

type SpotifyService struct {
	clientID     string
	clientSecret string
	cache        *cache.MemoryCache
	accessToken  string
	tokenExpiry  time.Time
}

type spotifyAuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type spotifyTrack struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Explicit    bool   `json:"explicit"`
	DurationMS  int    `json:"duration_ms"`
	ExternalIDs struct {
		ISRC string `json:"isrc"`
	} `json:"external_ids"`
	Album struct {
		Name   string `json:"name"`
		Images []struct {
			URL    string `json:"url"`
			Height int    `json:"height"`
			Width  int    `json:"width"`
		} `json:"images"`
		ReleaseDate string `json:"release_date"`
	} `json:"album"`
	Artists []struct {
		Name string `json:"name"`
	} `json:"artists"`
	ExternalURLs struct {
		Spotify string `json:"spotify"`
	} `json:"external_urls"`
}

type spotifySearchResponse struct {
	Tracks struct {
		Items []spotifyTrack `json:"items"`
	} `json:"tracks"`
}

func NewSpotifyService(clientID, clientSecret string, cache *cache.MemoryCache) *SpotifyService {
	return &SpotifyService{
		clientID:     clientID,
		clientSecret: clientSecret,
		cache:        cache,
	}
}

func (s *SpotifyService) authenticate() error {
	// Check if we have a valid cached token
	if time.Now().Before(s.tokenExpiry) && s.accessToken != "" {
		return nil
	}

	// Request new token
	auth := base64.StdEncoding.EncodeToString([]byte(s.clientID + ":" + s.clientSecret))

	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, err := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authResp spotifyAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	s.accessToken = authResp.AccessToken
	s.tokenExpiry = time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second)

	return nil
}

func (s *SpotifyService) Search(query string) ([]models.Song, error) {
	return s.SearchWithPagination(query, 0, 10)
}

func (s *SpotifyService) SearchWithPagination(query string, offset, limit int) ([]models.Song, error) {
	// Set default limit if not provided
	if limit <= 0 {
		limit = 10
	}
	// Spotify max limit is 50
	if limit > 50 {
		limit = 50
	}

	// Check cache first
	cacheKey := fmt.Sprintf("spotify:search:%s:%d:%d", query, offset, limit)
	if cached, found := s.cache.Get(cacheKey); found {
		return cached.([]models.Song), nil
	}

	// Authenticate
	if err := s.authenticate(); err != nil {
		return nil, err
	}

	// Build search URL with pagination
	searchURL := fmt.Sprintf("https://api.spotify.com/v1/search?q=%s&type=track&limit=%d&offset=%d",
		url.QueryEscape(query), limit, offset)

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp spotifySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	// Convert to our Song model
	songs := make([]models.Song, 0, len(searchResp.Tracks.Items))
	for _, track := range searchResp.Tracks.Items {
		song := s.trackToSong(track)
		songs = append(songs, song)
	}

	// Cache results for 1 hour
	s.cache.Set(cacheKey, songs, 1*time.Hour)

	return songs, nil
}

func (s *SpotifyService) GetTrackByID(trackID string) (*models.Song, error) {
	// Check cache first
	cacheKey := "spotify:track:" + trackID
	if cached, found := s.cache.Get(cacheKey); found {
		song := cached.(models.Song)
		return &song, nil
	}

	// Authenticate
	if err := s.authenticate(); err != nil {
		return nil, err
	}

	// Get track
	trackURL := fmt.Sprintf("https://api.spotify.com/v1/tracks/%s", trackID)

	req, err := http.NewRequest("GET", trackURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create track request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("track request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get track failed with status %d: %s", resp.StatusCode, string(body))
	}

	var track spotifyTrack
	if err := json.NewDecoder(resp.Body).Decode(&track); err != nil {
		return nil, fmt.Errorf("failed to decode track response: %w", err)
	}

	song := s.trackToSong(track)

	// Cache for 24 hours
	s.cache.Set(cacheKey, song, 24*time.Hour)

	return &song, nil
}

func (s *SpotifyService) GetSongByISRC(isrc string) (*models.Song, error) {
	// Check cache first
	cacheKey := "spotify:isrc:" + isrc
	if cached, found := s.cache.Get(cacheKey); found {
		song := cached.(models.Song)
		return &song, nil
	}

	// Authenticate
	if err := s.authenticate(); err != nil {
		return nil, err
	}

	// Search by ISRC
	searchURL := fmt.Sprintf("https://api.spotify.com/v1/search?q=isrc:%s&type=track&limit=1",
		url.QueryEscape(isrc))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create ISRC search request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ISRC search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ISRC search failed with status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp spotifySearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode ISRC search response: %w", err)
	}

	// Check if we found a track
	if len(searchResp.Tracks.Items) == 0 {
		return nil, nil // Not found, return nil without error
	}

	song := s.trackToSong(searchResp.Tracks.Items[0])

	// Cache for 24 hours
	s.cache.Set(cacheKey, song, 24*time.Hour)

	return &song, nil
}

func (s *SpotifyService) trackToSong(track spotifyTrack) models.Song {
	// Extract artists
	artists := make([]string, len(track.Artists))
	for i, artist := range track.Artists {
		artists[i] = artist.Name
	}

	// Get album art (prefer highest quality)
	albumArt := ""
	if len(track.Album.Images) > 0 {
		albumArt = track.Album.Images[0].URL
	}

	return models.Song{
		ISRC:        track.ExternalIDs.ISRC,
		Title:       track.Name,
		Artists:     artists,
		Album:       track.Album.Name,
		AlbumArt:    albumArt,
		Duration:    track.DurationMS,
		ReleaseDate: track.Album.ReleaseDate,
		Explicit:    track.Explicit,
		Platforms: []models.PlatformLink{
			{
				Platform: "spotify",
				ID:       track.ID,
				URL:      track.ExternalURLs.Spotify,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
