package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"songshare/internal/models"
	"songshare/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSongResolutionService is a mock for testing handlers
type MockSongResolutionService struct {
	mock.Mock
}

func (m *MockSongResolutionService) ResolveFromURL(ctx interface{}, url string) (*models.Song, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Song), args.Error(1)
}

func (m *MockSongResolutionService) RegisterPlatform(service services.PlatformService) {
	m.Called(service)
}

func (m *MockSongResolutionService) GetPlatformService(platformName string) services.PlatformService {
	args := m.Called(platformName)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(services.PlatformService)
}

// MockSongRepository is a mock for testing handlers
type MockSongRepository struct {
	mock.Mock
}

func (m *MockSongRepository) Save(ctx interface{}, song *models.Song) error {
	args := m.Called(ctx, song)
	return args.Error(0)
}

func (m *MockSongRepository) Update(ctx interface{}, song *models.Song) error {
	args := m.Called(ctx, song)
	return args.Error(0)
}

func (m *MockSongRepository) FindByID(ctx interface{}, id string) (*models.Song, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Song), args.Error(1)
}

func (m *MockSongRepository) FindByISRC(ctx interface{}, isrc string) (*models.Song, error) {
	args := m.Called(ctx, isrc)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Song), args.Error(1)
}

func (m *MockSongRepository) FindByTitleArtist(ctx interface{}, title, artist string) ([]*models.Song, error) {
	args := m.Called(ctx, title, artist)
	return args.Get(0).([]*models.Song), args.Error(1)
}

func (m *MockSongRepository) FindByPlatformID(ctx interface{}, platform, externalID string) (*models.Song, error) {
	args := m.Called(ctx, platform, externalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Song), args.Error(1)
}

func (m *MockSongRepository) Search(ctx interface{}, query string, limit int) ([]*models.Song, error) {
	args := m.Called(ctx, query, limit)
	return args.Get(0).([]*models.Song), args.Error(1)
}

func (m *MockSongRepository) FindSimilar(ctx interface{}, song *models.Song, limit int) ([]*models.Song, error) {
	args := m.Called(ctx, song, limit)
	return args.Get(0).([]*models.Song), args.Error(1)
}

func (m *MockSongRepository) FindByIDPrefix(ctx interface{}, prefix string) (*models.Song, error) {
	args := m.Called(ctx, prefix)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Song), args.Error(1)
}

func (m *MockSongRepository) FindMany(ctx interface{}, ids []string) ([]*models.Song, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).([]*models.Song), args.Error(1)
}

func (m *MockSongRepository) SaveMany(ctx interface{}, songs []*models.Song) error {
	args := m.Called(ctx, songs)
	return args.Error(0)
}

func (m *MockSongRepository) DeleteByID(ctx interface{}, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSongRepository) Count(ctx interface{}) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

// Helper functions for creating test data
func createTestSong() *models.Song {
	song := models.NewSong("Bohemian Rhapsody", "Queen")
	song.ISRC = "GBUM71505078"
	song.Album = "A Night at the Opera"
	song.Metadata.Duration = 355000
	song.Metadata.Popularity = 90
	song.AddPlatformLink("spotify", "4iV5W9uYEdYUVa79Axb7Rh", "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh", 1.0)
	song.AddPlatformLink("apple_music", "1440857781", "https://music.apple.com/us/song/bohemian-rhapsody/1440857781", 1.0)
	return song
}

func setupTestRouter(handler *SongHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	v1 := router.Group("/api/v1")
	{
		v1.POST("/songs/resolve", handler.ResolveSong)
		v1.POST("/songs/search", handler.SearchSongs)
	}
	
	router.GET("/s/:id", handler.RedirectToSong)
	
	return router
}

func TestSongHandler_ResolveSong_Success(t *testing.T) {
	mockService := &MockSongResolutionService{}
	mockRepo := &MockSongRepository{}
	
	handler := NewSongHandler(mockService, mockRepo, "https://songshare.example.com")
	router := setupTestRouter(handler)
	
	// Create test song
	testSong := createTestSong()
	
	// Setup expectations
	mockService.On("ResolveFromURL", mock.Anything, "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh").Return(testSong, nil)
	
	// Create request
	requestBody := ResolveSongRequest{
		URL: "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
	}
	body, _ := json.Marshal(requestBody)
	
	req, _ := http.NewRequest("POST", "/api/v1/songs/resolve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	
	// Assertions
	assert.Equal(t, http.StatusOK, recorder.Code)
	
	var response ResolveSongResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "Bohemian Rhapsody", response.Song.Title)
	assert.Equal(t, []string{"Queen"}, response.Song.Artists)
	assert.Equal(t, "A Night at the Opera", response.Song.Album)
	assert.Equal(t, 355000, response.Song.DurationMs)
	assert.Equal(t, "GBUM71505078", response.Song.ISRC)
	
	// Check platform links
	assert.Contains(t, response.Platforms, "spotify")
	assert.Contains(t, response.Platforms, "apple_music")
	
	spotifyLink := response.Platforms["spotify"]
	assert.Equal(t, "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh", spotifyLink.URL)
	assert.True(t, spotifyLink.Available)
	
	// Check universal link
	assert.Contains(t, response.UniversalLink, testSong.ISRC)
	
	mockService.AssertExpectations(t)
}

func TestSongHandler_ResolveSong_InvalidJSON(t *testing.T) {
	mockService := &MockSongResolutionService{}
	mockRepo := &MockSongRepository{}
	
	handler := NewSongHandler(mockService, mockRepo, "https://songshare.example.com")
	router := setupTestRouter(handler)
	
	req, _ := http.NewRequest("POST", "/api/v1/songs/resolve", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Contains(t, response, "error")
	assert.Equal(t, "Invalid request body", response["error"])
}

func TestSongHandler_ResolveSong_MissingURL(t *testing.T) {
	mockService := &MockSongResolutionService{}
	mockRepo := &MockSongRepository{}
	
	handler := NewSongHandler(mockService, mockRepo, "https://songshare.example.com")
	router := setupTestRouter(handler)
	
	// Create request without URL
	requestBody := ResolveSongRequest{}
	body, _ := json.Marshal(requestBody)
	
	req, _ := http.NewRequest("POST", "/api/v1/songs/resolve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Contains(t, response, "error")
	assert.Equal(t, "Invalid request body", response["error"])
}

func TestSongHandler_ResolveSong_ServiceError(t *testing.T) {
	mockService := &MockSongResolutionService{}
	mockRepo := &MockSongRepository{}
	
	handler := NewSongHandler(mockService, mockRepo, "https://songshare.example.com")
	router := setupTestRouter(handler)
	
	// Setup expectations with error
	mockService.On("ResolveFromURL", mock.Anything, "https://invalid.url").Return(nil, errors.New("unsupported platform"))
	
	// Create request
	requestBody := ResolveSongRequest{
		URL: "https://invalid.url",
	}
	body, _ := json.Marshal(requestBody)
	
	req, _ := http.NewRequest("POST", "/api/v1/songs/resolve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	
	// Assertions
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Contains(t, response, "error")
	assert.Equal(t, "Failed to resolve song from URL", response["error"])
	assert.Contains(t, response, "details")
	
	mockService.AssertExpectations(t)
}

func TestSongHandler_ResolveSong_NilSong(t *testing.T) {
	mockService := &MockSongResolutionService{}
	mockRepo := &MockSongRepository{}
	
	handler := NewSongHandler(mockService, mockRepo, "https://songshare.example.com")
	router := setupTestRouter(handler)
	
	// Setup expectations with nil song (no error but no result)
	mockService.On("ResolveFromURL", mock.Anything, "https://open.spotify.com/track/notfound").Return(nil, nil)
	
	// Create request
	requestBody := ResolveSongRequest{
		URL: "https://open.spotify.com/track/notfound",
	}
	body, _ := json.Marshal(requestBody)
	
	req, _ := http.NewRequest("POST", "/api/v1/songs/resolve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	
	// Assertions
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Contains(t, response, "error")
	assert.Equal(t, "Song not found", response["error"])
	
	mockService.AssertExpectations(t)
}

func TestSongHandler_ResolveSong_HTMXRequest(t *testing.T) {
	mockService := &MockSongResolutionService{}
	mockRepo := &MockSongRepository{}
	
	handler := NewSongHandler(mockService, mockRepo, "https://songshare.example.com")
	router := setupTestRouter(handler)
	
	// Create test song
	testSong := createTestSong()
	
	// Setup expectations
	mockService.On("ResolveFromURL", mock.Anything, "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh").Return(testSong, nil)
	
	// Create request with HTMX header
	requestBody := ResolveSongRequest{
		URL: "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
	}
	body, _ := json.Marshal(requestBody)
	
	req, _ := http.NewRequest("POST", "/api/v1/songs/resolve", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HX-Request", "true")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	
	// Assertions
	assert.Equal(t, http.StatusOK, recorder.Code)
	
	// Should have HX-Redirect header
	redirectHeader := recorder.Header().Get("HX-Redirect")
	assert.NotEmpty(t, redirectHeader)
	assert.Contains(t, redirectHeader, testSong.ISRC)
	
	// Response body should contain redirect URL
	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Contains(t, response, "redirect")
	
	mockService.AssertExpectations(t)
}

func TestSongHandler_SearchSongs_Success(t *testing.T) {
	mockService := &MockSongResolutionService{}
	mockRepo := &MockSongRepository{}
	
	// Create mock platform service
	mockSpotifyService := services.NewMockPlatformServiceForHandlers("spotify")
	
	handler := NewSongHandler(mockService, mockRepo, "https://songshare.example.com")
	router := setupTestRouter(handler)
	
	// Setup expectations
	mockService.On("GetPlatformService", "spotify").Return(mockSpotifyService)
	mockService.On("GetPlatformService", "apple_music").Return(nil) // Not available
	
	// Mock search results
	searchResults := []*services.TrackInfo{
		{
			Platform:   "spotify",
			ExternalID: "4iV5W9uYEdYUVa79Axb7Rh",
			URL:        "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh",
			Title:      "Bohemian Rhapsody",
			Artists:    []string{"Queen"},
			Album:      "A Night at the Opera",
			Available:  true,
		},
	}
	
	searchQuery := services.SearchQuery{
		Title:  "Bohemian Rhapsody",
		Artist: "Queen",
		Limit:  10,
	}
	
	mockSpotifyService.On("SearchTrack", mock.Anything, searchQuery).Return(searchResults, nil)
	
	// Create request
	requestBody := SearchSongsRequest{
		Title:  "Bohemian Rhapsody",
		Artist: "Queen",
		Limit:  10,
	}
	body, _ := json.Marshal(requestBody)
	
	req, _ := http.NewRequest("POST", "/api/v1/songs/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	
	// Assertions
	assert.Equal(t, http.StatusOK, recorder.Code)
	
	var response SearchSongsResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Contains(t, response.Results, "spotify")
	assert.Len(t, response.Results["spotify"], 1)
	
	result := response.Results["spotify"][0]
	assert.Equal(t, "Bohemian Rhapsody", result.Title)
	assert.Equal(t, []string{"Queen"}, result.Artists)
	assert.True(t, result.Available)
	
	mockService.AssertExpectations(t)
	mockSpotifyService.AssertExpectations(t)
}

func TestSongHandler_SearchSongs_InvalidRequest(t *testing.T) {
	mockService := &MockSongResolutionService{}
	mockRepo := &MockSongRepository{}
	
	handler := NewSongHandler(mockService, mockRepo, "https://songshare.example.com")
	router := setupTestRouter(handler)
	
	req, _ := http.NewRequest("POST", "/api/v1/songs/search", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Contains(t, response, "error")
}

func TestSongHandler_SearchSongs_NoParameters(t *testing.T) {
	mockService := &MockSongResolutionService{}
	mockRepo := &MockSongRepository{}
	
	handler := NewSongHandler(mockService, mockRepo, "https://songshare.example.com")
	router := setupTestRouter(handler)
	
	// Create request with no search parameters
	requestBody := SearchSongsRequest{}
	body, _ := json.Marshal(requestBody)
	
	req, _ := http.NewRequest("POST", "/api/v1/songs/search", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"], "At least one search parameter is required")
}

func TestSongHandler_RedirectToSong_JSON(t *testing.T) {
	mockService := &MockSongResolutionService{}
	mockRepo := &MockSongRepository{}
	
	handler := NewSongHandler(mockService, mockRepo, "https://songshare.example.com")
	router := setupTestRouter(handler)
	
	// Create test song
	testSong := createTestSong()
	testISRC := "GBUM71505078"
	
	// Setup expectations
	mockRepo.On("FindByISRC", mock.Anything, testISRC).Return(testSong, nil)
	
	req, _ := http.NewRequest("GET", "/s/"+testISRC, nil)
	req.Header.Set("Accept", "application/json")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	
	// Assertions
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
	
	var response ResolveSongResponse
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "Bohemian Rhapsody", response.Song.Title)
	assert.Contains(t, response.Platforms, "spotify")
	
	mockRepo.AssertExpectations(t)
}

func TestSongHandler_RedirectToSong_HTML(t *testing.T) {
	mockService := &MockSongResolutionService{}
	mockRepo := &MockSongRepository{}
	
	handler := NewSongHandler(mockService, mockRepo, "https://songshare.example.com")
	router := setupTestRouter(handler)
	
	// Create test song
	testSong := createTestSong()
	testISRC := "GBUM71505078"
	
	// Setup expectations
	mockRepo.On("FindByISRC", mock.Anything, testISRC).Return(testSong, nil)
	
	req, _ := http.NewRequest("GET", "/s/"+testISRC, nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	
	// Assertions
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"))
	
	html := recorder.Body.String()
	assert.Contains(t, html, "Bohemian Rhapsody")
	assert.Contains(t, html, "Queen")
	assert.Contains(t, html, "Open in Spotify")
	assert.Contains(t, html, "Open in Apple Music")
	
	mockRepo.AssertExpectations(t)
}

func TestSongHandler_RedirectToSong_NotFound(t *testing.T) {
	mockService := &MockSongResolutionService{}
	mockRepo := &MockSongRepository{}
	
	handler := NewSongHandler(mockService, mockRepo, "https://songshare.example.com")
	router := setupTestRouter(handler)
	
	testISRC := "NONEXISTENT"
	
	// Setup expectations
	mockRepo.On("FindByISRC", mock.Anything, testISRC).Return(nil, errors.New("not found"))
	
	req, _ := http.NewRequest("GET", "/s/"+testISRC, nil)
	req.Header.Set("Accept", "application/json")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	
	// Assertions
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Contains(t, response, "error")
	assert.Equal(t, "Song not found", response["error"])
	
	mockRepo.AssertExpectations(t)
}