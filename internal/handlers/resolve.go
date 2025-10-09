package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/apriljarosz/songshare/internal/models"
	"github.com/apriljarosz/songshare/internal/repository"
	"github.com/apriljarosz/songshare/internal/services"
	"github.com/gin-gonic/gin"
)

type ResolveHandler struct {
	spotifyService *services.SpotifyService
	songRepo       *repository.SongRepository
}

func NewResolveHandler(spotifyService *services.SpotifyService, songRepo *repository.SongRepository) *ResolveHandler {
	return &ResolveHandler{
		spotifyService: spotifyService,
		songRepo:       songRepo,
	}
}

func (h *ResolveHandler) Resolve(c *gin.Context) {
	var req models.ResolveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url is required"})
		return
	}

	// Parse URL to detect platform
	parsed, err := services.ParsePlatformURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or unsupported URL", "details": err.Error()})
		return
	}

	var song *models.Song

	// Get track from platform
	switch parsed.Platform {
	case "spotify":
		song, err = h.spotifyService.GetTrackByID(parsed.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get track from Spotify", "details": err.Error()})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported platform"})
		return
	}

	if song == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "song not found"})
		return
	}

	// Save to database if has ISRC
	if song.ISRC != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Check if song exists
		existing, err := h.songRepo.FindByISRC(ctx, song.ISRC)
		if err == nil && existing != nil {
			// Merge with existing
			song.ID = existing.ID
			song.Platforms = h.mergePlatforms(existing.Platforms, song.Platforms)
		}

		// Save
		if err := h.songRepo.Save(ctx, song); err != nil {
			// Log error but still return the song
		}
	}

	c.JSON(http.StatusOK, gin.H{"song": song})
}

func (h *ResolveHandler) mergePlatforms(existing, new []models.PlatformLink) []models.PlatformLink {
	platformMap := make(map[string]models.PlatformLink)
	for _, p := range existing {
		platformMap[p.Platform] = p
	}
	for _, p := range new {
		platformMap[p.Platform] = p
	}

	// Define consistent platform order
	platformOrder := []string{"spotify", "apple_music", "youtube_music", "tidal", "deezer"}

	// Convert back to slice in consistent order
	result := make([]models.PlatformLink, 0, len(platformMap))
	for _, platformName := range platformOrder {
		if p, exists := platformMap[platformName]; exists {
			result = append(result, p)
		}
	}
	return result
}
