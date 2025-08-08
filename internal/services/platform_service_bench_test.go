package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"songshare/internal/models"
)

func BenchmarkPlatformService_ParseURL(b *testing.B) {
	testURLs := []string{
		"https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
		"https://music.apple.com/us/album/song-name/123456789?i=987654321",
		"https://tidal.com/track/123456789",
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ",
		"https://open.spotify.com/intl-es/track/4iV5W9uYEdYUVa79Axb7Rh",
		"https://music.apple.com/gb/album/test/1234567890?i=0987654321",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := testURLs[i%len(testURLs)]
		_, _, err := ParsePlatformURL(url)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPlatformService_ParseURL_Parallel(b *testing.B) {
	testURLs := []string{
		"https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
		"https://music.apple.com/us/album/song-name/123456789?i=987654321",
		"https://tidal.com/track/123456789",
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			url := testURLs[i%len(testURLs)]
			_, _, err := ParsePlatformURL(url)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkPlatformService_ConcurrentSearch(b *testing.B) {
	ctx := context.Background()
	
	// Create multiple mock platform services
	services := make([]PlatformService, 3)
	for i := 0; i < 3; i++ {
		services[i] = &mockPlatformServiceWithLatency{
			name:    fmt.Sprintf("platform-%d", i),
			latency: time.Millisecond * 10, // Simulate API latency
			searchResults: []models.Song{
				{
					ID:       fmt.Sprintf("song-%d-1", i),
					Title:    fmt.Sprintf("Result 1 from Platform %d", i),
					Artist:   fmt.Sprintf("Artist %d", i),
					Album:    fmt.Sprintf("Album %d", i),
					Duration: 180000,
					Platform: fmt.Sprintf("platform-%d", i),
				},
				{
					ID:       fmt.Sprintf("song-%d-2", i),
					Title:    fmt.Sprintf("Result 2 from Platform %d", i),
					Artist:   fmt.Sprintf("Artist %d", i),
					Album:    fmt.Sprintf("Album %d", i),
					Duration: 200000,
					Platform: fmt.Sprintf("platform-%d", i),
				},
			},
		}
	}

	query := "test song"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate concurrent searches across platforms
			results := make(chan []models.Song, len(services))
			errors := make(chan error, len(services))

			for _, service := range services {
				go func(svc PlatformService) {
					songs, err := svc.SearchSongs(ctx, query)
					if err != nil {
						errors <- err
						return
					}
					results <- songs
				}(service)
			}

			// Collect results
			var allResults []models.Song
			for i := 0; i < len(services); i++ {
				select {
				case songs := <-results:
					allResults = append(allResults, songs...)
				case err := <-errors:
					b.Fatal(err)
				case <-time.After(time.Second):
					b.Fatal("Timeout waiting for search results")
				}
			}
		}
	})
}

func BenchmarkPlatformService_GetSongByID_WithCache(b *testing.B) {
	ctx := context.Background()
	
	// Mock service with simulated cache
	service := &mockPlatformServiceWithCache{
		name:      "spotify",
		cache:     make(map[string]*models.Song),
		cacheHits: 0,
		song: &models.Song{
			ID:       "test-id",
			Title:    "Test Song",
			Artist:   "Test Artist",
			Album:    "Test Album",
			Duration: 180000,
			Platform: "spotify",
		},
	}

	// Pre-populate cache for some IDs
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("cached-%d", i)
		service.cache[id] = &models.Song{
			ID:       id,
			Title:    fmt.Sprintf("Cached Song %d", i),
			Artist:   "Cached Artist",
			Album:    "Cached Album",
			Duration: 180000,
			Platform: "spotify",
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Mix cached and non-cached requests
			var id string
			if i%2 == 0 {
				id = fmt.Sprintf("cached-%d", i%100)
			} else {
				id = fmt.Sprintf("uncached-%d", i)
			}
			
			_, err := service.GetSongByID(ctx, id)
			if err != nil && err.Error() != "song not found" {
				b.Fatal(err)
			}
			i++
		}
	})

	b.Logf("Cache hit rate: %.2f%%", float64(service.cacheHits)/float64(b.N)*100)
}

func BenchmarkPlatformService_BatchResolve(b *testing.B) {
	ctx := context.Background()
	
	// Create a service that can resolve multiple songs
	service := &mockPlatformServiceWithLatency{
		name:    "spotify",
		latency: time.Millisecond * 5,
		song: &models.Song{
			ID:       "test-id",
			Title:    "Test Song",
			Artist:   "Test Artist",
			Album:    "Test Album",
			Duration: 180000,
			Platform: "spotify",
		},
	}

	// Simulate batch processing of song IDs
	songIDs := make([]string, 10)
	for i := range songIDs {
		songIDs[i] = fmt.Sprintf("song-%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := make([]*models.Song, 0, len(songIDs))
		for _, id := range songIDs {
			song, err := service.GetSongByID(ctx, id)
			if err != nil {
				b.Fatal(err)
			}
			results = append(results, song)
		}
	}
}

// Mock implementations for benchmarking
type mockPlatformServiceWithLatency struct {
	name          string
	song          *models.Song
	searchResults []models.Song
	latency       time.Duration
}

func (m *mockPlatformServiceWithLatency) GetPlatformName() string {
	return m.name
}

func (m *mockPlatformServiceWithLatency) GetSongByID(ctx context.Context, id string) (*models.Song, error) {
	if m.latency > 0 {
		time.Sleep(m.latency)
	}
	if m.song != nil {
		// Return a copy with the requested ID
		song := *m.song
		song.ID = id
		return &song, nil
	}
	return nil, fmt.Errorf("song not found")
}

func (m *mockPlatformServiceWithLatency) SearchSongs(ctx context.Context, query string) ([]models.Song, error) {
	if m.latency > 0 {
		time.Sleep(m.latency)
	}
	return m.searchResults, nil
}

func (m *mockPlatformServiceWithLatency) GetAlbumArt(ctx context.Context, song *models.Song) (string, error) {
	if m.latency > 0 {
		time.Sleep(m.latency)
	}
	return "https://example.com/album-art.jpg", nil
}

func (m *mockPlatformServiceWithLatency) IsAvailable() bool {
	return true
}

func (m *mockPlatformServiceWithLatency) Health(ctx context.Context) error {
	return nil
}

type mockPlatformServiceWithCache struct {
	name      string
	song      *models.Song
	cache     map[string]*models.Song
	cacheHits int
}

func (m *mockPlatformServiceWithCache) GetPlatformName() string {
	return m.name
}

func (m *mockPlatformServiceWithCache) GetSongByID(ctx context.Context, id string) (*models.Song, error) {
	// Check cache first
	if song, ok := m.cache[id]; ok {
		m.cacheHits++
		return song, nil
	}
	
	// Return default song for uncached IDs
	if m.song != nil {
		song := *m.song
		song.ID = id
		// Add to cache
		m.cache[id] = &song
		return &song, nil
	}
	
	return nil, fmt.Errorf("song not found")
}

func (m *mockPlatformServiceWithCache) SearchSongs(ctx context.Context, query string) ([]models.Song, error) {
	return []models.Song{}, nil
}

func (m *mockPlatformServiceWithCache) GetAlbumArt(ctx context.Context, song *models.Song) (string, error) {
	return "https://example.com/album-art.jpg", nil
}

func (m *mockPlatformServiceWithCache) IsAvailable() bool {
	return true
}

func (m *mockPlatformServiceWithCache) Health(ctx context.Context) error {
	return nil
}