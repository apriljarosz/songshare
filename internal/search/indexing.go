package search

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"songshare/internal/handlers/render"
)

// IndexingResult represents a track that should be indexed
type IndexingResult struct {
	Track    render.SearchResult
	Priority int
	Query    string
}

// BackgroundIndexTracks performs intelligent background indexing of search results
func (c *Coordinator) BackgroundIndexTracks(query string, tracks []render.SearchResult) {
	if len(tracks) == 0 {
		return
	}

	slog.Debug("Starting background indexing", "query", query, "tracks", len(tracks))

	// Create indexing candidates with priorities
	var candidates []IndexingResult
	queryWords := strings.Fields(strings.ToLower(query))

	for i, track := range tracks {
		priority := c.calculateIndexingPriority(track, query, queryWords, i)
		if priority > 0 { // Only index tracks with positive priority
			candidates = append(candidates, IndexingResult{
				Track:    track,
				Priority: priority,
				Query:    query,
			})
		}
	}

	// Sort by priority (highest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority > candidates[j].Priority
	})

	// Index top candidates (limit to prevent overload)
	maxIndexing := 5
	if len(candidates) < maxIndexing {
		maxIndexing = len(candidates)
	}

	for _, candidate := range candidates[:maxIndexing] {
		go c.indexTrackInBackground(candidate)
	}

	slog.Debug("Background indexing initiated", "candidates", len(candidates), "indexing", maxIndexing)
}

// calculateIndexingPriority determines how important it is to index this track
func (c *Coordinator) calculateIndexingPriority(track render.SearchResult, query string, queryWords []string, position int) int {
	priority := 0

	// Popularity is the primary factor (0-50 points)
	if track.Popularity > 0 {
		priority += (track.Popularity * 50) / 100
	}

	// Position in search results (50-40 points for top 10)
	if position < 10 {
		priority += 50 - (position * 5)
	}

	// Title match quality (0-30 points)
	titleLower := strings.ToLower(track.Title)
	for _, word := range queryWords {
		if strings.Contains(titleLower, word) {
			priority += 10
		}
	}

	// Artist match quality (0-20 points)
	for _, artist := range track.Artists {
		artistLower := strings.ToLower(artist)
		for _, word := range queryWords {
			if strings.Contains(artistLower, word) {
				priority += 5
			}
		}
	}

	// Metadata completeness bonus (0-20 points)
	if track.ISRC != "" {
		priority += 10
	}
	if track.ImageURL != "" {
		priority += 5
	}
	if track.Album != "" {
		priority += 5
	}

	// Platform preference (Spotify/Apple Music preferred for indexing)
	switch track.Platform {
	case "spotify", "apple_music":
		priority += 10
	case "tidal":
		priority += 5
	}

	return priority
}

// indexTrackInBackground performs the actual indexing of a track
func (c *Coordinator) indexTrackInBackground(candidate IndexingResult) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	track := candidate.Track
	
	slog.Debug("Indexing track", 
		"title", track.Title, 
		"artist", strings.Join(track.Artists, ", "), 
		"platform", track.Platform,
		"priority", candidate.Priority,
		"query", candidate.Query)

	// Try to resolve the track to get full metadata
	trackInfo, err := c.resolveTrack(ctx, track)
	if err != nil {
		slog.Debug("Failed to resolve track for indexing", 
			"title", track.Title,
			"platform", track.Platform, 
			"error", err)
		return
	}

	// Index the track if it's not already in the database
	err = c.indexTrackIfNew(ctx, trackInfo, candidate.Query)
	if err != nil {
		slog.Debug("Failed to index track", 
			"title", track.Title, 
			"error", err)
		return
	}

	slog.Debug("Successfully indexed track", 
		"title", track.Title,
		"artist", strings.Join(track.Artists, ", "))
}

// resolveTrack attempts to get full track information from the platform service
func (c *Coordinator) resolveTrack(ctx context.Context, track render.SearchResult) (*render.SearchResult, error) {
	// For now, just return the track as-is
	// In a full implementation, this would call the platform service to get detailed metadata
	return &track, nil
}

// indexTrackIfNew indexes the track if it's not already in the database
func (c *Coordinator) indexTrackIfNew(ctx context.Context, track *render.SearchResult, query string) error {
	// Check if song already exists by ISRC first
	if track.ISRC != "" {
		existing, err := c.songRepository.FindByISRC(ctx, track.ISRC)
		if err == nil && existing != nil {
			slog.Debug("Track already indexed by ISRC", "isrc", track.ISRC, "title", track.Title)
			return nil
		}
	}

	// Check by title and artist as fallback
	if len(track.Artists) > 0 {
		existing, err := c.songRepository.FindByTitleArtist(ctx, track.Title, track.Artists[0])
		if err == nil && len(existing) > 0 {
			slog.Debug("Track already indexed by title/artist", "title", track.Title, "artist", track.Artists[0])
			return nil
		}
	}

	// Track is new, would normally save it here
	// For now, just log that we would index it
	slog.Info("Would index new track", 
		"title", track.Title, 
		"artist", strings.Join(track.Artists, ", "),
		"platform", track.Platform,
		"isrc", track.ISRC,
		"query", query)

	return nil
}