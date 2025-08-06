package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"songshare/internal/models"
	"songshare/internal/repositories"
)

// SongResolutionService handles song resolution and cross-platform matching
type SongResolutionService struct {
	songRepo         repositories.SongRepository
	platformServices map[string]PlatformService
}

// NewSongResolutionService creates a new song resolution service
func NewSongResolutionService(songRepo repositories.SongRepository) *SongResolutionService {
	return &SongResolutionService{
		songRepo:         songRepo,
		platformServices: make(map[string]PlatformService),
	}
}

// RegisterPlatform registers a platform service
func (s *SongResolutionService) RegisterPlatform(service PlatformService) {
	s.platformServices[service.GetPlatformName()] = service
}

// GetPlatformService returns a platform service by name
func (s *SongResolutionService) GetPlatformService(platformName string) PlatformService {
	return s.platformServices[platformName]
}

// ResolveFromURL resolves a song from a platform URL and returns all available platform links
func (s *SongResolutionService) ResolveFromURL(ctx context.Context, url string) (*models.Song, error) {
	// Parse the URL to determine platform and track ID
	platform, trackID, err := ParsePlatformURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse platform URL: %w", err)
	}

	// Get the platform service
	platformService, exists := s.platformServices[platform]
	if !exists {
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}

	// Check if we already have this song by platform ID
	existingSong, err := s.songRepo.FindByPlatformID(ctx, platform, trackID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing song: %w", err)
	}

	if existingSong != nil {
		slog.Info("Found existing song by platform ID", 
			"platform", platform, 
			"trackID", trackID, 
			"songID", existingSong.ID.Hex())
		
		// Resolve on other platforms if needed
		return s.resolveOnOtherPlatforms(ctx, existingSong)
	}

	// Fetch track info from the platform
	trackInfo, err := platformService.GetTrackByID(ctx, trackID)
	if err != nil {
		return nil, fmt.Errorf("failed to get track info from %s: %w", platform, err)
	}

	// Try to find existing song by ISRC first
	if trackInfo.ISRC != "" {
		existingSong, err := s.songRepo.FindByISRC(ctx, trackInfo.ISRC)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing song by ISRC: %w", err)
		}

		if existingSong != nil {
			slog.Info("Found existing song by ISRC", 
				"isrc", trackInfo.ISRC, 
				"songID", existingSong.ID.Hex())
			
			// Add this platform link if it doesn't exist
			if !existingSong.HasPlatform(platform) {
				existingSong.AddPlatformLink(platform, trackID, trackInfo.URL, 1.0)
				if err := s.songRepo.Update(ctx, existingSong); err != nil {
					slog.Error("Failed to update song with new platform link", "error", err)
				}
			}

			return s.resolveOnOtherPlatforms(ctx, existingSong)
		}
	}

	// Try fuzzy matching by title and artist
	if trackInfo.Title != "" && len(trackInfo.Artists) > 0 {
		primaryArtist := trackInfo.Artists[0]
		similarSongs, err := s.songRepo.FindByTitleArtist(ctx, trackInfo.Title, primaryArtist)
		if err != nil {
			slog.Error("Failed to search for similar songs", "error", err)
		} else if len(similarSongs) > 0 {
			// Use the first match with high confidence
			existingSong := similarSongs[0]
			slog.Info("Found similar song by title/artist", 
				"title", trackInfo.Title,
				"artist", primaryArtist,
				"songID", existingSong.ID.Hex())

			// Add this platform link
			if !existingSong.HasPlatform(platform) {
				existingSong.AddPlatformLink(platform, trackID, trackInfo.URL, 0.8) // Lower confidence for fuzzy match
				if err := s.songRepo.Update(ctx, existingSong); err != nil {
					slog.Error("Failed to update song with new platform link", "error", err)
				}
			}

			return s.resolveOnOtherPlatforms(ctx, existingSong)
		}
	}

	// Create new song from track info
	song := trackInfo.ToSong()
	
	if err := s.songRepo.Save(ctx, song); err != nil {
		return nil, fmt.Errorf("failed to save new song: %w", err)
	}

	slog.Info("Created new song", 
		"songID", song.ID.Hex(),
		"title", song.Title,
		"platform", platform)

	// Try to find this song on other platforms
	return s.resolveOnOtherPlatforms(ctx, song)
}

// resolveOnOtherPlatforms attempts to find the song on other platforms
func (s *SongResolutionService) resolveOnOtherPlatforms(ctx context.Context, song *models.Song) (*models.Song, error) {
	updated := false

	for platformName, platformService := range s.platformServices {
		// Skip if we already have this platform
		if song.HasPlatform(platformName) {
			continue
		}

		// Try ISRC search first (most reliable)
		if song.ISRC != "" {
			trackInfo, err := platformService.GetTrackByISRC(ctx, song.ISRC)
			if err == nil && trackInfo != nil {
				song.AddPlatformLink(platformName, trackInfo.ExternalID, trackInfo.URL, 1.0)
				updated = true
				slog.Info("Found song on platform via ISRC", 
					"platform", platformName,
					"isrc", song.ISRC)
				continue
			}
		}

		// Try title/artist search as fallback
		if song.Title != "" && song.Artist != "" {
			query := SearchQuery{
				Title:  song.Title,
				Artist: song.Artist,
				Limit:  5,
			}

			tracks, err := platformService.SearchTrack(ctx, query)
			if err != nil {
				slog.Error("Failed to search on platform", 
					"platform", platformName, 
					"error", err)
				continue
			}

			// Find best match
			bestMatch := s.findBestMatch(song, tracks)
			if bestMatch != nil {
				confidence := s.calculateMatchConfidence(song, bestMatch)
				if confidence > 0.7 { // Only use high-confidence matches
					song.AddPlatformLink(platformName, bestMatch.ExternalID, bestMatch.URL, confidence)
					updated = true
					slog.Info("Found song on platform via search", 
						"platform", platformName,
						"confidence", confidence)
				}
			}
		}
	}

	// Update the song in database if we found new platforms
	if updated {
		if err := s.songRepo.Update(ctx, song); err != nil {
			slog.Error("Failed to update song with new platform links", "error", err)
		}
	}

	return song, nil
}

// findBestMatch finds the best matching track from search results
func (s *SongResolutionService) findBestMatch(song *models.Song, tracks []*TrackInfo) *TrackInfo {
	if len(tracks) == 0 {
		return nil
	}

	var bestMatch *TrackInfo
	bestScore := 0.0

	for _, track := range tracks {
		score := s.calculateMatchConfidence(song, track)
		if score > bestScore {
			bestScore = score
			bestMatch = track
		}
	}

	return bestMatch
}

// calculateMatchConfidence calculates match confidence between a song and track info
func (s *SongResolutionService) calculateMatchConfidence(song *models.Song, track *TrackInfo) float64 {
	score := 0.0

	// ISRC match is definitive
	if song.ISRC != "" && track.ISRC != "" && song.ISRC == track.ISRC {
		return 1.0
	}

	// Title match (case-insensitive)
	if strings.EqualFold(song.Title, track.Title) {
		score += 0.5
	} else if s.fuzzyStringMatch(song.Title, track.Title) {
		score += 0.3
	}

	// Artist match
	if s.artistsMatch(song.Artist, track.Artists) {
		score += 0.4
	}

	// Album match (if available)
	if song.Album != "" && track.Album != "" {
		if strings.EqualFold(song.Album, track.Album) {
			score += 0.1
		}
	}

	return score
}

// fuzzyStringMatch performs basic fuzzy string matching
func (s *SongResolutionService) fuzzyStringMatch(s1, s2 string) bool {
	// Simple fuzzy matching - remove common differences
	clean1 := strings.ToLower(strings.ReplaceAll(s1, " ", ""))
	clean2 := strings.ToLower(strings.ReplaceAll(s2, " ", ""))
	
	// Check if one contains the other (handles remix versions, etc.)
	return strings.Contains(clean1, clean2) || strings.Contains(clean2, clean1)
}

// artistsMatch checks if artists match between song and track info
func (s *SongResolutionService) artistsMatch(songArtist string, trackArtists []string) bool {
	if len(trackArtists) == 0 {
		return false
	}

	songArtistLower := strings.ToLower(songArtist)
	
	for _, artist := range trackArtists {
		if strings.EqualFold(songArtist, artist) {
			return true
		}
		
		// Check if song artist contains track artist or vice versa
		artistLower := strings.ToLower(artist)
		if strings.Contains(songArtistLower, artistLower) || strings.Contains(artistLower, songArtistLower) {
			return true
		}
	}

	return false
}

// GetSupportedPlatforms returns list of supported platforms
func (s *SongResolutionService) GetSupportedPlatforms() []string {
	platforms := make([]string, 0, len(s.platformServices))
	for platform := range s.platformServices {
		platforms = append(platforms, platform)
	}
	return platforms
}

// Health checks the health of all platform services
func (s *SongResolutionService) Health(ctx context.Context) map[string]error {
	results := make(map[string]error)
	
	for platform, service := range s.platformServices {
		results[platform] = service.Health(ctx)
	}
	
	return results
}