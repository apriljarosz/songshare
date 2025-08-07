package servicetest

import "context"

// PlatformService interface - local copy to avoid import cycle
type PlatformService interface {
	GetPlatformName() string
	ParseURL(url string) (*TrackInfo, error)
	GetTrackByID(ctx context.Context, trackID string) (*TrackInfo, error)
	SearchTrack(ctx context.Context, query SearchQuery) ([]*TrackInfo, error)
	GetTrackByISRC(ctx context.Context, isrc string) (*TrackInfo, error)
	BuildURL(trackID string) string
	Health(ctx context.Context) error
}

// TrackInfo represents track information from a platform
type TrackInfo struct {
	Platform   string   `json:"platform"`
	ExternalID string   `json:"external_id"`
	URL        string   `json:"url"`
	Title      string   `json:"title"`
	Artists    []string `json:"artists"`
	Album      string   `json:"album,omitempty"`
	ISRC       string   `json:"isrc,omitempty"`
	Duration   int      `json:"duration_ms,omitempty"`
	Available  bool     `json:"available"`
}

// SearchQuery represents a search query for tracks
type SearchQuery struct {
	Title  string `json:"title,omitempty"`
	Artist string `json:"artist,omitempty"`
	Album  string `json:"album,omitempty"`
	ISRC   string `json:"isrc,omitempty"`
	Query  string `json:"query,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// PlatformError represents an error from a platform service
type PlatformError struct {
	Platform  string
	Operation string
	Message   string
	URL       string
	Err       error
}

func (e *PlatformError) Error() string {
	msg := e.Platform + " " + e.Operation + " failed"
	if e.Message != "" {
		msg += ": " + e.Message
	}
	if e.URL != "" {
		msg += " (URL: " + e.URL + ")"
	}
	if e.Err != nil {
		msg += " - " + e.Err.Error()
	}
	return msg
}

func (e *PlatformError) Unwrap() error {
	return e.Err
}