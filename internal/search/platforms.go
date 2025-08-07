package search

import (
	"context"
	"log/slog"

	"songshare/internal/handlers/render"
	"songshare/internal/services"
)

// SearchPlatforms searches external platforms
func (c *Coordinator) SearchPlatforms(ctx context.Context, query string, platformFilter string, limit int) ([]render.SearchResult, error) {
	searchQuery := services.SearchQuery{
		Query: query,
		Limit: limit,
	}

	// Determine which platforms to search
	var platforms []string
	if platformFilter == "" {
		platforms = []string{"apple_music", "spotify", "tidal"}
	} else {
		platforms = []string{platformFilter}
	}

	var allResults []render.SearchResult

	// Search each platform
	for _, platform := range platforms {
		var platformService services.PlatformService
		switch platform {
		case "spotify":
			platformService = c.resolutionService.GetPlatformService("spotify")
		case "apple_music":
			platformService = c.resolutionService.GetPlatformService("apple_music")
		case "tidal":
			platformService = c.resolutionService.GetPlatformService("tidal")
		default:
			continue
		}

		if platformService == nil {
			slog.Warn("Platform service not available", "platform", platform)
			continue
		}

		tracks, err := platformService.SearchTrack(ctx, searchQuery)
		if err != nil {
			slog.Warn("Platform search failed", "platform", platform, "error", err)
			continue
		}

		// Convert to search results
		for _, track := range tracks {
			result := render.SearchResult{
				Title:       track.Title,
				Artists:     track.Artists,
				Album:       track.Album,
				URL:         track.URL,
				Platform:    track.Platform,
				ISRC:        track.ISRC,
				DurationMs:  track.Duration,
				ReleaseDate: track.ReleaseDate,
				ImageURL:    track.ImageURL,
				Popularity:  track.Popularity,
				Explicit:    track.Explicit,
				Available:   true,
			}
			allResults = append(allResults, result)
		}
	}

	return allResults, nil
}