package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPlatformUIConfig(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		expected *PlatformUIConfig
	}{
		{
			name:     "Spotify configuration",
			platform: "spotify",
			expected: &PlatformUIConfig{
				Name:        "Spotify",
				IconURL:     "https://upload.wikimedia.org/wikipedia/commons/8/84/Spotify_icon.svg",
				IconType:    "svg",
				Color:       "#1DB954",
				ButtonText:  "Open in Spotify",
				BadgeClass:  "platform-spotify",
				Description: "Listen on Spotify",
			},
		},
		{
			name:     "Apple Music configuration",
			platform: "apple_music",
			expected: &PlatformUIConfig{
				Name:        "Apple Music",
				IconURL:     "https://upload.wikimedia.org/wikipedia/commons/5/5f/Apple_Music_icon.svg",
				IconType:    "svg",
				Color:       "#FA233B",
				ButtonText:  "Open in Apple Music",
				BadgeClass:  "platform-apple-music",
				Description: "Listen on Apple Music",
			},
		},
		{
			name:     "Unknown platform",
			platform: "unknown_platform",
			expected: &PlatformUIConfig{
				Name:        "Unknown Platform",
				IconURL:     "",
				IconType:    "",
				Color:       "#666666",
				ButtonText:  "Open in Unknown Platform",
				BadgeClass:  "platform-unknown-platform",
				Description: "Listen on Unknown Platform",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := GetPlatformUIConfig(tt.platform)
			assert.Equal(t, tt.expected.Name, config.Name)
			assert.Equal(t, tt.expected.IconURL, config.IconURL)
			assert.Equal(t, tt.expected.Color, config.Color)
			assert.Equal(t, tt.expected.ButtonText, config.ButtonText)
			assert.Equal(t, tt.expected.BadgeClass, config.BadgeClass)
			assert.Equal(t, tt.expected.Description, config.Description)
		})
	}
}

func TestRegisterPlatformUI(t *testing.T) {
	// Register a new platform
	customConfig := &PlatformUIConfig{
		Name:        "Test Platform",
		IconURL:     "https://example.com/icon.svg",
		IconType:    "svg",
		Color:       "#FF0000",
		ButtonText:  "Open in Test Platform",
		BadgeClass:  "platform-test",
		Description: "Listen on Test Platform",
	}
	
	RegisterPlatformUI("test_platform", customConfig)
	
	// Verify it was registered
	retrieved := GetPlatformUIConfig("test_platform")
	assert.Equal(t, customConfig.Name, retrieved.Name)
	assert.Equal(t, customConfig.IconURL, retrieved.IconURL)
	assert.Equal(t, customConfig.Color, retrieved.Color)
	
	// Clean up
	delete(platformUIRegistry, "test_platform")
}

func TestFormatPlatformName(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		expected string
	}{
		{
			name:     "apple_music special case",
			platform: "apple_music",
			expected: "Apple Music",
		},
		{
			name:     "youtube_music special case",
			platform: "youtube_music",
			expected: "YouTube Music",
		},
		{
			name:     "single word",
			platform: "spotify",
			expected: "Spotify",
		},
		{
			name:     "snake_case to Title Case",
			platform: "sound_cloud",
			expected: "Sound Cloud",
		},
		{
			name:     "multiple underscores",
			platform: "my_custom_platform",
			expected: "My Custom Platform",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPlatformName(tt.platform)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenderPlatformIcon(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		cssClass string
		expected string
	}{
		{
			name:     "Spotify icon",
			platform: "spotify",
			cssClass: "test-icon",
			expected: `<img src="https://upload.wikimedia.org/wikipedia/commons/8/84/Spotify_icon.svg" alt="" class="test-icon" aria-hidden="true">`,
		},
		{
			name:     "Unknown platform (no icon)",
			platform: "unknown_platform",
			cssClass: "test-icon",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderPlatformIcon(tt.platform, tt.cssClass)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenderPlatformBadge(t *testing.T) {
	result := RenderPlatformBadge("spotify", "https://open.spotify.com/track/test")
	
	// Check that the result contains expected elements
	assert.Contains(t, result, `href="https://open.spotify.com/track/test"`)
	assert.Contains(t, result, `class="platform-badge platform-spotify"`)
	assert.Contains(t, result, `aria-label="Listen on Spotify"`)
	assert.Contains(t, result, `target="_blank"`)
	assert.Contains(t, result, "Spotify")
	assert.Contains(t, result, "https://upload.wikimedia.org/wikipedia/commons/8/84/Spotify_icon.svg")
}

func TestRenderPlatformButton(t *testing.T) {
	result := RenderPlatformButton("apple_music", "https://music.apple.com/song/test")
	
	// Check that the result contains expected elements
	assert.Contains(t, result, `href="https://music.apple.com/song/test"`)
	assert.Contains(t, result, `class="platform-btn platform-apple-music"`)
	assert.Contains(t, result, `aria-label="Listen on Apple Music"`)
	assert.Contains(t, result, `target="_blank"`)
	assert.Contains(t, result, `rel="noopener noreferrer"`)
	assert.Contains(t, result, "Open in Apple Music")
	assert.Contains(t, result, "https://upload.wikimedia.org/wikipedia/commons/5/5f/Apple_Music_icon.svg")
}

func TestGetPlatformCSS(t *testing.T) {
	css := GetPlatformCSS()
	
	// Should contain CSS variables for known platforms
	assert.Contains(t, css, ":root {")
	assert.Contains(t, css, "--color-spotify: #1DB954;")
	assert.Contains(t, css, "--color-apple-music: #FA233B;")
	assert.Contains(t, css, "}")
}

func TestValidateIconURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "Valid HTTPS URL",
			url:      "https://example.com/icon.svg",
			expected: true,
		},
		{
			name:     "Valid HTTP URL",
			url:      "http://example.com/icon.png",
			expected: true,
		},
		{
			name:     "Invalid URL - no protocol",
			url:      "example.com/icon.svg",
			expected: false,
		},
		{
			name:     "Invalid URL - empty",
			url:      "",
			expected: false,
		},
		{
			name:     "Invalid URL - file protocol",
			url:      "file:///path/to/icon.svg",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateIconURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenderPlatformWithOptions(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		url      string
		options  PlatformUIOptions
		contains []string
		notContains []string
	}{
		{
			name:     "Icon and text with wrapper",
			platform: "spotify",
			url:      "https://open.spotify.com/track/test",
			options: PlatformUIOptions{
				ShowIcon:    true,
				ShowText:    true,
				IconClass:   "custom-icon",
				LinkTarget:  "_blank",
				CustomClass: "custom-class",
				WrapperTag:  "div",
			},
			contains: []string{
				"<div",
				`class="custom-class"`,
				`href="https://open.spotify.com/track/test"`,
				`target="_blank"`,
				`rel="noopener noreferrer"`,
				`class="custom-icon"`,
				"Spotify",
				"</div>",
			},
		},
		{
			name:     "Icon only, no wrapper",
			platform: "apple_music",
			url:      "https://music.apple.com/song/test",
			options: PlatformUIOptions{
				ShowIcon: true,
				ShowText: false,
			},
			contains: []string{
				`<a href="https://music.apple.com/song/test"`,
				`class="platform-icon"`,
				"</a>",
			},
			notContains: []string{
				"<div",
			},
		},
		{
			name:     "Text only with custom aria-label",
			platform: "deezer",
			url:      "https://deezer.com/track/test",
			options: PlatformUIOptions{
				ShowIcon:  false,
				ShowText:  true,
				AriaLabel: "Custom label for Deezer",
			},
			contains: []string{
				`aria-label="Custom label for Deezer"`,
				"Deezer",
			},
			notContains: []string{
				"<img",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderPlatformWithOptions(tt.platform, tt.url, tt.options)
			
			for _, expected := range tt.contains {
				assert.Contains(t, result, expected, "Result should contain: %s", expected)
			}
			
			for _, notExpected := range tt.notContains {
				assert.NotContains(t, result, notExpected, "Result should not contain: %s", notExpected)
			}
		})
	}
}

func TestGetAllPlatformUIConfigs(t *testing.T) {
	configs := GetAllPlatformUIConfigs()
	
	// Should contain known platforms
	assert.Contains(t, configs, "spotify")
	assert.Contains(t, configs, "apple_music")
	assert.Contains(t, configs, "youtube_music")
	
	// Verify a specific config
	spotifyConfig := configs["spotify"]
	assert.Equal(t, "Spotify", spotifyConfig.Name)
	assert.Equal(t, "#1DB954", spotifyConfig.Color)
}

// Benchmark tests
func BenchmarkGetPlatformUIConfig(b *testing.B) {
	platforms := []string{"spotify", "apple_music", "youtube_music", "unknown_platform"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		platform := platforms[i%len(platforms)]
		_ = GetPlatformUIConfig(platform)
	}
}

func BenchmarkRenderPlatformBadge(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = RenderPlatformBadge("spotify", "https://open.spotify.com/track/test")
	}
}

func BenchmarkFormatPlatformName(b *testing.B) {
	platforms := []string{"spotify", "apple_music", "youtube_music", "sound_cloud"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		platform := platforms[i%len(platforms)]
		_ = formatPlatformName(platform)
	}
}