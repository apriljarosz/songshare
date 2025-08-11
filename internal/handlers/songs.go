package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"songshare/internal/handlers/render"
	"songshare/internal/models"
	"songshare/internal/repositories"
	"songshare/internal/services"

	"github.com/gin-gonic/gin"
)

// ResolveSongRequest represents the request to resolve a song from a platform URL
type ResolveSongRequest struct {
	URL string `json:"url" binding:"required"`
}

// SearchSongsRequest represents the request to search for songs
type SearchSongsRequest struct {
	Title    string `json:"title,omitempty"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	Query    string `json:"query,omitempty"`    // Free-form search query
	Platform string `json:"platform,omitempty"` // Optional: "spotify", "apple_music", or empty for both
	Limit    int    `json:"limit,omitempty"`    // Max results per platform (default: 10)
}

// SearchSongsResponse represents the response for search results
type SearchSongsResponse struct {
	Results map[string][]render.SearchResult `json:"results"` // platform -> results
	Query   SearchSongsRequest               `json:"query"`   // Echo back the query for reference
}

// Simple search cache entry
type searchCacheEntry struct {
	results   []render.SearchResult
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

func (sc *searchCache) get(key string) ([]render.SearchResult, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	
	entry, exists := sc.entries[key]
	if !exists || time.Since(entry.timestamp) > sc.ttl {
		return nil, false
	}
	return entry.results, true
}

func (sc *searchCache) set(key string, results []render.SearchResult) {
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
	renderer          *render.SongRenderer
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
		renderer:          render.NewSongRenderer(baseURL),
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

	// Convert to response format
	response := render.ResolveSongResponse{
		Song: render.SongMetadata{
			ID:          song.ID.Hex(),
			Title:       song.Title,
			Artists:     []string{song.Artist}, // TODO: Parse comma-separated artists
			Album:       song.Album,
			DurationMs:  song.Metadata.Duration,
			ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
			ISRC:        song.ISRC,
			ImageURL:    song.Metadata.ImageURL,
		},
		Platforms:     make(map[string]render.PlatformLink),
		UniversalLink: fmt.Sprintf("%s/s/%s", h.baseURL, song.ISRC), // ISRC-based universal links
	}

	// Add platform links
	for _, link := range song.PlatformLinks {
		response.Platforms[link.Platform] = render.PlatformLink{
			URL:       link.URL,
			Available: link.Available,
			Platform:  link.Platform,
		}
	}

	// Check if this is an HTMX request (for search page integration)
	if c.GetHeader("HX-Request") == "true" {
		// Return redirect URL with out-of-band badge updates
		c.Header("HX-Redirect", response.UniversalLink)
		
		// Generate OOB updates for all search results with the same ISRC
		oobHTML := "" // Simplified: no out-of-band badge updates
		
		// Return JSON response with redirect and OOB HTML
		responseHTML := fmt.Sprintf(`
			<div id="resolve-result">{"redirect": "%s"}</div>
			%s
		`, response.UniversalLink, oobHTML)
		
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, responseHTML)
		return
	}

	c.JSON(http.StatusOK, response)
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

	// Validate search query
	if req.Title == "" && req.Artist == "" && req.Album == "" && req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "At least one search parameter is required (title, artist, album, or query)",
		})
		return
	}

	// Set default limit
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 50 {
		req.Limit = 50
	}

	// Build search query string (use Query first, then combine Title + Artist)
	var searchTerm string
	if req.Query != "" {
		searchTerm = req.Query
	} else if req.Title != "" && req.Artist != "" {
		searchTerm = req.Title + " " + req.Artist
		if req.Album != "" {
			searchTerm += " " + req.Album
		}
	} else if req.Title != "" {
		searchTerm = req.Title
	} else {
		searchTerm = req.Artist
	}

	response := SearchSongsResponse{
		Results: make(map[string][]render.SearchResult),
		Query:   req,
	}

	// Search local database first
	localSongs, err := h.songRepository.Search(c.Request.Context(), searchTerm, req.Limit)
	if err != nil {
		slog.Error("Local search failed", "error", err)
	} else {
		localResults := make([]render.SearchResult, 0, len(localSongs))
		for _, song := range localSongs {
			// Create universal link for local songs
			universalLink := fmt.Sprintf("%s/s/%s", h.baseURL, song.ISRC)
			if song.ISRC == "" {
				universalLink = fmt.Sprintf("%s/s/%s", h.baseURL, song.ID.Hex()[:8])
			}
			
			localResults = append(localResults, render.SearchResult{
				Title:       song.Title,
				Artists:     []string{song.Artist},
				Album:       song.Album,
				URL:         universalLink,
				Platform:    "local",
				ISRC:        song.ISRC,
				DurationMs:  song.Metadata.Duration,
				ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
				ImageURL:    song.Metadata.ImageURL,
				Available:   true,
			})
		}
		response.Results["local"] = localResults
	}

	// Search platform services concurrently with caching
	platformServices := map[string]services.PlatformService{
		"spotify":     h.spotifyService,
		"apple_music": h.appleMusicService,
		"tidal":       h.tidalService,
	}

	// Use channels and goroutines for concurrent search
	type platformResult struct {
		platform string
		results  []render.SearchResult
		err      error
	}

	resultsChan := make(chan platformResult, 3)
	var wg sync.WaitGroup

	for platform, service := range platformServices {
		// Skip if platform filter specified and doesn't match
		if req.Platform != "" && req.Platform != platform {
			continue
		}
		
		if service == nil {
			continue
		}

		wg.Add(1)
		go func(platform string, service services.PlatformService) {
			defer wg.Done()

			// Check cache first
			cacheKey := fmt.Sprintf("%s:%s:%d", platform, searchTerm, req.Limit)
			if cached, found := h.searchCache.get(cacheKey); found {
				resultsChan <- platformResult{platform: platform, results: cached}
				return
			}

			// Create platform search query
			searchQuery := services.SearchQuery{
				Title:  req.Title,
				Artist: req.Artist,
				Album:  req.Album,
				Query:  searchTerm,
				Limit:  req.Limit,
			}

			// Search with timeout
			ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
			defer cancel()

			tracks, err := service.SearchTrack(ctx, searchQuery)
			if err != nil {
				resultsChan <- platformResult{platform: platform, err: err}
				return
			}

			// Convert to render format
			results := make([]render.SearchResult, 0, len(tracks))
			for _, track := range tracks {
				results = append(results, render.SearchResult{
					Title:       track.Title,
					Artists:     track.Artists,
					Album:       track.Album,
					URL:         track.URL,
					Platform:    platform,
					ISRC:        track.ISRC,
					DurationMs:  track.Duration,
					ReleaseDate: track.ReleaseDate,
					ImageURL:    track.ImageURL,
					Explicit:    track.Explicit,
					Available:   track.Available,
				})
			}

			// Cache the results
			h.searchCache.set(cacheKey, results)
			resultsChan <- platformResult{platform: platform, results: results}
		}(platform, service)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	for result := range resultsChan {
		if result.err != nil {
			slog.Error("Platform search failed", "platform", result.platform, "error", result.err)
			response.Results[result.platform] = []render.SearchResult{}
		} else {
			response.Results[result.platform] = result.results
		}
	}

	c.JSON(http.StatusOK, response)
}

// renderSongJSON returns JSON response for the song
func (h *SongHandler) renderSongJSON(c *gin.Context, song *models.Song) {
	h.renderer.RenderSongJSON(c, song)
}

// renderSongPage returns HTML page with HTMX support
func (h *SongHandler) renderSongPage(c *gin.Context, song *models.Song) {
	// Adapter function to convert PlatformUIConfig types
	adapter := func(platform string) *render.PlatformUIConfig {
		config := GetPlatformUIConfig(platform)
		return &render.PlatformUIConfig{
			Name:        config.Name,
			IconURL:     config.IconURL,
			ButtonText:  config.ButtonText,
			Description: config.Description,
			Color:       config.Color,
			BadgeClass:  config.BadgeClass,
		}
	}
	h.renderer.RenderSongPage(c, song, adapter)
}

// RedirectToSong handles GET /api/v1/s/:id - universal link redirects with dual-mode support
func (h *SongHandler) RedirectToSong(c *gin.Context) {
	songID := c.Param("id")
	if songID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing song ID",
		})
		return
	}

	// Look up song by ISRC
	song, err := h.findSongByISRC(c.Request.Context(), songID)
	if err != nil {
		slog.Error("Song lookup failed", "identifier", songID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Song not found",
		})
		return
	}

	if song == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Song not found",
		})
		return
	}

	// Check if song needs album art backfill
	if h.needsAlbumArtBackfill(song) {
		updatedSong := h.backfillAlbumArt(c.Request.Context(), song)
		if updatedSong != nil {
			song = updatedSong
		}
	}

	// Check Accept header for content negotiation
	accept := c.GetHeader("Accept")
	slog.Info("Accept header", "accept", accept) // Debug log

	// Browsers typically send text/html as the first preference
	if strings.Contains(accept, "text/html") {
		// Return HTML page with HTMX support
		h.renderSongPage(c, song)
	} else {
		// Return JSON response
		h.renderSongJSON(c, song)
	}
}

// needsAlbumArtBackfill checks if a song needs album art to be backfilled
func (h *SongHandler) needsAlbumArtBackfill(song *models.Song) bool {
	// Song needs backfill if it has no album art but has platform links
	return song.Metadata.ImageURL == "" && len(song.PlatformLinks) > 0
}

// backfillAlbumArt attempts to fetch and update album art for an existing song
func (h *SongHandler) backfillAlbumArt(ctx context.Context, song *models.Song) *models.Song {
	// Try to get album art from any available platform
	for _, link := range song.PlatformLinks {
		if !link.Available {
			continue
		}

		// Get the platform service
		var platformService services.PlatformService
		switch link.Platform {
		case "spotify":
			platformService = h.spotifyService
		case "apple_music":
			platformService = h.appleMusicService
		default:
			continue
		}

		if platformService == nil {
			continue
		}

		// Fetch track info from the platform
		trackInfo, err := platformService.GetTrackByID(ctx, link.ExternalID)
		if err != nil {
			slog.Warn("Failed to fetch track info for backfill",
				"platform", link.Platform,
				"trackID", link.ExternalID,
				"error", err)
			continue
		}

		// If we got an image URL, update the song
		if trackInfo != nil && trackInfo.ImageURL != "" {
			song.Metadata.ImageURL = trackInfo.ImageURL

			// Update the song in the database
			if err := h.songRepository.Update(ctx, song); err != nil {
				slog.Error("Failed to update song with album art",
					"songID", song.ID.Hex(),
					"error", err)
				return nil
			}

			slog.Info("Successfully backfilled album art",
				"songID", song.ID.Hex(),
				"platform", link.Platform,
				"imageURL", trackInfo.ImageURL)

			return song
		}
	}

	return nil
}

// SearchPage renders the search page HTML
func (h *SongHandler) SearchPage(c *gin.Context) {
	// Get query from URL path parameter or query parameter
	query := c.Param("query")
	if query == "" {
		query = c.Query("q")
	}

	// URL decode the query if it came from the path
	if query != "" {
		if decoded, err := url.QueryUnescape(query); err == nil {
			query = decoded
		}
	}

	h.renderer.RenderSearchPage(c, query)
}

// SearchResults handles GET /api/v1/search/results and returns simple HTML fragments
func (h *SongHandler) SearchResults(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	platform := strings.TrimSpace(c.Query("platform"))
	limitStr := c.Query("limit")

	// Parse limit
	limit := 10
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 50 {
			limit = parsedLimit
		}
	}

	if query == "" {
		c.String(http.StatusOK, `<div class="empty-state"><p>Enter a search term to find songs.</p></div>`)
		return
	}

	// Use the same search logic as SearchSongs but return HTML
	req := SearchSongsRequest{
		Query:    query,
		Platform: platform,
		Limit:    limit,
	}

	// Perform the search using our simplified search logic
	searchResponse := h.performSearch(c.Request.Context(), req)

	// Convert to HTML
	if len(searchResponse.Results) == 0 {
		c.String(http.StatusOK, `<div class="no-results"><p>No songs found for "%s". Try a different search term.</p></div>`, query)
		return
	}

	html := h.renderSearchResultsHTML(searchResponse.Results)
	c.String(http.StatusOK, html)
}

// performSearch extracts the search logic from SearchSongs for reuse
func (h *SongHandler) performSearch(ctx context.Context, req SearchSongsRequest) SearchSongsResponse {
	// Build search term
	var searchTerm string
	if req.Query != "" {
		searchTerm = req.Query
	} else if req.Title != "" && req.Artist != "" {
		searchTerm = req.Title + " " + req.Artist
		if req.Album != "" {
			searchTerm += " " + req.Album
		}
	} else if req.Title != "" {
		searchTerm = req.Title
	} else {
		searchTerm = req.Artist
	}

	response := SearchSongsResponse{
		Results: make(map[string][]render.SearchResult),
		Query:   req,
	}

	// Search local database first
	if localSongs, err := h.songRepository.Search(ctx, searchTerm, req.Limit); err == nil {
		localResults := make([]render.SearchResult, 0, len(localSongs))
		for _, song := range localSongs {
			universalLink := fmt.Sprintf("%s/s/%s", h.baseURL, song.ISRC)
			if song.ISRC == "" {
				universalLink = fmt.Sprintf("%s/s/%s", h.baseURL, song.ID.Hex()[:8])
			}
			
			localResults = append(localResults, render.SearchResult{
				Title:       song.Title,
				Artists:     []string{song.Artist},
				Album:       song.Album,
				URL:         universalLink,
				Platform:    "local",
				ISRC:        song.ISRC,
				DurationMs:  song.Metadata.Duration,
				ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
				ImageURL:    song.Metadata.ImageURL,
				Available:   true,
			})
		}
		response.Results["local"] = localResults
	}

	// Search platforms concurrently (same logic as SearchSongs)
	platformServices := map[string]services.PlatformService{
		"spotify":     h.spotifyService,
		"apple_music": h.appleMusicService,
		"tidal":       h.tidalService,
	}

	type platformResult struct {
		platform string
		results  []render.SearchResult
		err      error
	}

	resultsChan := make(chan platformResult, 3)
	var wg sync.WaitGroup

	for platform, service := range platformServices {
		if req.Platform != "" && req.Platform != platform {
			continue
		}
		if service == nil {
			continue
		}

		wg.Add(1)
		go func(platform string, service services.PlatformService) {
			defer wg.Done()

			cacheKey := fmt.Sprintf("%s:%s:%d", platform, searchTerm, req.Limit)
			if cached, found := h.searchCache.get(cacheKey); found {
				resultsChan <- platformResult{platform: platform, results: cached}
				return
			}

			searchQuery := services.SearchQuery{
				Title:  req.Title,
				Artist: req.Artist,
				Album:  req.Album,
				Query:  searchTerm,
				Limit:  req.Limit,
			}

			searchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			tracks, err := service.SearchTrack(searchCtx, searchQuery)
			if err != nil {
				resultsChan <- platformResult{platform: platform, err: err}
				return
			}

			results := make([]render.SearchResult, 0, len(tracks))
			for _, track := range tracks {
				results = append(results, render.SearchResult{
					Title:       track.Title,
					Artists:     track.Artists,
					Album:       track.Album,
					URL:         track.URL,
					Platform:    platform,
					ISRC:        track.ISRC,
					DurationMs:  track.Duration,
					ReleaseDate: track.ReleaseDate,
					ImageURL:    track.ImageURL,
					Explicit:    track.Explicit,
					Available:   track.Available,
				})
			}

			h.searchCache.set(cacheKey, results)
			resultsChan <- platformResult{platform: platform, results: results}
		}(platform, service)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	for result := range resultsChan {
		if result.err != nil {
			response.Results[result.platform] = []render.SearchResult{}
		} else {
			response.Results[result.platform] = result.results
		}
	}

	return response
}

// GroupedSong represents a song with multiple platform links
type GroupedSong struct {
	Title       string
	Artists     []string
	Album       string
	ISRC        string
	DurationMs  int
	ReleaseDate string
	ImageURL    string
	Explicit    bool
	Platforms   []render.SearchResult // All platform results for this song
}

// renderSearchResultsHTML generates HTML for search results grouped by ISRC
func (h *SongHandler) renderSearchResultsHTML(results map[string][]render.SearchResult) string {
	var html strings.Builder
	html.WriteString(`<div class="search-results">`)
	
	// Group results by ISRC
	groupedSongs := h.groupSongsByISRC(results)
	
	if len(groupedSongs) == 0 {
		html.WriteString(`<div class="no-results"><p>No results found.</p></div>`)
		html.WriteString(`</div>`)
		return html.String()
	}
	
	// Render grouped songs
	for i, song := range groupedSongs {
		html.WriteString(fmt.Sprintf(`<div class="result-item" id="result-%d">`, i))
		
		// Album art (prefer image from first platform that has one)
		imageURL := song.ImageURL
		if imageURL != "" {
			html.WriteString(fmt.Sprintf(`<img class="album-art" src="%s" alt="Album art" loading="lazy">`, imageURL))
		} else {
			html.WriteString(`<div class="result-image-placeholder">🎵</div>`)
		}
		
		// Song info
		html.WriteString(`<div class="song-info">`)
		html.WriteString(fmt.Sprintf(`<h2 class="title">%s`, song.Title))
		if song.Explicit {
			html.WriteString(`<span class="explicit-indicator">E</span>`)
		}
		html.WriteString(`</h2>`)
		
		if len(song.Artists) > 0 {
			html.WriteString(fmt.Sprintf(`<h3 class="artist">%s</h3>`, strings.Join(song.Artists, ", ")))
		}
		if song.Album != "" {
			html.WriteString(fmt.Sprintf(`<h4 class="album">%s</h4>`, song.Album))
		}
		
		// Platform badges
		html.WriteString(`<div class="result-platforms">`)
		for _, platform := range song.Platforms {
			badgeClass := fmt.Sprintf("platform-badge platform-%s", strings.ReplaceAll(platform.Platform, "_", "-"))
			html.WriteString(fmt.Sprintf(`<a href="%s" target="_blank" class="%s">`, platform.URL, badgeClass))
			
			// Platform icon if available
			iconURL := h.getPlatformIconURL(platform.Platform)
			if iconURL != "" {
				html.WriteString(fmt.Sprintf(`<img src="%s" alt="%s" class="platform-badge-icon">`, iconURL, platform.Platform))
			}
			html.WriteString(h.getPlatformDisplayName(platform.Platform))
			html.WriteString(`</a>`)
		}
		html.WriteString(`</div>`)
		
		html.WriteString(`</div>`) // Close song-info
		
		// Actions (moved outside song-info for proper alignment)
		html.WriteString(`<div class="result-actions">`)
		
		// Share button for creating universal links
		if len(song.Platforms) > 0 {
			firstPlatformURL := song.Platforms[0].URL
			html.WriteString(fmt.Sprintf(`<button class="action-btn action-secondary action-small" onclick="createShareLink('%s', this)">Share</button>`, firstPlatformURL))
		}
		
		html.WriteString(`</div>`) // Close result-actions
		html.WriteString(`</div>`) // Close result-item
	}
	
	html.WriteString(`</div>`) // Close search-results
	return html.String()
}

// groupSongsByISRC groups search results by ISRC, with fallback grouping by title+artist
func (h *SongHandler) groupSongsByISRC(results map[string][]render.SearchResult) []GroupedSong {
	// Map ISRC to grouped song
	isrcToSong := make(map[string]*GroupedSong)
	// Map title+artist combo to grouped song (for songs without ISRC)
	titleArtistToSong := make(map[string]*GroupedSong)
	
	// Helper function to create a normalized title+artist key
	normalizeKey := func(title string, artists []string) string {
		titleLower := strings.ToLower(strings.TrimSpace(title))
		artistLower := ""
		if len(artists) > 0 {
			artistLower = strings.ToLower(strings.TrimSpace(artists[0]))
		}
		return titleLower + "|" + artistLower
	}
	
	// Process all results from all platforms
	for _, platformResults := range results {
		for _, result := range platformResults {
			if result.ISRC != "" && result.ISRC != "unknown" {
				// Group by ISRC
				if existing, exists := isrcToSong[result.ISRC]; exists {
					// Check if this platform already exists for this song
					platformExists := false
					for _, existingPlatform := range existing.Platforms {
						if existingPlatform.Platform == result.Platform {
							platformExists = true
							break
						}
					}
					
					// Only add platform if it doesn't already exist
					if !platformExists {
						existing.Platforms = append(existing.Platforms, result)
					}
					
					// Update song metadata if this result has better data
					if existing.ImageURL == "" && result.ImageURL != "" {
						existing.ImageURL = result.ImageURL
					}
				} else {
					// Create new grouped song
					isrcToSong[result.ISRC] = &GroupedSong{
						Title:       result.Title,
						Artists:     result.Artists,
						Album:       result.Album,
						ISRC:        result.ISRC,
						DurationMs:  result.DurationMs,
						ReleaseDate: result.ReleaseDate,
						ImageURL:    result.ImageURL,
						Explicit:    result.Explicit,
						Platforms:   []render.SearchResult{result},
					}
				}
			} else {
				// Group songs without ISRC by title+artist
				titleArtistKey := normalizeKey(result.Title, result.Artists)
				
				if existing, exists := titleArtistToSong[titleArtistKey]; exists {
					// Check if this platform already exists for this song
					platformExists := false
					for _, existingPlatform := range existing.Platforms {
						if existingPlatform.Platform == result.Platform {
							platformExists = true
							break
						}
					}
					
					// Only add platform if it doesn't already exist
					if !platformExists {
						existing.Platforms = append(existing.Platforms, result)
					}
					
					// Update song metadata if this result has better data
					if existing.ImageURL == "" && result.ImageURL != "" {
						existing.ImageURL = result.ImageURL
					}
				} else {
					// Create new grouped song for title+artist combo
					titleArtistToSong[titleArtistKey] = &GroupedSong{
						Title:       result.Title,
						Artists:     result.Artists,
						Album:       result.Album,
						ISRC:        result.ISRC,
						DurationMs:  result.DurationMs,
						ReleaseDate: result.ReleaseDate,
						ImageURL:    result.ImageURL,
						Explicit:    result.Explicit,
						Platforms:   []render.SearchResult{result},
					}
				}
			}
		}
	}
	
	// Convert maps to slice with deterministic ordering
	var groupedSongs []GroupedSong
	
	// First handle ISRC-grouped songs
	isrcs := make([]string, 0, len(isrcToSong))
	for isrc := range isrcToSong {
		isrcs = append(isrcs, isrc)
	}
	
	// Sort ISRCs to ensure deterministic iteration
	for i := 0; i < len(isrcs)-1; i++ {
		for j := i + 1; j < len(isrcs); j++ {
			if isrcs[i] > isrcs[j] {
				isrcs[i], isrcs[j] = isrcs[j], isrcs[i]
			}
		}
	}
	
	// Add ISRC-grouped songs
	for _, isrc := range isrcs {
		song := isrcToSong[isrc]
		h.sortPlatformsByPreference(song.Platforms)
		groupedSongs = append(groupedSongs, *song)
	}
	
	// Then handle title+artist grouped songs (songs without ISRC)
	titleArtistKeys := make([]string, 0, len(titleArtistToSong))
	for key := range titleArtistToSong {
		titleArtistKeys = append(titleArtistKeys, key)
	}
	
	// Sort keys for deterministic iteration
	for i := 0; i < len(titleArtistKeys)-1; i++ {
		for j := i + 1; j < len(titleArtistKeys); j++ {
			if titleArtistKeys[i] > titleArtistKeys[j] {
				titleArtistKeys[i], titleArtistKeys[j] = titleArtistKeys[j], titleArtistKeys[i]
			}
		}
	}
	
	// Add title+artist grouped songs
	for _, key := range titleArtistKeys {
		song := titleArtistToSong[key]
		h.sortPlatformsByPreference(song.Platforms)
		groupedSongs = append(groupedSongs, *song)
	}
	
	// Sort grouped songs by relevance (number of platforms, then alphabetically)
	h.sortGroupedSongs(groupedSongs)
	
	return groupedSongs
}

// sortPlatformsByPreference sorts platforms in display preference order
func (h *SongHandler) sortPlatformsByPreference(platforms []render.SearchResult) {
	preferenceOrder := map[string]int{
		"local":       1,
		"apple_music": 2,
		"spotify":     3,
		"tidal":       4,
	}
	
	// Sort platforms by preference
	for i := 0; i < len(platforms)-1; i++ {
		for j := i + 1; j < len(platforms); j++ {
			prefI := preferenceOrder[platforms[i].Platform]
			prefJ := preferenceOrder[platforms[j].Platform]
			if prefI > prefJ {
				platforms[i], platforms[j] = platforms[j], platforms[i]
			}
		}
	}
}

// artistPopularityScore assigns popularity scores to artists (higher = more popular)
func (h *SongHandler) artistPopularityScore(artists []string) int {
	// Popular artists get higher scores
	popularArtists := map[string]int{
		"chappell roan":    1000,
		"taylor swift":     950,
		"billie eilish":    900,
		"dua lipa":        850,
		"ariana grande":   800,
		"olivia rodrigo":  750,
		"the weeknd":      700,
		"bad bunny":       650,
		"drake":           600,
		"ed sheeran":      550,
		// Add more as needed
	}
	
	maxScore := 0
	for _, artist := range artists {
		artistLower := strings.ToLower(strings.TrimSpace(artist))
		if score, exists := popularArtists[artistLower]; exists && score > maxScore {
			maxScore = score
		}
	}
	return maxScore
}

// calculateRelevanceScore calculates a comprehensive relevance score for a song
func (h *SongHandler) calculateRelevanceScore(song GroupedSong) int {
	score := 0
	
	// Platform availability (more platforms = higher score)
	score += len(song.Platforms) * 100
	
	// Artist popularity bonus
	score += h.artistPopularityScore(song.Artists)
	
	// Release date bonus (newer songs get slight preference)
	if song.ReleaseDate != "" {
		// Simple heuristic: if release date contains recent years, boost score
		if strings.Contains(song.ReleaseDate, "2024") {
			score += 50
		} else if strings.Contains(song.ReleaseDate, "2023") {
			score += 30
		} else if strings.Contains(song.ReleaseDate, "2022") {
			score += 10
		}
	}
	
	// Album art bonus (songs with art are likely better curated)
	if song.ImageURL != "" {
		score += 25
	}
	
	return score
}

// sortGroupedSongs sorts grouped songs by comprehensive relevance scoring
func (h *SongHandler) sortGroupedSongs(songs []GroupedSong) {
	// Calculate scores for all songs first
	scores := make([]int, len(songs))
	for i, song := range songs {
		scores[i] = h.calculateRelevanceScore(song)
	}
	
	// Sort by relevance score (descending), then by title (ascending) for tie-breaking
	for i := 0; i < len(songs)-1; i++ {
		for j := i + 1; j < len(songs); j++ {
			shouldSwap := false
			
			// Primary sort: higher relevance score first
			if scores[i] < scores[j] {
				shouldSwap = true
			} else if scores[i] == scores[j] {
				// Tie-breaker: alphabetical by title
				if songs[i].Title > songs[j].Title {
					shouldSwap = true
				}
			}
			
			if shouldSwap {
				songs[i], songs[j] = songs[j], songs[i]
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}
}

// getPlatformIconURL returns the icon URL for a platform using WikiMedia URLs
func (h *SongHandler) getPlatformIconURL(platform string) string {
	config := GetPlatformUIConfig(platform)
	return config.IconURL
}

// getPlatformDisplayName returns human-readable platform names
func (h *SongHandler) getPlatformDisplayName(platform string) string {
	switch platform {
	case "apple_music":
		return "Apple Music"
	case "spotify":
		return "Spotify"
	case "tidal":
		return "TIDAL"
	case "local":
		return "Local Library"
	default:
		return strings.Title(platform)
	}
}

// findSongByISRC finds a song by ISRC or ID prefix
func (h *SongHandler) findSongByISRC(ctx context.Context, identifier string) (*models.Song, error) {
	// Try ISRC first
	song, err := h.songRepository.FindByISRC(ctx, identifier)
	if err != nil {
		return nil, err
	}
	if song != nil {
		return song, nil
	}
	
	// Try ID prefix as fallback
	return h.songRepository.FindByIDPrefix(ctx, identifier)
}

// EnhancedPlatformBadges - simple stub for backward compatibility
func (h *SongHandler) EnhancedPlatformBadges(c *gin.Context) {
	c.String(http.StatusOK, `<div>Enhanced badges not implemented</div>`)
}

// EnhanceBadges - simple stub for backward compatibility  
func (h *SongHandler) EnhanceBadges(c *gin.Context) {
	c.String(http.StatusOK, `<div>Badge enhancement not implemented</div>`)
}

// resolveSongFromPlatform resolves a song from a platform track ID
func (h *SongHandler) resolveSongFromPlatform(ctx context.Context, platformService services.PlatformService, trackID string) (*models.Song, error) {
	// Check if we already have this song by platform ID
	existingSong, err := h.songRepository.FindByPlatformID(ctx, platformService.GetPlatformName(), trackID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing song: %w", err)
	}

	if existingSong != nil {
		return existingSong, nil
	}

	// Fetch track info from the platform
	trackInfo, err := platformService.GetTrackByID(ctx, trackID)
	if err != nil {
		return nil, fmt.Errorf("failed to get track info: %w", err)
	}

	// Try to find existing song by ISRC
	if trackInfo.ISRC != "" {
		existingSong, err := h.songRepository.FindByISRC(ctx, trackInfo.ISRC)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing song by ISRC: %w", err)
		}

		if existingSong != nil {
			// Add this platform link if it doesn't exist
			if !existingSong.HasPlatform(platformService.GetPlatformName()) {
				existingSong.AddPlatformLink(platformService.GetPlatformName(), trackID, trackInfo.URL, 1.0)
				if err := h.songRepository.Update(ctx, existingSong); err != nil {
					slog.Error("Failed to update song with new platform link", "error", err)
				}
			}
			return existingSong, nil
		}
	}

	// Create new song from track info
	song := trackInfo.ToSong()
	if err := h.songRepository.Save(ctx, song); err != nil {
		return nil, fmt.Errorf("failed to save new song: %w", err)
	}

	return song, nil
}
