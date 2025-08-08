package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"songshare/internal/models"
)

func BenchmarkSongHandlers_ResolveSong(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	mockService := &mockSongResolutionService{
		song: &models.Song{
			ID:       "test-id",
			Title:    "Test Song",
			Artist:   "Test Artist",
			Album:    "Test Album",
			Duration: 180000,
			Platform: models.PlatformSpotify,
			Links: models.PlatformLinks{
				Spotify:     "https://open.spotify.com/track/test123",
				AppleMusic:  "https://music.apple.com/song/test456",
			},
		},
	}
	
	handler := NewSongHandlers(mockService)
	router := gin.New()
	router.POST("/api/v1/songs/resolve", handler.ResolveSong)
	
	requestBody := map[string]string{
		"url": "https://open.spotify.com/track/test123",
	}
	bodyBytes, _ := json.Marshal(requestBody)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/api/v1/songs/resolve", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				b.Fatalf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

func BenchmarkSongHandlers_SearchSongs(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	mockService := &mockSongResolutionService{
		searchResults: []models.Song{
			{
				ID:       "song1",
				Title:    "Test Song 1",
				Artist:   "Artist 1",
				Album:    "Album 1",
				Duration: 180000,
				Platform: models.PlatformSpotify,
			},
			{
				ID:       "song2",
				Title:    "Test Song 2",
				Artist:   "Artist 2",
				Album:    "Album 2",
				Duration: 200000,
				Platform: models.PlatformAppleMusic,
			},
		},
	}
	
	handler := NewSongHandlers(mockService)
	router := gin.New()
	router.POST("/api/v1/songs/search", handler.SearchSongs)
	
	requestBody := map[string]string{
		"query": "test song",
	}
	bodyBytes, _ := json.Marshal(requestBody)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/api/v1/songs/search", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				b.Fatalf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

func BenchmarkSongHandlers_GetUniversalLink(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	mockService := &mockSongResolutionService{
		song: &models.Song{
			ID:       "abcd1234",
			Title:    "Test Song",
			Artist:   "Test Artist",
			Album:    "Test Album",
			Duration: 180000,
			Platform: models.PlatformSpotify,
			Links: models.PlatformLinks{
				Spotify:     "https://open.spotify.com/track/test123",
				AppleMusic:  "https://music.apple.com/song/test456",
			},
		},
	}
	
	handler := NewSongHandlers(mockService)
	router := gin.New()
	router.GET("/s/:id", handler.GetUniversalLink)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/s/abcd1234", nil)
			req.Header.Set("Accept", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				b.Fatalf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

func BenchmarkSongHandlers_GetUniversalLink_HTML(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	mockService := &mockSongResolutionService{
		song: &models.Song{
			ID:       "abcd1234",
			Title:    "Test Song",
			Artist:   "Test Artist",
			Album:    "Test Album",
			Duration: 180000,
			Platform: models.PlatformSpotify,
			Links: models.PlatformLinks{
				Spotify:     "https://open.spotify.com/track/test123",
				AppleMusic:  "https://music.apple.com/song/test456",
			},
		},
	}
	
	handler := NewSongHandlers(mockService)
	router := gin.New()
	router.GET("/s/:id", handler.GetUniversalLink)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/s/abcd1234", nil)
			req.Header.Set("Accept", "text/html")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			if w.Code != http.StatusOK {
				b.Fatalf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

func BenchmarkSongHandlers_ResolveSong_LargeResponse(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	
	// Create a song with many platform links
	mockService := &mockSongResolutionService{
		song: &models.Song{
			ID:       "test-id",
			Title:    "Test Song with Very Long Title That Contains Many Words",
			Artist:   "Test Artist with Multiple Names and Collaborators",
			Album:    "Test Album with Extended Deluxe Edition Title",
			Duration: 180000,
			Platform: models.PlatformSpotify,
			ISRC:     "USRC12345678",
			Links: models.PlatformLinks{
				Spotify:     "https://open.spotify.com/track/verylongtrackidthatcontainsmanycharacters",
				AppleMusic:  "https://music.apple.com/us/album/song-name/123456789?i=987654321",
				YouTube:     "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
				Tidal:       "https://tidal.com/track/123456789",
			},
			AlbumArt: "https://example.com/very-long-url-to-high-resolution-album-artwork-image.jpg",
			Preview:  "https://example.com/preview-url-for-30-second-sample.mp3",
		},
	}
	
	handler := NewSongHandlers(mockService)
	router := gin.New()
	router.POST("/api/v1/songs/resolve", handler.ResolveSong)
	
	requestBody := map[string]string{
		"url": "https://open.spotify.com/track/test123",
	}
	bodyBytes, _ := json.Marshal(requestBody)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/v1/songs/resolve", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

// Mock service for benchmarking
type mockSongResolutionService struct {
	song          *models.Song
	searchResults []models.Song
}

func (m *mockSongResolutionService) ResolveFromURL(ctx context.Context, url string) (*models.Song, error) {
	return m.song, nil
}

func (m *mockSongResolutionService) SearchSongs(ctx context.Context, query string) ([]models.Song, error) {
	return m.searchResults, nil
}

func (m *mockSongResolutionService) GetSongByID(ctx context.Context, id string) (*models.Song, error) {
	return m.song, nil
}

func (m *mockSongResolutionService) GetPlatformService(platform string) interface{} {
	return nil
}