package search

import (
	"fmt"
	"strings"
)

// Formatter handles formatting search results for different output types
type Formatter struct {
	baseURL string
}

// NewFormatter creates a new result formatter
func NewFormatter(baseURL string) *Formatter {
	return &Formatter{
		baseURL: baseURL,
	}
}

// FormatHTML generates HTML for search results (compatible with existing UI)
func (f *Formatter) FormatHTML(results []SearchResult) string {
	if len(results) == 0 {
		return `<div class="no-results"><p>No songs found.</p></div>`
	}

	var html strings.Builder
	html.WriteString(`<div class="search-results-container">`)

	for _, result := range results {
		html.WriteString(f.formatSingleResult(result))
	}

	html.WriteString(`</div>`)
	return html.String()
}

// formatSingleResult formats a single search result as HTML
func (f *Formatter) formatSingleResult(result SearchResult) string {
	var html strings.Builder

	// Start result item
	html.WriteString(fmt.Sprintf(`<div class="result-item" id="%s">`, result.ID))

	// Album art or placeholder
	if result.ImageURL != "" {
		html.WriteString(fmt.Sprintf(`<img src="%s" alt="Album art" class="result-image">`, result.ImageURL))
	} else {
		html.WriteString(`<div class="result-image-placeholder">🎵</div>`)
	}

	// Content
	html.WriteString(`<div class="result-content">`)
	
	// Title with explicit indicator
	titleHTML := result.Title
	if result.Explicit {
		titleHTML += ` 🅴`
	}
	html.WriteString(fmt.Sprintf(`<div class="result-title">%s</div>`, titleHTML))
	
	// Artists
	html.WriteString(fmt.Sprintf(`<div class="result-artist">%s</div>`, strings.Join(result.Artists, ", ")))
	
	// Album (if present)
	if result.Album != "" {
		html.WriteString(fmt.Sprintf(`<div class="result-album">%s</div>`, result.Album))
	}

	// Platform badge
	html.WriteString(fmt.Sprintf(`<div class="result-platforms" id="%s-platforms">`, result.ID))
	html.WriteString(f.formatPlatformBadge(result))
	html.WriteString(`</div>`)

	html.WriteString(`</div>`) // End content

	// Actions
	html.WriteString(`<div class="result-actions">`)
	
	if result.Source == "local" || result.Platform == "local" {
		// Local result - direct link
		html.WriteString(fmt.Sprintf(`<a href="%s" class="action-btn action-primary">View Song</a>`, result.URL))
	} else {
		// Platform result - share button
		html.WriteString(fmt.Sprintf(`
			<button class="action-btn action-primary" 
			        onclick="createShareLink('%s', this)">
				Share
			</button>`, result.URL))
	}
	
	// Open in platform button
	platformName := f.formatPlatformName(result.Platform)
	html.WriteString(fmt.Sprintf(`<a href="%s" target="_blank" class="action-btn action-secondary">Open in %s</a>`, 
		result.URL, platformName))

	html.WriteString(`</div>`) // End actions
	html.WriteString(`</div>`)  // End result item

	return html.String()
}

// formatPlatformBadge creates a platform badge for the result
func (f *Formatter) formatPlatformBadge(result SearchResult) string {
	platformName := f.formatPlatformName(result.Platform)
	platformClass := fmt.Sprintf("platform-%s", strings.ReplaceAll(result.Platform, "_", "-"))
	
	iconURL := f.getPlatformIconURL(result.Platform)
	iconHTML := ""
	if iconURL != "" {
		iconHTML = fmt.Sprintf(`<img src="%s" alt="" class="platform-badge-icon" aria-hidden="true">`, iconURL)
	}

	return fmt.Sprintf(`<a href="%s" target="_blank" class="platform-badge %s" style="text-decoration: none; color: inherit;" aria-label="Listen on %s">%s%s</a>`,
		result.URL, platformClass, platformName, iconHTML, platformName)
}

// formatPlatformName returns a human-readable platform name
func (f *Formatter) formatPlatformName(platform string) string {
	switch platform {
	case "apple_music":
		return "Apple Music"
	case "spotify":
		return "Spotify"
	case "tidal":
		return "Tidal"
	case "youtube_music":
		return "YouTube Music"
	case "local":
		return "SongShare"
	default:
		return strings.Title(platform)
	}
}

// getPlatformIconURL returns the icon URL for a platform
func (f *Formatter) getPlatformIconURL(platform string) string {
	switch platform {
	case "spotify":
		return "https://upload.wikimedia.org/wikipedia/commons/8/84/Spotify_icon.svg"
	case "apple_music":
		return "https://upload.wikimedia.org/wikipedia/commons/5/5f/Apple_Music_icon.svg"
	case "tidal":
		return "https://upload.wikimedia.org/wikipedia/commons/thumb/5/55/Cib-tidal_%28CoreUI_Icons_v1.0.0%29.svg/640px-Cib-tidal_%28CoreUI_Icons_v1.0.0%29.svg.png"
	case "youtube_music":
		return "https://upload.wikimedia.org/wikipedia/commons/thumb/6/6a/Youtube_Music_icon.svg/240px-Youtube_Music_icon.svg.png"
	default:
		return ""
	}
}

// FormatJSON generates JSON response for API clients
func (f *Formatter) FormatJSON(response *SearchResponse) map[string]interface{} {
	return map[string]interface{}{
		"results":     response.Results,
		"query":       response.Query,
		"total":       response.Total,
		"from_cache":  response.FromCache,
		"duration":    response.Duration,
		"sources":     f.extractSources(response.Results),
	}
}

// extractSources returns the list of sources that provided results
func (f *Formatter) extractSources(results []SearchResult) []string {
	sourceMap := make(map[string]bool)
	for _, result := range results {
		sourceMap[result.Platform] = true
	}

	sources := make([]string, 0, len(sourceMap))
	for source := range sourceMap {
		sources = append(sources, source)
	}

	return sources
}

// FormatLegacyJSON formats results in the legacy API response format
func (f *Formatter) FormatLegacyJSON(response *SearchResponse) map[string]interface{} {
	// Group results by platform for legacy compatibility
	platformResults := make(map[string][]map[string]interface{})
	
	for _, result := range response.Results {
		platform := result.Platform
		if platform == "apple_music" {
			platform = "apple_music" // Keep underscore for legacy compatibility
		}

		if platformResults[platform] == nil {
			platformResults[platform] = []map[string]interface{}{}
		}

		legacyResult := map[string]interface{}{
			"title":        result.Title,
			"artists":      result.Artists,
			"album":        result.Album,
			"url":          result.URL,
			"platform":     result.Platform,
			"isrc":         result.ISRC,
			"duration_ms":  result.DurationMs,
			"release_date": result.ReleaseDate,
			"image_url":    result.ImageURL,
			"popularity":   result.Popularity,
			"explicit":     result.Explicit,
			"available":    result.Available,
		}

		platformResults[platform] = append(platformResults[platform], legacyResult)
	}

	return map[string]interface{}{
		"results": platformResults,
		"query": map[string]interface{}{
			"query":    response.Query.Query,
			"title":    response.Query.Title,
			"artist":   response.Query.Artist,
			"album":    response.Query.Album,
			"platform": response.Query.Platform,
			"limit":    response.Query.Limit,
		},
	}
}