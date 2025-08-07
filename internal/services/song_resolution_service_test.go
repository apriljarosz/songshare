package services

import (
	"context"
	"errors"
	"testing"

	"songshare/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewSongResolutionService(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	assert.NotNil(t, service)
	assert.Equal(t, mockRepo, service.songRepo)
	assert.NotNil(t, service.platformServices)
	assert.Empty(t, service.platformServices)
}

func TestSongResolutionService_RegisterPlatform(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	mockSpotify := NewMockPlatformService("spotify")
	mockAppleMusic := NewMockPlatformService("apple_music")
	
	// Register platforms
	service.RegisterPlatform(mockSpotify)
	service.RegisterPlatform(mockAppleMusic)
	
	// Verify platforms are registered
	assert.Equal(t, mockSpotify, service.GetPlatformService("spotify"))
	assert.Equal(t, mockAppleMusic, service.GetPlatformService("apple_music"))
	assert.Nil(t, service.GetPlatformService("youtube"))
}

func TestSongResolutionService_ResolveFromURL_UnsupportedPlatform(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	ctx := context.Background()
	url := "https://youtube.com/watch?v=test"
	
	song, err := service.ResolveFromURL(ctx, url)
	
	assert.Error(t, err)
	assert.Nil(t, song)
	assert.Contains(t, err.Error(), "unsupported platform URL")
}

func TestSongResolutionService_ResolveFromURL_InvalidURL(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	ctx := context.Background()
	url := "invalid-url"
	
	song, err := service.ResolveFromURL(ctx, url)
	
	assert.Error(t, err)
	assert.Nil(t, song)
	assert.Contains(t, err.Error(), "failed to parse platform URL")
}

func TestSongResolutionService_ResolveFromURL_PlatformNotRegistered(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	ctx := context.Background()
	url := "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
	
	song, err := service.ResolveFromURL(ctx, url)
	
	assert.Error(t, err)
	assert.Nil(t, song)
	assert.Contains(t, err.Error(), "unsupported platform: spotify")
}

func TestSongResolutionService_ResolveFromURL_ExistingSongByPlatformID(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	mockSpotify := NewMockPlatformService("spotify")
	service.RegisterPlatform(mockSpotify)
	
	ctx := context.Background()
	url := "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
	trackID := "4iV5W9uYEdYUVa79Axb7Rh"
	
	// Create existing song with Spotify platform link
	existingSong := createTestSong()
	existingSong.AddPlatformLink("spotify", trackID, url, 1.0)
	
	// Setup mock expectations for existing song
	mockRepo.On("FindByPlatformID", ctx, "spotify", trackID).Return(existingSong, nil)
	
	// Mock expectations for resolveOnOtherPlatforms - will call GetTrackByISRC since song has ISRC
	// but the existing song already has Spotify link, so it should skip
	// Since we only have Spotify registered, no other platforms will be called
	
	// No Update call should happen since no new platforms were added
	
	song, err := service.ResolveFromURL(ctx, url)
	
	require.NoError(t, err)
	require.NotNil(t, song)
	assert.Equal(t, existingSong.ID, song.ID)
	
	mockRepo.AssertExpectations(t)
	mockSpotify.AssertExpectations(t)
}

func TestSongResolutionService_ResolveFromURL_NewSong(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	mockSpotify := NewMockPlatformService("spotify")
	service.RegisterPlatform(mockSpotify)
	
	ctx := context.Background()
	url := "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
	trackID := "4iV5W9uYEdYUVa79Axb7Rh"
	
	// Create track info
	trackInfo := &TrackInfo{
		Platform:   "spotify",
		ExternalID: trackID,
		URL:        url,
		Title:      "Test Song",
		Artists:    []string{"Test Artist"},
		ISRC:       TestISRC1,
		Available:  true,
	}
	
	// Setup mock expectations
	mockRepo.On("FindByPlatformID", ctx, "spotify", trackID).Return(nil, errors.New("not found"))
	mockSpotify.On("GetTrackByID", ctx, trackID).Return(trackInfo, nil)
	mockRepo.On("FindByISRC", ctx, TestISRC1).Return(nil, errors.New("not found"))
	mockRepo.On("Save", ctx, mock.AnythingOfType("*models.Song")).Return(nil)
	
	song, err := service.ResolveFromURL(ctx, url)
	
	require.NoError(t, err)
	require.NotNil(t, song)
	assert.Equal(t, "Test Song", song.Title)
	assert.Equal(t, "Test Artist", song.Artist)
	assert.Equal(t, TestISRC1, song.ISRC)
	assert.Len(t, song.PlatformLinks, 1)
	assert.Equal(t, "spotify", song.PlatformLinks[0].Platform)
	
	mockRepo.AssertExpectations(t)
	mockSpotify.AssertExpectations(t)
}

func TestSongResolutionService_ResolveFromURL_ExistingSongByISRC(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	mockSpotify := NewMockPlatformService("spotify")
	service.RegisterPlatform(mockSpotify)
	
	ctx := context.Background()
	url := "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
	trackID := "4iV5W9uYEdYUVa79Axb7Rh"
	
	// Create existing song with different platform
	existingSong := models.NewSong("Existing Song", "Existing Artist")
	existingSong.ISRC = TestISRC1
	existingSong.AddPlatformLink("apple_music", "123456", "https://music.apple.com/us/song/existing/123456", 1.0)
	
	// Create track info for Spotify
	trackInfo := &TrackInfo{
		Platform:   "spotify",
		ExternalID: trackID,
		URL:        url,
		Title:      "Existing Song",
		Artists:    []string{"Existing Artist"},
		ISRC:       TestISRC1,
		Available:  true,
	}
	
	// Setup mock expectations
	mockRepo.On("FindByPlatformID", ctx, "spotify", trackID).Return(nil, errors.New("not found"))
	mockSpotify.On("GetTrackByID", ctx, trackID).Return(trackInfo, nil)
	mockRepo.On("FindByISRC", ctx, TestISRC1).Return(existingSong, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*models.Song")).Return(nil)
	
	song, err := service.ResolveFromURL(ctx, url)
	
	require.NoError(t, err)
	require.NotNil(t, song)
	assert.Equal(t, existingSong.ID, song.ID)
	
	// Should have added Spotify link to existing song
	spotifyLink := song.GetPlatformLink("spotify")
	require.NotNil(t, spotifyLink, "Should have added Spotify link")
	assert.Equal(t, trackID, spotifyLink.ExternalID)
	
	mockRepo.AssertExpectations(t)
	mockSpotify.AssertExpectations(t)
}

func TestSongResolutionService_ResolveFromURL_PlatformServiceError(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	mockSpotify := NewMockPlatformService("spotify")
	service.RegisterPlatform(mockSpotify)
	
	ctx := context.Background()
	url := "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
	trackID := "4iV5W9uYEdYUVa79Axb7Rh"
	
	// Setup mock expectations
	mockRepo.On("FindByPlatformID", ctx, "spotify", trackID).Return(nil, errors.New("not found"))
	mockSpotify.On("GetTrackByID", ctx, trackID).Return(nil, errors.New("API error"))
	
	song, err := service.ResolveFromURL(ctx, url)
	
	assert.Error(t, err)
	assert.Nil(t, song)
	assert.Contains(t, err.Error(), "failed to get track info from spotify")
	
	mockRepo.AssertExpectations(t)
	mockSpotify.AssertExpectations(t)
}

func TestSongResolutionService_ResolveFromURL_RepositoryError(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	mockSpotify := NewMockPlatformService("spotify")
	service.RegisterPlatform(mockSpotify)
	
	ctx := context.Background()
	url := "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
	trackID := "4iV5W9uYEdYUVa79Axb7Rh"
	
	// Setup mock expectations - repository error
	mockRepo.On("FindByPlatformID", ctx, "spotify", trackID).Return(nil, errors.New("database error"))
	
	song, err := service.ResolveFromURL(ctx, url)
	
	assert.Error(t, err)
	assert.Nil(t, song)
	assert.Contains(t, err.Error(), "failed to check existing song")
	
	mockRepo.AssertExpectations(t)
}

func TestSongResolutionService_ResolveFromURL_SaveError(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	mockSpotify := NewMockPlatformService("spotify")
	service.RegisterPlatform(mockSpotify)
	
	ctx := context.Background()
	url := "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
	trackID := "4iV5W9uYEdYUVa79Axb7Rh"
	
	// Create track info
	trackInfo := &TrackInfo{
		Platform:   "spotify",
		ExternalID: trackID,
		URL:        url,
		Title:      "Test Song",
		Artists:    []string{"Test Artist"},
		ISRC:       TestISRC1,
		Available:  true,
	}
	
	// Setup mock expectations
	mockRepo.On("FindByPlatformID", ctx, "spotify", trackID).Return(nil, errors.New("not found"))
	mockSpotify.On("GetTrackByID", ctx, trackID).Return(trackInfo, nil)
	mockRepo.On("FindByISRC", ctx, TestISRC1).Return(nil, errors.New("not found"))
	mockRepo.On("Save", ctx, mock.AnythingOfType("*models.Song")).Return(errors.New("save error"))
	
	song, err := service.ResolveFromURL(ctx, url)
	
	assert.Error(t, err)
	assert.Nil(t, song)
	assert.Contains(t, err.Error(), "save error")
	
	mockRepo.AssertExpectations(t)
	mockSpotify.AssertExpectations(t)
}

func TestSongResolutionService_ResolveFromURL_EmptyISRC(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	mockSpotify := NewMockPlatformService("spotify")
	service.RegisterPlatform(mockSpotify)
	
	ctx := context.Background()
	url := "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
	trackID := "4iV5W9uYEdYUVa79Axb7Rh"
	
	// Create track info without ISRC
	trackInfo := &TrackInfo{
		Platform:   "spotify",
		ExternalID: trackID,
		URL:        url,
		Title:      "Song Without ISRC",
		Artists:    []string{"Unknown Artist"},
		Available:  true,
	} // No ISRC set
	
	// Setup mock expectations - should skip ISRC lookup
	mockRepo.On("FindByPlatformID", ctx, "spotify", trackID).Return(nil, errors.New("not found"))
	mockSpotify.On("GetTrackByID", ctx, trackID).Return(trackInfo, nil)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*models.Song")).Return(nil)
	
	song, err := service.ResolveFromURL(ctx, url)
	
	require.NoError(t, err)
	require.NotNil(t, song)
	assert.Equal(t, "Song Without ISRC", song.Title)
	assert.Empty(t, song.ISRC)
	
	mockRepo.AssertExpectations(t)
	mockSpotify.AssertExpectations(t)
}

func TestSongResolutionService_GetPlatformService(t *testing.T) {
	mockRepo := &MockSongRepository{}
	service := NewSongResolutionService(mockRepo)
	
	// Initially no services
	assert.Nil(t, service.GetPlatformService("spotify"))
	
	// Register a service
	mockSpotify := NewMockPlatformService("spotify")
	service.RegisterPlatform(mockSpotify)
	
	// Should return the registered service
	assert.Equal(t, mockSpotify, service.GetPlatformService("spotify"))
	
	// Non-existent service should return nil
	assert.Nil(t, service.GetPlatformService("youtube"))
}

func TestSongResolutionService_MultipleURLFormats(t *testing.T) {
	testCases := []struct {
		name     string
		url      string
		platform string
		trackID  string
	}{
		{
			name:     "Spotify with https",
			url:      "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			platform: "spotify",
			trackID:  "4iV5W9uYEdYUVa79Axb7Rh",
		},
		{
			name:     "Spotify without protocol",
			url:      "open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			platform: "spotify",
			trackID:  "4iV5W9uYEdYUVa79Axb7Rh",
		},
		{
			name:     "Apple Music song",
			url:      "https://music.apple.com/us/song/test-song/1440857781",
			platform: "apple_music",
			trackID:  "1440857781",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &MockSongRepository{}
			service := NewSongResolutionService(mockRepo)
			
			mockPlatform := NewMockPlatformService(tc.platform)
			service.RegisterPlatform(mockPlatform)
			
			ctx := context.Background()
			
			// Create track info
			trackInfo := &TrackInfo{
				Platform:   tc.platform,
				ExternalID: tc.trackID,
				URL:        tc.url,
				Title:      "Test Song",
				Artists:    []string{"Test Artist"},
				ISRC:       TestISRC1,
				Available:  true,
			}
			
			// Setup mock expectations
			mockRepo.On("FindByPlatformID", ctx, tc.platform, tc.trackID).Return(nil, errors.New("not found"))
			mockPlatform.On("GetTrackByID", ctx, tc.trackID).Return(trackInfo, nil)
			mockRepo.On("FindByISRC", ctx, TestISRC1).Return(nil, errors.New("not found"))
			mockRepo.On("Save", ctx, mock.AnythingOfType("*models.Song")).Return(nil)
			
			song, err := service.ResolveFromURL(ctx, tc.url)
			
			require.NoError(t, err)
			require.NotNil(t, song)
			
			// Verify platform link was added correctly
			platformLink := song.GetPlatformLink(tc.platform)
			require.NotNil(t, platformLink)
			assert.Equal(t, tc.trackID, platformLink.ExternalID)
			
			mockRepo.AssertExpectations(t)
			mockPlatform.AssertExpectations(t)
		})
	}
}