package render

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"songshare/internal/models"
	"songshare/internal/templates"
)

// SongMetadata represents core song information for responses
type SongMetadata struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Artists     []string `json:"artists"`
	Album       string   `json:"album"`
	DurationMs  int      `json:"duration_ms"`
	ReleaseDate string   `json:"release_date"`
	ISRC        string   `json:"isrc,omitempty"`
	ImageURL    string   `json:"image_url,omitempty"`
}

// PlatformLink represents a link to a song on a specific platform
type PlatformLink struct {
	URL       string `json:"url"`
	Available bool   `json:"available"`
	Platform  string `json:"platform"`
}

// ResolveSongResponse represents the response with song metadata and platform links
type ResolveSongResponse struct {
	Song          SongMetadata            `json:"song"`
	Platforms     map[string]PlatformLink `json:"platforms"`
	UniversalLink string                  `json:"universal_link"`
}

// PlatformDisplayData contains platform information for templates
type PlatformDisplayData struct {
	Platform    string
	URL         string
	Name        string
	IconURL     string
	ButtonText  string
	Description string
	Color       string
	CSSClass    string
}

// SearchResult represents a single search result for rendering
type SearchResult struct {
	Title       string   `json:"title"`
	Artists     []string `json:"artists"`
	Album       string   `json:"album"`
	URL         string   `json:"url"`
	Platform    string   `json:"platform"`
	ISRC        string   `json:"isrc,omitempty"`
	DurationMs  int      `json:"duration_ms,omitempty"`
	ReleaseDate string   `json:"release_date,omitempty"`
	ImageURL    string   `json:"image_url,omitempty"`
	Popularity  int      `json:"popularity,omitempty"`
	Explicit    bool     `json:"explicit,omitempty"`
	Available   bool     `json:"available"`
}


// SongRenderer handles rendering song responses in different formats
type SongRenderer struct {
	baseURL string
}

// NewSongRenderer creates a new song renderer
func NewSongRenderer(baseURL string) *SongRenderer {
	return &SongRenderer{
		baseURL: baseURL,
	}
}

// buildUniversalLink builds the universal link for a song
func (r *SongRenderer) buildUniversalLink(song *models.Song) string {
	if song.ISRC == "" {
		slog.Warn("Song missing ISRC", "songID", song.ID.Hex(), "title", song.Title)
		// This shouldn't happen with properly indexed songs
		return fmt.Sprintf("%s/s/unknown", r.baseURL)
	}
	return fmt.Sprintf("%s/s/%s", r.baseURL, song.ISRC)
}

// RenderSongJSON renders a song as JSON response
func (r *SongRenderer) RenderSongJSON(c *gin.Context, song *models.Song) {
	response := ResolveSongResponse{
		Song: SongMetadata{
			ID:          song.ID.Hex(),
			Title:       song.Title,
			Artists:     []string{song.Artist}, // TODO: Parse comma-separated artists
			Album:       song.Album,
			DurationMs:  song.Metadata.Duration,
			ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
			ISRC:        song.ISRC,
			ImageURL:    song.Metadata.ImageURL,
		},
		Platforms:     make(map[string]PlatformLink),
		UniversalLink: r.buildUniversalLink(song),
	}

	// Add platform links
	for _, link := range song.PlatformLinks {
		response.Platforms[link.Platform] = PlatformLink{
			URL:       link.URL,
			Available: link.Available,
			Platform:  link.Platform,
		}
	}

	c.JSON(http.StatusOK, response)
}

// PlatformUIConfig represents the configuration struct from handlers package
type PlatformUIConfig struct {
	Name        string
	IconURL     string
	ButtonText  string
	Description string
	Color       string
	BadgeClass  string
}

// RenderSongPage renders a song as HTML page  
func (r *SongRenderer) RenderSongPage(c *gin.Context, song *models.Song, getPlatformUIConfig func(string) *PlatformUIConfig) {
	// Create template data
	data := struct {
		Song         *models.Song
		PlatformURLs map[string]string
		Platforms    []PlatformDisplayData
		AlbumArt     string
	}{
		Song:         song,
		PlatformURLs: make(map[string]string),
		Platforms:    []PlatformDisplayData{},
		AlbumArt:     song.Metadata.ImageURL,
	}

	// Extract platform URLs and create platform display data
	for _, link := range song.PlatformLinks {
		if link.Available {
			data.PlatformURLs[link.Platform] = link.URL

			// Get UI configuration for this platform
			uiConfig := getPlatformUIConfig(link.Platform)
			data.Platforms = append(data.Platforms, PlatformDisplayData{
				Platform:    link.Platform,
				URL:         link.URL,
				Name:        uiConfig.Name,
				IconURL:     uiConfig.IconURL,
				ButtonText:  uiConfig.ButtonText,
				Description: uiConfig.Description,
				Color:       uiConfig.Color,
				CSSClass:    uiConfig.BadgeClass,
			})
		}
	}

	tmpl, err := templates.GetTemplate("song_page")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template error"})
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Render error"})
	}
}

// RenderSearchPage renders the search page
func (r *SongRenderer) RenderSearchPage(c *gin.Context, query string) {
	data := struct {
		Query string
	}{
		Query: query,
	}

	tmpl, err := templates.GetTemplate("search_page")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template error"})
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Render error"})
	}
}

