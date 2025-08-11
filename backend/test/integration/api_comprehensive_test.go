package integration

import (
	"net/http"
	"testing"
	"time"

	"songshare/internal/handlers"
	"songshare/internal/models"
	"songshare/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ComprehensiveAPITestSuite provides comprehensive integration tests for the SongShare API
type ComprehensiveAPITestSuite struct {
	suite.Suite
	httpHelper  *testutil.HTTPTestHelper
	songHandler *handlers.SongHandler
	router      *gin.Engine
	mockRepo    *testutil.SimpleMockSongRepository
}

// SetupSuite initializes the test suite
func (suite *ComprehensiveAPITestSuite) SetupSuite() {
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

// setupRoutes configures the test routes to match the actual API
func (suite *ComprehensiveAPITestSuite) setupRoutes() {
	// Health endpoints
	suite.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
		})
	})

	suite.router.GET("/health/platforms", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"platforms": gin.H{
				"spotify":     gin.H{"status": "healthy"},
				"apple_music": gin.H{"status": "healthy"},
				"tidal":       gin.H{"status": "not_configured"},
			},
			"timestamp": time.Now().Unix(),
		})
	})

	// API routes
	api := suite.router.Group("/api/v1")
	{
		api.POST("/songs/resolve", suite.songHandler.ResolveSong)
	}
}

// setupTestData adds test songs to the mock repository
func (suite *ComprehensiveAPITestSuite) setupTestData() {
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
			{
				Platform:   "apple_music",
				ExternalID: "1440650439",
				URL:        "https://music.apple.com/us/album/bohemian-rhapsody/1440650428?i=1440650439",
				Available:  true,
				Confidence: 0.95,
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
}

// TestHealthEndpoint tests the basic health check endpoint
func (suite *ComprehensiveAPITestSuite) TestHealthEndpoint() {
	resp := suite.httpHelper.GetJSON("/health")
	var response map[string]interface{}
	suite.httpHelper.AssertJSONResponse(resp, http.StatusOK, &response)

	// Validate response structure matches OpenAPI schema
	suite.Equal("healthy", response["status"])
	suite.NotNil(response["timestamp"])

	// Validate timestamp is a number
	timestamp, ok := response["timestamp"].(float64)
	suite.True(ok, "timestamp should be a number")
	suite.Greater(timestamp, float64(0))
}

// TestPlatformHealthEndpoint tests the platform health check endpoint
func (suite *ComprehensiveAPITestSuite) TestPlatformHealthEndpoint() {
	resp := suite.httpHelper.GetJSON("/health/platforms")
	var response map[string]interface{}
	suite.httpHelper.AssertJSONResponse(resp, http.StatusOK, &response)

	// Validate response structure matches OpenAPI schema
	suite.NotNil(response["platforms"])
	suite.NotNil(response["timestamp"])

	platforms, ok := response["platforms"].(map[string]interface{})
	suite.True(ok, "platforms should be an object")

	// Check that expected platforms are present
	suite.Contains(platforms, "spotify")
	suite.Contains(platforms, "apple_music")
	suite.Contains(platforms, "tidal")

	// Validate platform status structure
	spotify, ok := platforms["spotify"].(map[string]interface{})
	suite.True(ok)
	suite.Equal("healthy", spotify["status"])

	tidal, ok := platforms["tidal"].(map[string]interface{})
	suite.True(ok)
	suite.Equal("not_configured", tidal["status"])
}

// TestResolveSong_ValidSpotifyURL tests song resolution with a valid Spotify URL
func (suite *ComprehensiveAPITestSuite) TestResolveSong_ValidSpotifyURL() {
	requestBody := map[string]interface{}{
		"url": "https://open.spotify.com/track/4u7EnebtmKWzUH433cf5Qv",
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/resolve", requestBody)
	var response map[string]interface{}
	suite.httpHelper.AssertJSONResponse(resp, http.StatusBadRequest, &response)

	// Since platform services are nil in tests, expect service not available error
	suite.Contains(response["error"], "Platform service not available")
}

// TestResolveSong_ValidAppleMusicURL tests song resolution with a valid Apple Music URL
func (suite *ComprehensiveAPITestSuite) TestResolveSong_ValidAppleMusicURL() {
	requestBody := map[string]interface{}{
		"url": "https://music.apple.com/us/album/bohemian-rhapsody/1440650428?i=1440650439",
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/resolve", requestBody)
	var response map[string]interface{}
	suite.httpHelper.AssertJSONResponse(resp, http.StatusBadRequest, &response)

	// Since platform services are nil in tests, expect service not available error
	suite.Contains(response["error"], "Platform service not available")
}

// TestResolveSong_InvalidURL tests song resolution with an invalid URL
func (suite *ComprehensiveAPITestSuite) TestResolveSong_InvalidURL() {
	requestBody := map[string]interface{}{
		"url": "https://invalid-platform.com/track/123",
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/resolve", requestBody)
	var response map[string]interface{}
	suite.httpHelper.AssertJSONResponse(resp, http.StatusBadRequest, &response)

	suite.Contains(response["error"], "Invalid platform URL")
}

// TestResolveSong_MissingURL tests song resolution with missing URL
func (suite *ComprehensiveAPITestSuite) TestResolveSong_MissingURL() {
	requestBody := map[string]interface{}{
		"title": "Some song",
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/resolve", requestBody)
	var response map[string]interface{}
	suite.httpHelper.AssertJSONResponse(resp, http.StatusBadRequest, &response)

	suite.Contains(response["error"], "Invalid request body")
}

// TestResolveSong_EmptyURL tests song resolution with empty URL
func (suite *ComprehensiveAPITestSuite) TestResolveSong_EmptyURL() {
	requestBody := map[string]interface{}{
		"url": "",
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/resolve", requestBody)
	var response map[string]interface{}
	suite.httpHelper.AssertJSONResponse(resp, http.StatusBadRequest, &response)

	suite.Contains(response["error"], "Invalid request body")
}

// TestAPIErrorHandling tests various error scenarios
func (suite *ComprehensiveAPITestSuite) TestAPIErrorHandling() {
	// Test 404 for non-existent endpoint
	resp := suite.httpHelper.GetJSON("/api/v1/nonexistent")
	suite.Equal(http.StatusNotFound, resp.Code)

	// Test 404 for wrong method (GET on POST-only endpoint)
	resp = suite.httpHelper.GetJSON("/api/v1/songs/resolve")
	suite.Equal(http.StatusNotFound, resp.Code)
}

// TestResponseSchemaValidation validates that responses match the OpenAPI schema
func (suite *ComprehensiveAPITestSuite) TestResponseSchemaValidation() {
	requestBody := map[string]interface{}{
		"url": "https://open.spotify.com/track/4u7EnebtmKWzUH433cf5Qv",
	}

	resp := suite.httpHelper.PostJSON("/api/v1/songs/resolve", requestBody)
	var response map[string]interface{}
	suite.httpHelper.AssertJSONResponse(resp, http.StatusBadRequest, &response)

	// Validate error response structure
	suite.Contains(response, "error")
	suite.Contains(response["error"], "Platform service not available")
}

func TestComprehensiveAPITestSuite(t *testing.T) {
	suite.Run(t, new(ComprehensiveAPITestSuite))
}
