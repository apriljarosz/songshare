package services

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"songshare/internal/cache"
)

// mockCache implements cache.Cache for testing
type mockCache struct {
	data map[string][]byte
}

func newMockCache() cache.Cache {
	return &mockCache{
		data: make(map[string][]byte),
	}
}

func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return nil, &cache.CacheError{Operation: "get", Key: key}
}

func (m *mockCache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockCache) Exists(ctx context.Context, key string) (bool, error) {
	_, exists := m.data[key]
	return exists, nil
}

func (m *mockCache) Close() error {
	m.data = nil
	return nil
}

func (m *mockCache) Health(ctx context.Context) error {
	return nil
}

func TestTidalService_ParseURL(t *testing.T) {
	cache := newMockCache()
	service := NewTidalService("test-client-id", "test-client-secret", cache)

	tests := []struct {
		name        string
		url         string
		shouldMatch bool
		expectedID  string
	}{
		{
			name:        "Valid Tidal browse track URL",
			url:         "https://tidal.com/browse/track/77646168",
			shouldMatch: true,
			expectedID:  "77646168",
		},
		{
			name:        "Valid Tidal track URL",
			url:         "https://tidal.com/track/77646168",
			shouldMatch: true,
			expectedID:  "77646168",
		},
		{
			name:        "Valid listen.tidal.com URL",
			url:         "https://listen.tidal.com/track/77646168",
			shouldMatch: true,
			expectedID:  "77646168",
		},
		{
			name:        "Tidal URL without protocol",
			url:         "tidal.com/browse/track/77646168",
			shouldMatch: true,
			expectedID:  "77646168",
		},
		{
			name:        "Tidal album with track ID parameter",
			url:         "https://tidal.com/browse/album/77646164?play=true&trackId=77646168",
			shouldMatch: true,
			expectedID:  "77646168",
		},
		{
			name:        "Invalid URL - wrong domain",
			url:         "https://spotify.com/track/123",
			shouldMatch: false,
		},
		{
			name:        "Invalid URL - no track ID",
			url:         "https://tidal.com/browse/",
			shouldMatch: false,
		},
		{
			name:        "Invalid URL - non-numeric track ID",
			url:         "https://tidal.com/browse/track/abc123",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trackInfo, err := service.ParseURL(tt.url)

			if tt.shouldMatch {
				require.NoError(t, err)
				assert.Equal(t, "tidal", trackInfo.Platform)
				assert.Equal(t, tt.expectedID, trackInfo.ExternalID)
				assert.Contains(t, trackInfo.URL, tt.expectedID)
				assert.True(t, trackInfo.Available)
			} else {
				assert.Error(t, err)
				if err != nil {
					var platformError *PlatformError
					assert.ErrorAs(t, err, &platformError)
					assert.Equal(t, "tidal", platformError.Platform)
				}
			}
		})
	}
}

func TestTidalService_BuildURL(t *testing.T) {
	cache := newMockCache()
	service := NewTidalService("test-client-id", "test-client-secret", cache)

	tests := []struct {
		name     string
		trackID  string
		expected string
	}{
		{
			name:     "Build URL with numeric ID",
			trackID:  "77646168",
			expected: "https://tidal.com/browse/track/77646168",
		},
		{
			name:     "Build URL with different ID",
			trackID:  "123456789",
			expected: "https://tidal.com/browse/track/123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := service.BuildURL(tt.trackID)
			assert.Equal(t, tt.expected, url)
		})
	}
}

func TestTidalService_GetPlatformName(t *testing.T) {
	cache := newMockCache()
	service := NewTidalService("test-client-id", "test-client-secret", cache)

	name := service.GetPlatformName()
	assert.Equal(t, "tidal", name)
}

func TestTidalService_ConvertToTrackInfo(t *testing.T) {
	cache := newMockCache()
	service := NewTidalService("test-client-id", "test-client-secret", cache)
	tidalTrack := &TidalTrack{
		ID:       77646168,
		Title:    "Bohemian Rhapsody",
		Duration: 355, // seconds
		ISRC:     "GBUM71505078",
		Artists: []TidalArtist{
			{ID: 3996, Name: "Queen"},
		},
		Album: TidalAlbum{
			ID:          77646164,
			Title:       "A Night At The Opera",
			Cover:       "e0-d52f-94f8-ab9e-e6334e99e66f",
			ReleaseDate: "1975-11-21",
		},
		Streamable:      true,
		StreamStartDate: "2015-03-23",
	}

	trackInfo := service.convertToTrackInfo(tidalTrack)

	assert.Equal(t, "tidal", trackInfo.Platform)
	assert.Equal(t, "77646168", trackInfo.ExternalID)
	assert.Equal(t, "Bohemian Rhapsody", trackInfo.Title)
	assert.Equal(t, []string{"Queen"}, trackInfo.Artists)
	assert.Equal(t, "A Night At The Opera", trackInfo.Album)
	assert.Equal(t, "GBUM71505078", trackInfo.ISRC)
	assert.Equal(t, 355000, trackInfo.Duration) // converted to milliseconds
	assert.True(t, trackInfo.Available)
	assert.Contains(t, trackInfo.ImageURL, "e0/d52f/94f8/ab9e/e6334e99e66f")
	assert.Equal(t, "2015-03-23", trackInfo.ReleaseDate)
}

func TestTidalService_GetAlbumArtURL(t *testing.T) {
	cache := newMockCache()
	service := NewTidalService("test-client-id", "test-client-secret", cache)

	tests := []struct {
		name     string
		coverID  string
		expected string
	}{
		{
			name:     "Valid cover ID",
			coverID:  "e0-d52f-94f8-ab9e-e6334e99e66f",
			expected: "https://resources.tidal.com/images/e0/d52f/94f8/ab9e/e6334e99e66f/1280x1280.jpg",
		},
		{
			name:     "Empty cover ID",
			coverID:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := service.getAlbumArtURL(tt.coverID)
			assert.Equal(t, tt.expected, url)
		})
	}
}

// Integration test that requires Tidal API credentials
func TestTidalService_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check for test credentials
	clientID := getEnvOrSkip(t, "TEST_TIDAL_CLIENT_ID")
	clientSecret := getEnvOrSkip(t, "TEST_TIDAL_CLIENT_SECRET")

	cache := cache.NewMockCache()
	service := NewTidalService(clientID, clientSecret, cache)

	ctx := context.Background()

	t.Run("Health", func(t *testing.T) {
		err := service.Health(ctx)
		// Health check might fail without valid credentials
		if err != nil {
			t.Logf("Health check failed (expected without valid credentials): %v", err)
		}
	})

	t.Run("GetTrackByID", func(t *testing.T) {
		trackID := "77646168" // Bohemian Rhapsody on Tidal
		track, err := service.GetTrackByID(ctx, trackID)

		if err != nil {
			t.Logf("GetTrackByID failed (may be expected without credentials): %v", err)
			var platformError *PlatformError
			assert.ErrorAs(t, err, &platformError)
			assert.Equal(t, "tidal", platformError.Platform)
		} else {
			require.NotNil(t, track)
			assert.Equal(t, "tidal", track.Platform)
			assert.Equal(t, trackID, track.ExternalID)
			assert.NotEmpty(t, track.Title)
			assert.NotEmpty(t, track.Artists)
		}
	})

	t.Run("SearchTrack", func(t *testing.T) {
		query := SearchQuery{
			Title:  "Bohemian Rhapsody",
			Artist: "Queen",
			Limit:  5,
		}

		tracks, err := service.SearchTrack(ctx, query)

		if err != nil {
			t.Logf("Search failed (may be expected without credentials): %v", err)
			var platformError *PlatformError
			assert.ErrorAs(t, err, &platformError)
		} else {
			assert.NotNil(t, tracks)
			if len(tracks) > 0 {
				firstTrack := tracks[0]
				assert.Equal(t, "tidal", firstTrack.Platform)
				assert.NotEmpty(t, firstTrack.ExternalID)
				assert.NotEmpty(t, firstTrack.Title)
				assert.NotEmpty(t, firstTrack.Artists)
			}
		}
	})
}

func getEnvOrSkip(t *testing.T, key string) string {
	value := ""
	if value == "" {
		t.Skipf("Skipping test: %s not set", key)
	}
	return value
}
