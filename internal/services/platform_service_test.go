package services

import (
	"fmt"
	"regexp"
	"testing"

	"songshare/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePlatformURL(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		expectedPlatform string
		expectedTrackID  string
		expectError     bool
	}{
		{
			name:             "Spotify URL with https",
			url:              "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			expectedPlatform: "spotify",
			expectedTrackID:  "4iV5W9uYEdYUVa79Axb7Rh",
			expectError:      false,
		},
		{
			name:             "Spotify URL without protocol",
			url:              "open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			expectedPlatform: "spotify",
			expectedTrackID:  "4iV5W9uYEdYUVa79Axb7Rh",
			expectError:      false,
		},
		{
			name:             "Spotify URL with http",
			url:              "http://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			expectedPlatform: "spotify",
			expectedTrackID:  "4iV5W9uYEdYUVa79Axb7Rh",
			expectError:      false,
		},
		{
			name:             "Spotify URL without subdomain",
			url:              "spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			expectedPlatform: "spotify",
			expectedTrackID:  "4iV5W9uYEdYUVa79Axb7Rh",
			expectError:      false,
		},
		{
			name:             "Apple Music URL with song ID",
			url:              "https://music.apple.com/us/song/bohemian-rhapsody/1440857781",
			expectedPlatform: "apple_music",
			expectedTrackID:  "1440857781",
			expectError:      false,
		},
		{
			name:             "Apple Music URL with album and track ID",
			url:              "https://music.apple.com/us/album/a-night-at-the-opera/1440857777?i=1440857781",
			expectedPlatform: "apple_music",
			expectedTrackID:  "1440857777",
			expectError:      false,
		},
		{
			name:             "Apple Music URL without protocol",
			url:              "music.apple.com/us/song/bohemian-rhapsody/1440857781",
			expectedPlatform: "apple_music",
			expectedTrackID:  "1440857781",
			expectError:      false,
		},
		{
			name:             "Apple Music URL with different country code",
			url:              "https://music.apple.com/gb/song/bohemian-rhapsody/1440857781",
			expectedPlatform: "apple_music",
			expectedTrackID:  "1440857781",
			expectError:      false,
		},
		{
			name:        "Invalid URL",
			url:         "https://youtube.com/watch?v=fJ9rUzIMcZQ",
			expectError: true,
		},
		{
			name:        "Empty URL",
			url:         "",
			expectError: true,
		},
		{
			name:        "Malformed URL",
			url:         "not-a-url",
			expectError: true,
		},
		{
			name:        "Spotify URL without track ID",
			url:         "https://open.spotify.com/track/",
			expectError: true,
		},
		{
			name:        "Apple Music URL without track ID",
			url:         "https://music.apple.com/us/song/bohemian-rhapsody/",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			platform, trackID, err := ParsePlatformURL(tc.url)

			if tc.expectError {
				assert.Error(t, err)
				assert.Equal(t, "", platform)
				assert.Equal(t, "", trackID)
				
				// Verify error type
				var platformError *PlatformError
				assert.ErrorAs(t, err, &platformError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedPlatform, platform)
				assert.Equal(t, tc.expectedTrackID, trackID)
			}
		})
	}
}

func TestSpotifyURLPattern(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		shouldMatch bool
		expectedID  string
	}{
		{
			name:        "Standard Spotify URL",
			url:         "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			shouldMatch: true,
			expectedID:  "4iV5W9uYEdYUVa79Axb7Rh",
		},
		{
			name:        "Spotify URL with query parameters",
			url:         "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh?si=abcd1234",
			shouldMatch: true,
			expectedID:  "4iV5W9uYEdYUVa79Axb7Rh",
		},
		{
			name:        "Spotify URL without protocol",
			url:         "open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			shouldMatch: true,
			expectedID:  "4iV5W9uYEdYUVa79Axb7Rh",
		},
		{
			name:        "Spotify URL with www",
			url:         "https://www.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			shouldMatch: true,
			expectedID:  "4iV5W9uYEdYUVa79Axb7Rh",
		},
		{
			name:        "Non-Spotify URL",
			url:         "https://music.apple.com/us/song/test/123",
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := SpotifyURLPattern.Regex.FindStringSubmatch(tc.url)
			
			if tc.shouldMatch {
				require.NotEmpty(t, matches, "Expected URL to match Spotify pattern")
				require.Len(t, matches, 2, "Expected regex to capture track ID")
				assert.Equal(t, tc.expectedID, matches[SpotifyURLPattern.TrackIDIndex])
			} else {
				assert.Empty(t, matches, "Expected URL to not match Spotify pattern")
			}
		})
	}
}

func TestAppleMusicURLPattern(t *testing.T) {
	testCases := []struct {
		name        string
		url         string
		shouldMatch bool
		expectedID  string
	}{
		{
			name:        "Apple Music song URL",
			url:         "https://music.apple.com/us/song/bohemian-rhapsody/1440857781",
			shouldMatch: true,
			expectedID:  "1440857781",
		},
		{
			name:        "Apple Music album with track ID",
			url:         "https://music.apple.com/us/album/a-night-at-the-opera/1440857777?i=1440857781",
			shouldMatch: true,
			expectedID:  "1440857777",
		},
		{
			name:        "Apple Music without protocol",
			url:         "music.apple.com/us/song/bohemian-rhapsody/1440857781",
			shouldMatch: true,
			expectedID:  "1440857781",
		},
		{
			name:        "Apple Music with different country",
			url:         "https://music.apple.com/gb/song/bohemian-rhapsody/1440857781",
			shouldMatch: true,
			expectedID:  "1440857781",
		},
		{
			name:        "Apple Music album without track ID",
			url:         "https://music.apple.com/us/album/a-night-at-the-opera/1440857777",
			shouldMatch: true,
			expectedID:  "1440857777",
		},
		{
			name:        "Non-Apple Music URL",
			url:         "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := AppleMusicURLPattern.Regex.FindStringSubmatch(tc.url)
			
			if tc.shouldMatch {
				require.NotEmpty(t, matches, "Expected URL to match Apple Music pattern")
				require.Len(t, matches, 2, "Expected regex to capture track ID")
				assert.Equal(t, tc.expectedID, matches[AppleMusicURLPattern.TrackIDIndex])
			} else {
				assert.Empty(t, matches, "Expected URL to not match Apple Music pattern")
			}
		})
	}
}

func TestPlatformError(t *testing.T) {
	err := &PlatformError{
		Platform:  "spotify",
		Operation: "parse_url",
		Message:   "invalid URL format",
		URL:       "https://invalid.url",
		Err:       assert.AnError,
	}
	
	expectedMessage := "spotify parse_url failed: invalid URL format (URL: https://invalid.url) - assert.AnError general error for testing"
	assert.Equal(t, expectedMessage, err.Error())
	
	// Test Unwrap
	assert.Equal(t, assert.AnError, err.Unwrap())
}

func TestPlatformError_MinimalFields(t *testing.T) {
	err := &PlatformError{
		Platform:  "apple_music",
		Operation: "search",
	}
	
	expectedMessage := "apple_music search failed"
	assert.Equal(t, expectedMessage, err.Error())
	assert.Nil(t, err.Unwrap())
}

func TestTrackInfo_ToSong(t *testing.T) {
	releaseDate := "2023-05-15"
	trackInfo := &TrackInfo{
		Platform:    "spotify",
		ExternalID:  "4iV5W9uYEdYUVa79Axb7Rh",
		URL:         "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
		Title:       "Bohemian Rhapsody",
		Artists:     []string{"Queen"},
		Album:       "A Night at the Opera",
		ISRC:        "GBUM71505078",
		Duration:    355000,
		ReleaseDate: releaseDate,
		Genres:      []string{"Rock", "Progressive Rock"},
		Explicit:    false,
		Popularity:  90,
		ImageURL:    "https://example.com/album-art.jpg",
		Available:   true,
	}
	
	song := trackInfo.ToSong()
	
	// Verify basic song properties
	assert.Equal(t, "Bohemian Rhapsody", song.Title)
	assert.Equal(t, "Queen", song.Artist)
	assert.Equal(t, "A Night at the Opera", song.Album)
	assert.Equal(t, "GBUM71505078", song.ISRC)
	
	// Verify platform link was added
	require.Len(t, song.PlatformLinks, 1)
	platformLink := song.PlatformLinks[0]
	assert.Equal(t, "spotify", platformLink.Platform)
	assert.Equal(t, "4iV5W9uYEdYUVa79Axb7Rh", platformLink.ExternalID)
	assert.Equal(t, "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh", platformLink.URL)
	assert.Equal(t, 1.0, platformLink.Confidence)
	assert.True(t, platformLink.Available)
	
	// Verify metadata
	assert.Equal(t, 355000, song.Metadata.Duration)
	assert.Equal(t, []string{"Rock", "Progressive Rock"}, song.Metadata.Genre)
	assert.False(t, song.Metadata.Explicit)
	assert.Equal(t, 90, song.Metadata.Popularity)
	assert.Equal(t, "https://example.com/album-art.jpg", song.Metadata.ImageURL)
	
	// Verify schema version and timestamps
	assert.Equal(t, models.CurrentSchemaVersion, song.SchemaVersion)
	assert.NotZero(t, song.CreatedAt)
	assert.NotZero(t, song.UpdatedAt)
}

func TestTrackInfo_ToSong_MultipleArtists(t *testing.T) {
	trackInfo := &TrackInfo{
		Platform:   "spotify",
		ExternalID: "test-id",
		URL:        "https://open.spotify.com/track/test-id",
		Title:      "Collaboration Song",
		Artists:    []string{"Artist One", "Artist Two", "Artist Three"},
		Available:  true,
	}
	
	song := trackInfo.ToSong()
	
	// Should join multiple artists with commas
	assert.Equal(t, "Artist One, Artist Two, Artist Three", song.Artist)
}

func TestTrackInfo_ToSong_EmptyFields(t *testing.T) {
	trackInfo := &TrackInfo{
		Platform:   "apple_music",
		ExternalID: "123456",
		URL:        "https://music.apple.com/us/song/test/123456",
		Title:      "Minimal Song",
		Artists:    []string{"Solo Artist"},
		Available:  true,
	}
	
	song := trackInfo.ToSong()
	
	assert.Equal(t, "Minimal Song", song.Title)
	assert.Equal(t, "Solo Artist", song.Artist)
	assert.Empty(t, song.Album)
	assert.Empty(t, song.ISRC)
	assert.Zero(t, song.Metadata.Duration)
	assert.Empty(t, song.Metadata.Genre)
	assert.Zero(t, song.Metadata.Popularity)
	assert.Empty(t, song.Metadata.ImageURL)
}

func TestJoinArtists(t *testing.T) {
	testCases := []struct {
		name     string
		artists  []string
		expected string
	}{
		{
			name:     "Empty slice",
			artists:  []string{},
			expected: "",
		},
		{
			name:     "Single artist",
			artists:  []string{"Queen"},
			expected: "Queen",
		},
		{
			name:     "Two artists",
			artists:  []string{"Queen", "David Bowie"},
			expected: "Queen, David Bowie",
		},
		{
			name:     "Multiple artists",
			artists:  []string{"The Beatles", "George Martin", "Billy Preston"},
			expected: "The Beatles, George Martin, Billy Preston",
		},
		{
			name:     "Nil slice",
			artists:  nil,
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := joinArtists(tc.artists)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSearchQuery_EmptyFields(t *testing.T) {
	query := SearchQuery{
		Limit: 10,
	}
	
	assert.Empty(t, query.Title)
	assert.Empty(t, query.Artist)
	assert.Empty(t, query.Album)
	assert.Empty(t, query.ISRC)
	assert.Empty(t, query.Query)
	assert.Equal(t, 10, query.Limit)
}

func TestTrackInfo_DefaultValues(t *testing.T) {
	trackInfo := &TrackInfo{}
	
	assert.Empty(t, trackInfo.Platform)
	assert.Empty(t, trackInfo.ExternalID)
	assert.Empty(t, trackInfo.URL)
	assert.Empty(t, trackInfo.Title)
	assert.Empty(t, trackInfo.Artists)
	assert.Empty(t, trackInfo.Album)
	assert.Empty(t, trackInfo.ISRC)
	assert.Zero(t, trackInfo.Duration)
	assert.Empty(t, trackInfo.ReleaseDate)
	assert.Empty(t, trackInfo.Genres)
	assert.False(t, trackInfo.Explicit)
	assert.Zero(t, trackInfo.Popularity)
	assert.Empty(t, trackInfo.ImageURL)
	assert.False(t, trackInfo.Available)
}

// Benchmark tests for URL parsing performance
func BenchmarkParsePlatformURL_Spotify(b *testing.B) {
	url := "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ParsePlatformURL(url)
	}
}

func BenchmarkParsePlatformURL_AppleMusic(b *testing.B) {
	url := "https://music.apple.com/us/song/bohemian-rhapsody/1440857781"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ParsePlatformURL(url)
	}
}

func BenchmarkParsePlatformURL_Invalid(b *testing.B) {
	url := "https://youtube.com/watch?v=fJ9rUzIMcZQ"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ParsePlatformURL(url)
	}
}

// Tests for the new URL pattern registry system

func TestURLPatternRegistry_RegisterURLPattern(t *testing.T) {
	// Create a fresh registry for testing
	registry := &URLPatternRegistry{
		patterns: []URLPattern{},
	}

	// Test registering a valid pattern
	pattern := URLPattern{
		Regex:        regexp.MustCompile(`(?:https?://)?test\.com/track/([a-zA-Z0-9]+)`),
		Platform:     "test_platform",
		TrackIDIndex: 1,
		Description:  "Test platform URLs",
		Examples:     []string{"https://test.com/track/abc123"},
	}

	err := registry.RegisterURLPattern(pattern)
	assert.NoError(t, err)

	patterns := registry.GetPatterns()
	require.Len(t, patterns, 1)
	assert.Equal(t, "test_platform", patterns[0].Platform)

	// Test replacing existing pattern
	newPattern := URLPattern{
		Regex:        regexp.MustCompile(`(?:https?://)?test\.com/song/([a-zA-Z0-9]+)`),
		Platform:     "test_platform", // Same platform name
		TrackIDIndex: 1,
		Description:  "Updated test platform URLs",
		Examples:     []string{"https://test.com/song/xyz789"},
	}

	err = registry.RegisterURLPattern(newPattern)
	assert.NoError(t, err)

	patterns = registry.GetPatterns()
	require.Len(t, patterns, 1) // Should still be 1, not 2
	assert.Contains(t, patterns[0].Description, "Updated")
}

func TestURLPatternRegistry_RegisterURLPattern_ValidationErrors(t *testing.T) {
	registry := &URLPatternRegistry{
		patterns: []URLPattern{},
	}

	tests := []struct {
		name    string
		pattern URLPattern
		wantErr string
	}{
		{
			name: "nil regex",
			pattern: URLPattern{
				Platform:     "test",
				TrackIDIndex: 1,
			},
			wantErr: "regex cannot be nil",
		},
		{
			name: "empty platform name",
			pattern: URLPattern{
				Regex:        regexp.MustCompile(`test`),
				Platform:     "",
				TrackIDIndex: 1,
			},
			wantErr: "platform name cannot be empty",
		},
		{
			name: "invalid track ID index",
			pattern: URLPattern{
				Regex:        regexp.MustCompile(`test`),
				Platform:     "test",
				TrackIDIndex: 0,
			},
			wantErr: "trackIDIndex must be >= 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.RegisterURLPattern(tt.pattern)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestURLPatternRegistry_ValidatePattern(t *testing.T) {
	registry := &URLPatternRegistry{}

	tests := []struct {
		name    string
		pattern URLPattern
		wantErr bool
	}{
		{
			name: "valid pattern with examples",
			pattern: URLPattern{
				Regex:        regexp.MustCompile(`(?:https?://)?test\.com/track/([a-zA-Z0-9]+)`),
				Platform:     "test",
				TrackIDIndex: 1,
				Examples:     []string{"https://test.com/track/abc123", "test.com/track/xyz789"},
			},
			wantErr: false,
		},
		{
			name: "pattern without examples",
			pattern: URLPattern{
				Regex:        regexp.MustCompile(`test`),
				Platform:     "test",
				TrackIDIndex: 1,
				Examples:     []string{},
			},
			wantErr: false, // Should be valid since no examples to validate
		},
		{
			name: "pattern with failing example",
			pattern: URLPattern{
				Regex:        regexp.MustCompile(`(?:https?://)?test\.com/track/([a-zA-Z0-9]+)`),
				Platform:     "test",
				TrackIDIndex: 1,
				Examples:     []string{"https://other.com/track/abc123"}, // Won't match
			},
			wantErr: true,
		},
		{
			name: "pattern with empty capture group",
			pattern: URLPattern{
				Regex:        regexp.MustCompile(`(?:https?://)?test\.com/track/()`), // Empty capture group
				Platform:     "test",
				TrackIDIndex: 1,
				Examples:     []string{"https://test.com/track/"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.ValidatePattern(tt.pattern)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegisterURLPattern_ConvenienceFunction(t *testing.T) {
	// Test the global convenience function
	err := RegisterURLPattern(
		"test_global",
		`(?:https?://)?testglobal\.com/track/([a-zA-Z0-9]+)`,
		1,
		"Test global platform",
		[]string{"https://testglobal.com/track/abc123"},
	)
	assert.NoError(t, err)

	// Verify it was registered
	patterns := GetURLPatterns()
	found := false
	for _, pattern := range patterns {
		if pattern.Platform == "test_global" {
			found = true
			assert.Equal(t, "Test global platform", pattern.Description)
			break
		}
	}
	assert.True(t, found, "Pattern should have been registered globally")

	// Test with invalid regex
	err = RegisterURLPattern(
		"test_invalid",
		`[unclosed`,
		1,
		"Invalid regex",
		[]string{},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex pattern")

	// Test with validation failure
	err = RegisterURLPattern(
		"test_validation_fail",
		`valid_regex`,
		1,
		"Pattern that fails validation",
		[]string{"example_that_wont_match"},
	)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pattern validation failed")
}

func TestURLPatternRegistry_GetSupportedPlatforms(t *testing.T) {
	registry := &URLPatternRegistry{
		patterns: []URLPattern{
			{Platform: "platform_a"},
			{Platform: "platform_b"},
			{Platform: "platform_c"},
		},
	}

	platforms := registry.GetSupportedPlatforms()
	assert.Len(t, platforms, 3)
	assert.Contains(t, platforms, "platform_a")
	assert.Contains(t, platforms, "platform_b")
	assert.Contains(t, platforms, "platform_c")
}

func TestURLPatternRegistry_ThreadSafety(t *testing.T) {
	registry := &URLPatternRegistry{
		patterns: []URLPattern{},
	}

	// Test concurrent registration and reading
	const numGoroutines = 10
	const numOperations = 100

	done := make(chan bool, numGoroutines)

	// Start multiple goroutines doing reads
	for i := 0; i < numGoroutines/2; i++ {
		go func() {
			for j := 0; j < numOperations; j++ {
				_ = registry.GetPatterns()
				_ = registry.GetSupportedPlatforms()
			}
			done <- true
		}()
	}

	// Start multiple goroutines doing writes
	for i := 0; i < numGoroutines/2; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				pattern := URLPattern{
					Regex:        regexp.MustCompile(`test`),
					Platform:     fmt.Sprintf("platform_%d_%d", id, j),
					TrackIDIndex: 1,
				}
				_ = registry.RegisterURLPattern(pattern)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final state
	patterns := registry.GetPatterns()
	platforms := registry.GetSupportedPlatforms()
	assert.Equal(t, len(patterns), len(platforms))
}

// Benchmark the enhanced URL pattern system
func BenchmarkURLPatternRegistry_ParsePlatformURL(b *testing.B) {
	url := "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ParsePlatformURL(url)
	}
}

func BenchmarkURLPatternRegistry_RegisterPattern(b *testing.B) {
	registry := &URLPatternRegistry{patterns: []URLPattern{}}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pattern := URLPattern{
			Regex:        regexp.MustCompile(fmt.Sprintf(`test%d`, i)),
			Platform:     fmt.Sprintf("platform_%d", i),
			TrackIDIndex: 1,
		}
		_ = registry.RegisterURLPattern(pattern)
	}
}