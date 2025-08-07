package render

import (
	"fmt"
	"strings"
)

// SearchResultWithSource pairs a search result with its source information
type SearchResultWithSource struct {
	SearchResult SearchResult
	Source       string // "local" or "platform"
}

// RenderSearchResultsHTML generates HTML for search results
func RenderSearchResultsHTML(results []SearchResultWithSource) string {
	if len(results) == 0 {
		return `<div class="no-results"><p>No songs found.</p></div>`
	}

	var html strings.Builder
	html.WriteString(`<div class="search-results-container">`)

	for _, item := range results {
		result := item.SearchResult

		// Start result item
		html.WriteString(`<div class="result-item">`)

		// Album art or placeholder
		if result.ImageURL != "" {
			html.WriteString(fmt.Sprintf(`<img src="%s" alt="Album art" class="result-image">`, result.ImageURL))
		} else {
			html.WriteString(`<div class="result-image-placeholder">🎵</div>`)
		}

		// Content
		html.WriteString(`<div class="result-content">`)
		titleHTML := result.Title
		if result.Explicit {
			titleHTML += ` 🅴`
		}
		html.WriteString(fmt.Sprintf(`<div class="result-title">%s</div>`, titleHTML))
		html.WriteString(fmt.Sprintf(`<div class="result-artist">%s</div>`, strings.Join(result.Artists, ", ")))
		if result.Album != "" {
			html.WriteString(fmt.Sprintf(`<div class="result-album">%s</div>`, result.Album))
		}

		// Platform badge
		html.WriteString(`<div class="result-platforms">`)
		platformClass := fmt.Sprintf("platform-%s", result.Platform)
		platformName := result.Platform
		if result.Platform == "apple_music" {
			platformName = "Apple Music"
		} else if result.Platform == "spotify" {
			platformName = "Spotify"
		} else if result.Platform == "local" {
			platformName = "SongShare"
		} else if result.Platform == "tidal" {
			platformName = "Tidal"
		}
		html.WriteString(fmt.Sprintf(`<span class="platform-badge %s">%s</span>`, platformClass, platformName))
		html.WriteString(`</div>`)

		html.WriteString(`</div>`) // End content

		// Actions
		html.WriteString(`<div class="result-actions">`)

		if result.Platform == "local" {
			html.WriteString(fmt.Sprintf(`<a href="%s" class="action-btn action-primary">View Song</a>`, result.URL))
		} else {
			html.WriteString(fmt.Sprintf(`
				<button onclick="createShareLink('%s', this)" class="action-btn action-primary">
					Share
				</button>
			`, result.URL))
		}
		html.WriteString(fmt.Sprintf(`<a href="%s" target="_blank" class="action-btn action-secondary">Open in %s</a>`, result.URL, platformName))
		html.WriteString(`</div>`) // End actions

		html.WriteString(`</div>`) // End result item
	}

	html.WriteString(`</div>`) // End container

	return html.String()
}

// GroupedSearchResult represents search results grouped by song
type GroupedSearchResult struct {
	ID           string
	Title        string
	Artists      []string
	Album        string
	ImageURL     string
	Platforms    []PlatformBadge
	LocalURL     string
	HasLocalCopy bool
}

// PlatformBadge represents a platform badge for display
type PlatformBadge struct {
	Platform  string
	Name      string
	URL       string
	IconURL   string
	Available bool
	CSSClass  string
}

// RenderGroupedSearchResultsHTML generates HTML for grouped search results
func RenderGroupedSearchResultsHTML(results []GroupedSearchResult) string {
	if len(results) == 0 {
		return `<div class="no-results"><p>No songs found.</p></div>`
	}

	var html strings.Builder
	html.WriteString(`<div class="search-results-container">`)

	for _, result := range results {
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

		// Title
		html.WriteString(fmt.Sprintf(`<div class="result-title">%s</div>`, result.Title))
		html.WriteString(fmt.Sprintf(`<div class="result-artist">%s</div>`, strings.Join(result.Artists, ", ")))
		if result.Album != "" {
			html.WriteString(fmt.Sprintf(`<div class="result-album">%s</div>`, result.Album))
		}

		// Platform badges
		html.WriteString(fmt.Sprintf(`<div class="result-platforms" id="%s-platforms">`, result.ID))

		// Render platform badges
		for _, platform := range result.Platforms {
			badgeHTML := renderPlatformBadge(platform)
			html.WriteString(badgeHTML)
		}

		html.WriteString(`</div>`)

		html.WriteString(`</div>`) // End content

		// Actions
		html.WriteString(`<div class="result-actions">`)

		if result.HasLocalCopy {
			html.WriteString(fmt.Sprintf(`<a href="%s" class="action-btn action-primary">Share</a>`, result.LocalURL))
		}

		// Platform actions
		html.WriteString(`<div class="platform-actions">`)
		for _, platform := range result.Platforms {
			if platform.Available {
				html.WriteString(fmt.Sprintf(`
					<a href="%s" target="_blank" class="action-btn action-small action-secondary">
						Open in %s
					</a>
				`, platform.URL, platform.Name))
			}
		}
		html.WriteString(`</div>`)

		html.WriteString(`</div>`) // End actions

		html.WriteString(`</div>`) // End result item
	}

	html.WriteString(`</div>`) // End container

	return html.String()
}

// renderPlatformBadge renders a single platform badge
func renderPlatformBadge(badge PlatformBadge) string {
	if !badge.Available {
		return ""
	}

	iconHTML := ""
	if badge.IconURL != "" {
		iconHTML = fmt.Sprintf(`<img src="%s" alt="" class="platform-badge-icon">`, badge.IconURL)
	}

	badgeHTML := fmt.Sprintf(`
		<span class="platform-badge %s" onclick="window.open('%s', '_blank')">
			%s%s
		</span>
	`, badge.CSSClass, badge.URL, iconHTML, badge.Name)

	return badgeHTML
}

// NormalizePlatformName converts platform identifiers to display names
func NormalizePlatformName(platform string) string {
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
		return platform
	}
}