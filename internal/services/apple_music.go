package services

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/apriljarosz/songshare/internal/cache"
	"github.com/apriljarosz/songshare/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

type AppleMusicService struct {
	teamID       string
	keyID        string
	privateKey   *ecdsa.PrivateKey
	cache        *cache.MemoryCache
	developerToken string
	tokenExpiry    time.Time
}

type appleMusicSong struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		AlbumName        string `json:"albumName"`
		ArtistName       string `json:"artistName"`
		Artwork          struct {
			URL    string `json:"url"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"artwork"`
		ISRC             string   `json:"isrc"`
		Name             string   `json:"name"`
		DurationInMillis int      `json:"durationInMillis"`
		GenreNames       []string `json:"genreNames"`
		ReleaseDate      string   `json:"releaseDate"`
		URL              string   `json:"url"`
	} `json:"attributes"`
}

type appleMusicSearchResponse struct {
	Results struct {
		Songs struct {
			Data []appleMusicSong `json:"data"`
		} `json:"songs"`
	} `json:"results"`
}

type appleMusicSongsResponse struct {
	Data []appleMusicSong `json:"data"`
}

func NewAppleMusicService(teamID, keyID, privateKeyPath string, cache *cache.MemoryCache) (*AppleMusicService, error) {
	// Read private key file
	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	// Parse PEM block
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	// Parse private key
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	ecdsaKey, ok := privateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not ECDSA")
	}

	return &AppleMusicService{
		teamID:     teamID,
		keyID:      keyID,
		privateKey: ecdsaKey,
		cache:      cache,
	}, nil
}

func (s *AppleMusicService) generateToken() error {
	// Check if we have a valid cached token
	if time.Now().Before(s.tokenExpiry) && s.developerToken != "" {
		return nil
	}

	// Create JWT token
	now := time.Now()
	expiry := now.Add(6 * 30 * 24 * time.Hour) // 6 months

	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": s.teamID,
		"iat": now.Unix(),
		"exp": expiry.Unix(),
	})

	// Set key ID in header
	token.Header["kid"] = s.keyID

	// Sign token
	tokenString, err := token.SignedString(s.privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign token: %w", err)
	}

	s.developerToken = tokenString
	s.tokenExpiry = expiry

	return nil
}

func (s *AppleMusicService) Search(query string) ([]models.Song, error) {
	// Check cache first
	cacheKey := "apple_music:search:" + query
	if cached, found := s.cache.Get(cacheKey); found {
		return cached.([]models.Song), nil
	}

	// Generate token
	if err := s.generateToken(); err != nil {
		return nil, err
	}

	// Build search URL
	searchURL := fmt.Sprintf("https://api.music.apple.com/v1/catalog/us/search?term=%s&types=songs&limit=10",
		url.QueryEscape(query))

	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create search request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.developerToken)

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

	var searchResp appleMusicSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	// Convert to our Song model
	songs := make([]models.Song, 0, len(searchResp.Results.Songs.Data))
	for _, track := range searchResp.Results.Songs.Data {
		song := s.trackToSong(track)
		songs = append(songs, song)
	}

	// Cache results for 1 hour
	s.cache.Set(cacheKey, songs, 1*time.Hour)

	return songs, nil
}

func (s *AppleMusicService) GetSongByISRC(isrc string) (*models.Song, error) {
	// Check cache first
	cacheKey := "apple_music:isrc:" + isrc
	if cached, found := s.cache.Get(cacheKey); found {
		song := cached.(models.Song)
		return &song, nil
	}

	// Generate token
	if err := s.generateToken(); err != nil {
		return nil, err
	}

	// Get song by ISRC
	songURL := fmt.Sprintf("https://api.music.apple.com/v1/catalog/us/songs?filter[isrc]=%s", isrc)

	req, err := http.NewRequest("GET", songURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.developerToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get song failed with status %d: %s", resp.StatusCode, string(body))
	}

	var songsResp appleMusicSongsResponse
	if err := json.NewDecoder(resp.Body).Decode(&songsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(songsResp.Data) == 0 {
		return nil, nil
	}

	song := s.trackToSong(songsResp.Data[0])

	// Cache for 24 hours
	s.cache.Set(cacheKey, song, 24*time.Hour)

	return &song, nil
}

func (s *AppleMusicService) GetSongByID(songID string) (*models.Song, error) {
	// Check cache first
	cacheKey := "apple_music:song:" + songID
	if cached, found := s.cache.Get(cacheKey); found {
		song := cached.(models.Song)
		return &song, nil
	}

	// Generate token
	if err := s.generateToken(); err != nil {
		return nil, err
	}

	// Get song
	songURL := fmt.Sprintf("https://api.music.apple.com/v1/catalog/us/songs/%s", songID)

	req, err := http.NewRequest("GET", songURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.developerToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get song failed with status %d: %s", resp.StatusCode, string(body))
	}

	var songsResp appleMusicSongsResponse
	if err := json.NewDecoder(resp.Body).Decode(&songsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(songsResp.Data) == 0 {
		return nil, nil
	}

	song := s.trackToSong(songsResp.Data[0])

	// Cache for 24 hours
	s.cache.Set(cacheKey, song, 24*time.Hour)

	return &song, nil
}

func (s *AppleMusicService) trackToSong(track appleMusicSong) models.Song {
	// Get album art URL (replace placeholders with actual dimensions)
	albumArt := strings.Replace(track.Attributes.Artwork.URL, "{w}", "600", 1)
	albumArt = strings.Replace(albumArt, "{h}", "600", 1)

	return models.Song{
		ISRC:        track.Attributes.ISRC,
		Title:       track.Attributes.Name,
		Artists:     []string{track.Attributes.ArtistName},
		Album:       track.Attributes.AlbumName,
		AlbumArt:    albumArt,
		Duration:    track.Attributes.DurationInMillis,
		ReleaseDate: track.Attributes.ReleaseDate,
		Platforms: []models.PlatformLink{
			{
				Platform: "apple_music",
				ID:       track.ID,
				URL:      track.Attributes.URL,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
