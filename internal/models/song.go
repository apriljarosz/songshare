package models

import "time"

type Song struct {
	ID          string         `bson:"_id" json:"id"`
	ISRC        string         `bson:"isrc" json:"isrc"`
	Title       string         `bson:"title" json:"title"`
	Artists     []string       `bson:"artists" json:"artists"`
	Album       string         `bson:"album" json:"album"`
	AlbumArt    string         `bson:"album_art" json:"albumArt"`
	Duration    int            `bson:"duration" json:"duration"` // milliseconds
	ReleaseDate string         `bson:"release_date" json:"releaseDate"`
	Explicit    bool           `bson:"explicit" json:"explicit"`
	Platforms   []PlatformLink `bson:"platforms" json:"platforms"`
	CreatedAt   time.Time      `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time      `bson:"updated_at" json:"updatedAt"`
}

type PlatformLink struct {
	Platform string `bson:"platform" json:"platform"` // "spotify", "apple_music"
	ID       string `bson:"id" json:"id"`
	URL      string `bson:"url" json:"url"`
}

// SearchRequest represents a text search query
type SearchRequest struct {
	Query  string `json:"query" binding:"required"`
	Offset int    `json:"offset"` // Number of results to skip (for pagination)
	Limit  int    `json:"limit"`  // Number of results to return (default 10)
}

// ResolveRequest represents a platform URL to resolve
type ResolveRequest struct {
	URL string `json:"url" binding:"required"`
}

// Platform represents a supported music platform
type Platform struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
