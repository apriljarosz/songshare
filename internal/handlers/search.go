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

type SearchHandler struct {
	spotifyService    *services.SpotifyService
	appleMusicService *services.AppleMusicService
	songRepo          *repository.SongRepository
}

func NewSearchHandler(spotifyService *services.SpotifyService, appleMusicService *services.AppleMusicService, songRepo *repository.SongRepository) *SearchHandler {
	return &SearchHandler{
		spotifyService:    spotifyService,
		appleMusicService: appleMusicService,
		songRepo:          songRepo,
	}
}

func (h *SearchHandler) Search(c *gin.Context) {
	var req models.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query is required"})
		return
	}

	// Search both Spotify and Apple Music in parallel
	type searchResult struct {
		songs []models.Song
		err   error
	}

	spotifyResults := make(chan searchResult, 1)
	appleMusicResults := make(chan searchResult, 1)

	// Search Spotify
	go func() {
		songs, err := h.spotifyService.Search(req.Query)
		spotifyResults <- searchResult{songs: songs, err: err}
	}()

	// Search Apple Music
	go func() {
		if h.appleMusicService != nil {
			songs, err := h.appleMusicService.Search(req.Query)
			appleMusicResults <- searchResult{songs: songs, err: err}
		} else {
			appleMusicResults <- searchResult{songs: nil, err: nil}
		}
	}()

	// Wait for results
	spotifyRes := <-spotifyResults
	appleMusicRes := <-appleMusicResults

	if spotifyRes.err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "spotify search failed", "details": spotifyRes.err.Error()})
		return
	}

	// Merge songs by ISRC, preserving Spotify's order (better relevance ranking)
	songMap := make(map[string]int) // ISRC -> index in songs slice
	songs := make([]models.Song, 0, len(spotifyRes.songs))

	// Add Spotify songs first (preserves their relevance order)
	for i := range spotifyRes.songs {
		if spotifyRes.songs[i].ISRC != "" {
			songMap[spotifyRes.songs[i].ISRC] = len(songs)
			songs = append(songs, spotifyRes.songs[i])
		}
	}

	// Merge or append Apple Music songs
	if appleMusicRes.songs != nil {
		for i := range appleMusicRes.songs {
			if appleMusicRes.songs[i].ISRC != "" {
				if existingIdx, ok := songMap[appleMusicRes.songs[i].ISRC]; ok {
					// Merge Apple Music platform into existing Spotify song
					songs[existingIdx].Platforms = h.mergePlatforms(
						songs[existingIdx].Platforms,
						appleMusicRes.songs[i].Platforms,
					)
				} else {
					// New song only on Apple Music - append to end
					songMap[appleMusicRes.songs[i].ISRC] = len(songs)
					songs = append(songs, appleMusicRes.songs[i])
				}
			}
		}
	}

	// Save to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for i := range songs {
		if songs[i].ISRC != "" {
			// Check if song exists in database
			existing, err := h.songRepo.FindByISRC(ctx, songs[i].ISRC)
			if err != nil {
				continue // Skip on error
			}

			if existing != nil {
				// Update with merged platforms from database
				songs[i].ID = existing.ID
				songs[i].Platforms = h.mergePlatforms(existing.Platforms, songs[i].Platforms)
			}

			// Save to database
			if err := h.songRepo.Save(ctx, &songs[i]); err != nil {
				// Log error but don't fail the request
				continue
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"songs": songs})
}

func (h *SearchHandler) mergePlatforms(existing, new []models.PlatformLink) []models.PlatformLink {
	// Create map of existing platforms
	platformMap := make(map[string]models.PlatformLink)
	for _, p := range existing {
		platformMap[p.Platform] = p
	}

	// Add or update with new platforms
	for _, p := range new {
		platformMap[p.Platform] = p
	}

	// Convert back to slice
	result := make([]models.PlatformLink, 0, len(platformMap))
	for _, p := range platformMap {
		result = append(result, p)
	}

	return result
}
