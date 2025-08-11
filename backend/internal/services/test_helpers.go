package services

import (
	"context"

	"songshare/internal/models"

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

func (m *MockPlatformService) ParseURL(url string) (*TrackInfo, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TrackInfo), args.Error(1)
}

func (m *MockPlatformService) GetTrackByID(ctx context.Context, trackID string) (*TrackInfo, error) {
	args := m.Called(ctx, trackID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TrackInfo), args.Error(1)
}

func (m *MockPlatformService) SearchTrack(ctx context.Context, query SearchQuery) ([]*TrackInfo, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]*TrackInfo), args.Error(1)
}

func (m *MockPlatformService) GetTrackByISRC(ctx context.Context, isrc string) (*TrackInfo, error) {
	args := m.Called(ctx, isrc)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TrackInfo), args.Error(1)
}

func (m *MockPlatformService) BuildURL(trackID string) string {
	args := m.Called(trackID)
	return args.String(0)
}

func (m *MockPlatformService) Health(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockPlatformService for use in other packages - embedded struct to avoid interface issues
type MockPlatformServiceForHandlers struct {
	mock.Mock
	platformName string
}

func NewMockPlatformServiceForHandlers(platformName string) *MockPlatformServiceForHandlers {
	return &MockPlatformServiceForHandlers{
		platformName: platformName,
	}
}

func (m *MockPlatformServiceForHandlers) GetPlatformName() string {
	return m.platformName
}

func (m *MockPlatformServiceForHandlers) ParseURL(url string) (*TrackInfo, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TrackInfo), args.Error(1)
}

func (m *MockPlatformServiceForHandlers) GetTrackByID(ctx context.Context, trackID string) (*TrackInfo, error) {
	args := m.Called(ctx, trackID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TrackInfo), args.Error(1)
}

func (m *MockPlatformServiceForHandlers) SearchTrack(ctx context.Context, query SearchQuery) ([]*TrackInfo, error) {
	args := m.Called(ctx, query)
	return args.Get(0).([]*TrackInfo), args.Error(1)
}

func (m *MockPlatformServiceForHandlers) GetTrackByISRC(ctx context.Context, isrc string) (*TrackInfo, error) {
	args := m.Called(ctx, isrc)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*TrackInfo), args.Error(1)
}

func (m *MockPlatformServiceForHandlers) BuildURL(trackID string) string {
	args := m.Called(trackID)
	return args.String(0)
}

func (m *MockPlatformServiceForHandlers) Health(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Test constants
const (
	TestISRC1 = "USUM71703861"
	TestISRC2 = "USRC17607839"
	TestISRC3 = "GBUM71505078"

	SpotifyTrackID1 = "4iV5W9uYEdYUVa79Axb7Rh"
	SpotifyTrackID2 = "1YLJVMUFYjwdAF4lDPqH7G"

	AppleMusicTrackID1 = "1440857781"
	AppleMusicTrackID2 = "1440857782"

	SpotifyURL1    = "https://open.spotify.com/track/" + SpotifyTrackID1
	SpotifyURL2    = "https://open.spotify.com/track/" + SpotifyTrackID2
	AppleMusicURL1 = "https://music.apple.com/us/song/test-song/" + AppleMusicTrackID1
	AppleMusicURL2 = "https://music.apple.com/us/album/test-album/123456789?i=" + AppleMusicTrackID2
)

// Helper functions for creating test data
func createTestSong() *models.Song {
	return createTestSongWithISRC(TestISRC1)
}

func createTestSongWithISRC(isrc string) *models.Song {
	song := models.NewSong("Test Song", "Test Artist")
	song.ISRC = isrc
	song.Album = "Test Album"
	song.Metadata.Duration = 240000
	song.Metadata.Popularity = 75
	return song
}

func createTestTrackInfo(platform, externalID, url string) *TrackInfo {
	return &TrackInfo{
		Platform:   platform,
		ExternalID: externalID,
		URL:        url,
		Title:      "Test Song",
		Artists:    []string{"Test Artist"},
		Album:      "Test Album",
		ISRC:       TestISRC1,
		Duration:   240000,
		Available:  true,
	}
}
