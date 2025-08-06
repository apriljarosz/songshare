package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const CurrentSchemaVersion = 1

// Song represents a song with metadata and platform links
type Song struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	SchemaVersion int               `bson:"schema_version" json:"schema_version"`
	
	// Core Identifiers
	ISRC   string `bson:"isrc" json:"isrc"`                    // International Standard Recording Code
	Title  string `bson:"title" json:"title"`
	Artist string `bson:"artist" json:"artist"`
	Album  string `bson:"album,omitempty" json:"album,omitempty"`
	
	// Platform Links (Embedded for Performance)
	PlatformLinks []PlatformLink `bson:"platform_links" json:"platform_links"`
	
	// Additional Metadata
	Metadata SongMetadata `bson:"metadata" json:"metadata"`
	
	// Timestamps
	CreatedAt time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

// PlatformLink represents a link to a song on a specific music platform
type PlatformLink struct {
	Platform     string    `bson:"platform" json:"platform"`           // "spotify", "apple_music", etc.
	ExternalID   string    `bson:"external_id" json:"external_id"`     // Platform-specific track ID
	URL          string    `bson:"url" json:"url"`                     // Direct link to the song
	Available    bool      `bson:"available" json:"available"`         // Whether the song is currently available
	Confidence   float64   `bson:"confidence" json:"confidence"`       // Match confidence score (0-1)
	LastVerified time.Time `bson:"last_verified" json:"last_verified"` // When this link was last checked
}

// SongMetadata contains additional song information
type SongMetadata struct {
	Genre        []string  `bson:"genre,omitempty" json:"genre,omitempty"`
	Duration     int       `bson:"duration_ms,omitempty" json:"duration_ms,omitempty"` // Duration in milliseconds
	ReleaseDate  time.Time `bson:"release_date,omitempty" json:"release_date,omitempty"`
	Language     string    `bson:"language,omitempty" json:"language,omitempty"`
	Popularity   int       `bson:"popularity,omitempty" json:"popularity,omitempty"`   // Platform-specific popularity score
	Explicit     bool      `bson:"explicit,omitempty" json:"explicit,omitempty"`
}

// NewSong creates a new Song with default values
func NewSong(title, artist string) *Song {
	now := time.Now()
	return &Song{
		SchemaVersion: CurrentSchemaVersion,
		Title:         title,
		Artist:        artist,
		PlatformLinks: make([]PlatformLink, 0),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// AddPlatformLink adds or updates a platform link for the song
func (s *Song) AddPlatformLink(platform, externalID, url string, confidence float64) {
	now := time.Now()
	
	// Check if platform link already exists
	for i, link := range s.PlatformLinks {
		if link.Platform == platform {
			// Update existing link
			s.PlatformLinks[i].ExternalID = externalID
			s.PlatformLinks[i].URL = url
			s.PlatformLinks[i].Available = true
			s.PlatformLinks[i].Confidence = confidence
			s.PlatformLinks[i].LastVerified = now
			s.UpdatedAt = now
			return
		}
	}
	
	// Add new platform link
	s.PlatformLinks = append(s.PlatformLinks, PlatformLink{
		Platform:     platform,
		ExternalID:   externalID,
		URL:          url,
		Available:    true,
		Confidence:   confidence,
		LastVerified: now,
	})
	s.UpdatedAt = now
}

// GetPlatformLink returns the platform link for a specific platform
func (s *Song) GetPlatformLink(platform string) *PlatformLink {
	for _, link := range s.PlatformLinks {
		if link.Platform == platform {
			return &link
		}
	}
	return nil
}

// HasPlatform checks if the song has a link for the specified platform
func (s *Song) HasPlatform(platform string) bool {
	return s.GetPlatformLink(platform) != nil
}

// GetAvailablePlatforms returns a slice of platforms where the song is available
func (s *Song) GetAvailablePlatforms() []string {
	var platforms []string
	for _, link := range s.PlatformLinks {
		if link.Available {
			platforms = append(platforms, link.Platform)
		}
	}
	return platforms
}