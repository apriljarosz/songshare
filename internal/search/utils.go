package search

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"songshare/internal/handlers/render"
	"songshare/internal/scoring"
)

// generateSongKey creates a unique key for grouping songs
func (c *Coordinator) generateSongKey(result render.SearchResult) string {
	// Primary: Group by ISRC if available - this preserves different versions (clean vs explicit)
	if result.ISRC != "" {
		return "isrc:" + result.ISRC
	}

	// Heuristic grouping when ISRC is missing:
	// Use normalized title + primary artist + duration bucket to combine platforms
	// even if album strings differ (single vs album, remaster, etc.). Duration helps
	// avoid grouping distinct versions (live/remix) that typically differ in length.
	title := c.normalizeString(result.Title)
	artist := ""
	if len(result.Artists) > 0 {
		artist = c.normalizeString(result.Artists[0])
	}

	// Bucket duration to nearest 2 seconds (2000ms). If duration unknown, use "0".
	durationBucket := 0
	if result.DurationMs > 0 {
		durationBucket = (result.DurationMs + 1000) / 2000 // round to nearest bucket
	}

	return fmt.Sprintf("song:%s:%s:dur%d", title, artist, durationBucket)
}

// normalizeString normalizes strings for comparison (lowercase, no special chars)
func (c *Coordinator) normalizeString(s string) string {
	normalized := strings.ToLower(s)
	// Remove common punctuation and extra spaces
	normalized = strings.ReplaceAll(normalized, "'", "")
	normalized = strings.ReplaceAll(normalized, "\"", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, ".", "")
	normalized = strings.ReplaceAll(normalized, ",", "")
	normalized = strings.ReplaceAll(normalized, "!", "")
	normalized = strings.ReplaceAll(normalized, "?", "")
	normalized = strings.ReplaceAll(normalized, "&", "and")
	// Collapse multiple spaces
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}

// convertToScoringSearchResult converts a render SearchResult to scoring package SearchResult
func (c *Coordinator) convertToScoringSearchResult(result render.SearchResult) scoring.SearchResult {
	return scoring.SearchResult{
		Platform:    result.Platform,
		ExternalID:  "", // Handler SearchResult doesn't have ExternalID field
		URL:         result.URL,
		Title:       result.Title,
		Artists:     result.Artists,
		Album:       result.Album,
		ISRC:        result.ISRC,
		DurationMs:  result.DurationMs,
		ReleaseDate: result.ReleaseDate,
		ImageURL:    result.ImageURL,
		Popularity:  result.Popularity,
		Explicit:    result.Explicit,
		Available:   result.Available,
	}
}

// generateSearchResultID creates a unique ID for a search result
func (c *Coordinator) generateSearchResultID(result render.SearchResult) string {
	// Use ISRC if available for deterministic IDs
	if result.ISRC != "" {
		return "result-" + result.ISRC
	}

	// Fallback to normalized title + artist for songs without ISRC
	key := c.generateSongKey(result)

	// Create a short hash from the key
	hash := make([]byte, 4)
	rand.Read(hash)
	hashStr := hex.EncodeToString(hash)

	return "result-" + key + "-" + hashStr
}
