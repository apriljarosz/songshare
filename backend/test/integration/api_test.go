package integration

import (
	"encoding/json"
	"testing"

	"songshare/internal/handlers"
	"songshare/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// APITestSuite provides integration tests for the SongShare API
type APITestSuite struct {
	suite.Suite
	httpHelper  *testutil.HTTPTestHelper
	songHandler *handlers.SongHandler
	router      *gin.Engine
	mockRepo    *testutil.MockSongRepository
}

// SetupSuite initializes the test suite
func (suite *APITestSuite) SetupSuite() {
	// Set up HTTP test helper
	suite.httpHelper = testutil.NewHTTPTestHelper(suite.T())

	// Set up mock repository
	suite.mockRepo = &testutil.MockSongRepository{}

	// Create song handler with mock services
	suite.songHandler = handlers.NewSongHandler(
		suite.mockRepo,
		"https://songshare.example.com",
		nil, // spotify service - nil for basic tests
		nil, // apple music service - nil for basic tests
		nil, // tidal service - nil for basic tests
	)

	// Set up router
	suite.router = gin.New()
	api := suite.router.Group("/api/v1")
	{
		api.POST("/songs/resolve", suite.songHandler.ResolveSong)
		api.POST("/songs/search", suite.songHandler.SearchSongs)
	}

	// Universal song links
	suite.router.GET("/api/v1/s/:isrc", suite.songHandler.RedirectToSong)

	suite.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"timestamp": 1234567890,
		})
	})

	// Set the router for the HTTP helper
	suite.httpHelper.SetRouter(suite.router)
}

// TearDownSuite cleans up after tests
func (suite *APITestSuite) TearDownSuite() {
	// Clean up any resources if needed
}

// TestHealthEndpoint tests the health check endpoint
func (suite *APITestSuite) TestHealthEndpoint() {
	response := suite.httpHelper.GetJSON("/health")

	assert.Equal(suite.T(), 200, response.Code)

	var healthResponse map[string]interface{}
	err := json.Unmarshal(response.Body.Bytes(), &healthResponse)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "healthy", healthResponse["status"])
	assert.NotNil(suite.T(), healthResponse["timestamp"])
}

// TestResolveSongInvalidURL tests song resolution with invalid URL
func (suite *APITestSuite) TestResolveSongInvalidURL() {
	requestBody := handlers.ResolveSongRequest{
		URL: "https://invalid-platform.com/track/123",
	}

	response := suite.httpHelper.PostJSON("/api/v1/songs/resolve", requestBody)

	assert.Equal(suite.T(), 400, response.Code)

	var errorResponse map[string]interface{}
	err := json.Unmarshal(response.Body.Bytes(), &errorResponse)
	require.NoError(suite.T(), err)

	assert.Contains(suite.T(), errorResponse["error"], "Invalid platform URL")
}

// TestResolveSongMissingURL tests song resolution with missing URL
func (suite *APITestSuite) TestResolveSongMissingURL() {
	requestBody := handlers.ResolveSongRequest{
		URL: "",
	}

	response := suite.httpHelper.PostJSON("/api/v1/songs/resolve", requestBody)

	assert.Equal(suite.T(), 400, response.Code)
}

// TestSearchSongsEmptyQuery tests search with empty query
func (suite *APITestSuite) TestSearchSongsEmptyQuery() {
	requestBody := handlers.SearchSongsRequest{
		Query: "",
		Limit: 10,
	}

	response := suite.httpHelper.PostJSON("/api/v1/songs/search", requestBody)

	// Should return 400 for empty query
	assert.Equal(suite.T(), 400, response.Code)

	var errorResponse map[string]interface{}
	err := json.Unmarshal(response.Body.Bytes(), &errorResponse)
	require.NoError(suite.T(), err)

	assert.Contains(suite.T(), errorResponse["error"], "At least one search parameter is required")
}

// TestRedirectToSongNotFound tests redirect with non-existent song
func (suite *APITestSuite) TestRedirectToSongNotFound() {
	// Mock repository to return not found
	suite.mockRepo.On("FindByISRC", mock.Anything, "NONEXISTENT").Return(nil, nil)
	suite.mockRepo.On("FindByID", mock.Anything, "NONEXISTENT").Return(nil, nil)

	response := suite.httpHelper.GetJSON("/api/v1/s/NONEXISTENT")

	assert.Equal(suite.T(), 404, response.Code)

	var errorResponse map[string]interface{}
	err := json.Unmarshal(response.Body.Bytes(), &errorResponse)
	require.NoError(suite.T(), err)

	assert.Contains(suite.T(), errorResponse["error"], "Song not found")
}

// Run the test suite
func TestAPITestSuite(t *testing.T) {
	suite.Run(t, new(APITestSuite))
}
