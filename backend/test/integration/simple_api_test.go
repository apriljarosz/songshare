package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"songshare/internal/handlers"
	"songshare/internal/models"
	"songshare/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SimpleAPITestSuite provides basic integration tests for the SongShare API
type SimpleAPITestSuite struct {
	suite.Suite
	httpHelper  *testutil.HTTPTestHelper
	songHandler *handlers.SongHandler
	router      *gin.Engine
	mockRepo    *testutil.SimpleMockSongRepository
}

// SetupSuite initializes the test suite
func (suite *SimpleAPITestSuite) SetupSuite() {
	// Set up HTTP test helper
	suite.httpHelper = testutil.NewHTTPTestHelper(suite.T())

	// Create simple mock repository
	suite.mockRepo = testutil.NewSimpleMockSongRepository()
	suite.setupTestData()

	// Create song handler with mock services (nil for external services)
	suite.songHandler = handlers.NewSongHandler(
		suite.mockRepo,
		"http://localhost:8080",
		nil, // Spotify service
		nil, // Apple Music service
		nil, // Tidal service
	)

	// Set up router
	suite.router = gin.New()
	suite.setupRoutes()
	suite.httpHelper.SetRouter(suite.router)
}

// setupRoutes configures the test routes
func (suite *SimpleAPITestSuite) setupRoutes() {
	api := suite.router.Group("/api/v1")
	{
		api.POST("/songs/resolve", suite.songHandler.ResolveSong)
		api.POST("/songs/search", suite.songHandler.SearchSongs)
	}

	// Universal song links
	suite.router.GET("/api/v1/s/:isrc", suite.songHandler.RedirectToSong)

	// Health endpoint
	suite.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
		})
	})
}

// setupTestData adds test songs to the mock repository
func (suite *SimpleAPITestSuite) setupTestData() {
	// Create test song
	testSong := &models.Song{
		ID:     primitive.NewObjectID(),
		Title:  "Bohemian Rhapsody",
		Artist: "Queen",
		Album:  "A Night at the Opera",
		ISRC:   testutil.TestISRC1,
		PlatformLinks: []models.PlatformLink{
			{
				Platform:   "spotify",
				ExternalID: "4u7EnebtmKWzUH433cf5Qv",
				URL:        "https://open.spotify.com/track/4u7EnebtmKWzUH433cf5Qv",
				Available:  true,
				Confidence: 1.0,
			},
		},
		Metadata: models.SongMetadata{
			Duration: 354320,
			ImageURL: "https://i.scdn.co/image/ab67616d0000b273ce4f1737bc8a646c8c4bd25a",
			Explicit: false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	suite.mockRepo.AddTestSong(testSong)

	// Add another test song
	testSong2 := &models.Song{
		ID:     primitive.NewObjectID(),
		Title:  "We Will Rock You",
		Artist: "Queen",
		Album:  "News of the World",
		ISRC:   testutil.TestISRC2,
		PlatformLinks: []models.PlatformLink{
			{
				Platform:   "spotify",
				ExternalID: "54flyrjcdnQdco7300avMJ",
				URL:        "https://open.spotify.com/track/54flyrjcdnQdco7300avMJ",
				Available:  true,
				Confidence: 0.95,
			},
		},
		Metadata: models.SongMetadata{
			Duration: 122000,
			ImageURL: "https://i.scdn.co/image/ab67616d0000b273f4d5cc8e2c48f7b610160e7e",
			Explicit: false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	suite.mockRepo.AddTestSong(testSong2)
}

// TestHealthEndpoint tests the health check endpoint
func (suite *SimpleAPITestSuite) TestHealthEndpoint() {
	resp := suite.httpHelper.GetJSON("/health")

	var healthResponse map[string]interface{}
	suite.httpHelper.AssertJSONResponse(resp, http.StatusOK, &healthResponse)

	assert.Equal(suite.T(), "healthy", healthResponse["status"])
	assert.NotNil(suite.T(), healthResponse["timestamp"])
}

// TestSearchSongs_LocalResults tests the search endpoint with local repository results
func (suite *SimpleAPITestSuite) TestSearchSongs_LocalResults() {
	searchRequest := handlers.SearchSongsRequest{
		Query: "Queen",
		Limit: 5,
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/search", searchRequest)

	var searchResponse handlers.SearchSongsResponse
	suite.httpHelper.AssertJSONResponse(resp, http.StatusOK, &searchResponse)

	// Verify response structure
	assert.NotNil(suite.T(), searchResponse.Results)
	assert.Equal(suite.T(), searchRequest, searchResponse.Query)

	// Should have local results
	assert.Contains(suite.T(), searchResponse.Results, "local")
	localResults := searchResponse.Results["local"]

	// Should find both Queen songs
	assert.Len(suite.T(), localResults, 2)

	// Verify first result
	assert.Equal(suite.T(), "Bohemian Rhapsody", localResults[0].Title)
	assert.Equal(suite.T(), "Queen", localResults[0].Artist)
	assert.Equal(suite.T(), "A Night at the Opera", localResults[0].Album)
}

// TestSearchSongs_SpecificTitle tests searching for a specific song title
func (suite *SimpleAPITestSuite) TestSearchSongs_SpecificTitle() {
	searchRequest := handlers.SearchSongsRequest{
		Query: "Bohemian Rhapsody",
		Limit: 5,
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/search", searchRequest)

	var searchResponse handlers.SearchSongsResponse
	suite.httpHelper.AssertJSONResponse(resp, http.StatusOK, &searchResponse)

	// Should find the specific song
	localResults := searchResponse.Results["local"]
	assert.Len(suite.T(), localResults, 1)
	assert.Equal(suite.T(), "Bohemian Rhapsody", localResults[0].Title)
	assert.Equal(suite.T(), testutil.TestISRC1, localResults[0].ISRC)
}

// TestSearchSongs_EmptyQuery tests the search endpoint with an empty query
func (suite *SimpleAPITestSuite) TestSearchSongs_EmptyQuery() {
	searchRequest := handlers.SearchSongsRequest{
		Limit: 5,
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/search", searchRequest)
	suite.httpHelper.AssertErrorResponse(resp, http.StatusBadRequest, "At least one search parameter is required")
}

// TestSearchSongs_StructuredQuery tests the search endpoint with structured query
func (suite *SimpleAPITestSuite) TestSearchSongs_StructuredQuery() {
	searchRequest := handlers.SearchSongsRequest{
		Title:  "We Will Rock You",
		Artist: "Queen",
		Limit:  3,
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/search", searchRequest)

	var searchResponse handlers.SearchSongsResponse
	suite.httpHelper.AssertJSONResponse(resp, http.StatusOK, &searchResponse)

	assert.NotNil(suite.T(), searchResponse.Results)
	assert.Equal(suite.T(), searchRequest, searchResponse.Query)

	// Should find the specific song
	localResults := searchResponse.Results["local"]
	assert.Len(suite.T(), localResults, 1)
	assert.Equal(suite.T(), "We Will Rock You", localResults[0].Title)
}

// TestSearchSongs_NoResults tests search with no matching results
func (suite *SimpleAPITestSuite) TestSearchSongs_NoResults() {
	searchRequest := handlers.SearchSongsRequest{
		Query: "NonexistentSong",
		Limit: 5,
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/search", searchRequest)

	var searchResponse handlers.SearchSongsResponse
	suite.httpHelper.AssertJSONResponse(resp, http.StatusOK, &searchResponse)

	// Should have empty local results
	localResults := searchResponse.Results["local"]
	assert.Len(suite.T(), localResults, 0)
}

// TestUniversalLink_ValidISRC tests the universal link endpoint with a valid ISRC
func (suite *SimpleAPITestSuite) TestUniversalLink_ValidISRC() {
	// Test JSON response
	resp := suite.httpHelper.GetJSON("/api/v1/s/" + testutil.TestISRC1)

	// Parse the response manually since the structure might be different
	assert.Equal(suite.T(), http.StatusOK, resp.Code)

	var responseData map[string]interface{}
	err := json.Unmarshal(resp.Body.Bytes(), &responseData)
	assert.NoError(suite.T(), err)

	// Should contain song information
	assert.Contains(suite.T(), responseData, "song")
	song := responseData["song"].(map[string]interface{})
	assert.Equal(suite.T(), "Bohemian Rhapsody", song["title"])
	assert.Equal(suite.T(), testutil.TestISRC1, song["isrc"])
}

// TestUniversalLink_ValidISRC_HTML tests the universal link endpoint returning HTML
func (suite *SimpleAPITestSuite) TestUniversalLink_ValidISRC_HTML() {
	resp := suite.httpHelper.GetHTML("/api/v1/s/" + testutil.TestISRC1)
	html := suite.httpHelper.AssertHTMLResponse(resp, http.StatusOK)

	// Should contain song information
	assert.Contains(suite.T(), html, "Bohemian Rhapsody")
	assert.Contains(suite.T(), html, "Queen")
}

// TestUniversalLink_InvalidISRC tests the universal link endpoint with invalid ISRC
func (suite *SimpleAPITestSuite) TestUniversalLink_InvalidISRC() {
	resp := suite.httpHelper.GetJSON("/api/v1/s/INVALID123")
	suite.httpHelper.AssertErrorResponse(resp, http.StatusNotFound, "Song not found")
}

// TestSearchSongs_LimitValidation tests search endpoint limit validation
func (suite *SimpleAPITestSuite) TestSearchSongs_LimitValidation() {
	// Test with limit too high
	searchRequest := handlers.SearchSongsRequest{
		Query: "Queen",
		Limit: 100, // Should be capped at 50
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/search", searchRequest)

	var searchResponse handlers.SearchSongsResponse
	suite.httpHelper.AssertJSONResponse(resp, http.StatusOK, &searchResponse)

	// Verify the limit was adjusted
	assert.Equal(suite.T(), 50, searchResponse.Query.Limit)
}

// TestSearchSongs_DefaultLimit tests search endpoint default limit
func (suite *SimpleAPITestSuite) TestSearchSongs_DefaultLimit() {
	searchRequest := handlers.SearchSongsRequest{
		Query: "Queen",
		// No limit specified
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/search", searchRequest)

	var searchResponse handlers.SearchSongsResponse
	suite.httpHelper.AssertJSONResponse(resp, http.StatusOK, &searchResponse)

	// Verify default limit was applied
	assert.Equal(suite.T(), 10, searchResponse.Query.Limit)
}

// TestAPIErrorHandling tests various error conditions
func (suite *SimpleAPITestSuite) TestAPIErrorHandling() {
	// Test malformed JSON
	resp := suite.httpHelper.PostJSON("/api/v1/songs/search", "invalid-json")
	assert.Equal(suite.T(), http.StatusBadRequest, resp.Code)
}

// TestConcurrentRequests tests that the API handles concurrent requests properly
func (suite *SimpleAPITestSuite) TestConcurrentRequests() {
	const numRequests = 5
	results := make(chan int, numRequests)

	// Make concurrent search requests
	for i := 0; i < numRequests; i++ {
		go func() {
			searchRequest := handlers.SearchSongsRequest{
				Query: "Queen",
				Limit: 5,
			}
			resp := suite.httpHelper.PostJSON("/api/v1/songs/search", searchRequest)
			results <- resp.Code
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		statusCode := <-results
		if statusCode == http.StatusOK {
			successCount++
		}
	}

	// All requests should succeed
	assert.Equal(suite.T(), numRequests, successCount)
}

// TestRepositoryIntegration tests that the repository integration works correctly
func (suite *SimpleAPITestSuite) TestRepositoryIntegration() {
	// Verify we can find songs by different methods
	ctx := context.Background()

	// Test FindByISRC
	song, err := suite.mockRepo.FindByISRC(ctx, testutil.TestISRC1)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), song)
	assert.Equal(suite.T(), "Bohemian Rhapsody", song.Title)

	// Test Search
	songs, err := suite.mockRepo.Search(ctx, "Queen", 10)
	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), songs, 2)

	// Test Count
	count, err := suite.mockRepo.Count(ctx)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(2), count)
}

// Run the test suite
func TestSimpleAPITestSuite(t *testing.T) {
	suite.Run(t, new(SimpleAPITestSuite))
}
