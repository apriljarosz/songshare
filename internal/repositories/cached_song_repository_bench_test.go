package repositories

import (
	"context"
	"fmt"
	"testing"
	"time"

	"songshare/internal/cache"
	"songshare/internal/models"
)

func BenchmarkCachedSongRepository_GetByID_CacheHit(b *testing.B) {
	ctx := context.Background()
	
	// Setup mock repository and cache
	mockRepo := &mockBaseSongRepository{
		songs: map[string]*models.Song{
			"test-id": {
				ID:       "test-id",
				Title:    "Test Song",
				Artist:   "Test Artist",
				Album:    "Test Album",
				Duration: 180000,
				Platform: models.PlatformSpotify,
			},
		},
	}
	
	mockCache := cache.NewMockCache()
	cachedRepo := NewCachedSongRepository(mockRepo, mockCache)
	
	// Pre-populate cache
	song, _ := mockRepo.GetByID(ctx, "test-id")
	cachedRepo.GetByID(ctx, "test-id") // This will cache it
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := cachedRepo.GetByID(ctx, "test-id")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCachedSongRepository_GetByID_CacheMiss(b *testing.B) {
	ctx := context.Background()
	
	mockRepo := &mockBaseSongRepository{
		songs: map[string]*models.Song{},
	}
	
	// Create songs that will be fetched
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("song-%d", i)
		mockRepo.songs[id] = &models.Song{
			ID:       id,
			Title:    fmt.Sprintf("Song %d", i),
			Artist:   fmt.Sprintf("Artist %d", i),
			Album:    fmt.Sprintf("Album %d", i),
			Duration: 180000,
			Platform: models.PlatformSpotify,
		}
	}
	
	mockCache := cache.NewMockCache()
	cachedRepo := NewCachedSongRepository(mockRepo, mockCache)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("song-%d", i%1000)
		_, err := cachedRepo.GetByID(ctx, id)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCachedSongRepository_Create(b *testing.B) {
	ctx := context.Background()
	
	mockRepo := &mockBaseSongRepository{
		songs: make(map[string]*models.Song),
	}
	mockCache := cache.NewMockCache()
	cachedRepo := NewCachedSongRepository(mockRepo, mockCache)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		song := &models.Song{
			ID:       fmt.Sprintf("song-%d", i),
			Title:    fmt.Sprintf("Song %d", i),
			Artist:   fmt.Sprintf("Artist %d", i),
			Album:    fmt.Sprintf("Album %d", i),
			Duration: 180000,
			Platform: models.PlatformSpotify,
		}
		
		err := cachedRepo.Create(ctx, song)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCachedSongRepository_Search(b *testing.B) {
	ctx := context.Background()
	
	mockRepo := &mockBaseSongRepository{
		songs: make(map[string]*models.Song),
	}
	
	// Populate with test data
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("song-%d", i)
		mockRepo.songs[id] = &models.Song{
			ID:       id,
			Title:    fmt.Sprintf("Test Song %d", i),
			Artist:   fmt.Sprintf("Test Artist %d", i),
			Album:    fmt.Sprintf("Test Album %d", i),
			Duration: 180000,
			Platform: models.PlatformSpotify,
		}
	}
	
	mockCache := cache.NewMockCache()
	cachedRepo := NewCachedSongRepository(mockRepo, mockCache)
	
	queries := []string{"test", "song", "artist", "album", "music"}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			query := queries[i%len(queries)]
			_, err := cachedRepo.Search(ctx, query, 10)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkCachedSongRepository_GetByPlatformID(b *testing.B) {
	ctx := context.Background()
	
	mockRepo := &mockBaseSongRepository{
		songs:         make(map[string]*models.Song),
		platformIndex: make(map[string]*models.Song),
	}
	
	// Populate with test data
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("song-%d", i)
		platformID := fmt.Sprintf("platform-%d", i)
		song := &models.Song{
			ID:         id,
			Title:      fmt.Sprintf("Song %d", i),
			Artist:     fmt.Sprintf("Artist %d", i),
			Album:      fmt.Sprintf("Album %d", i),
			Duration:   180000,
			Platform:   models.PlatformSpotify,
			PlatformID: platformID,
		}
		mockRepo.songs[id] = song
		mockRepo.platformIndex[fmt.Sprintf("%s:%s", models.PlatformSpotify, platformID)] = song
	}
	
	mockCache := cache.NewMockCache()
	cachedRepo := NewCachedSongRepository(mockRepo, mockCache)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			platformID := fmt.Sprintf("platform-%d", i%1000)
			_, err := cachedRepo.GetByPlatformID(ctx, models.PlatformSpotify, platformID)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkCachedSongRepository_Update(b *testing.B) {
	ctx := context.Background()
	
	mockRepo := &mockBaseSongRepository{
		songs: make(map[string]*models.Song),
	}
	
	// Pre-populate songs
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("song-%d", i)
		mockRepo.songs[id] = &models.Song{
			ID:       id,
			Title:    fmt.Sprintf("Song %d", i),
			Artist:   fmt.Sprintf("Artist %d", i),
			Album:    fmt.Sprintf("Album %d", i),
			Duration: 180000,
			Platform: models.PlatformSpotify,
		}
	}
	
	mockCache := cache.NewMockCache()
	cachedRepo := NewCachedSongRepository(mockRepo, mockCache)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("song-%d", i%100)
		song := mockRepo.songs[id]
		song.Title = fmt.Sprintf("Updated Song %d", i)
		
		err := cachedRepo.Update(ctx, song)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Mock base repository for benchmarking
type mockBaseSongRepository struct {
	songs         map[string]*models.Song
	platformIndex map[string]*models.Song
	delay         time.Duration // Simulate database latency
}

func (m *mockBaseSongRepository) Create(ctx context.Context, song *models.Song) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	m.songs[song.ID] = song
	return nil
}

func (m *mockBaseSongRepository) GetByID(ctx context.Context, id string) (*models.Song, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	if song, ok := m.songs[id]; ok {
		return song, nil
	}
	return nil, ErrSongNotFound
}

func (m *mockBaseSongRepository) GetByPlatformID(ctx context.Context, platform, platformID string) (*models.Song, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	key := fmt.Sprintf("%s:%s", platform, platformID)
	if song, ok := m.platformIndex[key]; ok {
		return song, nil
	}
	return nil, ErrSongNotFound
}

func (m *mockBaseSongRepository) Update(ctx context.Context, song *models.Song) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	m.songs[song.ID] = song
	return nil
}

func (m *mockBaseSongRepository) Delete(ctx context.Context, id string) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	delete(m.songs, id)
	return nil
}

func (m *mockBaseSongRepository) Search(ctx context.Context, query string, limit int) ([]*models.Song, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	var results []*models.Song
	for _, song := range m.songs {
		if len(results) >= limit {
			break
		}
		// Simple mock search - just return first N songs
		results = append(results, song)
	}
	return results, nil
}

func (m *mockBaseSongRepository) GetByISRC(ctx context.Context, isrc string) (*models.Song, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	for _, song := range m.songs {
		if song.ISRC == isrc {
			return song, nil
		}
	}
	return nil, ErrSongNotFound
}

func (m *mockBaseSongRepository) Health(ctx context.Context) error {
	return nil
}