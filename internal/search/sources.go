package search

import (
	"context"
	"fmt"
	"time"

	"songshare/internal/repositories"
	"songshare/internal/services"
)

// LocalSource searches the local MongoDB database
type LocalSource struct {
	repository repositories.SongRepository
	baseURL    string
}

// NewLocalSource creates a new local database search source
func NewLocalSource(repository repositories.SongRepository) *LocalSource {
	return &LocalSource{
		repository: repository,
		baseURL:    "http://localhost:8080", // TODO: make configurable
	}
}

// Name returns the source identifier
func (ls *LocalSource) Name() string {
	return "local"
}

// IsEnabled returns whether this source is available
func (ls *LocalSource) IsEnabled() bool {
	return ls.repository != nil
}

// Search searches the local database for songs
func (ls *LocalSource) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	// Build query string - prefer specific fields over general query
	query := req.Query
	if req.Title != "" || req.Artist != "" || req.Album != "" {
		// For structured queries, combine fields
		parts := []string{}
		if req.Title != "" {
			parts = append(parts, req.Title)
		}
		if req.Artist != "" {
			parts = append(parts, req.Artist)
		}
		if req.Album != "" {
			parts = append(parts, req.Album)
		}
		query = fmt.Sprintf("%s", parts[0]) // Use first non-empty field as primary query
	}

	songs, err := ls.repository.Search(ctx, query, req.GetEffectiveLimit())
	if err != nil {
		return nil, fmt.Errorf("local database search failed: %w", err)
	}

	results := make([]SearchResult, 0, len(songs))
	for _, song := range songs {
		// Create results for each available platform link
		if len(song.PlatformLinks) > 0 {
			for _, link := range song.PlatformLinks {
				if link.Available {
					result := SearchResult{
						ID:          fmt.Sprintf("local-%s-%s", song.ID.Hex(), link.Platform),
						Title:       song.Title,
						Artists:     []string{song.Artist}, // TODO: support multiple artists
						Album:       song.Album,
						Platform:    link.Platform,
						URL:         link.URL,
						ImageURL:    song.Metadata.ImageURL,
						Popularity:  song.Metadata.Popularity,
						DurationMs:  song.Metadata.Duration,
						ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
						ISRC:        song.ISRC,
						Explicit:    song.Metadata.Explicit,
						Available:   true,
						Source:      "local",
						CachedAt:    time.Now(),
					}
					results = append(results, result)
				}
			}
		} else {
			// Fallback: create a local-only result
			result := SearchResult{
				ID:          fmt.Sprintf("local-%s", song.ID.Hex()),
				Title:       song.Title,
				Artists:     []string{song.Artist},
				Album:       song.Album,
				Platform:    "local",
				URL:         ls.buildUniversalLink(song.ISRC),
				ImageURL:    song.Metadata.ImageURL,
				Popularity:  song.Metadata.Popularity,
				DurationMs:  song.Metadata.Duration,
				ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
				ISRC:        song.ISRC,
				Explicit:    song.Metadata.Explicit,
				Available:   true,
				Source:      "local",
				CachedAt:    time.Now(),
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// buildUniversalLink creates a universal link for a song
func (ls *LocalSource) buildUniversalLink(isrc string) string {
	if isrc == "" {
		return fmt.Sprintf("%s/s/unknown", ls.baseURL)
	}
	return fmt.Sprintf("%s/s/%s", ls.baseURL, isrc)
}

// PlatformSource wraps a platform service as a search source
type PlatformSource struct {
	service  services.PlatformService
	platform string
}

// NewSpotifySource creates a Spotify search source
func NewSpotifySource(service services.PlatformService) *PlatformSource {
	return &PlatformSource{
		service:  service,
		platform: "spotify",
	}
}

// NewAppleSource creates an Apple Music search source
func NewAppleSource(service services.PlatformService) *PlatformSource {
	return &PlatformSource{
		service:  service,
		platform: "apple_music",
	}
}

// NewTidalSource creates a Tidal search source
func NewTidalSource(service services.PlatformService) *PlatformSource {
	return &PlatformSource{
		service:  service,
		platform: "tidal",
	}
}

// Name returns the platform name
func (ps *PlatformSource) Name() string {
	return ps.platform
}

// IsEnabled returns whether the platform service is available
func (ps *PlatformSource) IsEnabled() bool {
	if ps.service == nil {
		return false
	}
	
	// Quick health check
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	return ps.service.Health(ctx) == nil
}

// Search searches the platform API
func (ps *PlatformSource) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	// Build platform search query
	searchQuery := services.SearchQuery{
		Query:  req.Query,
		Title:  req.Title,
		Artist: req.Artist,
		Album:  req.Album,
		Limit:  req.GetEffectiveLimit(),
	}

	tracks, err := ps.service.SearchTrack(ctx, searchQuery)
	if err != nil {
		return nil, fmt.Errorf("platform %s search failed: %w", ps.platform, err)
	}

	results := make([]SearchResult, 0, len(tracks))
	for _, track := range tracks {
		result := SearchResult{
			ID:          fmt.Sprintf("%s-%s", ps.platform, track.ExternalID),
			Title:       track.Title,
			Artists:     track.Artists,
			Album:       track.Album,
			Platform:    track.Platform,
			URL:         track.URL,
			ImageURL:    track.ImageURL,
			Popularity:  track.Popularity,
			DurationMs:  track.Duration,
			ReleaseDate: track.ReleaseDate,
			ISRC:        track.ISRC,
			Explicit:    track.Explicit,
			Available:   track.Available,
			Source:      "platform",
			CachedAt:    time.Now(),
		}
		results = append(results, result)
	}

	return results, nil
}