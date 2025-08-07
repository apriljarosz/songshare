package search

import (
	"context"
	"fmt"
	"log/slog"

	"songshare/internal/handlers/render"
	"songshare/internal/models"
)

// SearchLocal searches the local MongoDB database
func (c *Coordinator) SearchLocal(ctx context.Context, query string, limit int) ([]render.SearchResult, error) {
	songs, err := c.songRepository.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	var results []render.SearchResult
	for _, song := range songs {
		// For each song, create search results for each available platform
		// This allows the grouping logic to properly show platform badges
		if len(song.PlatformLinks) > 0 {
			for _, link := range song.PlatformLinks {
				if link.Available {
					result := render.SearchResult{
						Title:       song.Title,
						Artists:     []string{song.Artist},
						Album:       song.Album,
						URL:         link.URL, // Use the actual platform URL, not universal link
						Platform:    link.Platform,
						ISRC:        song.ISRC,
						DurationMs:  song.Metadata.Duration,
						ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
						ImageURL:    song.Metadata.ImageURL,
						Popularity:  song.Metadata.Popularity,
						Explicit:    song.Metadata.Explicit,
						Available:   true,
					}
					results = append(results, result)
				}
			}
		} else {
			// Fallback for songs without platform links
			result := render.SearchResult{
				Title:       song.Title,
				Artists:     []string{song.Artist},
				Album:       song.Album,
				URL:         c.buildUniversalLink(song),
				Platform:    "local",
				ISRC:        song.ISRC,
				DurationMs:  song.Metadata.Duration,
				ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
				ImageURL:    song.Metadata.ImageURL,
				Popularity:  song.Metadata.Popularity,
				Explicit:    song.Metadata.Explicit,
				Available:   true,
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// buildUniversalLink creates a universal link for a song using ISRC
func (c *Coordinator) buildUniversalLink(song *models.Song) string {
	if song.ISRC == "" {
		slog.Warn("Song missing ISRC", "songID", song.ID.Hex(), "title", song.Title)
		// This shouldn't happen with properly indexed songs
		return fmt.Sprintf("%s/s/unknown", c.baseURL)
	}
	return fmt.Sprintf("%s/s/%s", c.baseURL, song.ISRC)
}