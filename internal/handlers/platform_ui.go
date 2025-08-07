package handlers

import (
	"fmt"
	"strings"
)

// PlatformUIConfig represents UI configuration for a platform
type PlatformUIConfig struct {
	Name        string // Display name (e.g., "Apple Music", "Spotify")
	IconURL     string // URL to the platform's icon
	IconType    string // "svg", "png", etc.
	Color       string // Brand color (hex code)
	ButtonText  string // Text for platform buttons (e.g., "Open in Spotify")
	BadgeClass  string // CSS class for badges
	Description string // Accessibility description
}

// platformUIRegistry holds UI configuration for all platforms
var platformUIRegistry = map[string]*PlatformUIConfig{
	"spotify": {
		Name:        "Spotify",
		IconURL:     "https://upload.wikimedia.org/wikipedia/commons/8/84/Spotify_icon.svg",
		IconType:    "svg",
		Color:       "#1DB954",
		ButtonText:  "Open in Spotify",
		BadgeClass:  "platform-spotify",
		Description: "Listen on Spotify",
	},
	"apple_music": {
		Name:        "Apple Music",
		IconURL:     "https://upload.wikimedia.org/wikipedia/commons/5/5f/Apple_Music_icon.svg",
		IconType:    "svg",
		Color:       "#FA233B",
		ButtonText:  "Open in Apple Music",
		BadgeClass:  "platform-apple-music",
		Description: "Listen on Apple Music",
	},
	// Future platforms can be easily added here
	"youtube_music": {
		Name:        "YouTube Music",
		IconURL:     "https://upload.wikimedia.org/wikipedia/commons/6/6a/Youtube_Music_icon.svg",
		IconType:    "svg",
		Color:       "#FF0000",
		ButtonText:  "Open in YouTube Music",
		BadgeClass:  "platform-youtube-music",
		Description: "Listen on YouTube Music",
	},
	"deezer": {
		Name:        "Deezer",
		IconURL:     "https://upload.wikimedia.org/wikipedia/commons/3/32/Deezer_logo.svg",
		IconType:    "svg",
		Color:       "#FEAA2D",
		ButtonText:  "Open in Deezer",
		BadgeClass:  "platform-deezer",
		Description: "Listen on Deezer",
	},
	"tidal": {
		Name:        "Tidal",
		IconURL:     "https://upload.wikimedia.org/wikipedia/commons/thumb/5/55/Cib-tidal_%28CoreUI_Icons_v1.0.0%29.svg/640px-Cib-tidal_%28CoreUI_Icons_v1.0.0%29.svg.png",
		IconType:    "png",
		Color:       "#000000",
		ButtonText:  "Open in Tidal",
		BadgeClass:  "platform-tidal",
		Description: "Listen on Tidal",
	},
	"soundcloud": {
		Name:        "SoundCloud",
		IconURL:     "https://upload.wikimedia.org/wikipedia/commons/3/3e/SoundCloud_logo.svg",
		IconType:    "svg",
		Color:       "#FF8800",
		ButtonText:  "Open in SoundCloud",
		BadgeClass:  "platform-soundcloud",
		Description: "Listen on SoundCloud",
	},
}

// GetPlatformUIConfig returns UI configuration for a platform
func GetPlatformUIConfig(platform string) *PlatformUIConfig {
	if config, exists := platformUIRegistry[platform]; exists {
		return config
	}

	// Return default config for unknown platforms
	return &PlatformUIConfig{
		Name:        formatPlatformName(platform),
		IconURL:     "", // No icon for unknown platforms
		IconType:    "",
		Color:       "#666666",
		ButtonText:  fmt.Sprintf("Open in %s", formatPlatformName(platform)),
		BadgeClass:  fmt.Sprintf("platform-%s", strings.ReplaceAll(platform, "_", "-")),
		Description: fmt.Sprintf("Listen on %s", formatPlatformName(platform)),
	}
}

// RegisterPlatformUI registers UI configuration for a new platform
func RegisterPlatformUI(platform string, config *PlatformUIConfig) {
	platformUIRegistry[platform] = config
}

// GetAllPlatformUIConfigs returns all registered platform UI configs
func GetAllPlatformUIConfigs() map[string]*PlatformUIConfig {
	return platformUIRegistry
}

// formatPlatformName converts platform_name to Platform Name
func formatPlatformName(platform string) string {
	// Handle special cases
	switch platform {
	case "apple_music":
		return "Apple Music"
	case "youtube_music":
		return "YouTube Music"
	default:
		// Convert snake_case to Title Case
		words := strings.Split(platform, "_")
		for i, word := range words {
			words[i] = strings.Title(strings.ToLower(word))
		}
		return strings.Join(words, " ")
	}
}

// RenderPlatformIcon generates HTML for a platform icon
func RenderPlatformIcon(platform string, cssClass string) string {
	config := GetPlatformUIConfig(platform)

	if config.IconURL == "" {
		// Return empty string or text fallback for platforms without icons
		return ""
	}

	return fmt.Sprintf(
		`<img src="%s" alt="" class="%s" aria-hidden="true">`,
		config.IconURL,
		cssClass,
	)
}

// RenderPlatformBadge generates HTML for a platform badge (used in search results)
func RenderPlatformBadge(platform, platformURL string) string {
	config := GetPlatformUIConfig(platform)

	var iconHTML string
	if config.IconURL != "" {
		iconHTML = fmt.Sprintf(
			`<img src="%s" alt="" class="platform-badge-icon" aria-hidden="true">`,
			config.IconURL,
		)
	}

	return fmt.Sprintf(
		`<a href="%s" target="_blank" class="platform-badge %s" style="text-decoration: none; color: inherit;" aria-label="%s">%s%s</a>`,
		platformURL,
		config.BadgeClass,
		config.Description,
		iconHTML,
		config.Name,
	)
}

// RenderPlatformButton generates HTML for a platform button (used in share pages)
func RenderPlatformButton(platform, platformURL string) string {
	config := GetPlatformUIConfig(platform)

	var iconHTML string
	if config.IconURL != "" {
		iconHTML = fmt.Sprintf(
			`<img src="%s" alt="" class="platform-icon" aria-hidden="true">`,
			config.IconURL,
		)
	}

	return fmt.Sprintf(
		`<a href="%s" target="_blank" class="platform-btn %s" rel="noopener noreferrer" aria-label="%s">%s%s</a>`,
		platformURL,
		config.BadgeClass,
		config.Description,
		iconHTML,
		config.ButtonText,
	)
}

// GetPlatformCSS generates CSS variables for platform colors (for dynamic styling)
func GetPlatformCSS() string {
	var css strings.Builder
	css.WriteString(":root {\n")

	for platform, config := range platformUIRegistry {
		variableName := fmt.Sprintf("--color-%s", strings.ReplaceAll(platform, "_", "-"))
		css.WriteString(fmt.Sprintf("  %s: %s;\n", variableName, config.Color))
	}

	css.WriteString("}\n")
	return css.String()
}

// ValidateIconURL checks if an icon URL is accessible (basic validation)
func ValidateIconURL(url string) bool {
	// Basic validation - check if URL looks reasonable
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// PlatformUIOptions represents options for customizing platform UI rendering
type PlatformUIOptions struct {
	ShowIcon    bool
	ShowText    bool
	IconClass   string
	LinkTarget  string // "_blank", "_self", etc.
	CustomClass string
	AriaLabel   string
	WrapperTag  string // "div", "span", etc.
}

// RenderPlatformWithOptions provides more flexible platform UI rendering
func RenderPlatformWithOptions(platform, platformURL string, options PlatformUIOptions) string {
	config := GetPlatformUIConfig(platform)

	var html strings.Builder

	// Start wrapper if specified
	if options.WrapperTag != "" {
		html.WriteString(fmt.Sprintf("<%s", options.WrapperTag))
		if options.CustomClass != "" {
			html.WriteString(fmt.Sprintf(` class="%s"`, options.CustomClass))
		}
		html.WriteString(">")
	}

	// Start link
	html.WriteString(fmt.Sprintf(`<a href="%s"`, platformURL))

	// Add target if specified
	if options.LinkTarget != "" {
		html.WriteString(fmt.Sprintf(` target="%s"`, options.LinkTarget))
		if options.LinkTarget == "_blank" {
			html.WriteString(` rel="noopener noreferrer"`)
		}
	}

	// Add classes
	classes := []string{config.BadgeClass}
	if options.CustomClass != "" {
		classes = append(classes, options.CustomClass)
	}
	html.WriteString(fmt.Sprintf(` class="%s"`, strings.Join(classes, " ")))

	// Add aria-label
	ariaLabel := options.AriaLabel
	if ariaLabel == "" {
		ariaLabel = config.Description
	}
	html.WriteString(fmt.Sprintf(` aria-label="%s"`, ariaLabel))

	html.WriteString(">")

	// Add icon if requested and available
	if options.ShowIcon && config.IconURL != "" {
		iconClass := "platform-icon"
		if options.IconClass != "" {
			iconClass = options.IconClass
		}
		html.WriteString(fmt.Sprintf(
			`<img src="%s" alt="" class="%s" aria-hidden="true">`,
			config.IconURL,
			iconClass,
		))
	}

	// Add text if requested
	if options.ShowText {
		html.WriteString(config.Name)
	}

	// Close link
	html.WriteString("</a>")

	// Close wrapper if specified
	if options.WrapperTag != "" {
		html.WriteString(fmt.Sprintf("</%s>", options.WrapperTag))
	}

	return html.String()
}
