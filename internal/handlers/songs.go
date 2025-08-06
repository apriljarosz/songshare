package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
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

// SongHandler handles song-related requests
type SongHandler struct {
	resolutionService *services.SongResolutionService
}

// NewSongHandler creates a new song handler
func NewSongHandler(resolutionService *services.SongResolutionService) *SongHandler {
	return &SongHandler{
		resolutionService: resolutionService,
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

// RedirectToSong handles GET /api/v1/s/:id - universal link redirects
func RedirectToSong(c *gin.Context) {
	songID := c.Param("id")
	if songID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing song ID",
		})
		return
	}

	// TODO: Implement redirect logic
	// 1. Look up song by short ID
	// 2. Determine user's preferred platform (from user-agent, query params, etc.)
	// 3. Redirect to appropriate platform URL
	// 4. Track analytics/metrics

	// For now, return mock data
	c.JSON(http.StatusOK, gin.H{
		"song_id": songID,
		"message": "Redirect functionality coming soon",
		"platforms": map[string]string{
			"spotify":     "https://open.spotify.com/track/example",
			"apple_music": "https://music.apple.com/us/album/example",
		},
	})
}