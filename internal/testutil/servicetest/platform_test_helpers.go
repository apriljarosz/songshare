package servicetest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PlatformServiceTestSuite provides a comprehensive test suite for platform services
type PlatformServiceTestSuite struct {
	Service      PlatformService
	PlatformName string
	TestTrackID  string          // A known track ID for testing
	TestURL      string          // A known URL for testing
	TestISRC     string          // A known ISRC for testing
	TestQueries  []TestQuery     // Test search queries
	URLPatterns  []URLTestCase   // URL pattern test cases
	SkipISRC     bool            // Skip ISRC tests if platform doesn't support it
	SkipSearch   bool            // Skip search tests if not implemented
}

// TestQuery represents a test search query
type TestQuery struct {
	Name     string
	Query    SearchQuery
	Expected ExpectedResult
}

// URLTestCase represents a URL parsing test case
type URLTestCase struct {
	Name        string
	URL         string
	ShouldMatch bool
	ExpectedID  string
}

// ExpectedResult represents expected search/track results
type ExpectedResult struct {
	MinResults  int    // Minimum number of results expected
	ShouldFind  bool   // Whether we expect to find results
	TrackTitle  string // Expected track title (partial match)
	TrackArtist string // Expected track artist (partial match)
}

// RunFullTestSuite runs all tests for a platform service
func (suite *PlatformServiceTestSuite) RunFullTestSuite(t *testing.T) {
	t.Run("GetPlatformName", suite.TestGetPlatformName)
	t.Run("Health", suite.TestHealth)
	t.Run("ParseURL", suite.TestParseURL)
	t.Run("BuildURL", suite.TestBuildURL)
	
	if suite.TestTrackID != "" {
		t.Run("GetTrackByID", suite.TestGetTrackByID)
	}
	
	if !suite.SkipISRC && suite.TestISRC != "" {
		t.Run("GetTrackByISRC", suite.TestGetTrackByISRC)
	}
	
	if !suite.SkipSearch && len(suite.TestQueries) > 0 {
		t.Run("SearchTrack", suite.TestSearchTrack)
	}
	
	t.Run("ErrorHandling", suite.TestErrorHandling)
	t.Run("Caching", suite.TestCaching)
}

// TestGetPlatformName tests the GetPlatformName method
func (suite *PlatformServiceTestSuite) TestGetPlatformName(t *testing.T) {
	name := suite.Service.GetPlatformName()
	assert.Equal(t, suite.PlatformName, name)
	assert.NotEmpty(t, name)
}

// TestHealth tests the Health method
func (suite *PlatformServiceTestSuite) TestHealth(t *testing.T) {
	ctx := context.Background()
	err := suite.Service.Health(ctx)
	
	// Health should either pass or fail with a descriptive error
	if err != nil {
		t.Logf("Health check failed (this may be expected if no credentials are provided): %v", err)
		// Verify it's a platform error
		var platformError *PlatformError
		assert.ErrorAs(t, err, &platformError)
		assert.Equal(t, suite.PlatformName, platformError.Platform)
	}
}

// TestParseURL tests URL parsing functionality
func (suite *PlatformServiceTestSuite) TestParseURL(t *testing.T) {
	if len(suite.URLPatterns) == 0 {
		t.Skip("No URL patterns provided for testing")
	}
	
	for _, testCase := range suite.URLPatterns {
		t.Run(testCase.Name, func(t *testing.T) {
			trackInfo, err := suite.Service.ParseURL(testCase.URL)
			
			if testCase.ShouldMatch {
				require.NoError(t, err)
				assert.Equal(t, suite.PlatformName, trackInfo.Platform)
				assert.Equal(t, testCase.ExpectedID, trackInfo.ExternalID)
				assert.Contains(t, trackInfo.URL, testCase.ExpectedID)
				assert.True(t, trackInfo.Available)
			} else {
				assert.Error(t, err)
				var platformError *PlatformError
				assert.ErrorAs(t, err, &platformError)
			}
		})
	}
}

// TestBuildURL tests URL building functionality
func (suite *PlatformServiceTestSuite) TestBuildURL(t *testing.T) {
	if suite.TestTrackID == "" {
		t.Skip("No test track ID provided")
	}
	
	url := suite.Service.BuildURL(suite.TestTrackID)
	assert.NotEmpty(t, url)
	assert.Contains(t, url, suite.TestTrackID)
	assert.Contains(t, url, "http")
}

// TestGetTrackByID tests fetching track by ID (requires API credentials)
func (suite *PlatformServiceTestSuite) TestGetTrackByID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API test in short mode")
	}
	
	ctx := context.Background()
	trackInfo, err := suite.Service.GetTrackByID(ctx, suite.TestTrackID)
	
	if err != nil {
		t.Logf("GetTrackByID failed (may be expected without credentials): %v", err)
		// Verify error structure
		var platformError *PlatformError
		assert.ErrorAs(t, err, &platformError)
		assert.Equal(t, suite.PlatformName, platformError.Platform)
		return
	}
	
	// If successful, verify track info
	require.NotNil(t, trackInfo)
	assert.Equal(t, suite.PlatformName, trackInfo.Platform)
	assert.Equal(t, suite.TestTrackID, trackInfo.ExternalID)
	assert.NotEmpty(t, trackInfo.Title)
	assert.NotEmpty(t, trackInfo.Artists)
	assert.NotEmpty(t, trackInfo.URL)
	assert.True(t, trackInfo.Available)
}

// TestGetTrackByISRC tests ISRC-based track lookup
func (suite *PlatformServiceTestSuite) TestGetTrackByISRC(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API test in short mode")
	}
	
	ctx := context.Background()
	trackInfo, err := suite.Service.GetTrackByISRC(ctx, suite.TestISRC)
	
	if err != nil {
		t.Logf("GetTrackByISRC failed (may be expected without credentials): %v", err)
		var platformError *PlatformError
		assert.ErrorAs(t, err, &platformError)
		return
	}
	
	require.NotNil(t, trackInfo)
	assert.Equal(t, suite.PlatformName, trackInfo.Platform)
	assert.Equal(t, suite.TestISRC, trackInfo.ISRC)
	assert.NotEmpty(t, trackInfo.Title)
	assert.NotEmpty(t, trackInfo.Artists)
}

// TestSearchTrack tests search functionality
func (suite *PlatformServiceTestSuite) TestSearchTrack(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping API test in short mode")
	}
	
	ctx := context.Background()
	
	for _, testQuery := range suite.TestQueries {
		t.Run(testQuery.Name, func(t *testing.T) {
			tracks, err := suite.Service.SearchTrack(ctx, testQuery.Query)
			
			if err != nil {
				t.Logf("Search failed (may be expected without credentials): %v", err)
				var platformError *PlatformError
				assert.ErrorAs(t, err, &platformError)
				return
			}
			
			if testQuery.Expected.ShouldFind {
				assert.GreaterOrEqual(t, len(tracks), testQuery.Expected.MinResults)
				
				if len(tracks) > 0 {
					firstTrack := tracks[0]
					assert.Equal(t, suite.PlatformName, firstTrack.Platform)
					assert.NotEmpty(t, firstTrack.ExternalID)
					assert.NotEmpty(t, firstTrack.URL)
					assert.NotEmpty(t, firstTrack.Title)
					assert.NotEmpty(t, firstTrack.Artists)
					
					// Check expected content if provided
					if testQuery.Expected.TrackTitle != "" {
						found := false
						for _, track := range tracks {
							if contains(track.Title, testQuery.Expected.TrackTitle) {
								found = true
								break
							}
						}
						assert.True(t, found, "Expected to find track with title containing: %s", testQuery.Expected.TrackTitle)
					}
					
					if testQuery.Expected.TrackArtist != "" {
						found := false
						for _, track := range tracks {
							for _, artist := range track.Artists {
								if contains(artist, testQuery.Expected.TrackArtist) {
									found = true
									break
								}
							}
							if found {
								break
							}
						}
						assert.True(t, found, "Expected to find track with artist containing: %s", testQuery.Expected.TrackArtist)
					}
				}
			} else {
				assert.Empty(t, tracks)
			}
		})
	}
}

// TestErrorHandling tests various error conditions
func (suite *PlatformServiceTestSuite) TestErrorHandling(t *testing.T) {
	ctx := context.Background()
	
	t.Run("InvalidTrackID", func(t *testing.T) {
		_, err := suite.Service.GetTrackByID(ctx, "invalid_track_id_12345")
		if err != nil {
			var platformError *PlatformError
			assert.ErrorAs(t, err, &platformError)
			assert.Equal(t, suite.PlatformName, platformError.Platform)
		}
	})
	
	t.Run("InvalidISRC", func(t *testing.T) {
		if suite.SkipISRC {
			t.Skip("ISRC not supported by this platform")
		}
		
		_, err := suite.Service.GetTrackByISRC(ctx, "INVALID_ISRC")
		if err != nil {
			var platformError *PlatformError
			assert.ErrorAs(t, err, &platformError)
		}
	})
	
	t.Run("EmptySearch", func(t *testing.T) {
		if suite.SkipSearch {
			t.Skip("Search not supported by this platform")
		}
		
		query := SearchQuery{Limit: 5}
		tracks, err := suite.Service.SearchTrack(ctx, query)
		
		// Should either return results or an error, never panic
		if err != nil {
			var platformError *PlatformError
			assert.ErrorAs(t, err, &platformError)
		} else {
			// Empty search might return results or empty list
			assert.NotNil(t, tracks)
		}
	})
}

// TestCaching tests that caching is working properly
func (suite *PlatformServiceTestSuite) TestCaching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping caching test in short mode")
	}
	
	if suite.TestTrackID == "" {
		t.Skip("No test track ID provided for caching test")
	}
	
	ctx := context.Background()
	
	// First call (should hit API)
	track1, err1 := suite.Service.GetTrackByID(ctx, suite.TestTrackID)
	
	// Second call (should hit cache if implemented)
	track2, err2 := suite.Service.GetTrackByID(ctx, suite.TestTrackID)
	
	// Both calls should have same result
	assert.Equal(t, err1 != nil, err2 != nil, "Both calls should have same error status")
	
	if err1 == nil && err2 == nil {
		assert.Equal(t, track1.Title, track2.Title)
		assert.Equal(t, track1.ExternalID, track2.ExternalID)
	}
}

// MockPlatformServiceTestSuite provides tests that can run without API credentials
type MockPlatformServiceTestSuite struct {
	Service      PlatformService
	PlatformName string
	URLPatterns  []URLTestCase
}

// RunMockTestSuite runs tests that don't require API calls
func (suite *MockPlatformServiceTestSuite) RunMockTestSuite(t *testing.T) {
	t.Run("GetPlatformName", func(t *testing.T) {
		name := suite.Service.GetPlatformName()
		assert.Equal(t, suite.PlatformName, name)
		assert.NotEmpty(t, name)
	})
	
	t.Run("ParseURL", func(t *testing.T) {
		for _, testCase := range suite.URLPatterns {
			t.Run(testCase.Name, func(t *testing.T) {
				trackInfo, err := suite.Service.ParseURL(testCase.URL)
				
				if testCase.ShouldMatch {
					require.NoError(t, err)
					assert.Equal(t, suite.PlatformName, trackInfo.Platform)
					assert.Equal(t, testCase.ExpectedID, trackInfo.ExternalID)
				} else {
					assert.Error(t, err)
				}
			})
		}
	})
	
	t.Run("BuildURL", func(t *testing.T) {
		testID := "test123"
		url := suite.Service.BuildURL(testID)
		assert.NotEmpty(t, url)
		assert.Contains(t, url, testID)
	})
}

// Helper functions

// contains performs case-insensitive substring check
func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	if len(haystack) == 0 {
		return false
	}
	
	// Convert to lowercase for case-insensitive comparison
	haystackLower := strings.ToLower(haystack)
	needleLower := strings.ToLower(needle)
	
	return strings.Contains(haystackLower, needleLower)
}

// Cache interface for testing
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Close() error
}

// TestCache is a simple in-memory cache for testing
type TestCache struct {
	data map[string][]byte
}

func (c *TestCache) Get(ctx context.Context, key string) ([]byte, error) {
	if val, ok := c.data[key]; ok {
		return val, nil
	}
	return nil, fmt.Errorf("key not found")
}

func (c *TestCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if c.data == nil {
		c.data = make(map[string][]byte)
	}
	c.data[key] = value
	return nil
}

func (c *TestCache) Delete(ctx context.Context, key string) error {
	delete(c.data, key)
	return nil
}

func (c *TestCache) Close() error {
	return nil
}

// CreateTestCache creates a cache instance for testing
func CreateTestCache() Cache {
	return &TestCache{
		data: make(map[string][]byte),
	}
}

// Common test data generators

// GenerateCommonURLTests returns common URL test cases
func GenerateCommonURLTests(platform, baseURL, validTrackID string) []URLTestCase {
	return []URLTestCase{
		{
			Name:        fmt.Sprintf("Valid %s URL with HTTPS", platform),
			URL:         fmt.Sprintf("%s/%s", baseURL, validTrackID),
			ShouldMatch: true,
			ExpectedID:  validTrackID,
		},
		{
			Name:        fmt.Sprintf("Valid %s URL without protocol", platform),
			URL:         fmt.Sprintf("%s/%s", baseURL[8:], validTrackID), // Remove https://
			ShouldMatch: true,
			ExpectedID:  validTrackID,
		},
		{
			Name:        "Invalid URL - wrong domain",
			URL:         "https://example.com/track/123",
			ShouldMatch: false,
		},
		{
			Name:        "Invalid URL - empty",
			URL:         "",
			ShouldMatch: false,
		},
		{
			Name:        "Invalid URL - malformed",
			URL:         "not-a-url",
			ShouldMatch: false,
		},
	}
}

// GenerateCommonSearchTests returns common search test cases
func GenerateCommonSearchTests() []TestQuery {
	return []TestQuery{
		{
			Name: "Search by title and artist",
			Query: SearchQuery{
				Title:  "Bohemian Rhapsody",
				Artist: "Queen",
				Limit:  5,
			},
			Expected: ExpectedResult{
				ShouldFind:  true,
				MinResults:  1,
				TrackTitle:  "Bohemian Rhapsody",
				TrackArtist: "Queen",
			},
		},
		{
			Name: "Search by title only",
			Query: SearchQuery{
				Title: "Shape of You",
				Limit: 10,
			},
			Expected: ExpectedResult{
				ShouldFind: true,
				MinResults: 1,
			},
		},
		{
			Name: "Search with no results expected",
			Query: SearchQuery{
				Title:  "NonexistentSong12345",
				Artist: "NonexistentArtist12345",
				Limit:  5,
			},
			Expected: ExpectedResult{
				ShouldFind: false,
			},
		},
	}
}

// PlatformServiceBenchmarkSuite provides benchmark tests for platform services
type PlatformServiceBenchmarkSuite struct {
	Service     PlatformService
	TestTrackID string
	TestURL     string
	TestQuery   SearchQuery
}

// RunBenchmarks runs performance benchmarks for platform services
func (suite *PlatformServiceBenchmarkSuite) RunBenchmarks(b *testing.B) {
	if suite.TestURL != "" {
		b.Run("ParseURL", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = suite.Service.ParseURL(suite.TestURL)
			}
		})
	}
	
	if suite.TestTrackID != "" {
		b.Run("BuildURL", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = suite.Service.BuildURL(suite.TestTrackID)
			}
		})
		
		b.Run("GetTrackByID", func(b *testing.B) {
			if testing.Short() {
				b.Skip("Skipping API benchmark in short mode")
			}
			
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = suite.Service.GetTrackByID(ctx, suite.TestTrackID)
			}
		})
	}
	
	b.Run("SearchTrack", func(b *testing.B) {
		if testing.Short() {
			b.Skip("Skipping API benchmark in short mode")
		}
		
		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = suite.Service.SearchTrack(ctx, suite.TestQuery)
		}
	})
}