package search

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"songshare/internal/services"
)

// EnhanceResultsPlatforms performs background platform enhancement for search results
func (c *Coordinator) EnhanceResultsPlatforms(ctx context.Context, results []GroupedSearchResult) {
	slog.Debug("Starting platform enhancement", "results", len(results))

	// Enhance each result in background
	for _, result := range results {
		go c.enhanceSingleResultPlatforms(ctx, result)
	}
}

// enhanceSingleResultPlatforms enhances a single result with additional platform links
func (c *Coordinator) enhanceSingleResultPlatforms(ctx context.Context, result GroupedSearchResult) {
	// Skip if already has links for all major platforms
	hasSpotify, hasAppleMusic, hasTidal := false, false, false
	for _, link := range result.PlatformLinks {
		switch link.Platform {
		case "spotify":
			hasSpotify = true
		case "apple_music":
			hasAppleMusic = true
		case "tidal":
			hasTidal = true
		}
	}

	// If we have all major platforms, no need to enhance
	if hasSpotify && hasAppleMusic && hasTidal {
		return
	}

	// Try to find the song on missing platforms using ISRC or title/artist search
	if result.ISRC != "" {
		c.enhanceByISRC(ctx, result, hasSpotify, hasAppleMusic, hasTidal)
	} else {
		c.enhanceBySearch(ctx, result, hasSpotify, hasAppleMusic, hasTidal)
	}
}

// enhanceByISRC attempts to find the song on missing platforms using ISRC
func (c *Coordinator) enhanceByISRC(ctx context.Context, result GroupedSearchResult, hasSpotify, hasAppleMusic, hasTidal bool) {
	enhanceCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	platforms := []struct {
		name    string
		missing bool
		service services.PlatformService
	}{
		{"spotify", !hasSpotify, c.resolutionService.GetPlatformService("spotify")},
		{"apple_music", !hasAppleMusic, c.resolutionService.GetPlatformService("apple_music")},
		{"tidal", !hasTidal, c.resolutionService.GetPlatformService("tidal")},
	}

	for _, platform := range platforms {
		if !platform.missing || platform.service == nil {
			continue
		}

		track, err := platform.service.GetTrackByISRC(enhanceCtx, result.ISRC)
		if err != nil {
			slog.Debug("ISRC lookup failed", 
				"platform", platform.name, 
				"isrc", result.ISRC, 
				"error", err)
			continue
		}

		if track != nil {
			slog.Debug("Enhanced result with ISRC lookup", 
				"platform", platform.name,
				"title", result.Title,
				"isrc", result.ISRC)
			
			// In a full implementation, this would update the result
			// For now, just log the successful enhancement
		}
	}
}

// enhanceBySearch attempts to find the song on missing platforms using title/artist search
func (c *Coordinator) enhanceBySearch(ctx context.Context, result GroupedSearchResult, hasSpotify, hasAppleMusic, hasTidal bool) {
	enhanceCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Create search query from title and primary artist
	searchQuery := result.Title
	if len(result.Artists) > 0 {
		searchQuery += " " + result.Artists[0]
	}

	platforms := []struct {
		name    string
		missing bool
		service services.PlatformService
	}{
		{"spotify", !hasSpotify, c.resolutionService.GetPlatformService("spotify")},
		{"apple_music", !hasAppleMusic, c.resolutionService.GetPlatformService("apple_music")},
		{"tidal", !hasTidal, c.resolutionService.GetPlatformService("tidal")},
	}

	for _, platform := range platforms {
		if !platform.missing || platform.service == nil {
			continue
		}

		tracks, err := platform.service.SearchTrack(enhanceCtx, services.SearchQuery{
			Query: searchQuery,
			Limit: 3, // Only get top 3 results for enhancement
		})

		if err != nil {
			slog.Debug("Enhancement search failed", 
				"platform", platform.name, 
				"query", searchQuery, 
				"error", err)
			continue
		}

		// Find best match
		bestMatch := c.findBestMatch(result, tracks)
		if bestMatch != nil {
			slog.Debug("Enhanced result with search", 
				"platform", platform.name,
				"title", result.Title,
				"match_title", bestMatch.Title)
			
			// In a full implementation, this would update the result
			// For now, just log the successful enhancement
		}
	}
}

// findBestMatch finds the best matching track from search results
func (c *Coordinator) findBestMatch(target GroupedSearchResult, candidates []*services.TrackInfo) *services.TrackInfo {
	if len(candidates) == 0 {
		return nil
	}

	bestMatch := candidates[0]
	bestScore := c.calculateMatchScore(target, candidates[0])

	for _, candidate := range candidates[1:] {
		score := c.calculateMatchScore(target, candidate)
		if score > bestScore {
			bestScore = score
			bestMatch = candidate
		}
	}

	// Only return match if score is above threshold
	if bestScore < 70 { // 70% similarity threshold
		return nil
	}

	return bestMatch
}

// calculateMatchScore calculates similarity score between target and candidate
func (c *Coordinator) calculateMatchScore(target GroupedSearchResult, candidate *services.TrackInfo) int {
	score := 0

	// Title similarity (40 points max)
	titleSim := c.stringSimilarity(target.Title, candidate.Title)
	score += int(float64(titleSim) * 0.4)

	// Artist similarity (35 points max)
	if len(target.Artists) > 0 && len(candidate.Artists) > 0 {
		artistSim := c.stringSimilarity(target.Artists[0], candidate.Artists[0])
		score += int(float64(artistSim) * 0.35)
	}

	// Album similarity (15 points max)
	if target.Album != "" && candidate.Album != "" {
		albumSim := c.stringSimilarity(target.Album, candidate.Album)
		score += int(float64(albumSim) * 0.15)
	}

	// Duration similarity (10 points max)
	if target.DurationMs > 0 && candidate.Duration > 0 {
		durationDiff := abs(target.DurationMs - candidate.Duration)
		maxDiff := 30000 // 30 seconds tolerance
		if durationDiff <= maxDiff {
			durationSim := 100 - (durationDiff*100)/maxDiff
			score += int(float64(durationSim) * 0.1)
		}
	}

	return score
}

// stringSimilarity calculates similarity between two strings (0-100)
func (c *Coordinator) stringSimilarity(s1, s2 string) int {
	s1 = c.normalizeString(s1)
	s2 = c.normalizeString(s2)

	if s1 == s2 {
		return 100
	}

	// Simple Jaccard similarity on words
	words1 := strings.Fields(s1)
	words2 := strings.Fields(s2)

	if len(words1) == 0 && len(words2) == 0 {
		return 100
	}
	if len(words1) == 0 || len(words2) == 0 {
		return 0
	}

	// Calculate intersection and union
	intersection := 0
	wordSet2 := make(map[string]bool)
	for _, word := range words2 {
		wordSet2[word] = true
	}

	for _, word := range words1 {
		if wordSet2[word] {
			intersection++
		}
	}

	union := len(words1) + len(words2) - intersection
	if union == 0 {
		return 100
	}

	return (intersection * 100) / union
}

// abs returns absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}