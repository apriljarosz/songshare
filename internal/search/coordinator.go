package search

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"songshare/internal/config"
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

	// Optional debug breakdown
	Debug *scoring.RelevanceBreakdown
	// Debug-only fields for visibility into popularity handling
	DebugRepPlatform   string
	DebugAggPopularity int
	DebugPlatformPops  map[string]int
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
func (c *Coordinator) GroupResults(results []render.SearchResultWithSource, query string, includeDebug bool) []GroupedSearchResult {
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

			// Calculate aggregate popularity using scoring results only when ISRC is known
			if i < len(scoringResults) && existing.ISRC != "" {
				existing.Popularity = c.scorer.CalculateAggregatePopularity(scoringResults, existing.ISRC)
			}
		} else {
			// Create new grouped result
			grouped := &GroupedSearchResult{
				ID:          c.generateSearchResultID(result),
				Title:       result.Title,
				Artists:     result.Artists,
				Album:       result.Album,
				ISRC:        result.ISRC,
				DurationMs:  result.DurationMs,
				ReleaseDate: result.ReleaseDate,
				ImageURL:    result.ImageURL,
				Explicit:    result.Explicit,
				PlatformLinks: []PlatformResult{{
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

			// Set popularity: aggregate across platforms when ISRC is known; otherwise use this track's popularity
			if i < len(scoringResults) && result.ISRC != "" {
				grouped.Popularity = c.scorer.CalculateAggregatePopularity(scoringResults, result.ISRC)
			} else {
				grouped.Popularity = result.Popularity
			}

			songMap[songKey] = grouped
		}
	}

	// Convert map to slice
	var groupedResults []GroupedSearchResult
	for _, result := range songMap {
		groupedResults = append(groupedResults, *result)
	}

	// Rank by relevance using scoring.RelevanceScorer with the original flat results as context
	// Build a flat scoring list for representative items (choose platform by max per-platform popularity)
	var representative []scoring.SearchResultWithSource
	for gi := range groupedResults {
		grp := &groupedResults[gi]
		// Choose representative platform by highest platform-specific popularity when possible
		repPlatform := c.chooseRepresentativePlatform(*grp, scoringResults)
		// Determine source: "platform" unless we had a local link
		source := "platform"
		if grp.HasLocalLink {
			source = "local"
		}
		// Find URL for the chosen platform
		repURL := ""
		for _, pl := range grp.PlatformLinks {
			if pl.Platform == repPlatform {
				repURL = pl.URL
				break
			}
		}
		if repURL == "" && len(grp.PlatformLinks) > 0 {
			repPlatform = grp.PlatformLinks[0].Platform
			repURL = grp.PlatformLinks[0].URL
		}
		// Select platform-specific popularity for the representative if available
		repPopularity := grp.Popularity
		if grp.ISRC != "" {
			for _, sr := range scoringResults {
				if sr.SearchResult.ISRC == grp.ISRC && sr.SearchResult.Platform == repPlatform && sr.SearchResult.Popularity > 0 {
					repPopularity = sr.SearchResult.Popularity
					break
				}
			}
		}

		// Populate debug fields
		if grp.ISRC != "" {
			platformPops := make(map[string]int)
			for _, sr := range scoringResults {
				if sr.SearchResult.ISRC == grp.ISRC && sr.SearchResult.Popularity > 0 {
					platformPops[sr.SearchResult.Platform] = sr.SearchResult.Popularity
				}
			}
			grp.DebugPlatformPops = platformPops
			grp.DebugRepPlatform = repPlatform
			grp.DebugAggPopularity = c.scorer.CalculateAggregatePopularity(scoringResults, grp.ISRC)
		} else {
			grp.DebugRepPlatform = repPlatform
		}

		representative = append(representative, scoring.SearchResultWithSource{
			SearchResult: scoring.SearchResult{
				Platform:    repPlatform,
				URL:         repURL,
				Title:       grp.Title,
				Artists:     grp.Artists,
				Album:       grp.Album,
				ISRC:        grp.ISRC,
				DurationMs:  grp.DurationMs,
				ReleaseDate: grp.ReleaseDate,
				ImageURL:    grp.ImageURL,
				Popularity:  repPopularity,
				Explicit:    grp.Explicit,
				Available:   true,
			},
			Source: source,
		})
	}

	// Compute relevance scores per group
	for i := range groupedResults {
		if i < len(representative) {
			groupedResults[i].RelevanceScore = c.scorer.CalculateRelevanceScore(
				representative[i].SearchResult,
				representative[i].Source,
				query,
				i,
				representative,
			)
			groupedResults[i].OriginalIndex = i
			// Attach debug breakdown if requested
			if includeDebug {
				bd := c.scorer.CalculateRelevanceBreakdown(
					representative[i].SearchResult,
					representative[i].Source,
					query,
					i,
					representative,
				)
				groupedResults[i].Debug = &bd
			}
		}
	}

	// Sort by relevance score desc; if close, prefer higher popularity; ensure deterministic final order
	cfg := config.GetRankingConfig()
	epsilon := cfg.TieEpsilon
	if epsilon <= 0 {
		epsilon = 2.5
	}
	sort.SliceStable(groupedResults, func(i, j int) bool {
		si := groupedResults[i].RelevanceScore
		sj := groupedResults[j].RelevanceScore
		if si != sj {
			// Treat small differences as ties and fall back to popularity
			if diff := si - sj; diff > 0 {
				if diff < epsilon { // epsilon threshold
					if groupedResults[i].Popularity != groupedResults[j].Popularity {
						return groupedResults[i].Popularity > groupedResults[j].Popularity
					}
					// Deterministic fallbacks
					ti := strings.ToLower(groupedResults[i].Title)
					tj := strings.ToLower(groupedResults[j].Title)
					if ti != tj {
						return ti < tj
					}
					ai := ""
					if len(groupedResults[i].Artists) > 0 {
						ai = strings.ToLower(groupedResults[i].Artists[0])
					}
					aj := ""
					if len(groupedResults[j].Artists) > 0 {
						aj = strings.ToLower(groupedResults[j].Artists[0])
					}
					if ai != aj {
						return ai < aj
					}
					albi := strings.ToLower(groupedResults[i].Album)
					albj := strings.ToLower(groupedResults[j].Album)
					if albi != albj {
						return albi < albj
					}
					return groupedResults[i].OriginalIndex < groupedResults[j].OriginalIndex
				}
				return true
			}
			// si < sj
			if sj-si < epsilon {
				if groupedResults[i].Popularity != groupedResults[j].Popularity {
					return groupedResults[i].Popularity > groupedResults[j].Popularity
				}
				// Deterministic fallbacks
				ti := strings.ToLower(groupedResults[i].Title)
				tj := strings.ToLower(groupedResults[j].Title)
				if ti != tj {
					return ti < tj
				}
				ai := ""
				if len(groupedResults[i].Artists) > 0 {
					ai = strings.ToLower(groupedResults[i].Artists[0])
				}
				aj := ""
				if len(groupedResults[j].Artists) > 0 {
					aj = strings.ToLower(groupedResults[j].Artists[0])
				}
				if ai != aj {
					return ai < aj
				}
				albi := strings.ToLower(groupedResults[i].Album)
				albj := strings.ToLower(groupedResults[j].Album)
				if albi != albj {
					return albi < albj
				}
				return groupedResults[i].OriginalIndex < groupedResults[j].OriginalIndex
			}
			return false
		}
		if groupedResults[i].Popularity != groupedResults[j].Popularity {
			return groupedResults[i].Popularity > groupedResults[j].Popularity
		}
		// Deterministic fallback on title, first artist, and album
		ti := strings.ToLower(groupedResults[i].Title)
		tj := strings.ToLower(groupedResults[j].Title)
		if ti != tj {
			return ti < tj
		}
		ai := ""
		if len(groupedResults[i].Artists) > 0 {
			ai = strings.ToLower(groupedResults[i].Artists[0])
		}
		aj := ""
		if len(groupedResults[j].Artists) > 0 {
			aj = strings.ToLower(groupedResults[j].Artists[0])
		}
		if ai != aj {
			return ai < aj
		}
		albi := strings.ToLower(groupedResults[i].Album)
		albj := strings.ToLower(groupedResults[j].Album)
		if albi != albj {
			return albi < albj
		}
		return groupedResults[i].OriginalIndex < groupedResults[j].OriginalIndex
	})

	return groupedResults
}

// chooseRepresentativePlatform selects the platform to represent a group when scoring
// Prefers the platform with the highest popularity for the same ISRC across scoringResults.
// Falls back to the first available platform when data is missing.
func (c *Coordinator) chooseRepresentativePlatform(grp GroupedSearchResult, scoringResults []scoring.SearchResultWithSource) string {
	if grp.ISRC != "" {
		bestPlatform := ""
		bestPopularity := -1
		// Look through all scoring results for same ISRC
		for _, sr := range scoringResults {
			if sr.SearchResult.ISRC == grp.ISRC {
				if sr.SearchResult.Popularity > bestPopularity {
					bestPopularity = sr.SearchResult.Popularity
					bestPlatform = sr.SearchResult.Platform
				}
			}
		}
		if bestPlatform != "" {
			return bestPlatform
		}
	}
	// Fallback to first platform link
	if len(grp.PlatformLinks) > 0 {
		return grp.PlatformLinks[0].Platform
	}
	return ""
}
