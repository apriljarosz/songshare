package handlers

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"songshare/internal/models"
	"songshare/internal/repositories"
	"songshare/internal/services"
)

// ResolveSongRequest represents the request to resolve a song from a platform URL
type ResolveSongRequest struct {
	URL string `json:"url" binding:"required"`
}

// ResolveSongResponse represents the response with song metadata and platform links
type ResolveSongResponse struct {
	Song      SongMetadata           `json:"song"`
	Platforms map[string]PlatformLink `json:"platforms"`
	UniversalLink string             `json:"universal_link"`
}

// SongMetadata represents core song information
type SongMetadata struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Artists     []string `json:"artists"`
	Album       string   `json:"album"`
	DurationMs  int      `json:"duration_ms"`
	ReleaseDate string   `json:"release_date"`
	ISRC        string   `json:"isrc,omitempty"`
}

// PlatformLink represents a link to a song on a specific platform
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
	Platform string `json:"platform,omitempty"` // Optional: "spotify", "apple_music", or empty for both
	Limit    int    `json:"limit,omitempty"`    // Max results per platform (default: 10)
}

// SearchSongsResponse represents the response for search results
type SearchSongsResponse struct {
	Results map[string][]SearchResult `json:"results"` // platform -> results
	Query   SearchSongsRequest        `json:"query"`   // Echo back the query for reference
}

// SearchResult represents a single search result
type SearchResult struct {
	Title       string   `json:"title"`
	Artists     []string `json:"artists"`
	Album       string   `json:"album"`
	URL         string   `json:"url"`
	Platform    string   `json:"platform"`
	ISRC        string   `json:"isrc,omitempty"`
	DurationMs  int      `json:"duration_ms,omitempty"`
	ReleaseDate string   `json:"release_date,omitempty"`
	ImageURL    string   `json:"image_url,omitempty"`
	Available   bool     `json:"available"`
}

// SongHandler handles song-related requests
type SongHandler struct {
	resolutionService *services.SongResolutionService
	songRepository    repositories.SongRepository
}

// NewSongHandler creates a new song handler
func NewSongHandler(resolutionService *services.SongResolutionService, songRepository repositories.SongRepository) *SongHandler {
	return &SongHandler{
		resolutionService: resolutionService,
		songRepository:    songRepository,
	}
}

// ResolveSong handles POST /api/v1/songs/resolve
func (h *SongHandler) ResolveSong(c *gin.Context) {
	var req ResolveSongRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Resolve the song from the URL
	song, err := h.resolutionService.ResolveFromURL(c.Request.Context(), req.URL)
	if err != nil {
		slog.Error("Failed to resolve song", "url", req.URL, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to resolve song from URL",
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
	response := ResolveSongResponse{
		Song: SongMetadata{
			ID:          song.ID.Hex(),
			Title:       song.Title,
			Artists:     []string{song.Artist}, // TODO: Parse comma-separated artists
			Album:       song.Album,
			DurationMs:  song.Metadata.Duration,
			ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
			ISRC:        song.ISRC,
		},
		Platforms: make(map[string]PlatformLink),
		UniversalLink: "https://songshare.app/s/" + song.ID.Hex()[:8], // Short ID for universal links
	}

	// Add platform links
	for _, link := range song.PlatformLinks {
		response.Platforms[link.Platform] = PlatformLink{
			URL:       link.URL,
			Available: link.Available,
			Platform:  link.Platform,
		}
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
		Results: make(map[string][]SearchResult),
		Query:   req,
	}

	// Search platforms based on filter
	platforms := []string{}
	if req.Platform == "" {
		platforms = []string{"spotify", "apple_music"} // Both platforms
	} else if req.Platform == "spotify" || req.Platform == "apple_music" {
		platforms = []string{req.Platform}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid platform. Use 'spotify', 'apple_music', or omit for both",
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
			response.Results[platform] = []SearchResult{}
			continue
		}

		// Convert tracks to search results
		results := make([]SearchResult, len(tracks))
		for i, track := range tracks {
			results[i] = SearchResult{
				Title:       track.Title,
				Artists:     track.Artists,
				Album:       track.Album,
				URL:         track.URL,
				Platform:    track.Platform,
				ISRC:        track.ISRC,
				DurationMs:  track.Duration,
				ReleaseDate: track.ReleaseDate,
				ImageURL:    track.ImageURL,
				Available:   track.Available,
			}
		}

		response.Results[platform] = results
	}

	c.JSON(http.StatusOK, response)
}

// expandShortID converts a short ID back to full ObjectID by searching songs
func (h *SongHandler) expandShortID(ctx context.Context, shortID string) (string, error) {
	// For now, use a simple approach - in production, you'd want a proper mapping system
	// This searches for songs where the first 8 characters of the hex ID match
	songs, err := h.songRepository.Search(ctx, "", 100) // Search all songs with limit
	if err != nil {
		return "", err
	}

	for _, song := range songs {
		if song.ID.Hex()[:8] == shortID {
			return song.ID.Hex(), nil
		}
	}

	return "", fmt.Errorf("song not found")
}

// renderSongJSON returns JSON response for the song
func (h *SongHandler) renderSongJSON(c *gin.Context, song *models.Song) {
	response := ResolveSongResponse{
		Song: SongMetadata{
			ID:          song.ID.Hex(),
			Title:       song.Title,
			Artists:     []string{song.Artist}, // TODO: Parse comma-separated artists
			Album:       song.Album,
			DurationMs:  song.Metadata.Duration,
			ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
			ISRC:        song.ISRC,
		},
		Platforms:     make(map[string]PlatformLink),
		UniversalLink: "https://songshare.app/s/" + song.ID.Hex()[:8],
	}

	// Add platform links
	for _, link := range song.PlatformLinks {
		response.Platforms[link.Platform] = PlatformLink{
			URL:       link.URL,
			Available: link.Available,
			Platform:  link.Platform,
		}
	}

	c.JSON(http.StatusOK, response)
}

// renderSongPage returns HTML page with HTMX support
func (h *SongHandler) renderSongPage(c *gin.Context, song *models.Song) {
	// Create template data
	data := struct {
		Song         *models.Song
		PlatformURLs map[string]string
	}{
		Song:         song,
		PlatformURLs: make(map[string]string),
	}

	// Extract platform URLs
	for _, link := range song.PlatformLinks {
		if link.Available {
			data.PlatformURLs[link.Platform] = link.URL
		}
	}

	// HTML template with HTMX support
	htmlTemplate := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Song.Title}} - {{.Song.Artist}}</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 2rem auto; padding: 1rem; }
        .song-header { text-align: center; margin-bottom: 2rem; }
        .song-title { font-size: 2rem; font-weight: bold; margin-bottom: 0.5rem; }
        .song-artist { font-size: 1.2rem; color: #666; margin-bottom: 0.5rem; }
        .song-album { font-size: 1rem; color: #888; }
        .platforms { display: flex; flex-direction: column; gap: 1rem; }
        .platform-button { display: block; padding: 1rem; border: 2px solid #ddd; border-radius: 8px; text-decoration: none; color: inherit; transition: all 0.2s; }
        .platform-button:hover { border-color: #007AFF; transform: translateY(-1px); }
        .spotify { border-color: #1DB954; }
        .apple-music { border-color: #FA243C; }
        .platform-name { font-weight: bold; font-size: 1.1rem; }
        .platform-desc { font-size: 0.9rem; color: #666; margin-top: 0.25rem; }
    </style>
</head>
<body>
    <div class="song-header">
        <div class="song-title">{{.Song.Title}}</div>
        <div class="song-artist">{{.Song.Artist}}</div>
        {{if .Song.Album}}<div class="song-album">{{.Song.Album}}</div>{{end}}
    </div>
    
    <div class="platforms">
        {{range $platform, $url := .PlatformURLs}}
        <a href="{{$url}}" class="platform-button {{$platform}}" 
           hx-get="/api/v1/analytics/click?platform={{$platform}}&song={{$.Song.ID.Hex}}"
           hx-trigger="click"
           hx-swap="none">
            <div class="platform-name">
                {{if eq $platform "spotify"}}🎵 Open in Spotify{{end}}
                {{if eq $platform "apple_music"}}🎵 Open in Apple Music{{end}}
            </div>
            <div class="platform-desc">
                {{if eq $platform "spotify"}}Listen on Spotify{{end}}
                {{if eq $platform "apple_music"}}Listen on Apple Music{{end}}
            </div>
        </a>
        {{end}}
        
        {{if not .PlatformURLs}}
        <div style="text-align: center; color: #666;">
            <p>This song is not currently available on supported platforms.</p>
        </div>
        {{end}}
    </div>
    
    <div style="text-align: center; margin-top: 2rem; font-size: 0.8rem; color: #999;">
        <p>Powered by SongShare</p>
    </div>
</body>
</html>`

	tmpl, err := template.New("song").Parse(htmlTemplate)
	if err != nil {
		slog.Error("Failed to parse song template", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template error"})
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		slog.Error("Failed to execute song template", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Render error"})
	}
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

	// Convert short ID back to full ObjectID
	fullID, err := h.expandShortID(c.Request.Context(), songID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Song not found",
		})
		return
	}

	// Look up the song
	song, err := h.songRepository.FindByID(c.Request.Context(), fullID)
	if err != nil || song == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Song not found",
		})
		return
	}

	// Check Accept header for content negotiation
	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*") {
		// Return HTML page with HTMX support
		h.renderSongPage(c, song)
	} else {
		// Return JSON response
		h.renderSongJSON(c, song)
	}
}