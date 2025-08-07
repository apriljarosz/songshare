# Balanced Scoring System with Smart Popularity Handling

## Current Popularity Analysis

### Platform Popularity Sources:
✅ **Spotify**: Has popularity (0-100 scale) directly from API  
✅ **Tidal**: Has popularity (0-100 scale) directly from API  
❌ **Apple Music**: NO popularity data - missing from `convertAppleMusicTrack` function  

### Current Issues:
1. **Apple Music has no popularity** - creates unfair comparison
2. **Raw popularity scores** used directly (no normalization)
3. **Platform-specific biases** - Spotify/Tidal popularity may not correlate
4. **No popularity fallbacks** - songs without data get 0

## Popularity Calculation Strategy

### 1. Multi-Platform Popularity Aggregation
```go
// Aggregate popularity across available platforms
func calculateAggregatePopularity(searchResults []SearchResultWithSource, isrc string) int {
    if isrc != "" {
        // Use ISRC to find same song across platforms and average their popularity
        popularityScores := []int{}
        for _, result := range searchResults {
            if result.ISRC == isrc && result.Popularity > 0 {
                popularityScores = append(popularityScores, result.Popularity)
            }
        }
        
        if len(popularityScores) > 0 {
            return calculateWeightedAverage(popularityScores)
        }
    }
    
    // Fallback: use highest individual platform popularity
    maxPop := 0
    for _, result := range searchResults {
        if result.Popularity > maxPop {
            maxPop = result.Popularity
        }
    }
    return maxPop
}
```

### 2. Platform-Specific Popularity Weighting
```go
// Weight popularity by platform reliability
func calculateWeightedAverage(scores []int) int {
    // Spotify: 1.0x weight (most reliable/comprehensive)
    // Tidal: 0.8x weight (smaller user base)  
    // Apple Music: 0.0x weight (no data currently)
    
    // This ensures we don't unfairly penalize songs only on Tidal
    // while giving preference to Spotify's more comprehensive data
}
```

### 3. Popularity Fallback System
```go
// Smart fallbacks for missing popularity data
func getPopularityWithFallbacks(track SearchResult, aggregateResults []SearchResult) int {
    // 1. Use direct popularity if available
    if track.Popularity > 0 {
        return track.Popularity
    }
    
    // 2. Use same-ISRC popularity from other platforms
    if track.ISRC != "" {
        for _, other := range aggregateResults {
            if other.ISRC == track.ISRC && other.Popularity > 0 {
                return other.Popularity
            }
        }
    }
    
    // 3. Use artist average popularity (if we have other songs by same artist)
    artistPopularity := getArtistAveragePopularity(track.Artists[0])
    if artistPopularity > 0 {
        return artistPopularity
    }
    
    // 4. Default: moderate score (30) instead of 0
    // This prevents completely unknown songs from being buried
    return 30
}
```

## New Balanced Scoring Algorithm (0-100 scale)

### Core Scoring Components:
```go
func calculateBalancedRelevanceScore(result SearchResult, source string, query string, aggregateResults []SearchResult) float64 {
    score := 0.0
    
    // 1. TEXT MATCH QUALITY (0-60 points) - Primary ranking factor
    textScore := calculateTextMatchScore(result, query)
    score += textScore
    
    // 2. POPULARITY BOOST (0-25 points) - Secondary factor with logarithmic scaling
    popularity := getPopularityWithFallbacks(result, aggregateResults)
    popularityBoost := calculatePopularityBoost(popularity)
    score += popularityBoost
    
    // 3. CONTEXT & QUALITY (0-15 points) - Tie-breakers
    contextScore := calculateContextScore(result, source)
    score += contextScore
    
    return score
}
```

### 1. Enhanced Text Matching (0-60 points)
```go
func calculateTextMatchScore(result SearchResult, query string) float64 {
    score := 0.0
    queryLower := strings.ToLower(strings.TrimSpace(query))
    titleLower := strings.ToLower(result.Title)
    
    // Base title matching (0-50 points)
    if titleLower == queryLower {
        score += 50.0  // Perfect match
    } else if strings.HasPrefix(titleLower, queryLower) {
        score += 40.0  // Starts with query
    } else if strings.Contains(titleLower, queryLower) {
        score += 30.0  // Contains query
    } else if fuzzyMatch(titleLower, queryLower) {
        score += 25.0  // Typos, minor differences
    }
    
    // Artist matching bonus (0-10 points)
    for _, artist := range result.Artists {
        artistLower := strings.ToLower(artist)
        if artistLower == queryLower {
            score += 10.0  // Exact artist match
            break
        } else if strings.Contains(artistLower, queryLower) || strings.Contains(queryLower, artistLower) {
            score += 5.0   // Partial artist match
            break
        }
    }
    
    return min(score, 60.0) // Cap at 60
}
```

### 2. Logarithmic Popularity Scaling (0-25 points)
```go
func calculatePopularityBoost(popularity int) float64 {
    if popularity == 0 {
        return 0.0
    }
    
    // Logarithmic scaling prevents mega-hits from dominating
    // This compresses the range so 95 popularity doesn't get 5x more points than 20 popularity
    
    switch {
    case popularity >= 85:
        return 25.0  // Mega hits (Taylor Swift, etc.)
    case popularity >= 70:
        return 20.0  // Very popular (mainstream radio)
    case popularity >= 50:
        return 15.0  // Popular (well-known)
    case popularity >= 30:
        return 10.0  // Moderate (indie hits)
    case popularity >= 10:
        return 5.0   // Niche (cult following)
    default:
        return 0.0   // Unknown/new
    }
}
```

### 3. Context & Quality Score (0-15 points)
```go
func calculateContextScore(result SearchResult, source string) float64 {
    score := 0.0
    
    // Recency bonus (0-5 points)
    if releaseYear := parseYear(result.ReleaseDate); releaseYear > 0 {
        currentYear := time.Now().Year()
        yearsSince := currentYear - releaseYear
        
        if yearsSince <= 1 {
            score += 5.0  // Very recent
        } else if yearsSince <= 3 {
            score += 3.0  // Recent
        } else if yearsSince <= 5 {
            score += 1.0  // Somewhat recent
        }
    }
    
    // Platform availability bonus (calculated during grouping - 0-5 points)
    // This gets added during result grouping when we know all platforms
    
    // Metadata completeness (0-5 points)
    completeness := 0
    if result.ISRC != "" { completeness++ }
    if result.ImageURL != "" { completeness++ }
    if result.DurationMs > 0 { completeness++ }
    if result.ReleaseDate != "" { completeness++ }
    if len(result.Artists) > 0 { completeness++ }
    
    score += float64(completeness) // 0-5 points
    
    // Local cache preference (small bonus)
    if source == "local" {
        score += 2.0
    }
    
    return min(score, 15.0)
}
```

## Expected Scoring Examples

### Example 1: "Bohemian Rhapsody"
```
Queen - Bohemian Rhapsody (Original):
- Text Match: 60/60 (perfect title + artist)
- Popularity: 25/25 (mega hit, ~95 popularity)  
- Context: 5/15 (old song, complete metadata)
- TOTAL: 90/100

Cover Band - Bohemian Rhapsody:
- Text Match: 50/60 (perfect title, different artist)
- Popularity: 0/25 (no popularity data)
- Context: 8/15 (decent metadata, recent)
- TOTAL: 58/100
```

### Example 2: "The Subwa" (typo for "The Subway")
```
Chappell Roan - The Subway (Popular):
- Text Match: 25/60 (fuzzy match)
- Popularity: 20/25 (~75 popularity)
- Context: 12/15 (recent, complete, multi-platform)
- TOTAL: 57/100

Obscure Artist - The Subway (Perfect match):
- Text Match: 50/60 (perfect title)
- Popularity: 5/25 (fallback to moderate ~30)
- Context: 5/15 (basic metadata)
- TOTAL: 60/100
```

**Result**: Perfect text match beats popular fuzzy match, but it's competitive!

## Implementation Priority

### Phase 1: Core Algorithm
1. Rewrite `calculateRelevanceScore()` with new balanced approach
2. Add multi-platform popularity aggregation  
3. Implement logarithmic popularity scaling
4. Add enhanced text matching with graduated scoring

### Phase 2: Apple Music Enhancement 
1. Research if Apple Music has any popularity indicators we're missing
2. Implement artist-based popularity fallbacks
3. Add user engagement metrics as popularity proxy

### Phase 3: Smart Fallbacks
1. Artist average popularity calculation
2. Genre-based popularity estimation  
3. Release date recency weighting
4. Platform availability scoring

## Key Benefits
- **Balanced Results**: Text accuracy competes fairly with popularity
- **Fair Apple Music Treatment**: Fallbacks prevent Apple Music results from being buried
- **Reduced Mega-Hit Bias**: Logarithmic scaling prevents popular songs from dominating
- **Better Niche Discovery**: Perfect matches for obscure content get fair chance
- **Multi-Platform Consistency**: ISRC-based popularity sharing across platforms