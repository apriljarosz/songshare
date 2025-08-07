package search

import (
	"context"
	"log/slog"

	"songshare/internal/handlers/render"
	"songshare/internal/repositories"
	"songshare/internal/scoring"
	"songshare/internal/services"
)

// PlatformResult represents a single platform result for a song
type PlatformResult struct {
	Platform  string
	URL       string
	Available bool
	Source    string // "local" or "platform"
}

// GroupedSearchResult represents a song with multiple platform links
type GroupedSearchResult struct {
	ID          string // Unique identifier for this result
	Title       string
	Artists     []string
	Album       string
	ISRC        string
	DurationMs  int
	ReleaseDate string
	ImageURL    string
	Popularity  int  // Highest popularity across all platforms (0-100)
	Explicit    bool // True if this version is explicit

	// Multiple platform links for the same song
	PlatformLinks []PlatformResult

	// Indicate if this song is already in local database
	HasLocalLink bool
	LocalURL     string

	// For sorting - track the best relevance score and position
	RelevanceScore float64
	OriginalIndex  int
}

// Coordinator handles search operations across local and platform sources
type Coordinator struct {
	songRepository    repositories.SongRepository
	resolutionService *services.SongResolutionService
	scorer            *scoring.RelevanceScorer
	baseURL           string
}

// NewCoordinator creates a new search coordinator
func NewCoordinator(songRepo repositories.SongRepository, resolutionService *services.SongResolutionService, baseURL string) *Coordinator {
	return &Coordinator{
		songRepository:    songRepo,
		resolutionService: resolutionService,
		scorer:            scoring.NewRelevanceScorer(),
		baseURL:           baseURL,
	}
}

// SearchAll performs comprehensive search across all sources
func (c *Coordinator) SearchAll(ctx context.Context, query string, platformFilter string, limit int) ([]render.SearchResultWithSource, error) {
	var allResults []render.SearchResultWithSource

	// Search local database
	localResults, err := c.SearchLocal(ctx, query, limit)
	if err != nil {
		slog.Warn("Local search failed", "error", err)
	} else {
		for _, result := range localResults {
			allResults = append(allResults, render.SearchResultWithSource{
				SearchResult: result,
				Source:       "local",
			})
		}
	}

	// Search platforms
	platformResults, err := c.SearchPlatforms(ctx, query, platformFilter, limit)
	if err != nil {
		slog.Warn("Platform search failed", "error", err)
	} else {
		for _, track := range platformResults {
			allResults = append(allResults, render.SearchResultWithSource{
				SearchResult: track,
				Source:       "platform",
			})
		}
	}

	return allResults, nil
}

// GroupResults groups search results by song, combining multiple platform entries
func (c *Coordinator) GroupResults(results []render.SearchResultWithSource, query string) []GroupedSearchResult {
	// Map to group results by unique songs
	songMap := make(map[string]*GroupedSearchResult)

	// Convert to scoring format for relevance calculation
	var scoringResults []scoring.SearchResultWithSource
	for _, result := range results {
		scoringResults = append(scoringResults, scoring.SearchResultWithSource{
			SearchResult: c.convertToScoringSearchResult(result.SearchResult),
			Source:       result.Source,
		})
	}

	for i, item := range results {
		result := item.SearchResult

		// Generate a unique key for grouping similar songs
		songKey := c.generateSongKey(result)

		if existing, exists := songMap[songKey]; exists {
			// Check if this platform already exists to prevent duplicates
			platformExists := false
			for i, existingPlatform := range existing.PlatformLinks {
				if existingPlatform.Platform == result.Platform {
					// Platform already exists, update it if this one has better info
					if existingPlatform.URL == "" && result.URL != "" {
						existing.PlatformLinks[i].URL = result.URL
					}
					if !existingPlatform.Available && result.Available {
						existing.PlatformLinks[i].Available = result.Available
					}
					platformExists = true
					break
				}
			}

			// Only add platform if it doesn't already exist
			if !platformExists {
				existing.PlatformLinks = append(existing.PlatformLinks, PlatformResult{
					Platform:  result.Platform,
					URL:       result.URL,
					Available: result.Available,
					Source:    item.Source,
				})
			}

			// Update highest popularity and other metadata
			if result.Popularity > existing.Popularity {
				existing.Popularity = result.Popularity
			}

			// Use highest quality metadata (prefer longer descriptions, newer release dates, etc.)
			if len(result.Album) > len(existing.Album) {
				existing.Album = result.Album
			}
			if result.ImageURL != "" && existing.ImageURL == "" {
				existing.ImageURL = result.ImageURL
			}

			// Track if song has local link
			if item.Source == "local" {
				existing.HasLocalLink = true
				existing.LocalURL = result.URL
			}

			// Calculate aggregate popularity using scoring results
			if i < len(scoringResults) {
				existing.Popularity = c.scorer.CalculateAggregatePopularity(scoringResults, existing.ISRC)
			}
		} else {
			// Create new grouped result
			grouped := &GroupedSearchResult{
				ID:             c.generateSearchResultID(result),
				Title:          result.Title,
				Artists:        result.Artists,
				Album:          result.Album,
				ISRC:           result.ISRC,
				DurationMs:     result.DurationMs,
				ReleaseDate:    result.ReleaseDate,
				ImageURL:       result.ImageURL,
				Explicit:       result.Explicit,
				PlatformLinks:  []PlatformResult{{
					Platform:  result.Platform,
					URL:       result.URL,
					Available: result.Available,
					Source:    item.Source,
				}},
				HasLocalLink: item.Source == "local",
				LocalURL:     "", // Will be set below if applicable
			}

			if item.Source == "local" {
				grouped.LocalURL = result.URL
			}

			// Set popularity from scoring results if available
			if i < len(scoringResults) {
				grouped.Popularity = c.scorer.CalculateAggregatePopularity(scoringResults, result.ISRC)
			} else {
				grouped.Popularity = result.Popularity
			}

			songMap[songKey] = grouped
		}
	}

	// Convert map to slice and sort by relevance/popularity
	var groupedResults []GroupedSearchResult
	for _, result := range songMap {
		groupedResults = append(groupedResults, *result)
	}

	// Sort by popularity (highest first)
	for i := 0; i < len(groupedResults); i++ {
		for j := i + 1; j < len(groupedResults); j++ {
			if groupedResults[j].Popularity > groupedResults[i].Popularity {
				groupedResults[i], groupedResults[j] = groupedResults[j], groupedResults[i]
			}
		}
	}

	return groupedResults
}