package services

import (
	"fmt"

	"github.com/apriljarosz/songshare/internal/models"
)

type ResolverService struct {
	spotifyService    *SpotifyService
	appleMusicService *AppleMusicService
}

func NewResolverService(spotifyService *SpotifyService, appleMusicService *AppleMusicService) *ResolverService {
	return &ResolverService{
		spotifyService:    spotifyService,
		appleMusicService: appleMusicService,
	}
}

// ResolveByISRC finds the song on all platforms using ISRC
func (r *ResolverService) ResolveByISRC(isrc string) (*models.Song, error) {
	if isrc == "" {
		return nil, fmt.Errorf("ISRC is required")
	}

	var mergedSong *models.Song

	// Try Apple Music first (has direct ISRC lookup)
	if r.appleMusicService != nil {
		appleSong, err := r.appleMusicService.GetSongByISRC(isrc)
		if err == nil && appleSong != nil {
			mergedSong = appleSong
		}
	}

	// If we don't have a song yet, we'd need to search Spotify
	// (Spotify doesn't have direct ISRC lookup in their public API)
	// For now, if we have Apple Music data, we use that

	if mergedSong == nil {
		return nil, fmt.Errorf("song not found with ISRC: %s", isrc)
	}

	return mergedSong, nil
}

// MergePlatformLinks combines platform links from multiple songs
func (r *ResolverService) MergePlatformLinks(existing, new []models.PlatformLink) []models.PlatformLink {
	platformMap := make(map[string]models.PlatformLink)

	// Add existing platforms
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
