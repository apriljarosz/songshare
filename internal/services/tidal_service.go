package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/jsonapi"
	"songshare/internal/config"
)

// TidalService implements the PlatformService interface for Tidal
type TidalService struct {
	config      *config.PlatformConfig
	httpClient  *http.Client
	accessToken string
	tokenExpiry time.Time
	tokenMu     sync.RWMutex
}

// NewTidalService creates a new Tidal service instance
func NewTidalService(cfg *config.PlatformConfig) (*TidalService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("tidal configuration is required")
	}

	if cfg.AuthMethod != config.AuthMethodOAuth2 {
		return nil, fmt.Errorf("tidal requires OAuth2 authentication, got %s", cfg.AuthMethod)
	}

	httpClient := &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}

	service := &TidalService{
		config:     cfg,
		httpClient: httpClient,
	}

	// Get initial access token
	if err := service.refreshToken(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to get initial access token: %w", err)
	}

	return service, nil
}

// GetPlatformName returns the platform name
func (t *TidalService) GetPlatformName() string {
	return "tidal"
}

// BuildURL constructs a Tidal URL from track ID
func (t *TidalService) BuildURL(trackID string) string {
	return buildTidalURL(trackID)
}

// ParseURL extracts track information from a Tidal URL
func (t *TidalService) ParseURL(url string) (*TrackInfo, error) {
	trackID, err := ParseTidalTrackID(url)
	if err != nil {
		return nil, err
	}

	// Get track info from API
	return t.GetTrackByID(context.Background(), trackID)
}

// GetTrackByID fetches track information using Tidal track ID
func (t *TidalService) GetTrackByID(ctx context.Context, trackID string) (*TrackInfo, error) {
	endpoint := fmt.Sprintf("/tracks/%s", trackID)
	params := url.Values{
		"countryCode": {"US"},
		"include":     {"artists,albums,providers"},
	}

	// Use raw request to handle JSON:API response manually
	respBody, err := t.makeRawAPIRequest(ctx, "GET", endpoint, params)
	if err != nil {
		return nil, &PlatformError{
			Platform:  "tidal",
			Operation: "get_track",
			Message:   fmt.Sprintf("failed to get track %s", trackID),
			Err:       err,
		}
	}

	// Parse single track response
	var response struct {
		Data     map[string]interface{}   `json:"data"`
		Included []map[string]interface{} `json:"included"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse track response: %w", err)
	}

	trackInfo := t.parseTrackFromResource(response.Data, response.Included)
	if trackInfo == nil {
		return nil, &PlatformError{
			Platform:  "tidal",
			Operation: "get_track",
			Message:   fmt.Sprintf("failed to parse track %s", trackID),
		}
	}

	return trackInfo, nil
}

// SearchTrack searches for tracks on Tidal
func (t *TidalService) SearchTrack(ctx context.Context, query SearchQuery) ([]*TrackInfo, error) {
	// Build search query string
	searchQuery := t.buildSearchQuery(query)
	if searchQuery == "" {
		return nil, &PlatformError{
			Platform:  "tidal",
			Operation: "search",
			Message:   "empty search query",
		}
	}

	// URL encode the search query for the path
	encodedQuery := url.QueryEscape(searchQuery)

	// Get the search result with included tracks
	endpoint := fmt.Sprintf("/searchResults/%s", encodedQuery)
	params := url.Values{
		"countryCode":    {"US"},
		"explicitFilter": {"include,exclude"},
		"include":        {"tracks,tracks.artists,tracks.album,tracks.album.coverArt"},
	}

	// For now, let's use a simpler approach - make raw HTTP request and parse manually
	respBody, err := t.makeRawAPIRequest(ctx, "GET", endpoint, params)
	if err != nil {
		return nil, &PlatformError{
			Platform:  "tidal",
			Operation: "search",
			Message:   fmt.Sprintf("search failed for query: %s", searchQuery),
			Err:       err,
		}
	}

	// Parse JSON:API response manually to extract track data
	trackInfos, err := t.parseSearchResponse(respBody)
	if err != nil {
		return nil, &PlatformError{
			Platform:  "tidal",
			Operation: "search",
			Message:   fmt.Sprintf("failed to parse search response: %v", err),
			Err:       err,
		}
	}

	return trackInfos, nil
}

// GetTrackByISRC finds a track by its ISRC code
func (t *TidalService) GetTrackByISRC(ctx context.Context, isrc string) (*TrackInfo, error) {
	if isrc == "" {
		return nil, &PlatformError{
			Platform:  "tidal",
			Operation: "search_isrc",
			Message:   "ISRC cannot be empty",
		}
	}

	// Search for tracks with the specific ISRC using proper filter format
	endpoint := "/tracks"
	params := url.Values{
		"countryCode":  {"US"},
		"filter[isrc]": {isrc},
		"include":      {"artists,albums"},
	}

	// Use raw request since ISRC returns an array
	respBody, err := t.makeRawAPIRequest(ctx, "GET", endpoint, params)
	if err != nil {
		return nil, &PlatformError{
			Platform:  "tidal",
			Operation: "search_isrc",
			Message:   fmt.Sprintf("ISRC search failed for: %s", isrc),
			Err:       err,
		}
	}

	// Parse the array response
	var response struct {
		Data     []map[string]interface{} `json:"data"`
		Included []map[string]interface{} `json:"included"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse ISRC response: %w", err)
	}

	if len(response.Data) == 0 {
		return nil, &PlatformError{
			Platform:  "tidal",
			Operation: "search_isrc",
			Message:   fmt.Sprintf("no tracks found for ISRC: %s", isrc),
		}
	}

	// Parse the first track
	trackInfo := t.parseTrackFromResource(response.Data[0], response.Included)
	if trackInfo == nil {
		return nil, &PlatformError{
			Platform:  "tidal",
			Operation: "search_isrc",
			Message:   fmt.Sprintf("failed to parse track for ISRC: %s", isrc),
		}
	}

	return trackInfo, nil
}

// Health checks if the Tidal service is healthy
func (t *TidalService) Health(ctx context.Context) error {
	// Try to make a simple API call to verify connectivity and authentication
	endpoint := "/tracks"
	params := url.Values{
		"countryCode": {"US"},
		"page[limit]": {"1"},
	}

	var tracks []TidalTrack
	if err := t.makeAPIRequest(ctx, "GET", endpoint, params, nil, &tracks); err != nil {
		return &PlatformError{
			Platform:  "tidal",
			Operation: "health_check",
			Message:   "health check failed",
			Err:       err,
		}
	}

	return nil
}

// buildSearchQuery constructs a search query string from SearchQuery
func (t *TidalService) buildSearchQuery(query SearchQuery) string {
	if query.Query != "" {
		return query.Query
	}

	var parts []string

	if query.Title != "" {
		parts = append(parts, query.Title)
	}
	if query.Artist != "" {
		parts = append(parts, query.Artist)
	}
	if query.Album != "" {
		parts = append(parts, query.Album)
	}

	return strings.Join(parts, " ")
}

// makeAPIRequest makes an authenticated request to the Tidal API
func (t *TidalService) makeAPIRequest(ctx context.Context, method, endpoint string, params url.Values, body interface{}, result interface{}) error {
	// Ensure we have a valid token
	if err := t.ensureValidToken(ctx); err != nil {
		return fmt.Errorf("failed to get valid token: %w", err)
	}

	// Build URL
	apiURL := t.config.BaseURL + endpoint
	if params != nil && len(params) > 0 {
		apiURL += "?" + params.Encode()
	}
	// Prepare request body
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, apiURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	t.tokenMu.RLock()
	accessToken := t.accessToken
	t.tokenMu.RUnlock()

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.api+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/vnd.api+json")
	}

	// Make request
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for API errors
	if resp.StatusCode >= 400 {
		var apiError TidalAPIResponse
		if err := json.Unmarshal(respBody, &apiError); err == nil && len(apiError.Errors) > 0 {
			return fmt.Errorf("tidal API error: %s - %s", apiError.Errors[0].Title, apiError.Errors[0].Detail)
		}
		return fmt.Errorf("tidal API error: %s (status: %d)", string(respBody), resp.StatusCode)
	}

	// Parse JSON:API response
	if result != nil {
		if err := jsonapi.UnmarshalPayload(bytes.NewReader(respBody), result); err != nil {
			return fmt.Errorf("failed to unmarshal JSON:API response: %w", err)
		}
	}

	return nil
}

// ensureValidToken ensures we have a valid access token
func (t *TidalService) ensureValidToken(ctx context.Context) error {
	t.tokenMu.RLock()
	isExpired := time.Now().Add(1 * time.Minute).After(t.tokenExpiry)
	t.tokenMu.RUnlock()

	if isExpired {
		return t.refreshToken(ctx)
	}

	return nil
}

// refreshToken gets a new access token using OAuth2 client credentials flow
func (t *TidalService) refreshToken(ctx context.Context) error {
	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {t.config.ClientID},
		"client_secret": {t.config.ClientSecret},
		"scope":         {"READ_SEARCH"}, // Tidal API search scope
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}

	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return fmt.Errorf("received empty access token")
	}

	// Update token info
	t.tokenMu.Lock()
	t.accessToken = tokenResp.AccessToken
	t.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	t.tokenMu.Unlock()

	return nil
}

// makeRawAPIRequest makes an API request and returns the raw response body
func (t *TidalService) makeRawAPIRequest(ctx context.Context, method, endpoint string, params url.Values) ([]byte, error) {
	// Ensure we have a valid token
	if err := t.ensureValidToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to get valid token: %w", err)
	}

	// Build URL
	apiURL := t.config.BaseURL + endpoint
	if params != nil && len(params) > 0 {
		apiURL += "?" + params.Encode()
	}
	// Create request
	req, err := http.NewRequestWithContext(ctx, method, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	t.tokenMu.RLock()
	accessToken := t.accessToken
	t.tokenMu.RUnlock()

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.api+json")

	// Make request
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for API errors
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tidal API error: %s (status: %d)", string(respBody), resp.StatusCode)
	}

	return respBody, nil
}

// parseSearchResponse parses Tidal search response and extracts tracks
func (t *TidalService) parseSearchResponse(respBody []byte) ([]*TrackInfo, error) {
	// Try to parse the response as a TidalSearchResult using jsonapi
	var searchResult TidalSearchResult
	if err := jsonapi.UnmarshalPayload(bytes.NewReader(respBody), &searchResult); err != nil {
		// Fall back to manual parsing if jsonapi fails
		return t.parseSearchResponseManually(respBody)
	}

	var trackInfos []*TrackInfo
	
	// Convert TidalTrack objects to TrackInfo
	for _, track := range searchResult.Tracks {
		if track != nil {
			trackInfo := track.ToTrackInfo()
			if trackInfo != nil {
				trackInfos = append(trackInfos, trackInfo)
			}
		}
	}
	
	return trackInfos, nil
}

// parseSearchResponseManually is a fallback parser for when jsonapi unmarshaling fails
func (t *TidalService) parseSearchResponseManually(respBody []byte) ([]*TrackInfo, error) {
	var response struct {
		Data struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
		Included []map[string]interface{} `json:"included"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	var trackInfos []*TrackInfo

	// Extract tracks from included resources
	for _, resource := range response.Included {
		if resourceType, ok := resource["type"].(string); ok && resourceType == "tracks" {
			trackInfo := t.parseTrackFromResource(resource, response.Included)
			if trackInfo != nil {
				trackInfos = append(trackInfos, trackInfo)
			}
		}
	}

	return trackInfos, nil
}

// parseTrackFromResource converts a JSON resource to TrackInfo
func (t *TidalService) parseTrackFromResource(resource map[string]interface{}, included []map[string]interface{}) *TrackInfo {
	id, _ := resource["id"].(string)

	attributes, ok := resource["attributes"].(map[string]interface{})
	if !ok {
		return nil
	}

	title, _ := attributes["title"].(string)
	duration, _ := attributes["duration"].(float64) // Duration in seconds
	isrc, _ := attributes["isrc"].(string)
	explicit, _ := attributes["explicit"].(bool)
	streamReady, _ := attributes["streamReady"].(bool)
	popularity, _ := attributes["popularity"].(float64)

	// Convert duration from seconds to milliseconds
	durationMs := int(duration * 1000)

	// Parse relationships to get artists and albums
	artists := t.parseArtistsFromRelationships(resource, included)
	albumInfo := t.parseAlbumFromRelationships(resource, included)

	return &TrackInfo{
		Platform:    "tidal",
		ExternalID:  id,
		URL:         buildTidalURL(id),
		Title:       title,
		Artists:     artists,
		Album:       albumInfo.Title,
		ISRC:        isrc,
		Duration:    durationMs,
		ReleaseDate: albumInfo.ReleaseDate,
		Explicit:    explicit,
		Popularity:  int(popularity),
		ImageURL:    albumInfo.ImageURL,
		Available:   streamReady,
	}
}

// AlbumInfo holds basic album information
type AlbumInfo struct {
	Title       string
	ReleaseDate string
	ImageURL    string
}

// parseArtistsFromRelationships extracts artist names from JSON:API relationships
func (t *TidalService) parseArtistsFromRelationships(resource map[string]interface{}, included []map[string]interface{}) []string {
	var artists []string
	
	// Get relationships from the track resource
	relationships, ok := resource["relationships"].(map[string]interface{})
	if !ok {
		return artists
	}
	
	// Get artist relationships
	artistsRel, ok := relationships["artists"].(map[string]interface{})
	if !ok {
		return artists
	}
	
	// Get the data array from artists relationship
	artistsData, ok := artistsRel["data"].([]interface{})
	if !ok {
		return artists
	}
	
	// Extract artist IDs and find them in included data
	for _, artistRef := range artistsData {
		artistRefMap, ok := artistRef.(map[string]interface{})
		if !ok {
			continue
		}
		
		artistID, ok := artistRefMap["id"].(string)
		if !ok {
			continue
		}
		
		artistType, ok := artistRefMap["type"].(string)
		if !ok || artistType != "artists" {
			continue
		}
		
		// Find the artist in included data
		for _, includedItem := range included {
			if includedItem["type"] == "artists" && includedItem["id"] == artistID {
				if attributes, ok := includedItem["attributes"].(map[string]interface{}); ok {
					if artistName, ok := attributes["name"].(string); ok && artistName != "" {
						artists = append(artists, artistName)
					}
				}
				break
			}
		}
	}
	
	return artists
}

// parseAlbumFromRelationships extracts album information from JSON:API relationships
func (t *TidalService) parseAlbumFromRelationships(resource map[string]interface{}, included []map[string]interface{}) AlbumInfo {
	albumInfo := AlbumInfo{}
	
	// Get relationships from the track resource
	relationships, ok := resource["relationships"].(map[string]interface{})
	if !ok {
		return albumInfo
	}
	
	// Get album relationships
	albumRel, ok := relationships["album"].(map[string]interface{})
	if !ok {
		return albumInfo
	}
	
	// Get the data from album relationship
	albumData, ok := albumRel["data"].(map[string]interface{})
	if !ok {
		return albumInfo
	}
	
	albumID, ok := albumData["id"].(string)
	if !ok {
		return albumInfo
	}
	
	albumType, ok := albumData["type"].(string)
	if !ok || albumType != "albums" {
		return albumInfo
	}
	
	// Find the album in included data
	for _, includedItem := range included {
		if includedItem["type"] == "albums" && includedItem["id"] == albumID {
			if attributes, ok := includedItem["attributes"].(map[string]interface{}); ok {
				// Extract album title
				if albumTitle, ok := attributes["title"].(string); ok {
					albumInfo.Title = albumTitle
				}
				
				// Extract release date
				if releaseDate, ok := attributes["releaseDate"].(string); ok {
					albumInfo.ReleaseDate = releaseDate
				}
				
				// Extract cover art URL from cover attribute or coverArt relationships
				if coverURL, ok := attributes["cover"].(string); ok && coverURL != "" {
					albumInfo.ImageURL = coverURL
				} else {
					// Try to get cover art from relationships
					albumInfo.ImageURL = t.parseCoverArtFromRelationships(includedItem, included)
				}
			}
			break
		}
	}
	
	return albumInfo
}

// parseCoverArtFromRelationships extracts cover art URL from album relationships
func (t *TidalService) parseCoverArtFromRelationships(albumResource map[string]interface{}, included []map[string]interface{}) string {
	relationships, ok := albumResource["relationships"].(map[string]interface{})
	if !ok {
		return ""
	}
	
	coverArtRel, ok := relationships["coverArt"].(map[string]interface{})
	if !ok {
		return ""
	}
	
	// Cover art can be a single item or array
	var coverArtData []interface{}
	if coverArtArray, ok := coverArtRel["data"].([]interface{}); ok {
		coverArtData = coverArtArray
	} else if coverArtItem, ok := coverArtRel["data"].(map[string]interface{}); ok {
		coverArtData = []interface{}{coverArtItem}
	}
	
	if len(coverArtData) == 0 {
		return ""
	}
	
	// Get the first cover art item
	coverArtRef, ok := coverArtData[0].(map[string]interface{})
	if !ok {
		return ""
	}
	
	coverArtID, ok := coverArtRef["id"].(string)
	if !ok {
		return ""
	}
	
	coverArtType, ok := coverArtRef["type"].(string)
	if !ok || coverArtType != "artworks" {
		return ""
	}
	
	// Find the artwork in included data
	for _, includedItem := range included {
		if includedItem["type"] == "artworks" && includedItem["id"] == coverArtID {
			if attributes, ok := includedItem["attributes"].(map[string]interface{}); ok {
				if url, ok := attributes["url"].(string); ok {
					return url
				}
			}
			break
		}
	}
	
	return ""
}
