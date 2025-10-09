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

	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 10
	}

	// For pagination to work correctly with cross-platform merging,
	// we need to fetch more results than requested and then slice the final result
	// Fetch enough to account for deduplication and ensure we have enough unique songs
	fetchLimit := (req.Offset + req.Limit) * 2
	if fetchLimit > 50 {
		fetchLimit = 50 // Spotify's max
	}

	// Search both Spotify and Apple Music in parallel
	type searchResult struct {
		songs []models.Song
		err   error
	}

	spotifyResults := make(chan searchResult, 1)
	appleMusicResults := make(chan searchResult, 1)

	// Search Spotify - fetch from beginning with larger limit
	go func() {
		songs, err := h.spotifyService.SearchWithPagination(req.Query, 0, fetchLimit)
		spotifyResults <- searchResult{songs: songs, err: err}
	}()

	// Search Apple Music - fetch from beginning with larger limit
	go func() {
		if h.appleMusicService != nil {
			apLimit := fetchLimit
			if apLimit > 25 {
				apLimit = 25 // Apple Music's max
			}
			songs, err := h.appleMusicService.SearchWithPagination(req.Query, 0, apLimit)
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

	// Merge songs by ISRC, preserving relevance ranking from both platforms
	type songWithRank struct {
		song         models.Song
		spotifyRank  int // 0 if not on Spotify, otherwise position + 1
		appleMusicRank int // 0 if not on Apple Music, otherwise position + 1
	}

	songMap := make(map[string]*songWithRank) // ISRC -> song with rankings

	// Add Spotify songs with their rankings
	for i := range spotifyRes.songs {
		if spotifyRes.songs[i].ISRC != "" {
			if _, exists := songMap[spotifyRes.songs[i].ISRC]; !exists {
				songMap[spotifyRes.songs[i].ISRC] = &songWithRank{
					song:        spotifyRes.songs[i],
					spotifyRank: i + 1,
				}
			}
		}
	}

	// Merge Apple Music songs and add their rankings
	if appleMusicRes.songs != nil {
		for i := range appleMusicRes.songs {
			if appleMusicRes.songs[i].ISRC != "" {
				if existing, ok := songMap[appleMusicRes.songs[i].ISRC]; ok {
					// Merge Apple Music platform into existing song
					existing.song.Platforms = h.mergePlatforms(
						existing.song.Platforms,
						appleMusicRes.songs[i].Platforms,
					)
					existing.appleMusicRank = i + 1
					// Use Apple Music metadata if it has explicit flag and Spotify doesn't
					if appleMusicRes.songs[i].Explicit && !existing.song.Explicit {
						existing.song.Explicit = true
					}
				} else {
					// New song only on Apple Music
					songMap[appleMusicRes.songs[i].ISRC] = &songWithRank{
						song:           appleMusicRes.songs[i],
						appleMusicRank: i + 1,
					}
				}
			}
		}
	}

	// Look up each ISRC on the opposite platform to ensure complete platform coverage
	for isrc, songData := range songMap {
		// If song doesn't have Spotify, look it up
		hasSpotify := false
		for _, p := range songData.song.Platforms {
			if p.Platform == "spotify" {
				hasSpotify = true
				break
			}
		}
		if !hasSpotify {
			if spotifySong, err := h.spotifyService.GetSongByISRC(isrc); err == nil && spotifySong != nil {
				songData.song.Platforms = h.mergePlatforms(songData.song.Platforms, spotifySong.Platforms)
			}
		}

		// If song doesn't have Apple Music, look it up
		hasAppleMusic := false
		for _, p := range songData.song.Platforms {
			if p.Platform == "apple_music" {
				hasAppleMusic = true
				break
			}
		}
		if !hasAppleMusic && h.appleMusicService != nil {
			if appleSong, err := h.appleMusicService.GetSongByISRC(isrc); err == nil && appleSong != nil {
				songData.song.Platforms = h.mergePlatforms(songData.song.Platforms, appleSong.Platforms)
				// Use Apple Music metadata if it has explicit flag and current song doesn't
				if appleSong.Explicit && !songData.song.Explicit {
					songData.song.Explicit = true
				}
			}
		}
	}

	// Convert map to slice and sort by best ranking from either platform
	songs := make([]models.Song, 0, len(songMap))
	rankedSongs := make([]*songWithRank, 0, len(songMap))
	for _, sr := range songMap {
		rankedSongs = append(rankedSongs, sr)
	}

	// Sort by best (lowest) rank from either platform
	// If song is on both platforms, use the better ranking
	// Ties are broken by preferring Spotify (since it generally has better relevance)
	for i := 0; i < len(rankedSongs); i++ {
		for j := i + 1; j < len(rankedSongs); j++ {
			iRank := rankedSongs[i].spotifyRank
			if rankedSongs[i].appleMusicRank > 0 && (iRank == 0 || rankedSongs[i].appleMusicRank < iRank) {
				iRank = rankedSongs[i].appleMusicRank
			}
			if iRank == 0 {
				iRank = 999 // No ranking from either platform (shouldn't happen)
			}

			jRank := rankedSongs[j].spotifyRank
			if rankedSongs[j].appleMusicRank > 0 && (jRank == 0 || rankedSongs[j].appleMusicRank < jRank) {
				jRank = rankedSongs[j].appleMusicRank
			}
			if jRank == 0 {
				jRank = 999
			}

			if jRank < iRank {
				rankedSongs[i], rankedSongs[j] = rankedSongs[j], rankedSongs[i]
			}
		}
	}

	for _, sr := range rankedSongs {
		songs = append(songs, sr.song)
	}

	// Apply pagination to the final sorted results
	start := req.Offset
	end := req.Offset + req.Limit

	if start >= len(songs) {
		// Offset is beyond available results
		c.JSON(http.StatusOK, gin.H{"songs": []models.Song{}})
		return
	}

	if end > len(songs) {
		end = len(songs)
	}

	songs = songs[start:end]

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
