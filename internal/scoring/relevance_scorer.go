package scoring

import (
	"strconv"
	"strings"
	"time"

	"songshare/internal/config"
)

// SearchResult represents a song search result
// Note: This should match the SearchResult type in handlers package
type SearchResult struct {
	Platform    string   `json:"platform"`
	ExternalID  string   `json:"external_id"`
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	Artists     []string `json:"artists"`
	Album       string   `json:"album,omitempty"`
	ISRC        string   `json:"isrc,omitempty"`
	DurationMs  int      `json:"duration_ms,omitempty"`
	ReleaseDate string   `json:"release_date,omitempty"`
	ImageURL    string   `json:"image_url,omitempty"`
	Popularity  int      `json:"popularity,omitempty"`
	Explicit    bool     `json:"explicit,omitempty"`
	Available   bool     `json:"available"`
}

// SearchResultWithSource wraps a search result with its source information
type SearchResultWithSource struct {
	SearchResult SearchResult
	Source       string // "local" or "platform"
}

// RelevanceScorer handles all relevance scoring logic for song search results
type RelevanceScorer struct{}

// NewRelevanceScorer creates a new relevance scorer instance
func NewRelevanceScorer() *RelevanceScorer {
	return &RelevanceScorer{}
}

// RelevanceBreakdown provides detailed component scores used for ranking
type RelevanceBreakdown struct {
	TextMatch       float64 `json:"text_match"`
	PopularityInput int     `json:"popularity_input"`
	PopularityBoost float64 `json:"popularity_boost"`
	Context         float64 `json:"context"`
	Final           float64 `json:"final"`
}

// CalculateRelevanceScore calculates a balanced relevance score (0-100 scale) with enhanced popularity handling
func (rs *RelevanceScorer) CalculateRelevanceScore(result SearchResult, source string, query string, index int, allResults []SearchResultWithSource) float64 {
	// NEW BALANCED SCORING SYSTEM (0-100 scale)
	// - Text Match Quality: 0-60 points (Primary factor)
	// - Popularity Boost: 0-35 points (Secondary factor, logarithmic)
	// - Context & Quality: 0-15 points (Tie-breakers)

	score := 0.0

	// 1. TEXT MATCH QUALITY (0-60 points) - Primary ranking factor
	textScore := rs.calculateTextMatchScore(result, query)
	score += textScore

	// 2. POPULARITY BOOST (0-25 points) - Secondary factor with smart fallbacks
	enhancedPopularity := rs.getPopularityWithFallbacks(result, allResults)
	popularityBoost := rs.calculatePopularityBoost(enhancedPopularity)
	// Apply configurable multiplier to popularity influence
	if cfg := config.GetRankingConfig(); cfg != nil && cfg.PopularityBoostMultiplier > 0 {
		popularityBoost *= cfg.PopularityBoostMultiplier
	}
	score += popularityBoost

	// 3. CONTEXT & QUALITY (0-15 points) - Tie-breakers
	contextScore := rs.calculateContextScore(result, source)
	score += contextScore

	return score
}

// CalculateRelevanceBreakdown returns the individual components contributing to the final score
func (rs *RelevanceScorer) CalculateRelevanceBreakdown(result SearchResult, source string, query string, index int, allResults []SearchResultWithSource) RelevanceBreakdown {
	breakdown := RelevanceBreakdown{}
	textScore := rs.calculateTextMatchScore(result, query)
	breakdown.TextMatch = textScore

	enhancedPopularity := rs.getPopularityWithFallbacks(result, allResults)
	breakdown.PopularityInput = enhancedPopularity
	popularityBoost := rs.calculatePopularityBoost(enhancedPopularity)
	if cfg := config.GetRankingConfig(); cfg != nil && cfg.PopularityBoostMultiplier > 0 {
		popularityBoost *= cfg.PopularityBoostMultiplier
	}
	breakdown.PopularityBoost = popularityBoost

	contextScore := rs.calculateContextScore(result, source)
	breakdown.Context = contextScore

	breakdown.Final = textScore + popularityBoost + contextScore
	return breakdown
}

// calculateTextMatchScore calculates enhanced text matching score (0-60 points)
func (rs *RelevanceScorer) calculateTextMatchScore(result SearchResult, query string) float64 {
	score := 0.0
	queryLower := strings.ToLower(strings.TrimSpace(query))
	titleLower := strings.ToLower(result.Title)

	// Base title matching (0-50 points)
	if titleLower == queryLower {
		score += 50.0 // Perfect match
	} else if strings.HasPrefix(titleLower, queryLower) {
		score += 40.0 // Starts with query
	} else if strings.Contains(titleLower, queryLower) {
		score += 30.0 // Contains query
	} else if rs.fuzzyMatch(titleLower, queryLower) {
		score += 25.0 // Typos, minor differences
	}

	// Artist matching bonus (0-10 points)
	for _, artist := range result.Artists {
		artistLower := strings.ToLower(artist)
		if artistLower == queryLower {
			score += 10.0 // Exact artist match
			break
		} else if strings.Contains(artistLower, queryLower) || strings.Contains(queryLower, artistLower) {
			score += 5.0 // Partial artist match
			break
		}
	}

	// Cap at 60 points
	if score > 60.0 {
		score = 60.0
	}

	return score
}

// calculatePopularityBoost calculates logarithmic popularity scaling (0-35 points)
func (rs *RelevanceScorer) calculatePopularityBoost(popularity int) float64 {
	if popularity <= 0 {
		return 0.0
	}

	// Logarithmic scaling prevents mega-hits from dominating
	// This compresses the range so 95 popularity doesn't get 5x more points than 20 popularity
	switch {
	case popularity >= 85:
		return 35.0 // Mega hits
	case popularity >= 70:
		return 28.0 // Very popular
	case popularity >= 50:
		return 20.0 // Popular
	case popularity >= 30:
		return 12.0 // Moderate
	case popularity >= 10:
		return 6.0 // Niche
	default:
		return 0.0 // Unknown/new
	}
}

// calculateContextScore calculates context & quality score (0-15 points)
func (rs *RelevanceScorer) calculateContextScore(result SearchResult, source string) float64 {
	score := 0.0

	// Recency bonus (0-5 points)
	if releaseYear := rs.parseYear(result.ReleaseDate); releaseYear > 0 {
		currentYear := time.Now().Year()
		yearsSince := currentYear - releaseYear

		if yearsSince <= 1 {
			score += 5.0 // Very recent
		} else if yearsSince <= 3 {
			score += 3.0 // Recent
		} else if yearsSince <= 5 {
			score += 1.0 // Somewhat recent
		}
	}

	// Metadata completeness (0-5 points)
	completeness := 0
	if result.ISRC != "" {
		completeness++
	}
	if result.ImageURL != "" {
		completeness++
	}
	if result.DurationMs > 0 {
		completeness++
	}
	if result.ReleaseDate != "" {
		completeness++
	}
	if len(result.Artists) > 0 {
		completeness++
	}
	score += float64(completeness) // 0-5 points

	// Local cache preference (small bonus)
	if source == "local" {
		score += 2.0
	}

	// Cap at 15 points
	if score > 15.0 {
		score = 15.0
	}

	return score
}

// fuzzyMatch performs basic fuzzy string matching for typos and minor differences
func (rs *RelevanceScorer) fuzzyMatch(s1, s2 string) bool {
	// Simple fuzzy matching - remove spaces and check containment
	clean1 := strings.ReplaceAll(s1, " ", "")
	clean2 := strings.ReplaceAll(s2, " ", "")

	// Check if one contains the other (handles typos, extra words, etc.)
	return strings.Contains(clean1, clean2) || strings.Contains(clean2, clean1)
}

// parseYear extracts year from various date formats
func (rs *RelevanceScorer) parseYear(dateStr string) int {
	if dateStr == "" {
		return 0
	}

	// Try common formats: "2023", "2023-01-01", "2023-01"
	parts := strings.Split(dateStr, "-")
	if len(parts) > 0 {
		if year, err := strconv.Atoi(parts[0]); err == nil && year > 1900 && year <= time.Now().Year()+1 {
			return year
		}
	}

	return 0
}

// CalculateAggregatePopularity computes weighted average popularity across platforms for same ISRC
func (rs *RelevanceScorer) CalculateAggregatePopularity(searchResults []SearchResultWithSource, isrc string) int {
	if isrc == "" {
		// No ISRC provided; cannot aggregate reliably
		return 0
	}

	// Collect popularity scores from all platforms with same ISRC
	platformScores := make(map[string]int)
	for _, result := range searchResults {
		if result.SearchResult.ISRC == isrc && result.SearchResult.Popularity > 0 {
			platformScores[result.SearchResult.Platform] = result.SearchResult.Popularity
		}
	}

	if len(platformScores) == 0 {
		// No platform provided a popularity for this ISRC; treat as unknown
		return 0
	}

	// Use weighted average with platform reliability weights
	return rs.calculateWeightedPopularityAverage(platformScores)
}

// calculateWeightedPopularityAverage applies platform-specific weights to popularity scores
func (rs *RelevanceScorer) calculateWeightedPopularityAverage(platformScores map[string]int) int {
	totalScore := 0.0
	totalWeight := 0.0

	// Platform reliability weights (configurable)
	cfg := config.GetRankingConfig()
	platformWeights := cfg.PopularityPlatformWeights
	if platformWeights == nil || len(platformWeights) == 0 {
		platformWeights = map[string]float64{
			"spotify":     1.0,
			"tidal":       0.8,
			"apple_music": 0.0,
		}
	}

	for platform, score := range platformScores {
		weight := platformWeights[platform]
		if weight > 0 {
			totalScore += float64(score) * weight
			totalWeight += weight
		}
	}

	if totalWeight == 0 {
		// Fallback to simple average if no weighted platforms
		sum := 0
		for _, score := range platformScores {
			sum += score
		}
		return sum / len(platformScores)
	}

	return int(totalScore / totalWeight)
}

// getHighestPopularity returns the highest popularity score from all results
func (rs *RelevanceScorer) getHighestPopularity(searchResults []SearchResultWithSource) int {
	maxPop := 0
	for _, result := range searchResults {
		if result.SearchResult.Popularity > maxPop {
			maxPop = result.SearchResult.Popularity
		}
	}
	return maxPop
}

// getPopularityWithFallbacks provides smart fallback strategies for missing popularity data
func (rs *RelevanceScorer) getPopularityWithFallbacks(track SearchResult, allResults []SearchResultWithSource) int {
	// 1. Use direct popularity if available
	if track.Popularity > 0 {
		return track.Popularity
	}

	// 2. Use same-ISRC popularity from other platforms (take max, not first)
	if track.ISRC != "" {
		maxPop := 0
		for _, other := range allResults {
			if other.SearchResult.ISRC == track.ISRC && other.SearchResult.Popularity > maxPop {
				maxPop = other.SearchResult.Popularity
			}
		}
		if maxPop > 0 {
			return maxPop
		}
	}

	// 3. Use artist average popularity (basic implementation) only if no ISRC popularity found
	if len(track.Artists) > 0 {
		artistPopularity := rs.getArtistAveragePopularity(track.Artists[0], allResults)
		if artistPopularity > 0 {
			return artistPopularity
		}
	}

	// 4. Default moderate score instead of 0 to prevent unknown songs from being buried
	return 0
}

// getArtistAveragePopularity calculates average popularity for artist's other songs
func (rs *RelevanceScorer) getArtistAveragePopularity(artistName string, allResults []SearchResultWithSource) int {
	var popularityScores []int
	artistLower := strings.ToLower(artistName)

	for _, result := range allResults {
		if result.SearchResult.Popularity > 0 {
			// Check if this artist matches
			for _, resultArtist := range result.SearchResult.Artists {
				if strings.ToLower(resultArtist) == artistLower {
					popularityScores = append(popularityScores, result.SearchResult.Popularity)
					break
				}
			}
		}
	}

	if len(popularityScores) == 0 {
		return 0
	}

	sum := 0
	for _, score := range popularityScores {
		sum += score
	}
	return sum / len(popularityScores)
}
