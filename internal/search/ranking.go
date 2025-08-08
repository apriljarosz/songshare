package search

import (
	"sort"
	"strings"

	"songshare/internal/config"
)

// Ranker handles relevance scoring and ranking of search results
type Ranker struct {
	platformWeights map[string]float64
}

// NewRanker creates a new result ranker with default platform weights
func NewRanker() *Ranker {
	cfg := config.GetRankingConfig()
	weights := make(map[string]float64, len(cfg.PlatformWeights))
	for k, v := range cfg.PlatformWeights {
		weights[k] = v
	}
	return &Ranker{platformWeights: weights}
}

// RankResults sorts search results by relevance score
func (r *Ranker) RankResults(results []SearchResult, query string) []SearchResult {
	// Calculate relevance scores
	for i := range results {
		results[i].RelevanceScore = r.calculateRelevance(results[i], query)
	}

	// Sort by relevance score (highest first)
	sort.SliceStable(results, func(i, j int) bool {
		// Primary sort: relevance score
		if results[i].RelevanceScore != results[j].RelevanceScore {
			return results[i].RelevanceScore > results[j].RelevanceScore
		}

		// Secondary sort: popularity
		if results[i].Popularity != results[j].Popularity {
			return results[i].Popularity > results[j].Popularity
		}

		// Tertiary sort: platform preference
		weightI := r.getPlatformWeight(results[i].Platform)
		weightJ := r.getPlatformWeight(results[j].Platform)
		if weightI != weightJ {
			return weightI > weightJ
		}

		// Deterministic tie-breakers for stable ordering across runs
		ti := strings.ToLower(results[i].Title)
		tj := strings.ToLower(results[j].Title)
		if ti != tj {
			return ti < tj
		}
		ai := ""
		if len(results[i].Artists) > 0 {
			ai = strings.ToLower(results[i].Artists[0])
		}
		aj := ""
		if len(results[j].Artists) > 0 {
			aj = strings.ToLower(results[j].Artists[0])
		}
		if ai != aj {
			return ai < aj
		}
		albi := strings.ToLower(results[i].Album)
		albj := strings.ToLower(results[j].Album)
		if albi != albj {
			return albi < albj
		}
		// Final fallback: ID then URL
		if results[i].ID != results[j].ID {
			return results[i].ID < results[j].ID
		}
		return results[i].URL < results[j].URL
	})

	return results
}

// calculateRelevance computes a relevance score for a search result
func (r *Ranker) calculateRelevance(result SearchResult, query string) float64 {
	score := 0.0
	queryLower := strings.ToLower(strings.TrimSpace(query))

	if queryLower == "" {
		return 0.0
	}

	// Title matching (most important factor)
	titleScore := r.calculateTextMatch(result.Title, queryLower)
	score += titleScore * 100.0 // Title matches get up to 100 points

	// Artist matching
	for _, artist := range result.Artists {
		artistScore := r.calculateTextMatch(artist, queryLower)
		score += artistScore * 50.0 // Artist matches get up to 50 points each
	}

	// Album matching (lower weight)
	if result.Album != "" {
		albumScore := r.calculateTextMatch(result.Album, queryLower)
		score += albumScore * 30.0 // Album matches get up to 30 points
	}

	// Popularity boost scaled by config (default up to 0-80)
	if result.Popularity > 0 {
		popularityScale := config.GetRankingConfig().RankerPopularityScale
		if popularityScale <= 0 {
			popularityScale = 0.8
		}
		popularityScore := float64(result.Popularity) * popularityScale
		score += popularityScore
	}

	// Platform weight boost
	platformWeight := r.getPlatformWeight(result.Platform)
	score *= platformWeight

	// Availability bonus
	if result.Available {
		score += 5.0
	}

	// Local result bonus (songs we've already indexed are valuable)
	if result.Source == "local" {
		score += 10.0
	}

	return score
}

// calculateTextMatch returns a match score between 0.0 and 1.0
func (r *Ranker) calculateTextMatch(text, query string) float64 {
	if text == "" || query == "" {
		return 0.0
	}

	textLower := strings.ToLower(text)

	// Exact match (highest score)
	if textLower == query {
		return 1.0
	}

	// Starts with query (high score)
	if strings.HasPrefix(textLower, query) {
		return 0.9
	}

	// Contains query (medium score)
	if strings.Contains(textLower, query) {
		return 0.7
	}

	// Word-based matching for multi-word queries
	queryWords := strings.Fields(query)
	if len(queryWords) > 1 {
		return r.calculateWordMatch(textLower, queryWords)
	}

	// Fuzzy matching for single words
	return r.calculateFuzzyMatch(textLower, query)
}

// calculateWordMatch scores based on how many query words are found
func (r *Ranker) calculateWordMatch(text string, queryWords []string) float64 {
	matchCount := 0
	textWords := strings.Fields(text)

	for _, queryWord := range queryWords {
		for _, textWord := range textWords {
			if strings.Contains(textWord, queryWord) {
				matchCount++
				break
			}
		}
	}

	if len(queryWords) == 0 {
		return 0.0
	}

	matchRatio := float64(matchCount) / float64(len(queryWords))

	// Scale based on completeness of match
	if matchRatio == 1.0 {
		return 0.8 // All words matched
	} else if matchRatio >= 0.5 {
		return 0.6 // Most words matched
	} else if matchRatio > 0.0 {
		return 0.4 // Some words matched
	}

	return 0.0
}

// calculateFuzzyMatch provides basic fuzzy matching for single words
func (r *Ranker) calculateFuzzyMatch(text, query string) float64 {
	// Simple fuzzy matching based on common prefixes and substrings

	// Check if query is a substring with some flexibility
	if len(query) >= 3 {
		// Look for query as substring allowing for some character differences
		for i := 0; i <= len(text)-len(query); i++ {
			substr := text[i : i+len(query)]
			if r.similarStrings(substr, query, 1) { // Allow 1 character difference
				return 0.3
			}
		}
	}

	// Common prefix scoring
	commonLen := 0
	maxLen := min(len(text), len(query))
	for i := 0; i < maxLen && text[i] == query[i]; i++ {
		commonLen++
	}

	if commonLen >= 3 && commonLen > len(query)/2 {
		return float64(commonLen) / float64(max(len(text), len(query))) * 0.5
	}

	return 0.0
}

// similarStrings checks if two strings are similar within maxDiff character differences
func (r *Ranker) similarStrings(s1, s2 string, maxDiff int) bool {
	if len(s1) != len(s2) {
		return false
	}

	diff := 0
	for i := 0; i < len(s1); i++ {
		if s1[i] != s2[i] {
			diff++
			if diff > maxDiff {
				return false
			}
		}
	}

	return true
}

// getPlatformWeight returns the preference weight for a platform
func (r *Ranker) getPlatformWeight(platform string) float64 {
	if weight, exists := r.platformWeights[platform]; exists {
		return weight
	}
	return 1.0 // Default weight
}

// SetPlatformWeight allows adjusting platform preference weights
func (r *Ranker) SetPlatformWeight(platform string, weight float64) {
	r.platformWeights[platform] = weight
}

// GetPlatformWeights returns current platform weights
func (r *Ranker) GetPlatformWeights() map[string]float64 {
	weights := make(map[string]float64)
	for k, v := range r.platformWeights {
		weights[k] = v
	}
	return weights
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
