package testutil

import (
	"context"

	"songshare/internal/models"
	"songshare/internal/services"

	"github.com/stretchr/testify/mock"
)

// MockSongRepository is a mock implementation of SongRepository for testing
type MockSongRepository struct {
	mock.Mock
}

func (m *MockSongRepository) Save(ctx context.Context, song *models.Song) error {
	args := m.Called(ctx, song)
	return args.Error(0)
}

func (m *MockSongRepository) Update(ctx context.Context, song *models.Song) error {
	args := m.Called(ctx, song)
	return args.Error(0)
}

func (m *MockSongRepository) FindByID(ctx context.Context, id string) (*models.Song, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Song), args.Error(1)
}

func (m *MockSongRepository) FindByISRC(ctx context.Context, isrc string) (*models.Song, error) {
	args := m.Called(ctx, isrc)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Song), args.Error(1)
}

func (m *MockSongRepository) FindByTitleArtist(ctx context.Context, title, artist string) ([]*models.Song, error) {
	args := m.Called(ctx, title, artist)
	return args.Get(0).([]*models.Song), args.Error(1)
}

func (m *MockSongRepository) FindByPlatformID(ctx context.Context, platform, externalID string) (*models.Song, error) {
	args := m.Called(ctx, platform, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Song), args.Error(1)
}

func (m *MockSongRepository) Search(ctx context.Context, query string, limit int) ([]*models.Song, error) {
	args := m.Called(ctx, query, limit)
	return args.Get(0).([]*models.Song), args.Error(1)
}

func (m *MockSongRepository) FindSimilar(ctx context.Context, song *models.Song, limit int) ([]*models.Song, error) {
	args := m.Called(ctx, song, limit)
	return args.Get(0).([]*models.Song), args.Error(1)
}

func (m *MockSongRepository) FindByIDPrefix(ctx context.Context, prefix string) (*models.Song, error) {
	args := m.Called(ctx, prefix)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Song), args.Error(1)
}

func (m *MockSongRepository) FindMany(ctx context.Context, ids []string) ([]*models.Song, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).([]*models.Song), args.Error(1)
}

func (m *MockSongRepository) SaveMany(ctx context.Context, songs []*models.Song) error {
	args := m.Called(ctx, songs)
	return args.Error(0)
}

func (m *MockSongRepository) DeleteByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSongRepository) Count(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockSongRepository) FindByISRCBatch(ctx context.Context, isrcs []string) (map[string]*models.Song, error) {
	args := m.Called(ctx, isrcs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]*models.Song), args.Error(1)
}

// Test data storage for simple mock repository
type SimpleMockSongRepository struct {
	songs map[string]*models.Song // ISRC -> Song
}

func NewSimpleMockSongRepository() *SimpleMockSongRepository {
	return &SimpleMockSongRepository{
		songs: make(map[string]*models.Song),
	}
}

func (r *SimpleMockSongRepository) AddTestSong(song *models.Song) {
	r.songs[song.ISRC] = song
}

func (r *SimpleMockSongRepository) Save(ctx context.Context, song *models.Song) error {
	r.songs[song.ISRC] = song
	return nil
}

func (r *SimpleMockSongRepository) Update(ctx context.Context, song *models.Song) error {
	r.songs[song.ISRC] = song
	return nil
}

func (r *SimpleMockSongRepository) FindByID(ctx context.Context, id string) (*models.Song, error) {
	for _, song := range r.songs {
		if song.ID.Hex() == id {
			return song, nil
		}
	}
	return nil, nil
}

func (r *SimpleMockSongRepository) FindByISRC(ctx context.Context, isrc string) (*models.Song, error) {
	if song, exists := r.songs[isrc]; exists {
		return song, nil
	}
	return nil, nil
}

func (r *SimpleMockSongRepository) FindByTitleArtist(ctx context.Context, title, artist string) ([]*models.Song, error) {
	var results []*models.Song
	for _, song := range r.songs {
		if song.Title == title && song.Artist == artist {
			results = append(results, song)
		}
	}
	return results, nil
}

func (r *SimpleMockSongRepository) FindByPlatformID(ctx context.Context, platform, externalID string) (*models.Song, error) {
	for _, song := range r.songs {
		for _, link := range song.PlatformLinks {
			if link.Platform == platform && link.ExternalID == externalID {
				return song, nil
			}
		}
	}
	return nil, nil
}

func (r *SimpleMockSongRepository) Search(ctx context.Context, query string, limit int) ([]*models.Song, error) {
	var results []*models.Song
	count := 0
	for _, song := range r.songs {
		if count >= limit {
			break
		}
		// Simple search - check if query is in title or artist
		if contains(song.Title, query) || contains(song.Artist, query) {
			results = append(results, song)
			count++
		}
	}
	return results, nil
}

func (r *SimpleMockSongRepository) FindSimilar(ctx context.Context, song *models.Song, limit int) ([]*models.Song, error) {
	return []*models.Song{}, nil
}

func (r *SimpleMockSongRepository) FindByIDPrefix(ctx context.Context, prefix string) (*models.Song, error) {
	for _, song := range r.songs {
		if len(song.ID.Hex()) >= len(prefix) && song.ID.Hex()[:len(prefix)] == prefix {
			return song, nil
		}
	}
	return nil, nil
}

func (r *SimpleMockSongRepository) FindMany(ctx context.Context, ids []string) ([]*models.Song, error) {
	var results []*models.Song
	for _, id := range ids {
		if song, err := r.FindByID(ctx, id); err == nil && song != nil {
			results = append(results, song)
		}
	}
	return results, nil
}

func (r *SimpleMockSongRepository) SaveMany(ctx context.Context, songs []*models.Song) error {
	for _, song := range songs {
		r.songs[song.ISRC] = song
	}
	return nil
}

func (r *SimpleMockSongRepository) DeleteByID(ctx context.Context, id string) error {
	for isrc, song := range r.songs {
		if song.ID.Hex() == id {
			delete(r.songs, isrc)
			return nil
		}
	}
	return nil
}

func (r *SimpleMockSongRepository) Count(ctx context.Context) (int64, error) {
	return int64(len(r.songs)), nil
}

func (r *SimpleMockSongRepository) FindByISRCBatch(ctx context.Context, isrcs []string) (map[string]*models.Song, error) {
	results := make(map[string]*models.Song)
	for _, isrc := range isrcs {
		if song, exists := r.songs[isrc]; exists {
			results[isrc] = song
		}
	}
	return results, nil
}

// Helper function for case-insensitive contains
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// MockPlatformService is a mock implementation of PlatformService for testing
type MockPlatformService struct {
	mock.Mock
	platformName string
}

func NewMockPlatformService(platformName string) *MockPlatformService {
	return &MockPlatformService{
		platformName: platformName,
	}
}

func (m *MockPlatformService) GetPlatformName() string {
	return m.platformName
}

func (m *MockPlatformService) ParseURL(url string) (*services.TrackInfo, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.TrackInfo), args.Error(1)
}

func (m *MockPlatformService) GetTrackByID(ctx context.Context, trackID string) (*services.TrackInfo, error) {
	args := m.Called(ctx, trackID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.TrackInfo), args.Error(1)
}

func (m *MockPlatformService) SearchTrack(ctx context.Context, query services.SearchQuery) ([]*services.TrackInfo, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]*services.TrackInfo), args.Error(1)
}

func (m *MockPlatformService) GetTrackByISRC(ctx context.Context, isrc string) (*services.TrackInfo, error) {
	args := m.Called(ctx, isrc)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.TrackInfo), args.Error(1)
}

func (m *MockPlatformService) BuildURL(trackID string) string {
	args := m.Called(trackID)
	return args.String(0)
}

func (m *MockPlatformService) Health(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockCache is a mock implementation of cache.Cache for testing
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(key string) (interface{}, bool) {
	args := m.Called(key)
	return args.Get(0), args.Bool(1)
}

func (m *MockCache) Set(key string, value interface{}) {
	m.Called(key, value)
}

func (m *MockCache) Delete(key string) {
	m.Called(key)
}

func (m *MockCache) Clear() {
	m.Called()
}

func (m *MockCache) Size() int {
	args := m.Called()
	return args.Int(0)
}

// Helper functions for setting up mock expectations

// ExpectSongRepositoryFindByISRC sets up expectation for FindByISRC
func ExpectSongRepositoryFindByISRC(mockRepo *MockSongRepository, isrc string, song *models.Song, err error) {
	mockRepo.On("FindByISRC", mock.Anything, isrc).Return(song, err)
}

// ExpectSongRepositorySave sets up expectation for Save
func ExpectSongRepositorySave(mockRepo *MockSongRepository, song *models.Song, err error) {
	mockRepo.On("Save", mock.Anything, song).Return(err)
}

// ExpectPlatformServiceParseURL sets up expectation for ParseURL
func ExpectPlatformServiceParseURL(mock *MockPlatformService, url string, track *services.TrackInfo, err error) {
	mock.On("ParseURL", url).Return(track, err)
}

// ExpectPlatformServiceGetTrackByID sets up expectation for GetTrackByID
func ExpectPlatformServiceGetTrackByID(mockService *MockPlatformService, trackID string, track *services.TrackInfo, err error) {
	mockService.On("GetTrackByID", mock.Anything, trackID).Return(track, err)
}
