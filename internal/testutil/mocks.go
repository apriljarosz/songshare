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
