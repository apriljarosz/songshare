package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"songshare/internal/handlers/render"
	"songshare/internal/models"
	"songshare/internal/repositories"
	"songshare/internal/scoring"
	"songshare/internal/search"
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

// SongHandler handles song-related requests
type SongHandler struct {
	resolutionService *services.SongResolutionService
	songRepository    repositories.SongRepository
	baseURL           string
	renderer          *render.SongRenderer
	searchCoordinator *search.Coordinator
	scorer            *scoring.RelevanceScorer
}

// NewSongHandler creates a new song handler
func NewSongHandler(resolutionService *services.SongResolutionService, songRepository repositories.SongRepository, baseURL string) *SongHandler {
	return &SongHandler{
		resolutionService: resolutionService,
		songRepository:    songRepository,
		baseURL:           baseURL,
		renderer:          render.NewSongRenderer(baseURL),
		searchCoordinator: search.NewCoordinator(songRepository, resolutionService, baseURL),
		scorer:            scoring.NewRelevanceScorer(),
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

	// Resolve the song from the URL
	song, err := h.resolutionService.ResolveFromURL(c.Request.Context(), req.URL)
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
		UniversalLink: h.buildUniversalLink(song), // ISRC-based universal links
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
		// Return redirect URL in both header and body for JavaScript compatibility
		c.Header("HX-Redirect", response.UniversalLink)
		c.JSON(http.StatusOK, gin.H{
			"redirect": response.UniversalLink,
		})
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
		req.Limit = 50 // Cap at 50 results per platform
	}

	// Build search query
	searchQuery := services.SearchQuery{
		Title:  req.Title,
		Artist: req.Artist,
		Album:  req.Album,
		Query:  req.Query,
		Limit:  req.Limit,
	}

	response := SearchSongsResponse{
		Results: make(map[string][]render.SearchResult),
		Query:   req,
	}

	// Search platforms based on filter
	platforms := []string{}
	if req.Platform == "" {
		platforms = []string{"apple_music", "spotify", "tidal"} // All platforms
	} else if req.Platform == "spotify" || req.Platform == "apple_music" || req.Platform == "tidal" {
		platforms = []string{req.Platform}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid platform. Use 'spotify', 'apple_music', 'tidal', or omit for all",
		})
		return
	}

	// Search each platform
	for _, platform := range platforms {
		var platformService services.PlatformService

		// Get the appropriate service
		switch platform {
		case "spotify":
			platformService = h.resolutionService.GetPlatformService("spotify")
		case "apple_music":
			platformService = h.resolutionService.GetPlatformService("apple_music")
		case "tidal":
			platformService = h.resolutionService.GetPlatformService("tidal")
		default:
			continue
		}

		if platformService == nil {
			slog.Warn("Platform service not available", "platform", platform)
			continue
		}

		// Perform search
		tracks, err := platformService.SearchTrack(c.Request.Context(), searchQuery)
		if err != nil {
			slog.Error("Search failed for platform", "platform", platform, "error", err)
			// Continue with other platforms instead of failing entirely
			response.Results[platform] = []render.SearchResult{}
			continue
		}

		// Convert tracks to search results
		results := make([]render.SearchResult, len(tracks))
		for i, track := range tracks {
			results[i] = render.SearchResult{
				Title:       track.Title,
				Artists:     track.Artists,
				Album:       track.Album,
				URL:         track.URL,
				Platform:    track.Platform,
				ISRC:        track.ISRC,
				DurationMs:  track.Duration,
				ReleaseDate: track.ReleaseDate,
				ImageURL:    track.ImageURL,
				Explicit:    track.Explicit,
				Available:   track.Available,
			}
		}

		response.Results[platform] = results
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
			platformService = h.resolutionService.GetPlatformService("spotify")
		case "apple_music":
			platformService = h.resolutionService.GetPlatformService("apple_music")
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

// SearchResults handles GET /api/v1/search/results and returns HTML fragments
func (h *SongHandler) SearchResults(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	platform := strings.TrimSpace(c.Query("platform"))
	limitStr := c.Query("limit")
	sortBy := strings.ToLower(strings.TrimSpace(c.Query("sort")))

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

	// Search all sources using coordinator
	allResults, err := h.searchCoordinator.SearchAll(c.Request.Context(), query, platform, limit)
	if err != nil {
		slog.Error("Search failed", "error", err, "query", query)
		c.String(http.StatusOK, `<div class="no-results"><p>Search temporarily unavailable. Please try again.</p></div>`)
		return
	}

	if len(allResults) == 0 {
		c.String(http.StatusOK, `<div class="no-results"><p>No songs found for "%s". Try a different search term.</p></div>`, query)
		return
	}

	// Group results by song
	includeDebug := strings.EqualFold(c.Query("debug"), "1") || strings.EqualFold(c.Query("debug"), "true")
	groupedResults := h.searchCoordinator.GroupResults(allResults, query, includeDebug)

	// Optional client-controlled sorting
	switch sortBy {
	case "popular", "popularity":
		sort.SliceStable(groupedResults, func(i, j int) bool {
			if groupedResults[i].Popularity != groupedResults[j].Popularity {
				return groupedResults[i].Popularity > groupedResults[j].Popularity
			}
			// fall back to relevance order
			return groupedResults[i].RelevanceScore > groupedResults[j].RelevanceScore
		})
	default:
		// keep relevance-based order
	}

	// Start background enhancements
	go h.searchCoordinator.EnhanceResultsPlatforms(c.Request.Context(), groupedResults)

	// Background indexing of platform results
	var platformResults []render.SearchResult
	for _, result := range allResults {
		if result.Source == "platform" {
			platformResults = append(platformResults, result.SearchResult)
		}
	}
	if len(platformResults) > 0 {
		go h.searchCoordinator.BackgroundIndexTracks(query, platformResults)
	}

	// Generate HTML for grouped results
	html := h.renderGroupedSearchResultsHTML(groupedResults)
	c.String(http.StatusOK, html)
}

// searchLocalSongs searches the local MongoDB database

// searchPlatforms searches external platforms

// renderSearchResultsHTML generates HTML for search results

// groupSearchResults groups search results by song, combining multiple platform entries

// generateSongKey creates a unique key for grouping songs
func (h *SongHandler) generateSongKey(result render.SearchResult) string {
	// Primary: Group by ISRC if available - this preserves different versions (clean vs explicit)
	if result.ISRC != "" {
		return "isrc:" + result.ISRC
	}

	// Secondary: Group by normalized title + artist + album to distinguish different releases
	normalizedTitle := h.normalizeString(result.Title)
	normalizedArtists := h.normalizeString(strings.Join(result.Artists, ", "))
	normalizedAlbum := h.normalizeString(result.Album)

	key := "song:" + normalizedTitle + "|" + normalizedArtists + "|" + normalizedAlbum

	// Debug logging disabled - can be re-enabled for troubleshooting
	// slog.Debug("Generated song key", "title", result.Title, "key", key)

	return key
}

// normalizeString normalizes a string for comparison (lowercase, trim, etc.)
func (h *SongHandler) normalizeString(s string) string {
	// Convert to lowercase and trim spaces
	normalized := strings.ToLower(strings.TrimSpace(s))

	// Remove common variations that might cause false negatives
	normalized = strings.ReplaceAll(normalized, "feat.", "ft.")
	normalized = strings.ReplaceAll(normalized, "featuring", "ft.")
	normalized = strings.ReplaceAll(normalized, " ft ", " ft. ")

	// Remove punctuation that might differ between platforms
	// (Keep it simple for now - Unicode handling can be complex)

	// Remove extra spaces
	normalized = strings.Join(strings.Fields(normalized), " ")

	return normalized
}

// convertToScoringSearchResult converts a handler SearchResult to scoring package SearchResult
func (h *SongHandler) convertToScoringSearchResult(result render.SearchResult) scoring.SearchResult {
	return scoring.SearchResult{
		Platform:    result.Platform,
		ExternalID:  "", // Handler SearchResult doesn't have ExternalID field
		URL:         result.URL,
		Title:       result.Title,
		Artists:     result.Artists,
		Album:       result.Album,
		ISRC:        result.ISRC,
		DurationMs:  result.DurationMs,
		ReleaseDate: result.ReleaseDate,
		ImageURL:    result.ImageURL,
		Popularity:  result.Popularity,
		Explicit:    result.Explicit,
		Available:   result.Available,
	}
}

// convertToScoringSearchResultsWithSource converts handler render.SearchResultWithSource slice to scoring package slice
func (h *SongHandler) convertToScoringSearchResultsWithSource(results []render.SearchResultWithSource) []scoring.SearchResultWithSource {
	scoringResults := make([]scoring.SearchResultWithSource, len(results))
	for i, result := range results {
		scoringResults[i] = scoring.SearchResultWithSource{
			SearchResult: h.convertToScoringSearchResult(result.SearchResult),
			Source:       result.Source,
		}
	}
	return scoringResults
}

// renderGroupedSearchResultsHTML generates HTML for grouped search results
func (h *SongHandler) renderGroupedSearchResultsHTML(results []search.GroupedSearchResult) string {
	if len(results) == 0 {
		return `<div class="no-results"><p>No songs found.</p></div>`
	}

	var html strings.Builder
	html.WriteString(`<div class="search-results-container">`)

	for _, result := range results {
		// Start result item with unique ID for progressive enhancement
		html.WriteString(fmt.Sprintf(`<div class="result-item" id="%s">`, result.ID))

		// Album art or placeholder
		if result.ImageURL != "" {
			html.WriteString(fmt.Sprintf(`<img src="%s" alt="Album art" class="result-image">`, result.ImageURL))
		} else {
			html.WriteString(`<div class="result-image-placeholder">🎵</div>`)
		}

		// Content
		html.WriteString(`<div class="result-content">`)
		titleHTML := result.Title
		if result.Explicit {
			titleHTML += ` 🅴`
		}
		html.WriteString(fmt.Sprintf(`<div class="result-title">%s</div>`, titleHTML))
		html.WriteString(fmt.Sprintf(`<div class="result-artist">%s</div>`, strings.Join(result.Artists, ", ")))
		if result.Album != "" {
			html.WriteString(fmt.Sprintf(`<div class="result-album">%s</div>`, result.Album))
		}

		// If debug breakdown present, render it
		if result.Debug != nil {
			html.WriteString(`<div class="result-debug">`)
			html.WriteString(fmt.Sprintf(`<div>Text: %.1f</div>`, result.Debug.TextMatch))
			html.WriteString(fmt.Sprintf(`<div>Popularity input: %d</div>`, result.Debug.PopularityInput))
			html.WriteString(fmt.Sprintf(`<div>Popularity boost: %.1f</div>`, result.Debug.PopularityBoost))
			html.WriteString(fmt.Sprintf(`<div>Context: %.1f</div>`, result.Debug.Context))
			html.WriteString(fmt.Sprintf(`<div>Final: %.1f</div>`, result.Debug.Final))
			if len(result.DebugPlatformPops) > 0 {
				html.WriteString(`<div>Per-platform popularity:</div>`)
				for p, v := range result.DebugPlatformPops {
					html.WriteString(fmt.Sprintf(`<div>&nbsp;&nbsp;%s: %d</div>`, p, v))
				}
				if result.DebugRepPlatform != "" {
					html.WriteString(fmt.Sprintf(`<div>Rep platform: %s</div>`, result.DebugRepPlatform))
				}
				if result.DebugAggPopularity > 0 {
					html.WriteString(fmt.Sprintf(`<div>Aggregate popularity: %d</div>`, result.DebugAggPopularity))
				}
			}
			html.WriteString(`</div>`)
		}

		// Platform badges (multiple platforms) - clickable badges that link directly to platforms
		html.WriteString(fmt.Sprintf(`<div class="result-platforms" id="%s-platforms">`, result.ID))

		// Show clickable platform badges in alphabetical order
		sortedPlatforms := make([]search.PlatformResult, 0)
		for _, platform := range result.PlatformLinks {
			// Skip the "local" fallback platform - we'll handle that separately
			if platform.Platform == "local" {
				continue
			}
			sortedPlatforms = append(sortedPlatforms, platform)
		}

		// Sort platforms alphabetically
		sort.Slice(sortedPlatforms, func(i, j int) bool {
			return sortedPlatforms[i].Platform < sortedPlatforms[j].Platform
		})

		for _, platform := range sortedPlatforms {
			// Use dynamic platform UI system for badges
			badgeHTML := RenderPlatformBadge(platform.Platform, platform.URL)
			html.WriteString(badgeHTML)
		}

		// SongShare badge removed - users don't need to see this alongside platform badges

		html.WriteString(`</div>`)

		html.WriteString(`</div>`) // End content

		// Actions - simplified with only primary action
		html.WriteString(`<div class="result-actions">`)

		// Primary action: View local link if available, otherwise create universal link
		if result.HasLocalLink {
			html.WriteString(fmt.Sprintf(`<a href="%s" class="action-btn action-primary">Share</a>`, result.LocalURL))
		} else {
			// Find the first platform alphabetically to create a universal link from
			var firstPlatformURL string
			var firstPlatformName string
			for _, platform := range result.PlatformLinks {
				if platform.Source == "platform" {
					// Use the first platform alphabetically
					if firstPlatformName == "" || platform.Platform < firstPlatformName {
						firstPlatformName = platform.Platform
						firstPlatformURL = platform.URL
					}
				}
			}

			if firstPlatformURL != "" {
				html.WriteString(fmt.Sprintf(`
					<button class="action-btn action-primary" 
					        onclick="createShareLink('%s', this)">
						Share
					</button>
				`, firstPlatformURL))
			}
		}

		html.WriteString(`</div>`) // End actions

		html.WriteString(`</div>`) // End result item
	}

	html.WriteString(`</div>`) // End container
	return html.String()
} // backgroundIndexTracks performs intelligent background indexing of search results

// buildUniversalLink creates a universal link for a song using ISRC
func (h *SongHandler) buildUniversalLink(song *models.Song) string {
	if song.ISRC == "" {
		slog.Warn("Song missing ISRC", "songID", song.ID.Hex(), "title", song.Title)
		// This shouldn't happen with properly indexed songs
		return fmt.Sprintf("%s/s/unknown", h.baseURL)
	}
	return fmt.Sprintf("%s/s/%s", h.baseURL, song.ISRC)
}

// findSongByISRC finds a song by ISRC
func (h *SongHandler) findSongByISRC(ctx context.Context, isrc string) (*models.Song, error) {
	song, err := h.songRepository.FindByISRC(ctx, isrc)
	if err != nil {
		return nil, fmt.Errorf("song not found for ISRC %s: %w", isrc, err)
	}
	if song == nil {
		return nil, fmt.Errorf("song not found for ISRC: %s", isrc)
	}
	return song, nil
}

// invalidateSearchCache invalidates cached search results for a query
func (h *SongHandler) invalidateSearchCache(query string) {
	// Invalidate the search cache for this specific query
	// This integrates with our cached repository pattern
	if cachedRepo, ok := h.songRepository.(interface {
		InvalidateSearchCache(query string)
	}); ok {
		cachedRepo.InvalidateSearchCache(query)
		slog.Debug("Invalidated search cache", "query", query)
	} else {
		slog.Debug("Repository doesn't support cache invalidation", "query", query)
	}
}

// EnhancedPlatformBadges returns enhanced platform badges for a search result
func (h *SongHandler) EnhancedPlatformBadges(c *gin.Context) {
	resultID := c.Param("resultId")

	if resultID == "" {
		c.String(http.StatusBadRequest, "Missing result ID")
		return
	}

	// Extract ISRC from result ID if it follows the pattern "result-{ISRC}"
	var isrc string
	if strings.HasPrefix(resultID, "result-") {
		potentialISRC := strings.TrimPrefix(resultID, "result-")
		// Basic ISRC validation (should be 12 characters)
		if len(potentialISRC) == 12 {
			isrc = potentialISRC
		}
	}

	if isrc == "" {
		c.String(http.StatusOK, "") // Return empty if we can't enhance
		return
	}

	// Look for the song in our database (it might have been enhanced by now)
	song, err := h.songRepository.FindByISRC(c.Request.Context(), isrc)
	if err != nil || song == nil {
		c.String(http.StatusOK, "") // Return empty if not found
		return
	}

	// Generate platform badges HTML for all available platforms
	var html strings.Builder

	// Get all available platforms sorted alphabetically
	var sortedPlatforms []models.PlatformLink
	for _, link := range song.PlatformLinks {
		if link.Available && link.URL != "" {
			sortedPlatforms = append(sortedPlatforms, link)
		}
	}

	// Sort by platform name
	sort.Slice(sortedPlatforms, func(i, j int) bool {
		return sortedPlatforms[i].Platform < sortedPlatforms[j].Platform
	})

	for _, platformLink := range sortedPlatforms {
		badgeHTML := RenderPlatformBadge(platformLink.Platform, platformLink.URL)
		html.WriteString(badgeHTML)
	}

	c.String(http.StatusOK, html.String())
}

// generateSearchResultID creates a unique ID for a search result
func (h *SongHandler) generateSearchResultID(result render.SearchResult) string {
	// Use ISRC if available for deterministic IDs
	if result.ISRC != "" {
		return "result-" + result.ISRC
	}

	// Fall back to random ID for results without ISRC
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "result-" + hex.EncodeToString(bytes)
}

// enhanceSearchResultsPlatforms performs background platform enhancement for search results
