package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"songshare/internal/models"
	"songshare/internal/repositories"
	"songshare/internal/services"

	"github.com/gin-gonic/gin"
)

// ResolveSongRequest represents the request to resolve a song from a platform URL
type ResolveSongRequest struct {
	URL string `json:"url" binding:"required"`
}

// ResolveSongResponse represents the response for song resolution
type ResolveSongResponse struct {
	Song          SongMetadata            `json:"song"`
	Platforms     map[string]PlatformLink `json:"platforms"`
	UniversalLink string                  `json:"universal_link"`
}

// SongMetadata represents song metadata in API responses
type SongMetadata struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Artists     []string `json:"artists"`
	Album       string   `json:"album,omitempty"`
	DurationMs  int      `json:"duration_ms,omitempty"`
	ReleaseDate string   `json:"release_date,omitempty"`
	ISRC        string   `json:"isrc,omitempty"`
	ImageURL    string   `json:"image_url,omitempty"`
}

// PlatformLink represents a platform link in API responses
type PlatformLink struct {
	URL       string `json:"url"`
	Available bool   `json:"available"`
	Platform  string `json:"platform"`
}

// SearchSongsRequest represents the request to search for songs
type SearchSongsRequest struct {
	Title    string `json:"title,omitempty"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	Query    string `json:"query,omitempty"`    // Free-form search query
	ISRC     string `json:"isrc,omitempty"`     // ISRC code for exact matching
	Platform string `json:"platform,omitempty"` // Optional: "spotify", "apple_music", or empty for both
	Limit    int    `json:"limit,omitempty"`    // Max results per platform (default: 10)
}

// SearchResult represents a single search result
type SearchResult struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album,omitempty"`
	Platform    string `json:"platform"`
	URL         string `json:"url"`
	ImageURL    string `json:"image_url,omitempty"`
	DurationMs  int    `json:"duration_ms,omitempty"`
	ReleaseDate string `json:"release_date,omitempty"`
	ISRC        string `json:"isrc,omitempty"`
	Explicit    bool   `json:"explicit,omitempty"`
}

// SearchSongsResponse represents the response for search results
type SearchSongsResponse struct {
	Results map[string][]SearchResult `json:"results"` // platform -> results
	Query   SearchSongsRequest        `json:"query"`   // Echo back the query for reference
}

// Simple search cache entry
type searchCacheEntry struct {
	results   []SearchResult
	timestamp time.Time
}

// Simple search cache (5-minute TTL)
type searchCache struct {
	entries map[string]searchCacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
}

func newSearchCache() *searchCache {
	return &searchCache{
		entries: make(map[string]searchCacheEntry),
		ttl:     5 * time.Minute,
	}
}

func (sc *searchCache) get(key string) ([]SearchResult, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	entry, exists := sc.entries[key]
	if !exists || time.Since(entry.timestamp) > sc.ttl {
		return nil, false
	}
	return entry.results, true
}

func (sc *searchCache) set(key string, results []SearchResult) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.entries[key] = searchCacheEntry{
		results:   results,
		timestamp: time.Now(),
	}

	// Clean up old entries periodically (simple cleanup)
	if len(sc.entries) > 1000 {
		for k, v := range sc.entries {
			if time.Since(v.timestamp) > sc.ttl {
				delete(sc.entries, k)
			}
		}
	}
}

// SongHandler handles song-related requests
type SongHandler struct {
	songRepository    repositories.SongRepository
	baseURL           string
	spotifyService    services.PlatformService
	appleMusicService services.PlatformService
	tidalService      services.PlatformService
	searchCache       *searchCache
}

// NewSongHandler creates a new song handler
func NewSongHandler(songRepository repositories.SongRepository, baseURL string, spotifyService, appleMusicService, tidalService services.PlatformService) *SongHandler {
	return &SongHandler{
		songRepository:    songRepository,
		baseURL:           baseURL,
		spotifyService:    spotifyService,
		appleMusicService: appleMusicService,
		tidalService:      tidalService,
		searchCache:       newSearchCache(),
	}
}

// ResolveSong handles POST /api/v1/songs/resolve
func (h *SongHandler) ResolveSong(c *gin.Context) {
	var req ResolveSongRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Parse the platform URL
	platform, trackID, err := services.ParsePlatformURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid platform URL",
			"details": err.Error(),
		})
		return
	}

	// Get the platform service
	var platformService services.PlatformService
	switch platform {
	case "spotify":
		platformService = h.spotifyService
	case "apple_music":
		platformService = h.appleMusicService
	case "tidal":
		platformService = h.tidalService
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unsupported platform: " + platform,
		})
		return
	}

	if platformService == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Platform service not available: " + platform,
		})
		return
	}

	// Resolve the song
	song, err := h.resolveSongFromPlatform(c.Request.Context(), platformService, trackID)
	if err != nil {
		slog.Error("Failed to resolve song", "url", req.URL, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to resolve song from URL",
			"details": err.Error(),
		})
		return
	}

	if song == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Song not found",
		})
		return
	}

	// Check if client wants JSON response (API call)
	acceptHeader := c.GetHeader("Accept")
	if strings.Contains(acceptHeader, "application/json") {
		// Return JSON response for API clients
		c.JSON(http.StatusOK, gin.H{
			"song": song,
		})
		return
	}

	// Get user agent to determine preferred platform
	userAgent := c.GetHeader("User-Agent")
	preferredPlatform := h.getPreferredPlatformFromUserAgent(userAgent)

	// Find the best platform link to redirect to
	var redirectURL string
	if preferredPlatform != "" {
		for _, link := range song.PlatformLinks {
			if link.Platform == preferredPlatform && link.Available {
				redirectURL = link.URL
				break
			}
		}
	}

	// Fallback to first available platform
	if redirectURL == "" {
		for _, link := range song.PlatformLinks {
			if link.Available {
				redirectURL = link.URL
				break
			}
		}
	}

	if redirectURL == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No available platform links for this song",
		})
		return
	}

	c.Redirect(http.StatusFound, redirectURL)
}

// SearchSongs handles POST /api/v1/songs/search
func (h *SongHandler) SearchSongs(c *gin.Context) {
	var req SearchSongsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate that at least one search parameter is provided
	if req.Query == "" && req.Title == "" && req.Artist == "" && req.Album == "" && req.ISRC == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "At least one search parameter is required",
		})
		return
	}

	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 10
	}

	// Cap limit at 50
	if req.Limit > 50 {
		req.Limit = 50
	}

	// Perform search
	response := h.performSearch(c.Request.Context(), req)

	c.JSON(http.StatusOK, response)
}

// RedirectToSong handles GET /api/v1/s/{isrc} - universal song links
func (h *SongHandler) RedirectToSong(c *gin.Context) {
	isrc := c.Param("isrc")
	if isrc == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "ISRC parameter is required",
		})
		return
	}

	// Find song by ISRC
	song, err := h.songRepository.FindByISRC(c.Request.Context(), isrc)
	if err != nil {
		slog.Error("Failed to find song by ISRC", "isrc", isrc, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
		return
	}

	if song == nil {
		// Try by ID as fallback
		song, err = h.songRepository.FindByID(c.Request.Context(), isrc)
		if err != nil {
			slog.Error("Failed to find song by ID", "id", isrc, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
			})
			return
		}
	}

	if song == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Song not found",
		})
		return
	}

	// Check if client wants JSON response (API call)
	acceptHeader := c.GetHeader("Accept")
	if strings.Contains(acceptHeader, "application/json") {
		// Return JSON response for API clients
		c.JSON(http.StatusOK, gin.H{
			"song": song,
		})
		return
	}

	// Check if client wants HTML response
	if strings.Contains(acceptHeader, "text/html") {
		// Return simple HTML response for HTML clients
		html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>%s - %s</title>
    <meta charset="utf-8">
</head>
<body>
    <h1>%s</h1>
    <p>Artist: %s</p>
    <p>Album: %s</p>
    <p>ISRC: %s</p>
    <h2>Available Platforms:</h2>
    <ul>`, song.Title, song.Artist, song.Title, song.Artist, song.Album, song.ISRC)

		for _, link := range song.PlatformLinks {
			if link.Available {
				html += fmt.Sprintf(`<li><a href="%s">%s</a></li>`, link.URL, link.Platform)
			}
		}

		html += `
    </ul>
</body>
</html>`

		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, html)
		return
	}

	// Get user agent to determine preferred platform
	userAgent := c.GetHeader("User-Agent")
	preferredPlatform := h.getPreferredPlatformFromUserAgent(userAgent)

	// Find the best platform link to redirect to
	var redirectURL string
	if preferredPlatform != "" {
		for _, link := range song.PlatformLinks {
			if link.Platform == preferredPlatform && link.Available {
				redirectURL = link.URL
				break
			}
		}
	}

	// Fallback to first available platform
	if redirectURL == "" {
		for _, link := range song.PlatformLinks {
			if link.Available {
				redirectURL = link.URL
				break
			}
		}
	}

	if redirectURL == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No available platform links for this song",
		})
		return
	}

	c.Redirect(http.StatusFound, redirectURL)
}

// TestAlbumInfo handles GET /api/v1/test/album/{albumID} - test endpoint for album info
func (h *SongHandler) TestAlbumInfo(c *gin.Context) {
	albumID := c.Param("albumID")
	if albumID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Album ID parameter is required",
		})
		return
	}

	// Cast to appleMusicService to access GetAlbumByID method
	appleMusicService, ok := h.appleMusicService.(interface {
		GetAlbumByID(ctx context.Context, albumID string) (*services.AppleMusicAlbum, error)
	})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Apple Music service not available",
		})
		return
	}

	album, err := appleMusicService.GetAlbumByID(c.Request.Context(), albumID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"album": gin.H{
			"id":             album.ID,
			"name":           album.Attributes.Name,
			"artist_name":    album.Attributes.ArtistName,
			"is_compilation": album.Attributes.IsCompilation,
			"is_single":      album.Attributes.IsSingle,
			"is_complete":    album.Attributes.IsComplete,
			"release_date":   album.Attributes.ReleaseDate,
		},
	})
}

// performSearch executes the search across platforms
func (h *SongHandler) performSearch(ctx context.Context, req SearchSongsRequest) SearchSongsResponse {
	response := SearchSongsResponse{
		Results: make(map[string][]SearchResult),
		Query:   req,
	}

	// Search local database first
	if req.Platform == "" || req.Platform == "local" {
		localResults := make([]SearchResult, 0, req.Limit)

		// Build search query for local database
		var localSongs []*models.Song
		var err error

		// If ISRC is provided, try to find exact ISRC match first
		if req.ISRC != "" {
			song, err := h.songRepository.FindByISRC(ctx, req.ISRC)
			if err == nil && song != nil {
				localSongs = []*models.Song{song}
			} else {
				slog.Debug("Local ISRC search failed, falling back to other search methods", "isrc", req.ISRC, "error", err)
			}
		}

		// If no ISRC or ISRC search failed, use other search methods
		if len(localSongs) == 0 {
			if req.Query != "" {
				localSongs, err = h.songRepository.Search(ctx, req.Query, req.Limit)
			} else {
				// Use FindByTitleArtist for structured search
				localSongs, err = h.songRepository.FindByTitleArtist(ctx, req.Title, req.Artist)
				if err == nil && len(localSongs) > req.Limit {
					localSongs = localSongs[:req.Limit]
				}
			}

			// If we have ISRC and found songs, try to find exact ISRC match first
			if req.ISRC != "" && len(localSongs) > 0 {
				for _, song := range localSongs {
					if song.ISRC == req.ISRC {
						localSongs = []*models.Song{song}
						break
					}
				}
			}
		}

		if err == nil {
			for _, song := range localSongs {
				localResults = append(localResults, SearchResult{
					ID:          song.ID.Hex(),
					Title:       song.Title,
					Artist:      song.Artist,
					Album:       song.Album,
					Platform:    "local",
					URL:         fmt.Sprintf("%s/s/%s", h.baseURL, song.ISRC),
					ImageURL:    song.Metadata.ImageURL,
					DurationMs:  song.Metadata.Duration,
					ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
					ISRC:        song.ISRC,
				})
			}
		}

		if len(localResults) > 0 {
			response.Results["local"] = localResults
		}
	}

	// Search external platforms
	type searchResult struct {
		platform string
		results  []SearchResult
		err      error
	}

	searchChan := make(chan searchResult, 3)
	searchCount := 0

	// Search Spotify
	if (req.Platform == "" || req.Platform == "spotify") && h.spotifyService != nil {
		searchCount++
		go func() {
			cacheKey := fmt.Sprintf("spotify:%s:%s:%s:%s:%s:%d", req.Query, req.Title, req.Artist, req.Album, req.ISRC, req.Limit)
			if cached, found := h.searchCache.get(cacheKey); found {
				searchChan <- searchResult{platform: "spotify", results: cached}
				return
			}

			var trackInfos []*services.TrackInfo
			var err error

			searchQuery := services.SearchQuery{
				Title:  req.Title,
				Artist: req.Artist,
				Album:  req.Album,
				Query:  req.Query,
				ISRC:   req.ISRC, // Use ISRC from request
				Limit:  req.Limit,
			}

			trackInfos, err = h.spotifyService.SearchTrack(ctx, searchQuery)

			if err != nil {
				searchChan <- searchResult{platform: "spotify", err: err}
				return
			}

			// Convert to SearchResult format
			results := make([]SearchResult, 0, len(trackInfos))
			for _, track := range trackInfos {
				results = append(results, SearchResult{
					ID:          track.ExternalID,
					Title:       track.Title,
					Artist:      strings.Join(track.Artists, ", "),
					Album:       track.Album,
					Platform:    "spotify",
					URL:         track.URL,
					ImageURL:    track.ImageURL,
					DurationMs:  track.Duration,
					ReleaseDate: track.ReleaseDate,
					ISRC:        track.ISRC,
					Explicit:    track.Explicit,
				})
			}

			h.searchCache.set(cacheKey, results)
			searchChan <- searchResult{platform: "spotify", results: results}
		}()
	}

	// Search Apple Music
	if (req.Platform == "" || req.Platform == "apple_music") && h.appleMusicService != nil {
		searchCount++
		go func() {
			cacheKey := fmt.Sprintf("apple_music:%s:%s:%s:%s:%s:%d", req.Query, req.Title, req.Artist, req.Album, req.ISRC, req.Limit)
			if cached, found := h.searchCache.get(cacheKey); found {
				searchChan <- searchResult{platform: "apple_music", results: cached}
				return
			}

			var trackInfos []*services.TrackInfo
			var err error

			// If ISRC is provided, try to get track by ISRC first
			if req.ISRC != "" {
				track, err := h.appleMusicService.GetTrackByISRC(ctx, req.ISRC)
				if err == nil && track != nil {
					trackInfos = []*services.TrackInfo{track}
				} else {
					slog.Debug("Apple Music ISRC search failed, falling back to title/artist search", "isrc", req.ISRC, "error", err)
				}
			}

			// If no ISRC or ISRC search failed, do regular search
			if len(trackInfos) == 0 {
				searchQuery := services.SearchQuery{
					Title:  req.Title,
					Artist: req.Artist,
					Album:  req.Album,
					Query:  req.Query,
					ISRC:   req.ISRC, // Include ISRC in search if available
					Limit:  req.Limit,
				}

				trackInfos, err = h.appleMusicService.SearchTrack(ctx, searchQuery)

				if err != nil {
					searchChan <- searchResult{platform: "apple_music", err: err}
					return
				}

				// If we have ISRC and found tracks, try to find exact ISRC match first
				if req.ISRC != "" && len(trackInfos) > 0 {
					for _, track := range trackInfos {
						if track.ISRC == req.ISRC {
							trackInfos = []*services.TrackInfo{track}
							break
						}
					}
				}
			}

			// Convert to SearchResult format
			results := make([]SearchResult, 0, len(trackInfos))
			for _, track := range trackInfos {
				results = append(results, SearchResult{
					ID:          track.ExternalID,
					Title:       track.Title,
					Artist:      strings.Join(track.Artists, ", "),
					Album:       track.Album,
					Platform:    "apple_music",
					URL:         track.URL,
					ImageURL:    track.ImageURL,
					DurationMs:  track.Duration,
					ReleaseDate: track.ReleaseDate,
					ISRC:        track.ISRC,
					Explicit:    track.Explicit,
				})
			}

			h.searchCache.set(cacheKey, results)
			searchChan <- searchResult{platform: "apple_music", results: results}
		}()
	}

	// Search Tidal
	if (req.Platform == "" || req.Platform == "tidal") && h.tidalService != nil {
		searchCount++
		go func() {
			cacheKey := fmt.Sprintf("tidal:%s:%s:%s:%s:%s:%d", req.Query, req.Title, req.Artist, req.Album, req.ISRC, req.Limit)
			if cached, found := h.searchCache.get(cacheKey); found {
				searchChan <- searchResult{platform: "tidal", results: cached}
				return
			}

			var trackInfos []*services.TrackInfo
			var err error

			searchQuery := services.SearchQuery{
				Title:  req.Title,
				Artist: req.Artist,
				Album:  req.Album,
				Query:  req.Query,
				ISRC:   req.ISRC, // Use ISRC from request
				Limit:  req.Limit,
			}

			trackInfos, err = h.tidalService.SearchTrack(ctx, searchQuery)

			if err != nil {
				searchChan <- searchResult{platform: "tidal", err: err}
				return
			}

			// Convert to SearchResult format
			results := make([]SearchResult, 0, len(trackInfos))
			for _, track := range trackInfos {
				results = append(results, SearchResult{
					ID:          track.ExternalID,
					Title:       track.Title,
					Artist:      strings.Join(track.Artists, ", "),
					Album:       track.Album,
					Platform:    "tidal",
					URL:         track.URL,
					ImageURL:    track.ImageURL,
					DurationMs:  track.Duration,
					ReleaseDate: track.ReleaseDate,
					ISRC:        track.ISRC,
					Explicit:    track.Explicit,
				})
			}

			h.searchCache.set(cacheKey, results)
			searchChan <- searchResult{platform: "tidal", results: results}
		}()
	}

	// Collect results
	for i := 0; i < searchCount; i++ {
		result := <-searchChan
		if result.err != nil {
			slog.Error("Search failed", "platform", result.platform, "error", result.err)
			response.Results[result.platform] = []SearchResult{}
		} else {
			response.Results[result.platform] = result.results
		}
	}

	return response
}

// resolveSongFromPlatform resolves a song from a platform service and saves it to the database
func (h *SongHandler) resolveSongFromPlatform(ctx context.Context, platformService services.PlatformService, trackID string) (*models.Song, error) {
	// Get track info from platform
	track, err := platformService.GetTrackByID(ctx, trackID)
	if err != nil {
		return nil, fmt.Errorf("failed to get track from platform: %w", err)
	}

	if track == nil {
		return nil, fmt.Errorf("track not found on platform")
	}

	// Check if we already have this song by ISRC
	var existingSong *models.Song
	if track.ISRC != "" {
		existingSong, err = h.songRepository.FindByISRC(ctx, track.ISRC)
		if err != nil {
			slog.Error("Failed to check for existing song by ISRC", "isrc", track.ISRC, "error", err)
		}
	}

	// If not found by ISRC, try by title/artist (fuzzy match)
	if existingSong == nil {
		artistName := strings.Join(track.Artists, ", ")
		songs, err := h.songRepository.FindByTitleArtist(ctx, track.Title, artistName)
		if err == nil && len(songs) > 0 {
			// Simple fuzzy matching - could be improved
			for _, song := range songs {
				if strings.EqualFold(song.Title, track.Title) && strings.EqualFold(song.Artist, artistName) {
					existingSong = song
					break
				}
			}
		}
	}

	// Search for the same song on other platforms
	additionalPlatformLinks := h.searchForSongOnOtherPlatforms(ctx, track)

	if existingSong != nil {
		// Update existing song with new platform link
		updated := false
		for i, link := range existingSong.PlatformLinks {
			if link.Platform == track.Platform {
				existingSong.PlatformLinks[i].URL = track.URL
				existingSong.PlatformLinks[i].Available = true
				updated = true
				break
			}
		}

		if !updated {
			existingSong.PlatformLinks = append(existingSong.PlatformLinks, models.PlatformLink{
				Platform:  track.Platform,
				URL:       track.URL,
				Available: true,
			})
		}

		// Add any additional platform links found
		for _, additionalLink := range additionalPlatformLinks {
			// Check if this platform link already exists
			exists := false
			for _, existingLink := range existingSong.PlatformLinks {
				if existingLink.Platform == additionalLink.Platform {
					exists = true
					break
				}
			}
			if !exists {
				existingSong.PlatformLinks = append(existingSong.PlatformLinks, additionalLink)
			}
		}

		// Update ISRC if it was missing
		if existingSong.ISRC == "" && track.ISRC != "" {
			existingSong.ISRC = track.ISRC
		}

		// Update metadata if it was missing or less complete
		if existingSong.Metadata.ImageURL == "" && track.ImageURL != "" {
			existingSong.Metadata.ImageURL = track.ImageURL
		}
		if existingSong.Metadata.Duration == 0 && track.Duration > 0 {
			existingSong.Metadata.Duration = track.Duration
		}

		err = h.songRepository.Update(ctx, existingSong)
		if err != nil {
			return nil, fmt.Errorf("failed to update existing song: %w", err)
		}

		return existingSong, nil
	}

	// Create new song with all platform links
	platformLinks := []models.PlatformLink{
		{
			Platform:  track.Platform,
			URL:       track.URL,
			Available: true,
		},
	}
	platformLinks = append(platformLinks, additionalPlatformLinks...)

	song := &models.Song{
		Title:         track.Title,
		Artist:        strings.Join(track.Artists, ", "),
		Album:         track.Album,
		ISRC:          track.ISRC,
		PlatformLinks: platformLinks,
		Metadata: models.SongMetadata{
			Duration:    track.Duration,
			ReleaseDate: parseReleaseDate(track.ReleaseDate),
			ImageURL:    track.ImageURL,
		},
	}

	err = h.songRepository.Save(ctx, song)
	if err != nil {
		return nil, fmt.Errorf("failed to create song: %w", err)
	}

	return song, nil
}

// searchForSongOnOtherPlatforms searches for the same song on other platforms
func (h *SongHandler) searchForSongOnOtherPlatforms(ctx context.Context, originalTrack *services.TrackInfo) []models.PlatformLink {
	var platformLinks []models.PlatformLink

	// Search on other platforms concurrently
	type searchResult struct {
		platform string
		tracks   []*services.TrackInfo
		err      error
	}

	searchChan := make(chan searchResult, 3)
	searchCount := 0

	// Search Spotify (if not the original platform)
	if originalTrack.Platform != "spotify" && h.spotifyService != nil {
		searchCount++
		go func() {
			tracks, err := h.searchOnPlatform(ctx, h.spotifyService, originalTrack, "spotify")
			searchChan <- searchResult{platform: "spotify", tracks: tracks, err: err}
		}()
	}

	// Search Apple Music (if not the original platform)
	if originalTrack.Platform != "apple_music" && h.appleMusicService != nil {
		searchCount++
		go func() {
			tracks, err := h.searchOnPlatform(ctx, h.appleMusicService, originalTrack, "apple_music")
			searchChan <- searchResult{platform: "apple_music", tracks: tracks, err: err}
		}()
	}

	// Search Tidal (if not the original platform)
	if originalTrack.Platform != "tidal" && h.tidalService != nil {
		searchCount++
		go func() {
			tracks, err := h.searchOnPlatform(ctx, h.tidalService, originalTrack, "tidal")
			searchChan <- searchResult{platform: "tidal", tracks: tracks, err: err}
		}()
	}

	// Collect results
	for i := 0; i < searchCount; i++ {
		result := <-searchChan
		if result.err != nil {
			slog.Error("Failed to search on platform", "platform", result.platform, "error", result.err)
			continue
		}

		// Find the best match for this platform
		bestMatch := h.findBestMatch(originalTrack, result.tracks)
		if bestMatch != nil {
			platformLinks = append(platformLinks, models.PlatformLink{
				Platform:  result.platform,
				URL:       bestMatch.URL,
				Available: true,
			})
		}
	}

	return platformLinks
}

// searchOnPlatform searches for a track on a specific platform, prioritizing ISRC
func (h *SongHandler) searchOnPlatform(ctx context.Context, platformService services.PlatformService, originalTrack *services.TrackInfo, platformName string) ([]*services.TrackInfo, error) {
	// First, try to get the track directly by ISRC if available
	if originalTrack.ISRC != "" {
		track, err := platformService.GetTrackByISRC(ctx, originalTrack.ISRC)
		if err == nil && track != nil {
			slog.Info("Found track by ISRC", "platform", platformName, "isrc", originalTrack.ISRC)
			return []*services.TrackInfo{track}, nil
		}
		// Log if ISRC search failed but don't return error - fall back to search
		slog.Debug("ISRC search failed, falling back to title/artist search", "platform", platformName, "isrc", originalTrack.ISRC, "error", err)
	}

	// Always fall back to search by title/artist (some platforms don't support ISRC search)
	searchQuery := services.SearchQuery{
		Title:  originalTrack.Title,
		Artist: strings.Join(originalTrack.Artists, ", "),
		Album:  originalTrack.Album,
		ISRC:   originalTrack.ISRC, // Include ISRC in search if available
		Limit:  15,                 // Increase limit to get more candidates for better matching
	}

	tracks, err := platformService.SearchTrack(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to search on %s: %w", platformName, err)
	}

	// If we have ISRC and found tracks, try to find exact ISRC match first
	if originalTrack.ISRC != "" && len(tracks) > 0 {
		for _, track := range tracks {
			if track.ISRC == originalTrack.ISRC {
				slog.Info("Found exact ISRC match in search results", "platform", platformName, "isrc", originalTrack.ISRC, "title", track.Title)
				return []*services.TrackInfo{track}, nil
			}
		}
		slog.Debug("No exact ISRC match found in search results, will use fuzzy matching", "platform", platformName, "isrc", originalTrack.ISRC)
	}

	return tracks, nil
}

// findBestMatch finds the best matching track from search results
func (h *SongHandler) findBestMatch(originalTrack *services.TrackInfo, candidates []*services.TrackInfo) *services.TrackInfo {
	if len(candidates) == 0 {
		return nil
	}

	// If we have ISRC, prefer exact ISRC match
	if originalTrack.ISRC != "" {
		for _, candidate := range candidates {
			if candidate.ISRC == originalTrack.ISRC {
				slog.Info("Found exact ISRC match", "isrc", originalTrack.ISRC, "title", candidate.Title)
				return candidate
			}
		}
		// If we have ISRC but no exact match found, we'll still allow fuzzy matching
		// but with a higher threshold for confidence
		slog.Debug("No exact ISRC match found, will try fuzzy matching with higher threshold", "isrc", originalTrack.ISRC)
	}

	// Do fuzzy matching (with higher threshold if ISRC is available)
	originalTitle := strings.ToLower(strings.TrimSpace(originalTrack.Title))
	originalArtist := strings.ToLower(strings.TrimSpace(strings.Join(originalTrack.Artists, ", ")))

	var bestMatch *services.TrackInfo
	bestScore := 0

	for _, candidate := range candidates {
		candidateTitle := strings.ToLower(strings.TrimSpace(candidate.Title))
		candidateArtist := strings.ToLower(strings.TrimSpace(strings.Join(candidate.Artists, ", ")))

		score := 0

		// Title match (exact = 20, contains = 10)
		if candidateTitle == originalTitle {
			score += 20
		} else if strings.Contains(candidateTitle, originalTitle) || strings.Contains(originalTitle, candidateTitle) {
			score += 10
		}

		// Artist match (exact = 20, contains = 10)
		if candidateArtist == originalArtist {
			score += 20
		} else if strings.Contains(candidateArtist, originalArtist) || strings.Contains(originalArtist, candidateArtist) {
			score += 10
		}

		// Duration similarity (within 5 seconds = 5 points)
		if originalTrack.Duration > 0 && candidate.Duration > 0 {
			durationDiff := abs(originalTrack.Duration - candidate.Duration)
			if durationDiff <= 5000 { // 5 seconds in milliseconds
				score += 5
			}
		}

		// Album match (exact = 10, contains = 5)
		if originalTrack.Album != "" && candidate.Album != "" {
			originalAlbum := strings.ToLower(strings.TrimSpace(originalTrack.Album))
			candidateAlbum := strings.ToLower(strings.TrimSpace(candidate.Album))
			if candidateAlbum == originalAlbum {
				score += 10
			} else if strings.Contains(candidateAlbum, originalAlbum) || strings.Contains(originalAlbum, candidateAlbum) {
				score += 5
			}
		}

		if score > bestScore {
			bestScore = score
			bestMatch = candidate
		}
	}

	// Set threshold based on whether we have ISRC
	// If we have ISRC but no exact match, require higher confidence (40 out of 55 possible points)
	// If no ISRC, use standard threshold (35 out of 55 possible points)
	requiredScore := 35
	if originalTrack.ISRC != "" {
		requiredScore = 40 // Higher threshold when ISRC is available but no exact match
	}

	if bestScore >= requiredScore {
		slog.Info("Found high-confidence fuzzy match", "score", bestScore, "required", requiredScore, "title", bestMatch.Title, "artist", bestMatch.Artists)
		return bestMatch
	}

	slog.Debug("No high-confidence match found", "best_score", bestScore, "required_score", requiredScore)
	return nil
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// getPreferredPlatformFromUserAgent determines the preferred platform based on user agent
func (h *SongHandler) getPreferredPlatformFromUserAgent(userAgent string) string {
	userAgent = strings.ToLower(userAgent)

	// iOS devices prefer Apple Music
	if strings.Contains(userAgent, "iphone") || strings.Contains(userAgent, "ipad") || strings.Contains(userAgent, "ipod") {
		return "apple_music"
	}

	// Android devices prefer Spotify (most common)
	if strings.Contains(userAgent, "android") {
		return "spotify"
	}

	// Default to Spotify for other platforms
	return "spotify"
}

// parseReleaseDate parses a release date string into a time.Time
func parseReleaseDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	// Try different date formats
	formats := []string{
		"2006-01-02",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t
		}
	}

	return time.Time{}
}
