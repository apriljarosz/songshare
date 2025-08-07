package services

import (
	"os"
	"testing"

	"songshare/internal/testutil/servicetest"
)

// TestAppleMusicServiceIntegration runs the full platform service test suite for Apple Music
func TestAppleMusicServiceIntegration(t *testing.T) {
	// Check if we have Apple Music credentials for integration testing
	keyID := os.Getenv("TEST_APPLE_MUSIC_KEY_ID")
	teamID := os.Getenv("TEST_APPLE_MUSIC_TEAM_ID")
	keyFile := os.Getenv("TEST_APPLE_MUSIC_KEY_FILE")

	if keyID == "" || teamID == "" || keyFile == "" {
		t.Log("Skipping Apple Music integration tests - TEST_APPLE_MUSIC_* credentials not set")

		// Run mock tests instead
		cache := servicetest.CreateTestCache()
		service := NewAppleMusicService("fake-key-id", "fake-team-id", "nonexistent-key.p8", cache)

		mockSuite := &servicetest.MockPlatformServiceTestSuite{
			Service:      service,
			PlatformName: "apple_music",
			URLPatterns: append(
				servicetest.GenerateCommonURLTests(
					"apple_music",
					"https://music.apple.com/us/song/bohemian-rhapsody",
					"1440857781",
				),
				[]servicetest.URLTestCase{
					{
						Name:        "Apple Music album with track ID",
						URL:         "https://music.apple.com/us/album/a-night-at-the-opera/1440857777?i=1440857781",
						ShouldMatch: true,
						ExpectedID:  "1440857777",
					},
					{
						Name:        "Apple Music different country",
						URL:         "https://music.apple.com/gb/song/bohemian-rhapsody/1440857781",
						ShouldMatch: true,
						ExpectedID:  "1440857781",
					},
				}...,
			),
		}

		mockSuite.RunMockTestSuite(t)
		return
	}

	// Run full integration test suite
	cache := servicetest.CreateTestCache()
	service := NewAppleMusicService(keyID, teamID, keyFile, cache)

	suite := &servicetest.PlatformServiceTestSuite{
		Service:      service,
		PlatformName: "apple_music",
		TestTrackID:  "1440857781", // Bohemian Rhapsody by Queen on Apple Music
		TestURL:      "https://music.apple.com/us/song/bohemian-rhapsody/1440857781",
		TestISRC:     "GBUM71507208", // Bohemian Rhapsody ISRC
		TestQueries:  servicetest.GenerateCommonSearchTests(),
		URLPatterns: append(
			[]servicetest.URLTestCase{
				{
					Name:        "Apple Music song URL",
					URL:         "https://music.apple.com/us/song/bohemian-rhapsody/1440857781",
					ShouldMatch: true,
					ExpectedID:  "1440857781",
				},
				{
					Name:        "Apple Music album with track ID",
					URL:         "https://music.apple.com/us/album/a-night-at-the-opera/1440857777?i=1440857781",
					ShouldMatch: true,
					ExpectedID:  "1440857777",
				},
				{
					Name:        "Apple Music without protocol",
					URL:         "music.apple.com/us/song/bohemian-rhapsody/1440857781",
					ShouldMatch: true,
					ExpectedID:  "1440857781",
				},
				{
					Name:        "Apple Music different country",
					URL:         "https://music.apple.com/gb/song/bohemian-rhapsody/1440857781",
					ShouldMatch: true,
					ExpectedID:  "1440857781",
				},
				{
					Name:        "Invalid URL - wrong domain",
					URL:         "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
					ShouldMatch: false,
				},
			}...,
		),
		SkipISRC:   false,
		SkipSearch: false,
	}

	suite.RunFullTestSuite(t)
}

// BenchmarkAppleMusicService runs performance benchmarks for Apple Music service
func BenchmarkAppleMusicService(b *testing.B) {
	keyID := os.Getenv("TEST_APPLE_MUSIC_KEY_ID")
	teamID := os.Getenv("TEST_APPLE_MUSIC_TEAM_ID")
	keyFile := os.Getenv("TEST_APPLE_MUSIC_KEY_FILE")

	if keyID == "" || teamID == "" || keyFile == "" {
		b.Skip("Skipping Apple Music benchmarks - credentials not provided")
	}

	cache := servicetest.CreateTestCache()
	service := NewAppleMusicService(keyID, teamID, keyFile, cache)

	suite := &servicetest.PlatformServiceBenchmarkSuite{
		Service:     service,
		TestTrackID: "1440857781",
		TestURL:     "https://music.apple.com/us/song/bohemian-rhapsody/1440857781",
		TestQuery: SearchQuery{
			Title:  "Bohemian Rhapsody",
			Artist: "Queen",
			Limit:  5,
		},
	}

	suite.RunBenchmarks(b)
}
