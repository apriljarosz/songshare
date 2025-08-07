package services

import (
	"strconv"
	"strings"
	"time"
)

// TidalTrack represents a Tidal track resource in JSON:API format
type TidalTrack struct {
	ID   string `jsonapi:"primary,tracks"`
	Type string `jsonapi:"type"`

	// Track attributes
	Title        string `jsonapi:"attr,title"`
	Duration     int    `jsonapi:"attr,duration"` // Duration in seconds
	Explicit     bool   `jsonapi:"attr,explicit"`
	ISRC         string `jsonapi:"attr,isrc"`
	Copyright    string `jsonapi:"attr,copyright"`
	TrackNumber  int    `jsonapi:"attr,trackNumber"`
	VolumeNumber int    `jsonapi:"attr,volumeNumber"`
	Popularity   int    `jsonapi:"attr,popularity"`
	Available    bool   `jsonapi:"attr,streamReady"`
	PreviewURL   string `jsonapi:"attr,previewUrl"`
	Version      string `jsonapi:"attr,version"`

	// Relationships
	Artists   []*TidalArtist   `jsonapi:"relation,artists"`
	Album     *TidalAlbum      `jsonapi:"relation,album"`
	Providers []*TidalProvider `jsonapi:"relation,providers"`
}

// TidalArtist represents a Tidal artist resource
type TidalArtist struct {
	ID   string `jsonapi:"primary,artists"`
	Type string `jsonapi:"type"`

	// Artist attributes
	Name    string `jsonapi:"attr,name"`
	Picture string `jsonapi:"attr,picture"`
	Main    bool   `jsonapi:"attr,main"`
}

// TidalAlbum represents a Tidal album resource
type TidalAlbum struct {
	ID   string `jsonapi:"primary,albums"`
	Type string `jsonapi:"type"`

	// Album attributes
	Title           string    `jsonapi:"attr,title"`
	ReleaseDate     time.Time `jsonapi:"attr,releaseDate"`
	Duration        int       `jsonapi:"attr,duration"`
	NumberOfTracks  int       `jsonapi:"attr,numberOfTracks"`
	NumberOfVolumes int       `jsonapi:"attr,numberOfVolumes"`
	Copyright       string    `jsonapi:"attr,copyright"`
	UPC             string    `jsonapi:"attr,upc"`
	Explicit        bool      `jsonapi:"attr,explicit"`
	Popularity      int       `jsonapi:"attr,popularity"`
	Type_           string    `jsonapi:"attr,type"` // "ALBUM", "SINGLE", "EP", "COMPILATION"
	Cover           string    `jsonapi:"attr,cover"`
	VideoCover      string    `jsonapi:"attr,videoCover"`

	// Relationships
	Artists  []*TidalArtist  `jsonapi:"relation,artists"`
	CoverArt []*TidalArtwork `jsonapi:"relation,coverArt"`
}

// TidalArtwork represents artwork/cover art
type TidalArtwork struct {
	ID   string `jsonapi:"primary,artworks"`
	Type string `jsonapi:"type"`

	URL    string `jsonapi:"attr,url"`
	Width  int    `jsonapi:"attr,width"`
	Height int    `jsonapi:"attr,height"`
}

// TidalProvider represents a provider (streaming service/platform)
type TidalProvider struct {
	ID   string `jsonapi:"primary,providers"`
	Type string `jsonapi:"type"`

	Name string `jsonapi:"attr,name"`
}

// TidalSearchResult represents search results from Tidal API
type TidalSearchResult struct {
	ID   string `jsonapi:"primary,searchResults"`
	Type string `jsonapi:"type"`

	// Search result attributes
	Query string `jsonapi:"attr,query"`
	Total int    `jsonapi:"attr,total"`

	// Relationships to search results
	Tracks    []*TidalTrack    `jsonapi:"relation,tracks"`
	Artists   []*TidalArtist   `jsonapi:"relation,artists"`
	Albums    []*TidalAlbum    `jsonapi:"relation,albums"`
	TopHits   []*TidalTrack    `jsonapi:"relation,topHits"`
	Videos    []*TidalVideo    `jsonapi:"relation,videos"`
	Playlists []*TidalPlaylist `jsonapi:"relation,playlists"`
}

// TidalVideo represents a video resource
type TidalVideo struct {
	ID   string `jsonapi:"primary,videos"`
	Type string `jsonapi:"type"`

	Title    string `jsonapi:"attr,title"`
	Duration int    `jsonapi:"attr,duration"`
	Explicit bool   `jsonapi:"attr,explicit"`

	// Relationships
	Artists []*TidalArtist `jsonapi:"relation,artists"`
}

// TidalPlaylist represents a playlist resource
type TidalPlaylist struct {
	ID   string `jsonapi:"primary,playlists"`
	Type string `jsonapi:"type"`

	Title       string    `jsonapi:"attr,title"`
	Description string    `jsonapi:"attr,description"`
	Duration    int       `jsonapi:"attr,duration"`
	Public      bool      `jsonapi:"attr,public"`
	CreatedAt   time.Time `jsonapi:"attr,createdAt"`
	UpdatedAt   time.Time `jsonapi:"attr,updatedAt"`

	// Relationships
	CoverArt []*TidalArtwork `jsonapi:"relation,coverArt"`
}

// ToTrackInfo converts a TidalTrack to the internal TrackInfo format
func (t *TidalTrack) ToTrackInfo() *TrackInfo {
	// Extract artist names
	var artistNames []string
	for _, artist := range t.Artists {
		if artist != nil && artist.Name != "" {
			artistNames = append(artistNames, artist.Name)
		}
	}

	// Get album info
	albumTitle := ""
	releaseDate := ""
	imageURL := ""

	if t.Album != nil {
		albumTitle = t.Album.Title
		if !t.Album.ReleaseDate.IsZero() {
			releaseDate = t.Album.ReleaseDate.Format("2006-01-02")
		}

		// Get cover art URL
		if len(t.Album.CoverArt) > 0 && t.Album.CoverArt[0] != nil {
			imageURL = t.Album.CoverArt[0].URL
		} else if t.Album.Cover != "" {
			// Use direct cover URL if available
			imageURL = t.Album.Cover
		}
	}

	// Convert duration from seconds to milliseconds
	durationMs := t.Duration * 1000

	return &TrackInfo{
		Platform:    "tidal",
		ExternalID:  t.ID,
		URL:         buildTidalURL(t.ID),
		Title:       t.Title,
		Artists:     artistNames,
		Album:       albumTitle,
		ISRC:        t.ISRC,
		Duration:    durationMs,
		ReleaseDate: releaseDate,
		Explicit:    t.Explicit,
		Popularity:  t.Popularity,
		ImageURL:    imageURL,
		Available:   t.Available,
	}
}

// buildTidalURL constructs a Tidal URL from a track ID
func buildTidalURL(trackID string) string {
	return "https://tidal.com/track/" + trackID
}

// TidalAPIError represents an error response from Tidal API
type TidalAPIError struct {
	ID     string                 `json:"id,omitempty"`
	Status string                 `json:"status"`
	Code   string                 `json:"code,omitempty"`
	Title  string                 `json:"title"`
	Detail string                 `json:"detail,omitempty"`
	Source map[string]interface{} `json:"source,omitempty"`
}

// TidalAPIResponse represents the top-level API response structure
type TidalAPIResponse struct {
	Data     interface{}     `json:"data"`
	Included []interface{}   `json:"included,omitempty"`
	Meta     *TidalMeta      `json:"meta,omitempty"`
	Errors   []TidalAPIError `json:"errors,omitempty"`
}

// TidalMeta represents metadata in API responses
type TidalMeta struct {
	Total  int `json:"total,omitempty"`
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// ParseTrackID extracts track ID from various Tidal URL formats
func ParseTidalTrackID(url string) (string, error) {
	platform, trackID, err := ParsePlatformURL(url)
	if err != nil {
		return "", err
	}
	if platform != "tidal" {
		return "", &PlatformError{
			Platform:  "tidal",
			Operation: "parse_url",
			Message:   "not a Tidal URL",
			URL:       url,
		}
	}
	return trackID, nil
}

// FormatTidalImageURL formats Tidal image URLs with desired dimensions
func FormatTidalImageURL(baseURL string, width, height int) string {
	if baseURL == "" {
		return ""
	}

	// Tidal image URLs typically support dimension parameters
	if strings.Contains(baseURL, "resources.tidal.com") {
		return strings.ReplaceAll(baseURL, "{w}x{h}", strconv.Itoa(width)+"x"+strconv.Itoa(height))
	}

	return baseURL
}
