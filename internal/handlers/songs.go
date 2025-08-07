package handlers

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"songshare/internal/models"
	"songshare/internal/repositories"
	"songshare/internal/services"
)

// ResolveSongRequest represents the request to resolve a song from a platform URL
type ResolveSongRequest struct {
	URL string `json:"url" binding:"required"`
}

// ResolveSongResponse represents the response with song metadata and platform links
type ResolveSongResponse struct {
	Song      SongMetadata           `json:"song"`
	Platforms map[string]PlatformLink `json:"platforms"`
	UniversalLink string             `json:"universal_link"`
}

// SongMetadata represents core song information
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

// SearchSongsRequest represents the request to search for songs
type SearchSongsRequest struct {
	Title    string `json:"title,omitempty"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	Query    string `json:"query,omitempty"`    // Free-form search query
	Platform string `json:"platform,omitempty"` // Optional: "spotify", "apple_music", or empty for both
	Limit    int    `json:"limit,omitempty"`    // Max results per platform (default: 10)
}

// SearchSongsResponse represents the response for search results
type SearchSongsResponse struct {
	Results map[string][]SearchResult `json:"results"` // platform -> results
	Query   SearchSongsRequest        `json:"query"`   // Echo back the query for reference
}

// SearchResult represents a single search result
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
	Popularity  int      `json:"popularity,omitempty"` // 0-100, higher = more popular
	Available   bool     `json:"available"`
}

// SongHandler handles song-related requests
type SongHandler struct {
	resolutionService *services.SongResolutionService
	songRepository    repositories.SongRepository
	baseURL          string
}

// NewSongHandler creates a new song handler
func NewSongHandler(resolutionService *services.SongResolutionService, songRepository repositories.SongRepository, baseURL string) *SongHandler {
	return &SongHandler{
		resolutionService: resolutionService,
		songRepository:    songRepository,
		baseURL:          baseURL,
	}
}

// ResolveSong handles POST /api/v1/songs/resolve
func (h *SongHandler) ResolveSong(c *gin.Context) {
	var req ResolveSongRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Resolve the song from the URL
	song, err := h.resolutionService.ResolveFromURL(c.Request.Context(), req.URL)
	if err != nil {
		slog.Error("Failed to resolve song", "url", req.URL, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to resolve song from URL",
			"details": err.Error(),
		})
		return
	}

	if song == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Song not found",
		})
		return
	}

	// Convert to response format
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
		Platforms: make(map[string]PlatformLink),
		UniversalLink: h.buildUniversalLink(song), // ISRC-based universal links
	}

	// Add platform links
	for _, link := range song.PlatformLinks {
		response.Platforms[link.Platform] = PlatformLink{
			URL:       link.URL,
			Available: link.Available,
			Platform:  link.Platform,
		}
	}

	// Check if this is an HTMX request (for search page integration)
	if c.GetHeader("HX-Request") == "true" {
		// Return redirect URL in both header and body for JavaScript compatibility
		c.Header("HX-Redirect", response.UniversalLink)
		c.JSON(http.StatusOK, gin.H{
			"redirect": response.UniversalLink,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// SearchSongs handles POST /api/v1/songs/search
func (h *SongHandler) SearchSongs(c *gin.Context) {
	var req SearchSongsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate search query
	if req.Title == "" && req.Artist == "" && req.Album == "" && req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "At least one search parameter is required (title, artist, album, or query)",
		})
		return
	}

	// Set default limit
	if req.Limit <= 0 {
		req.Limit = 10
	}
	if req.Limit > 50 {
		req.Limit = 50 // Cap at 50 results per platform
	}

	// Build search query
	searchQuery := services.SearchQuery{
		Title:  req.Title,
		Artist: req.Artist,
		Album:  req.Album,
		Query:  req.Query,
		Limit:  req.Limit,
	}

	response := SearchSongsResponse{
		Results: make(map[string][]SearchResult),
		Query:   req,
	}

	// Search platforms based on filter
	platforms := []string{}
	if req.Platform == "" {
		platforms = []string{"spotify", "apple_music"} // Both platforms
	} else if req.Platform == "spotify" || req.Platform == "apple_music" {
		platforms = []string{req.Platform}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid platform. Use 'spotify', 'apple_music', or omit for both",
		})
		return
	}

	// Search each platform
	for _, platform := range platforms {
		var platformService services.PlatformService
		
		// Get the appropriate service
		switch platform {
		case "spotify":
			platformService = h.resolutionService.GetPlatformService("spotify")
		case "apple_music":
			platformService = h.resolutionService.GetPlatformService("apple_music")
		default:
			continue
		}

		if platformService == nil {
			slog.Warn("Platform service not available", "platform", platform)
			continue
		}

		// Perform search
		tracks, err := platformService.SearchTrack(c.Request.Context(), searchQuery)
		if err != nil {
			slog.Error("Search failed for platform", "platform", platform, "error", err)
			// Continue with other platforms instead of failing entirely
			response.Results[platform] = []SearchResult{}
			continue
		}

		// Convert tracks to search results
		results := make([]SearchResult, len(tracks))
		for i, track := range tracks {
			results[i] = SearchResult{
				Title:       track.Title,
				Artists:     track.Artists,
				Album:       track.Album,
				URL:         track.URL,
				Platform:    track.Platform,
				ISRC:        track.ISRC,
				DurationMs:  track.Duration,
				ReleaseDate: track.ReleaseDate,
				ImageURL:    track.ImageURL,
				Available:   track.Available,
			}
		}

		response.Results[platform] = results
	}

	c.JSON(http.StatusOK, response)
}


// renderSongJSON returns JSON response for the song
func (h *SongHandler) renderSongJSON(c *gin.Context, song *models.Song) {
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
		UniversalLink: h.buildUniversalLink(song),
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

// renderSongPage returns HTML page with HTMX support
func (h *SongHandler) renderSongPage(c *gin.Context, song *models.Song) {
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
			uiConfig := GetPlatformUIConfig(link.Platform)
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

	// HTML template with HTMX support
	htmlTemplate := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Song.Title}} - {{.Song.Artist}}</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    
    <!-- Apple Music SVG Icon -->
    <svg style="display: none;" xmlns="http://www.w3.org/2000/svg">
        <defs>
            <symbol id="apple-music-icon" viewBox="0 0 24 24">
                <!-- Apple Music circular icon with gradient -->
                <circle cx="12" cy="12" r="12" fill="url(#appleMusicGradient)"/>
                <path d="M18.013 12.063l-5.718-1.691c-.303-.09-.617.09-.705.399-.089.309.085.632.388.721l5.718 1.691c.193.057.402-.029.512-.211.11-.182.11-.406 0-.588-.11-.182-.319-.268-.195-.321zM9.845 18.857c-.89-.263-1.835.243-2.11 1.132-.275.89.243 1.835 1.132 2.11.89.275 1.835-.243 2.11-1.132.275-.89-.243-1.835-1.132-2.11zm4.458 3.338c-.302-.089-.617.09-.705.399-.089.309.085.632.388.721.302.089.617-.09.705-.399.089-.309-.085-.632-.388-.721z" fill="white"/>
                <defs>
                    <linearGradient id="appleMusicGradient" x1="0%" y1="0%" x2="100%" y2="100%">
                        <stop offset="0%" style="stop-color:#fa233b"/>
                        <stop offset="50%" style="stop-color:#fb5c74"/>
                        <stop offset="100%" style="stop-color:#ff9500"/>
                    </linearGradient>
                </defs>
            </symbol>
        </defs>
    </svg>
    
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 600px; margin: 2rem auto; padding: 1rem; }
        .song-header { text-align: center; margin-bottom: 2rem; }
        .album-art { width: 200px; height: 200px; border-radius: 12px; margin: 0 auto 1rem; box-shadow: 0 8px 32px rgba(0,0,0,0.2); display: block; }
        .song-title { font-size: 2rem; font-weight: bold; margin-bottom: 0.5rem; }
        .song-artist { font-size: 1.2rem; color: #666; margin-bottom: 0.5rem; }
        .song-album { font-size: 1rem; color: #888; }
        .platforms { display: flex; flex-direction: column; gap: 1rem; }
        .platform-button { display: block; padding: 1rem; border: 2px solid #ddd; border-radius: 8px; text-decoration: none; color: inherit; transition: all 0.2s; }
        .platform-button:hover { border-color: #007AFF; transform: translateY(-1px); }
        .platform-spotify { border-color: #1DB954; }
        .platform-apple-music { border-color: #FA243C; }
        .platform-youtube-music { border-color: #FF0000; }
        .platform-deezer { border-color: #FEAA2D; }
        .platform-tidal { border-color: #000000; }
        .platform-soundcloud { border-color: #FF8800; }
        .platform-name { font-weight: bold; font-size: 1.1rem; display: flex; align-items: center; gap: 0.5rem; }
        .platform-desc { font-size: 0.9rem; color: #666; margin-top: 0.25rem; }
        .platform-icon { width: 24px; height: 24px; flex-shrink: 0; object-fit: contain; }
    </style>
</head>
<body>
    <div class="song-header">
        {{if .AlbumArt}}<img src="{{.AlbumArt}}" alt="Album art for {{.Song.Title}}" class="album-art">{{end}}
        <div class="song-title">{{.Song.Title}}</div>
        <div class="song-artist">{{.Song.Artist}}</div>
        {{if .Song.Album}}<div class="song-album">{{.Song.Album}}</div>{{end}}
    </div>
    
    <div class="platforms">
        {{range .Platforms}}
        <a href="{{.URL}}" target="_blank" class="platform-button {{.Platform}}" 
           hx-get="/api/v1/analytics/click?platform={{.Platform}}&song={{$.Song.ID.Hex}}"
           hx-trigger="mouseup"
           hx-swap="none">
            <div class="platform-name">
                {{if .IconURL}}<img src="{{.IconURL}}" alt="" class="platform-icon" aria-hidden="true">{{end}}
                {{.ButtonText}}
            </div>
            <div class="platform-desc">
                {{.Description}}
            </div>
        </a>
        {{end}}
        
        {{if not .PlatformURLs}}
        <div style="text-align: center; color: #666;">
            <p>This song is not currently available on supported platforms.</p>
        </div>
        {{end}}
    </div>
    
    <div style="text-align: center; margin-top: 2rem; font-size: 0.8rem; color: #999;">
        <p>Powered by SongShare</p>
    </div>
</body>
</html>`

	tmpl, err := template.New("song").Parse(htmlTemplate)
	if err != nil {
		slog.Error("Failed to parse song template", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template error"})
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		slog.Error("Failed to execute song template", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Render error"})
	}
}

// RedirectToSong handles GET /api/v1/s/:id - universal link redirects with dual-mode support
func (h *SongHandler) RedirectToSong(c *gin.Context) {
	songID := c.Param("id")
	if songID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing song ID",
		})
		return
	}

	// Look up song by ISRC
	song, err := h.findSongByISRC(c.Request.Context(), songID)
	if err != nil {
		slog.Error("Song lookup failed", "identifier", songID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Song not found",
		})
		return
	}
	
	if song == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Song not found",
		})
		return
	}

	// Check if song needs album art backfill
	if h.needsAlbumArtBackfill(song) {
		updatedSong := h.backfillAlbumArt(c.Request.Context(), song)
		if updatedSong != nil {
			song = updatedSong
		}
	}

	// Check Accept header for content negotiation
	accept := c.GetHeader("Accept")
	slog.Info("Accept header", "accept", accept) // Debug log
	
	// Browsers typically send text/html as the first preference
	if strings.Contains(accept, "text/html") {
		// Return HTML page with HTMX support
		h.renderSongPage(c, song)
	} else {
		// Return JSON response
		h.renderSongJSON(c, song)
	}
}

// needsAlbumArtBackfill checks if a song needs album art to be backfilled
func (h *SongHandler) needsAlbumArtBackfill(song *models.Song) bool {
	// Song needs backfill if it has no album art but has platform links
	return song.Metadata.ImageURL == "" && len(song.PlatformLinks) > 0
}

// backfillAlbumArt attempts to fetch and update album art for an existing song
func (h *SongHandler) backfillAlbumArt(ctx context.Context, song *models.Song) *models.Song {
	// Try to get album art from any available platform
	for _, link := range song.PlatformLinks {
		if !link.Available {
			continue
		}

		// Get the platform service
		var platformService services.PlatformService
		switch link.Platform {
		case "spotify":
			platformService = h.resolutionService.GetPlatformService("spotify")
		case "apple_music":
			platformService = h.resolutionService.GetPlatformService("apple_music")
		default:
			continue
		}

		if platformService == nil {
			continue
		}

		// Fetch track info from the platform
		trackInfo, err := platformService.GetTrackByID(ctx, link.ExternalID)
		if err != nil {
			slog.Warn("Failed to fetch track info for backfill", 
				"platform", link.Platform, 
				"trackID", link.ExternalID, 
				"error", err)
			continue
		}

		// If we got an image URL, update the song
		if trackInfo != nil && trackInfo.ImageURL != "" {
			song.Metadata.ImageURL = trackInfo.ImageURL
			
			// Update the song in the database
			if err := h.songRepository.Update(ctx, song); err != nil {
				slog.Error("Failed to update song with album art", 
					"songID", song.ID.Hex(), 
					"error", err)
				return nil
			}

			slog.Info("Successfully backfilled album art", 
				"songID", song.ID.Hex(), 
				"platform", link.Platform,
				"imageURL", trackInfo.ImageURL)
			
			return song
		}
	}

	return nil
}

// SearchPage renders the search page HTML
func (h *SongHandler) SearchPage(c *gin.Context) {
	// Get any initial query from URL params
	query := c.Query("q")
	
	// Create template data
	data := struct {
		Query string
	}{
		Query: query,
	}

	// HTML template for search page
	htmlTemplate := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Search Songs - SongShare</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    
    <script>
        async function createShareLink(url, button) {
            const originalText = button.innerHTML;
            button.innerHTML = 'Creating link...';
            button.disabled = true;
            
            try {
                const controller = new AbortController();
                const timeoutId = setTimeout(() => controller.abort(), 15000); // 15 second timeout
                
                const response = await fetch('/api/v1/songs/resolve', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'HX-Request': 'true'
                    },
                    body: JSON.stringify({ url: url }),
                    signal: controller.signal
                });
                
                clearTimeout(timeoutId);
                
                console.log('Response status:', response.status);
                console.log('Response headers:', [...response.headers.entries()]);
                
                if (response.ok) {
                    // Try to get redirect URL from response body
                    const responseData = await response.json();
                    console.log('Response data:', responseData);
                    
                    const redirectUrl = responseData.redirect || 
                                      response.headers.get('HX-Redirect') || 
                                      response.headers.get('hx-redirect');
                    console.log('Redirect URL:', redirectUrl);
                    
                    if (redirectUrl) {
                        window.location.href = redirectUrl;
                        return;
                    } else {
                        console.log('No redirect URL found in headers or body');
                    }
                }
                
                // Fallback: show error
                console.log('Request failed or no redirect found');
                button.innerHTML = 'Error - Check Console';
                setTimeout(() => {
                    button.innerHTML = originalText;
                    button.disabled = false;
                }, 5000);
                
            } catch (error) {
                console.error('Error creating share link:', error);
                if (error.name === 'AbortError') {
                    button.innerHTML = 'Timeout - Try Again';
                } else {
                    button.innerHTML = 'Network Error';
                }
                setTimeout(() => {
                    button.innerHTML = originalText;
                    button.disabled = false;
                }, 5000);
            }
        }
        
        // Reset any stuck Share buttons
        function resetStuckButtons() {
            const stuckButtons = document.querySelectorAll('button[onclick*="createShareLink"]');
            let resetCount = 0;
            stuckButtons.forEach(button => {
                if (button.innerHTML === 'Creating link...' || button.disabled) {
                    console.log('Resetting stuck button:', button.innerHTML, 'disabled:', button.disabled);
                    button.innerHTML = 'Share';
                    button.disabled = false;
                    resetCount++;
                }
            });
            if (resetCount > 0) {
                console.log('Reset', resetCount, 'stuck buttons');
            }
        }
        
        // Reset buttons on page load (including back navigation)
        document.addEventListener('DOMContentLoaded', resetStuckButtons);
        
        // Reset buttons when returning from back/forward navigation
        window.addEventListener('pageshow', function(event) {
            // This fires when page is shown from cache (back button)
            resetStuckButtons();
        });
        
        // Reset buttons when HTMX loads new content  
        document.body.addEventListener('htmx:afterSwap', resetStuckButtons);
        
        // Also reset buttons periodically in case we miss events
        setInterval(resetStuckButtons, 2000);
    </script>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; max-width: 800px; margin: 2rem auto; padding: 1rem; line-height: 1.6; }
        .header { text-align: center; margin-bottom: 2rem; }
        .header h1 { font-size: 2.5rem; margin-bottom: 0.5rem; color: #1a202c; }
        .header p { color: #718096; font-size: 1.1rem; }
        
        .search-container { margin-bottom: 2rem; }
        .search-form { display: flex; flex-direction: column; gap: 1rem; }
        .search-input { padding: 1rem; font-size: 1.1rem; border: 2px solid #e2e8f0; border-radius: 8px; outline: none; transition: border-color 0.2s; }
        .search-input:focus { border-color: #4299e1; }
        
        .filters { display: flex; gap: 1rem; flex-wrap: wrap; align-items: center; }
        .filter-group { display: flex; flex-direction: column; gap: 0.25rem; }
        .filter-group label { font-size: 0.9rem; font-weight: 500; color: #4a5568; }
        .filter-select { padding: 0.5rem; border: 1px solid #e2e8f0; border-radius: 4px; background: white; }
        
        .loading { display: none; text-align: center; color: #718096; margin: 1rem 0; }
        .htmx-request .loading { display: block; }
        
        .search-results { margin-top: 2rem; }
        .result-item { display: flex; gap: 1rem; padding: 1rem; border: 1px solid #e2e8f0; border-radius: 8px; margin-bottom: 1rem; transition: box-shadow 0.2s; }
        .result-item:hover { box-shadow: 0 4px 12px rgba(0,0,0,0.1); }
        
        .result-image { width: 80px; height: 80px; border-radius: 8px; object-fit: cover; flex-shrink: 0; }
        .result-image-placeholder { width: 80px; height: 80px; border-radius: 8px; background: #f7fafc; display: flex; align-items: center; justify-content: center; color: #a0aec0; font-size: 2rem; }
        
        .result-content { flex-grow: 1; }
        .result-title { font-size: 1.1rem; font-weight: 600; margin-bottom: 0.25rem; color: #1a202c; }
        .result-artist { color: #4a5568; margin-bottom: 0.25rem; }
        .result-album { color: #718096; font-size: 0.9rem; margin-bottom: 0.5rem; }
        .result-platforms { display: flex; gap: 0.5rem; align-items: center; }
        .platform-badge { padding: 0.25rem 0.5rem; border-radius: 4px; font-size: 0.8rem; font-weight: 500; display: flex; align-items: center; gap: 0.25rem; }
        .platform-badge-icon { width: 14px; height: 14px; flex-shrink: 0; object-fit: contain; }
        .platform-spotify { background: #1db954; color: white; }
        .platform-apple_music { background: #fa243c; color: white; }
        .platform-local { background: #4299e1; color: white; }
        
        .result-actions { display: flex; flex-direction: column; gap: 0.5rem; align-items: flex-end; }
        .action-btn { padding: 0.5rem 1rem; border: none; border-radius: 4px; font-size: 0.9rem; cursor: pointer; transition: all 0.2s; text-decoration: none; display: inline-block; text-align: center; }
        .action-primary { background: #4299e1; color: white; }
        .action-primary:hover { background: #3182ce; }
        .action-secondary { background: #e2e8f0; color: #4a5568; }
        .action-secondary:hover { background: #cbd5e0; }
        .action-small { padding: 0.25rem 0.5rem; font-size: 0.8rem; }
        
        .platform-actions { display: flex; flex-wrap: wrap; gap: 0.25rem; margin-top: 0.5rem; }
        
        .action-success { text-align: center; padding: 1rem; background: #f0fff4; border: 1px solid #68d391; border-radius: 4px; }
        .action-success p { margin: 0 0 0.5rem 0; color: #2f855a; font-weight: 500; }
        
        .no-results { text-align: center; color: #718096; margin: 2rem 0; }
        .empty-state { text-align: center; color: #718096; margin: 3rem 0; }
        
        .footer { text-align: center; margin-top: 3rem; padding-top: 2rem; border-top: 1px solid #e2e8f0; color: #a0aec0; font-size: 0.9rem; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🎵 SongShare Search</h1>
        <p>Find your favorite songs across all platforms</p>
    </div>

    <div class="search-container">
        <form class="search-form" hx-get="/api/v1/search/results" hx-target="#search-results" hx-trigger="submit, keyup delay:500ms from:input[name='q']" hx-indicator="#loading">
            <input 
                type="text" 
                name="q" 
                class="search-input" 
                placeholder="Search for songs, artists, or albums..." 
                value="{{.Query}}"
                autocomplete="off"
            >
            
            <div class="filters">
                
                <div class="filter-group">
                    <label for="limit">Results</label>
                    <select name="limit" class="filter-select" hx-get="/api/v1/search/results" hx-target="#search-results" hx-trigger="change" hx-include="closest form" hx-indicator="#loading">
                        <option value="10">10 results</option>
                        <option value="25">25 results</option>
                        <option value="50">50 results</option>
                    </select>
                </div>
            </div>
        </form>
        
        <div id="loading" class="loading">🔍 Searching...</div>
    </div>

    <div id="search-results" class="search-results">
        {{if .Query}}
            <div hx-get="/api/v1/search/results?q={{.Query}}" hx-trigger="load" hx-target="this" hx-indicator="#loading"></div>
        {{else}}
            <div class="empty-state">
                <p>Start typing to search for songs across Spotify, Apple Music, and your local library.</p>
            </div>
        {{end}}
    </div>

    <div class="footer">
        <p>Powered by SongShare | <a href="/" style="color: #4299e1;">Home</a></p>
    </div>
</body>
</html>`

	tmpl, err := template.New("search").Parse(htmlTemplate)
	if err != nil {
		slog.Error("Failed to parse search template", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Template error"})
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(c.Writer, data); err != nil {
		slog.Error("Failed to execute search template", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Render error"})
	}
}

// SearchResults handles GET /api/v1/search/results and returns HTML fragments
func (h *SongHandler) SearchResults(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	limitStr := c.Query("limit")
	
	// Parse limit
	limit := 10
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 50 {
			limit = parsedLimit
		}
	}
	
	if query == "" {
		c.String(http.StatusOK, `<div class="empty-state"><p>Enter a search term to find songs.</p></div>`)
		return
	}

	// Search different sources based on platform filter
	var allResults []SearchResultWithSource
	
	// Search local database (cache-only approach)
	localResults, err := h.searchLocalSongs(c.Request.Context(), query, limit)
	if err != nil {
		slog.Error("Failed to search local songs", "error", err)
	} else {
		for _, result := range localResults {
			allResults = append(allResults, SearchResultWithSource{
				SearchResult: result,
				Source:      "local",
			})
		}
	}
	
	// Search platforms for fresh results with comprehensive auto-indexing
	platformResults, err := h.searchPlatforms(c.Request.Context(), query, "", limit)
	if err != nil {
		slog.Error("Failed to search platforms", "error", err)
	} else {
		// Add platform results to display immediately
		for _, track := range platformResults {
			allResults = append(allResults, SearchResultWithSource{
				SearchResult: track,
				Source:      "platform",
			})
		}
		
		// Comprehensive background auto-indexing for all results
		// This builds our local cache comprehensively while not blocking responses
		go h.backgroundIndexTracks(query, platformResults)
	}
	
	if len(allResults) == 0 {
		c.String(http.StatusOK, `<div class="no-results"><p>No songs found for "%s". Try a different search term.</p></div>`, query)
		return
	}
	
	// For cache-only search, always use grouped results for consistent UX
	groupedResults := h.groupSearchResults(allResults, query)
	
	// Generate HTML for grouped results  
	html := h.renderGroupedSearchResultsHTML(groupedResults)
	c.String(http.StatusOK, html)
}

// SearchResultWithSource combines search result with its source type
type SearchResultWithSource struct {
	SearchResult SearchResult
	Source       string // "local" or "platform"
}

// GroupedSearchResult represents a song with multiple platform links
type GroupedSearchResult struct {
	Title       string
	Artists     []string
	Album       string
	ISRC        string
	DurationMs  int
	ReleaseDate string
	ImageURL    string
	Popularity  int // Highest popularity across all platforms (0-100)
	
	// Multiple platform links for the same song
	PlatformLinks []PlatformResult
	
	// Indicate if this song is already in local database
	HasLocalLink bool
	LocalURL     string
	
	// For sorting - track the best relevance score and position
	RelevanceScore float64
	OriginalIndex  int
}

// PlatformResult represents availability on a specific platform
type PlatformResult struct {
	Platform  string
	URL       string
	Available bool
	Source    string // "local" or "platform"
}

// searchLocalSongs searches the local MongoDB database
func (h *SongHandler) searchLocalSongs(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	songs, err := h.songRepository.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	
	var results []SearchResult
	for _, song := range songs {
		// For each song, create search results for each available platform
		// This allows the grouping logic to properly show platform badges
		if len(song.PlatformLinks) > 0 {
			for _, link := range song.PlatformLinks {
				if link.Available {
					result := SearchResult{
						Title:       song.Title,
						Artists:     []string{song.Artist},
						Album:       song.Album,
						URL:         link.URL, // Use the actual platform URL, not universal link
						Platform:    link.Platform,
						ISRC:        song.ISRC,
						DurationMs:  song.Metadata.Duration,
						ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
						ImageURL:    song.Metadata.ImageURL,
						Popularity:  song.Metadata.Popularity,
						Available:   true,
					}
					results = append(results, result)
				}
			}
		} else {
			// Fallback for songs without platform links
			result := SearchResult{
				Title:       song.Title,
				Artists:     []string{song.Artist},
				Album:       song.Album,
				URL:         h.buildUniversalLink(song),
				Platform:    "local",
				ISRC:        song.ISRC,
				DurationMs:  song.Metadata.Duration,
				ReleaseDate: song.Metadata.ReleaseDate.Format("2006-01-02"),
				ImageURL:    song.Metadata.ImageURL,
				Popularity:  song.Metadata.Popularity,
				Available:   true,
			}
			results = append(results, result)
		}
	}
	
	return results, nil
}

// searchPlatforms searches external platforms
func (h *SongHandler) searchPlatforms(ctx context.Context, query string, platformFilter string, limit int) ([]SearchResult, error) {
	searchQuery := services.SearchQuery{
		Query: query,
		Limit: limit,
	}
	
	// Determine which platforms to search
	var platforms []string
	if platformFilter == "" {
		platforms = []string{"spotify", "apple_music"}
	} else {
		platforms = []string{platformFilter}
	}
	
	var allResults []SearchResult
	
	// Search each platform
	for _, platform := range platforms {
		var platformService services.PlatformService
		switch platform {
		case "spotify":
			platformService = h.resolutionService.GetPlatformService("spotify")
		case "apple_music":
			platformService = h.resolutionService.GetPlatformService("apple_music")
		default:
			continue
		}
		
		if platformService == nil {
			continue
		}
		
		tracks, err := platformService.SearchTrack(ctx, searchQuery)
		if err != nil {
			slog.Warn("Platform search failed", "platform", platform, "error", err)
			continue
		}
		
		// Convert to search results
		for _, track := range tracks {
			result := SearchResult{
				Title:       track.Title,
				Artists:     track.Artists,
				Album:       track.Album,
				URL:         track.URL,
				Platform:    track.Platform,
				ISRC:        track.ISRC,
				DurationMs:  track.Duration,
				ReleaseDate: track.ReleaseDate,
				ImageURL:    track.ImageURL,
				Popularity:  track.Popularity,
				Available:   track.Available,
			}
			allResults = append(allResults, result)
		}
	}
	
	return allResults, nil
}

// renderSearchResultsHTML generates HTML for search results
func (h *SongHandler) renderSearchResultsHTML(results []SearchResultWithSource) string {
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
		html.WriteString(fmt.Sprintf(`<div class="result-title">%s</div>`, result.Title))
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
		}
		html.WriteString(fmt.Sprintf(`<span class="platform-badge %s">%s</span>`, platformClass, platformName))
		html.WriteString(`</div>`)
		
		html.WriteString(`</div>`) // End content
		
		// Actions
		html.WriteString(`<div class="result-actions">`)
		if item.Source == "platform" {
			// For platform results, create universal link via resolve
			html.WriteString(fmt.Sprintf(`
				<button class="action-btn action-primary" 
				        hx-post="/api/v1/songs/resolve" 
				        hx-vals='{"url": "%s"}' 
				        hx-headers='{"Content-Type": "application/json"}'
				        hx-target="closest .result-actions"
				        hx-swap="innerHTML">
					Share
				</button>
			`, result.URL))
		} else {
			// For local results, show the universal link
			html.WriteString(fmt.Sprintf(`<a href="%s" class="action-btn action-primary">View Song</a>`, result.URL))
		}
		html.WriteString(fmt.Sprintf(`<a href="%s" target="_blank" class="action-btn action-secondary">Open in %s</a>`, result.URL, platformName))
		html.WriteString(`</div>`) // End actions
		
		html.WriteString(`</div>`) // End result item
	}
	
	html.WriteString(`</div>`) // End container
	return html.String()
}

// groupSearchResults groups search results by song, combining multiple platform entries
func (h *SongHandler) groupSearchResults(results []SearchResultWithSource, query string) []GroupedSearchResult {
	// Create a map to group results by song identity
	songMap := make(map[string]*GroupedSearchResult)
	// Keep track of insertion order to preserve relevance ranking
	var orderedKeys []string
	
	for i, item := range results {
		result := item.SearchResult
		
		// Create a unique key for this song
		key := h.generateSongKey(result)
		
		// Calculate relevance score for this specific result
		relevanceScore := h.calculateRelevanceScore(result, item.Source, query, i)
		
		if existing, exists := songMap[key]; exists {
			// Song already exists, add this platform to it (if not already present)
			platformResult := PlatformResult{
				Platform:  result.Platform,
				URL:       result.URL,
				Available: result.Available,
				Source:    item.Source,
			}
			
			// Check if we already have this platform
			hasThisPlatform := false
			for i, existingPlatform := range existing.PlatformLinks {
				if existingPlatform.Platform == result.Platform {
					// Update existing platform with better info if available
					if existingPlatform.URL == "" && result.URL != "" {
						existing.PlatformLinks[i].URL = result.URL
					}
					hasThisPlatform = true
					break
				}
			}
			
			if !hasThisPlatform {
				existing.PlatformLinks = append(existing.PlatformLinks, platformResult)
			}
			
			// Update local link info if this is a local result
			if item.Source == "local" {
				existing.HasLocalLink = true
				// For local results, use the universal link (ISRC-based) for sharing
				existing.LocalURL = fmt.Sprintf("%s/s/%s", h.baseURL, result.ISRC)
			}
			
			// Prefer non-empty values for metadata
			if existing.ImageURL == "" && result.ImageURL != "" {
				existing.ImageURL = result.ImageURL
			}
			if existing.Album == "" && result.Album != "" {
				existing.Album = result.Album
			}
			if existing.ISRC == "" && result.ISRC != "" {
				existing.ISRC = result.ISRC
			}
			if existing.DurationMs == 0 && result.DurationMs > 0 {
				existing.DurationMs = result.DurationMs
			}
			if existing.ReleaseDate == "" && result.ReleaseDate != "" {
				existing.ReleaseDate = result.ReleaseDate
			}
			
			// Use the highest popularity across all platforms
			if result.Popularity > existing.Popularity {
				existing.Popularity = result.Popularity
			}
			
			// Update relevance score if this one is better
			if relevanceScore > existing.RelevanceScore {
				existing.RelevanceScore = relevanceScore
				existing.OriginalIndex = i
			}
		} else {
			// New song, create grouped result
			platformResult := PlatformResult{
				Platform:  result.Platform,
				URL:       result.URL,
				Available: result.Available,
				Source:    item.Source,
			}
			
			grouped := &GroupedSearchResult{
				Title:          result.Title,
				Artists:        result.Artists,
				Album:          result.Album,
				ISRC:           result.ISRC,
				DurationMs:     result.DurationMs,
				ReleaseDate:    result.ReleaseDate,
				ImageURL:       result.ImageURL,
				Popularity:     result.Popularity,
				PlatformLinks:  []PlatformResult{platformResult},
				HasLocalLink:   item.Source == "local",
				LocalURL:       "",
				RelevanceScore: relevanceScore,
				OriginalIndex:  i,
			}
			
			if item.Source == "local" {
				// For local results, use the universal link (ISRC-based) for sharing
				grouped.LocalURL = fmt.Sprintf("%s/s/%s", h.baseURL, result.ISRC)
			}
			
			songMap[key] = grouped
			orderedKeys = append(orderedKeys, key) // Track order
		}
	}
	
	// Convert map to slice, removing duplicates from orderedKeys
	var groupedResults []GroupedSearchResult
	seenKeys := make(map[string]bool)
	for _, key := range orderedKeys {
		if !seenKeys[key] {
			seenKeys[key] = true
			if grouped, exists := songMap[key]; exists {
				groupedResults = append(groupedResults, *grouped)
			}
		}
	}
	
	// Sort by relevance score (higher is better)
	sort.Slice(groupedResults, func(i, j int) bool {
		// Primary sort by relevance score (descending)
		if groupedResults[i].RelevanceScore != groupedResults[j].RelevanceScore {
			return groupedResults[i].RelevanceScore > groupedResults[j].RelevanceScore
		}
		// Secondary sort by original index (ascending - earlier is better)
		return groupedResults[i].OriginalIndex < groupedResults[j].OriginalIndex
	})
	
	return groupedResults
}

// generateSongKey creates a unique key for grouping songs
func (h *SongHandler) generateSongKey(result SearchResult) string {
	// Primary: Group by ISRC if available - this preserves different versions (single vs album)
	if result.ISRC != "" {
		return "isrc:" + result.ISRC
	}
	
	// Secondary: Group by normalized title + artist + album to distinguish different releases
	normalizedTitle := h.normalizeString(result.Title)
	normalizedArtists := h.normalizeString(strings.Join(result.Artists, ", "))
	normalizedAlbum := h.normalizeString(result.Album)
	
	key := "song:" + normalizedTitle + "|" + normalizedArtists + "|" + normalizedAlbum
	
	// Debug logging to see what's happening
	slog.Debug("Generated song key", 
		"title", result.Title, 
		"artists", result.Artists, 
		"album", result.Album,
		"normalizedTitle", normalizedTitle,
		"normalizedArtists", normalizedArtists,
		"normalizedAlbum", normalizedAlbum,
		"key", key)
	
	return key
}

// normalizeString normalizes a string for comparison (lowercase, trim, etc.)
func (h *SongHandler) normalizeString(s string) string {
	// Convert to lowercase and trim spaces
	normalized := strings.ToLower(strings.TrimSpace(s))
	
	// Remove common variations that might cause false negatives
	normalized = strings.ReplaceAll(normalized, "feat.", "ft.")
	normalized = strings.ReplaceAll(normalized, "featuring", "ft.")
	normalized = strings.ReplaceAll(normalized, " ft ", " ft. ")
	
	// Remove punctuation that might differ between platforms
	// (Keep it simple for now - Unicode handling can be complex)
	
	// Remove extra spaces
	normalized = strings.Join(strings.Fields(normalized), " ")
	
	return normalized
}

// calculateRelevanceScore calculates a relevance score for a search result
func (h *SongHandler) calculateRelevanceScore(result SearchResult, source string, query string, index int) float64 {
	score := 0.0
	queryLower := strings.ToLower(query)
	titleLower := strings.ToLower(result.Title)
	queryWords := strings.Fields(queryLower)
	
	// **PRIMARY FACTOR: Popularity** - This should be the main ranking signal
	if result.Popularity > 0 {
		// Scale popularity to significant impact on scoring (0-100 → 0-1000 points)
		popularityBonus := float64(result.Popularity) * 10.0
		score += popularityBonus
		
		// Extra bonus tiers for mainstream appeal
		if result.Popularity >= 80 {
			score += 300.0 // Mega hits get top priority
		} else if result.Popularity >= 60 {
			score += 150.0 // Popular songs get good boost
		} else if result.Popularity >= 40 {
			score += 75.0  // Moderate popularity still matters
		}
	}
	
	// **SECONDARY FACTOR: Text matching quality**
	// Multi-word query handling
	if len(queryWords) > 1 {
		titleAndArtist := titleLower + " " + strings.ToLower(strings.Join(result.Artists, " "))
		matchedWords := 0
		for _, word := range queryWords {
			if strings.Contains(titleAndArtist, word) {
				matchedWords++
			}
		}
		
		// Bonus for matching words (but less than popularity)
		if matchedWords == len(queryWords) {
			score += 1500.0 // Perfect text match still very important
		} else if matchedWords > len(queryWords)/2 {
			score += 800.0 + float64(matchedWords)*50.0
		}
	}
	
	// Exact title matching
	if titleLower == queryLower {
		score += 1200.0 // Exact title match is very relevant
	} else if strings.HasPrefix(titleLower, queryLower) {
		score += 600.0  // Title starts with query
	} else if strings.Contains(titleLower, queryLower) {
		score += 250.0  // Title contains query
	}
	
	// Artist name matching
	for _, artist := range result.Artists {
		artistLower := strings.ToLower(artist)
		if artistLower == queryLower {
			score += 1000.0 // Exact artist match is very important
		} else if strings.Contains(artistLower, queryLower) {
			score += 150.0  // Partial artist match
		}
	}
	
	// **MINOR FACTORS:** Small adjustments
	// Local results get slight preference (not overwhelming)
	if source == "local" {
		score += 50.0
	}
	
	// Album matching (reduced importance - users don't search by albums much)
	if result.Album != "" {
		albumLower := strings.ToLower(result.Album)
		if albumLower == queryLower {
			score += 100.0 // Reduced from 300
		} else if strings.Contains(albumLower, queryLower) {
			score += 25.0  // Reduced from 50
		}
	}
	
	// Remove platform bias and album/single preference - let popularity handle quality
	
	return score
}

// renderGroupedSearchResultsHTML generates HTML for grouped search results
func (h *SongHandler) renderGroupedSearchResultsHTML(results []GroupedSearchResult) string {
	if len(results) == 0 {
		return `<div class="no-results"><p>No songs found.</p></div>`
	}
	
	var html strings.Builder
	html.WriteString(`<div class="search-results-container">`)
	
	for _, result := range results {
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
		html.WriteString(fmt.Sprintf(`<div class="result-title">%s</div>`, result.Title))
		html.WriteString(fmt.Sprintf(`<div class="result-artist">%s</div>`, strings.Join(result.Artists, ", ")))
		if result.Album != "" {
			html.WriteString(fmt.Sprintf(`<div class="result-album">%s</div>`, result.Album))
		}
		
		// Platform badges (multiple platforms) - clickable badges that link directly to platforms
		html.WriteString(`<div class="result-platforms">`)
		
		// Show clickable platform badges
		for _, platform := range result.PlatformLinks {
			// Skip the "local" fallback platform - we'll handle that separately
			if platform.Platform == "local" {
				continue
			}
			
			// Use dynamic platform UI system for badges
			badgeHTML := RenderPlatformBadge(platform.Platform, platform.URL)
			html.WriteString(badgeHTML)
		}
		
		// SongShare badge removed - users don't need to see this alongside platform badges
		
		html.WriteString(`</div>`)
		
		html.WriteString(`</div>`) // End content
		
		// Actions - simplified with only primary action
		html.WriteString(`<div class="result-actions">`)
		
		// Primary action: View local link if available, otherwise create universal link
		if result.HasLocalLink {
			html.WriteString(fmt.Sprintf(`<a href="%s" class="action-btn action-primary">Share</a>`, result.LocalURL))
		} else {
			// Find the first platform result to create a universal link from
			var firstPlatformURL string
			for _, platform := range result.PlatformLinks {
				if platform.Source == "platform" {
					firstPlatformURL = platform.URL
					break
				}
			}
			
			if firstPlatformURL != "" {
				html.WriteString(fmt.Sprintf(`
					<button class="action-btn action-primary" 
					        onclick="createShareLink('%s', this)">
						Share
					</button>
				`, firstPlatformURL))
			}
		}
		
		html.WriteString(`</div>`) // End actions
		
		html.WriteString(`</div>`) // End result item
	}
	
	html.WriteString(`</div>`) // End container
	return html.String()
}// backgroundIndexTracks performs intelligent background indexing of search results
func (h *SongHandler) backgroundIndexTracks(query string, tracks []SearchResult) {
	if len(tracks) == 0 {
		return
	}
	
	queryWords := strings.Fields(strings.ToLower(query))
	
	// Index tracks with priority scoring
	for i, track := range tracks {
		// Calculate indexing priority (0-100)
		priority := h.calculateIndexingPriority(track, query, queryWords, i)
		
		// Index tracks with sufficient priority
		if priority >= 20 { // Lower threshold for comprehensive indexing
			// Add small delay between requests to be respectful to APIs
			time.Sleep(time.Duration(i*50) * time.Millisecond)
			
			song, err := h.resolutionService.ResolveFromURL(context.Background(), track.URL)
			if err != nil {
				slog.Debug("Background indexing failed", 
					"query", query, 
					"track", track.Title, 
					"platform", track.Platform,
					"priority", priority,
					"error", err)
				continue
			}
			
			if song != nil {
				slog.Debug("Successfully indexed track", 
					"query", query, 
					"track", track.Title, 
					"platform", track.Platform, 
					"priority", priority,
					"popularity", track.Popularity)
				
				// Invalidate search cache since we have new indexed content
				h.invalidateSearchCache(query)
			}
		}
	}
}

// calculateIndexingPriority determines how important it is to index this track
func (h *SongHandler) calculateIndexingPriority(track SearchResult, query string, queryWords []string, position int) int {
	priority := 0
	
	// Popularity is the primary factor (0-50 points)
	if track.Popularity > 0 {
		priority += track.Popularity / 2 // Scale 0-100 to 0-50
	}
	
	// Text match quality (0-30 points)
	titleLower := strings.ToLower(track.Title)
	queryLower := strings.ToLower(query)
	
	if titleLower == queryLower {
		priority += 30 // Exact match
	} else if strings.HasPrefix(titleLower, queryLower) {
		priority += 20 // Prefix match
	} else if strings.Contains(titleLower, queryLower) {
		priority += 10 // Contains match
	}
	
	// Multi-word query bonus (0-15 points)
	if len(queryWords) > 1 {
		titleAndArtist := titleLower + " " + strings.ToLower(strings.Join(track.Artists, " "))
		matchedWords := 0
		for _, word := range queryWords {
			if strings.Contains(titleAndArtist, word) {
				matchedWords++
			}
		}
		if matchedWords == len(queryWords) {
			priority += 15
		} else if matchedWords > len(queryWords)/2 {
			priority += 10
		}
	}
	
	// Platform preference (0-5 points)
	if track.Platform == "spotify" {
		priority += 3
	}
	
	// Position penalty (top results are more likely to be clicked)
	priority -= position // Subtract position from priority
	
	return priority
}

// buildUniversalLink creates a universal link for a song using ISRC
func (h *SongHandler) buildUniversalLink(song *models.Song) string {
	if song.ISRC == "" {
		slog.Warn("Song missing ISRC", "songID", song.ID.Hex(), "title", song.Title)
		// This shouldn't happen with properly indexed songs
		return fmt.Sprintf("%s/s/unknown", h.baseURL)
	}
	return fmt.Sprintf("%s/s/%s", h.baseURL, song.ISRC)
}

// findSongByISRC finds a song by ISRC
func (h *SongHandler) findSongByISRC(ctx context.Context, isrc string) (*models.Song, error) {
	song, err := h.songRepository.FindByISRC(ctx, isrc)
	if err != nil {
		return nil, fmt.Errorf("song not found for ISRC %s: %w", isrc, err)
	}
	if song == nil {
		return nil, fmt.Errorf("song not found for ISRC: %s", isrc)
	}
	return song, nil
}

// invalidateSearchCache invalidates cached search results for a query
func (h *SongHandler) invalidateSearchCache(query string) {
	// Invalidate the search cache for this specific query
	// This integrates with our cached repository pattern
	if cachedRepo, ok := h.songRepository.(interface {
		InvalidateSearchCache(query string)
	}); ok {
		cachedRepo.InvalidateSearchCache(query)
		slog.Debug("Invalidated search cache", "query", query)
	} else {
		slog.Debug("Repository doesn't support cache invalidation", "query", query)
	}
}