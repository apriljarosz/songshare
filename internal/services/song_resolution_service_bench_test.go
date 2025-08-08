package services

import (
	"context"
	"testing"
	"time"

	"songshare/internal/models"
	"songshare/internal/repositories"
)

func BenchmarkSongResolutionService_ResolveFromURL(b *testing.B) {
	ctx := context.Background()
	mockRepo := &mockSongRepository{}
	service := NewSongResolutionService(mockRepo)

	// Register mock platform services
	mockSpotify := &mockPlatformService{
		name: "spotify",
		song: &models.Song{
			ID:       "test-song-id",
			Title:    "Test Song",
			Artist:   "Test Artist",
			Album:    "Test Album",
			Duration: 180000,
			Platform: models.PlatformSpotify,
			Links: models.PlatformLinks{
				Spotify: "https://open.spotify.com/track/test123",
			},
		},
	}
	service.RegisterPlatform(mockSpotify)

	testURL := "https://open.spotify.com/track/test123"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := service.ResolveFromURL(ctx, testURL)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkSongResolutionService_SearchSongs(b *testing.B) {
	ctx := context.Background()
	mockRepo := &mockSongRepository{}
	service := NewSongResolutionService(mockRepo)

	// Register mock platform services
	mockSpotify := &mockPlatformService{
		name: "spotify",
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
				Platform: models.PlatformSpotify,
			},
		},
	}
	service.RegisterPlatform(mockSpotify)

	query := "test song"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := service.SearchSongs(ctx, query)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkSongResolutionService_GetSongByID(b *testing.B) {
	ctx := context.Background()
	mockRepo := &mockSongRepository{
		song: &models.Song{
			ID:       "test-id",
			Title:    "Test Song",
			Artist:   "Test Artist",
			Album:    "Test Album",
			Duration: 180000,
			Platform: models.PlatformSpotify,
		},
	}
	service := NewSongResolutionService(mockRepo)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := service.GetSongByID(ctx, "test-id")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkSongResolutionService_MultiPlatformResolve(b *testing.B) {
	ctx := context.Background()
	mockRepo := &mockSongRepository{}
	service := NewSongResolutionService(mockRepo)

	// Register multiple platform services
	platforms := []string{"spotify", "apple_music", "tidal"}
	for _, platform := range platforms {
		mockService := &mockPlatformService{
			name: platform,
			song: &models.Song{
				ID:       "test-song-" + platform,
				Title:    "Test Song",
				Artist:   "Test Artist",
				Album:    "Test Album",
				Duration: 180000,
				Platform: platform,
			},
		}
		service.RegisterPlatform(mockService)
	}

	testURL := "https://open.spotify.com/track/test123"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ResolveFromURL(ctx, testURL)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Mock implementations for benchmarking
type mockSongRepository struct {
	song *models.Song
}

func (m *mockSongRepository) Create(ctx context.Context, song *models.Song) error {
	return nil
}

func (m *mockSongRepository) GetByID(ctx context.Context, id string) (*models.Song, error) {
	if m.song != nil {
		return m.song, nil
	}
	return nil, repositories.ErrSongNotFound
}

func (m *mockSongRepository) GetByPlatformID(ctx context.Context, platform, platformID string) (*models.Song, error) {
	if m.song != nil {
		return m.song, nil
	}
	return nil, repositories.ErrSongNotFound
}

func (m *mockSongRepository) Update(ctx context.Context, song *models.Song) error {
	return nil
}

func (m *mockSongRepository) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockSongRepository) Search(ctx context.Context, query string, limit int) ([]*models.Song, error) {
	return []*models.Song{m.song}, nil
}

func (m *mockSongRepository) GetByISRC(ctx context.Context, isrc string) (*models.Song, error) {
	if m.song != nil {
		return m.song, nil
	}
	return nil, repositories.ErrSongNotFound
}

func (m *mockSongRepository) Health(ctx context.Context) error {
	return nil
}

type mockPlatformService struct {
	name          string
	song          *models.Song
	searchResults []models.Song
	delay         time.Duration
}

func (m *mockPlatformService) GetPlatformName() string {
	return m.name
}

func (m *mockPlatformService) GetSongByID(ctx context.Context, id string) (*models.Song, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	if m.song != nil {
		return m.song, nil
	}
	return nil, nil
}

func (m *mockPlatformService) SearchSongs(ctx context.Context, query string) ([]models.Song, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	if m.searchResults != nil {
		return m.searchResults, nil
	}
	if m.song != nil {
		return []models.Song{*m.song}, nil
	}
	return []models.Song{}, nil
}

func (m *mockPlatformService) GetAlbumArt(ctx context.Context, song *models.Song) (string, error) {
	return "https://example.com/album-art.jpg", nil
}

func (m *mockPlatformService) IsAvailable() bool {
	return true
}

func (m *mockPlatformService) Health(ctx context.Context) error {
	return nil
}