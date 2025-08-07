package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// HTTPTestHelper provides utilities for HTTP testing
type HTTPTestHelper struct {
	t      *testing.T
	router *gin.Engine
}

// NewHTTPTestHelper creates a new HTTP test helper
func NewHTTPTestHelper(t *testing.T) *HTTPTestHelper {
	gin.SetMode(gin.TestMode)
	return &HTTPTestHelper{
		t:      t,
		router: gin.New(),
	}
}

// SetRouter sets the gin router to use for testing
func (h *HTTPTestHelper) SetRouter(router *gin.Engine) {
	h.router = router
}

// PostJSON performs a POST request with JSON payload
func (h *HTTPTestHelper) PostJSON(url string, payload interface{}) *httptest.ResponseRecorder {
	body, err := json.Marshal(payload)
	require.NoError(h.t, err, "Failed to marshal JSON payload")

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	require.NoError(h.t, err, "Failed to create HTTP request")
	
	req.Header.Set("Content-Type", "application/json")
	
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, req)
	
	return recorder
}

// GetJSON performs a GET request expecting JSON response
func (h *HTTPTestHelper) GetJSON(url string) *httptest.ResponseRecorder {
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(h.t, err, "Failed to create HTTP request")
	
	req.Header.Set("Accept", "application/json")
	
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, req)
	
	return recorder
}

// GetHTML performs a GET request expecting HTML response
func (h *HTTPTestHelper) GetHTML(url string) *httptest.ResponseRecorder {
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(h.t, err, "Failed to create HTTP request")
	
	req.Header.Set("Accept", "text/html")
	
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, req)
	
	return recorder
}

// GetWithHeaders performs a GET request with custom headers
func (h *HTTPTestHelper) GetWithHeaders(url string, headers map[string]string) *httptest.ResponseRecorder {
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(h.t, err, "Failed to create HTTP request")
	
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	
	recorder := httptest.NewRecorder()
	h.router.ServeHTTP(recorder, req)
	
	return recorder
}

// AssertJSONResponse asserts that the response is valid JSON and unmarshals it
func (h *HTTPTestHelper) AssertJSONResponse(recorder *httptest.ResponseRecorder, expectedStatus int, target interface{}) {
	require.Equal(h.t, expectedStatus, recorder.Code, "Unexpected status code")
	require.Equal(h.t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"), "Expected JSON content type")
	
	err := json.Unmarshal(recorder.Body.Bytes(), target)
	require.NoError(h.t, err, "Failed to unmarshal JSON response")
}

// AssertHTMLResponse asserts that the response is HTML
func (h *HTTPTestHelper) AssertHTMLResponse(recorder *httptest.ResponseRecorder, expectedStatus int) string {
	require.Equal(h.t, expectedStatus, recorder.Code, "Unexpected status code")
	require.Equal(h.t, "text/html; charset=utf-8", recorder.Header().Get("Content-Type"), "Expected HTML content type")
	
	return recorder.Body.String()
}

// AssertErrorResponse asserts that the response contains an error
func (h *HTTPTestHelper) AssertErrorResponse(recorder *httptest.ResponseRecorder, expectedStatus int, expectedErrorSubstring string) {
	require.Equal(h.t, expectedStatus, recorder.Code, "Unexpected status code")
	
	var errorResponse map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &errorResponse)
	require.NoError(h.t, err, "Failed to unmarshal error response")
	
	errorMessage, exists := errorResponse["error"]
	require.True(h.t, exists, "Expected error field in response")
	require.Contains(h.t, errorMessage, expectedErrorSubstring, "Error message should contain expected substring")
}

// MockHTTPServer provides a mock HTTP server for testing external API calls
type MockHTTPServer struct {
	server   *httptest.Server
	handlers map[string]http.HandlerFunc
}

// NewMockHTTPServer creates a new mock HTTP server
func NewMockHTTPServer() *MockHTTPServer {
	mock := &MockHTTPServer{
		handlers: make(map[string]http.HandlerFunc),
	}
	
	mux := http.NewServeMux()
	mux.HandleFunc("/", mock.routeRequest)
	
	mock.server = httptest.NewServer(mux)
	return mock
}

// URL returns the mock server URL
func (m *MockHTTPServer) URL() string {
	return m.server.URL
}

// Close closes the mock server
func (m *MockHTTPServer) Close() {
	m.server.Close()
}

// On registers a handler for a specific path
func (m *MockHTTPServer) On(path string, handler http.HandlerFunc) {
	m.handlers[path] = handler
}

// routeRequest routes requests to registered handlers
func (m *MockHTTPServer) routeRequest(w http.ResponseWriter, r *http.Request) {
	if handler, exists := m.handlers[r.URL.Path]; exists {
		handler(w, r)
		return
	}
	
	// Default handler returns 404
	http.NotFound(w, r)
}

// SpotifyTokenResponse creates a mock Spotify token response
func SpotifyTokenResponse() map[string]interface{} {
	return map[string]interface{}{
		"access_token": "mock-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
	}
}

// SpotifyTrackResponse creates a mock Spotify track response
func SpotifyTrackResponse(trackID, title, artist string) map[string]interface{} {
	return map[string]interface{}{
		"id":   trackID,
		"name": title,
		"artists": []map[string]interface{}{
			{
				"name": artist,
			},
		},
		"album": map[string]interface{}{
			"name": "Test Album",
			"images": []map[string]interface{}{
				{
					"url":    "https://example.com/image.jpg",
					"height": 640,
					"width":  640,
				},
			},
		},
		"duration_ms": 240000,
		"popularity":  75,
		"available_markets": []string{"US"},
		"external_ids": map[string]string{
			"isrc": TestISRC1,
		},
		"external_urls": map[string]string{
			"spotify": "https://open.spotify.com/track/" + trackID,
		},
	}
}

// SpotifySearchResponse creates a mock Spotify search response
func SpotifySearchResponse(tracks ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"tracks": map[string]interface{}{
			"items": tracks,
			"total": len(tracks),
		},
	}
}