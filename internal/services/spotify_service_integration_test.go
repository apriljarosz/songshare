package services

import (
	"os"
	"testing"

	"songshare/internal/testutil/servicetest"
)

// TestSpotifyServiceIntegration runs the full platform service test suite for Spotify
func TestSpotifyServiceIntegration(t *testing.T) {
	// Check if we have Spotify credentials for integration testing
	clientID := os.Getenv("TEST_SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("TEST_SPOTIFY_CLIENT_SECRET")
	
	if clientID == "" || clientSecret == "" {
		t.Log("Skipping Spotify integration tests - TEST_SPOTIFY_CLIENT_ID and TEST_SPOTIFY_CLIENT_SECRET not set")
		
		// Run mock tests instead
		cache := servicetest.CreateTestCache()
		service := NewSpotifyService("fake-client-id", "fake-client-secret", cache)
		
		mockSuite := &servicetest.MockPlatformServiceTestSuite{
			Service:      service,
			PlatformName: "spotify",
			URLPatterns: servicetest.GenerateCommonURLTests(
				"spotify",
				"https://open.spotify.com/track",
				"4iV5W9uYEdYUVa79Axb7Rh",
			),
		}
		
		mockSuite.RunMockTestSuite(t)
		return
	}
	
	// Run full integration test suite
	cache := servicetest.CreateTestCache()
	service := NewSpotifyService(clientID, clientSecret, cache)
	
	suite := &servicetest.PlatformServiceTestSuite{
		Service:      service,
		PlatformName: "spotify",
		TestTrackID:  "4iV5W9uYEdYUVa79Axb7Rh", // Bohemian Rhapsody by Queen
		TestURL:      "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
		TestISRC:     "GBUM71507208", // Bohemian Rhapsody ISRC
		TestQueries:  servicetest.GenerateCommonSearchTests(),
		URLPatterns: append(
			servicetest.GenerateCommonURLTests(
				"spotify",
				"https://open.spotify.com/track",
				"4iV5W9uYEdYUVa79Axb7Rh",
			),
			[]servicetest.URLTestCase{
				{
					Name:        "Spotify URL with query parameters",
					URL:         "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh?si=abc123",
					ShouldMatch: true,
					ExpectedID:  "4iV5W9uYEdYUVa79Axb7Rh",
				},
				{
					Name:        "Spotify URL without subdomain",
					URL:         "spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
					ShouldMatch: true,
					ExpectedID:  "4iV5W9uYEdYUVa79Axb7Rh",
				},
			}...,
		),
		SkipISRC:   false,
		SkipSearch: false,
	}
	
	suite.RunFullTestSuite(t)
}

// BenchmarkSpotifyService runs performance benchmarks for Spotify service
func BenchmarkSpotifyService(b *testing.B) {
	clientID := os.Getenv("TEST_SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("TEST_SPOTIFY_CLIENT_SECRET")
	
	if clientID == "" || clientSecret == "" {
		b.Skip("Skipping Spotify benchmarks - credentials not provided")
	}
	
	cache := servicetest.CreateTestCache()
	service := NewSpotifyService(clientID, clientSecret, cache)
	
	suite := &servicetest.PlatformServiceBenchmarkSuite{
		Service:     service,
		TestTrackID: "4iV5W9uYEdYUVa79Axb7Rh",
		TestURL:     "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
		TestQuery: SearchQuery{
			Title:  "Bohemian Rhapsody",
			Artist: "Queen",
			Limit:  5,
		},
	}
	
	suite.RunBenchmarks(b)
}