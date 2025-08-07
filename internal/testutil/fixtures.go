package testutil

import (
	"time"

	"songshare/internal/models"
	"songshare/internal/services"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SongBuilder provides a fluent interface for creating test songs
type SongBuilder struct {
	song *models.Song
}

// NewSongBuilder creates a new song builder with default values
func NewSongBuilder() *SongBuilder {
	return &SongBuilder{
		song: models.NewSong("Test Song", "Test Artist"),
	}
}

// WithID sets the song ID
func (b *SongBuilder) WithID(id string) *SongBuilder {
	objID, _ := primitive.ObjectIDFromHex(id)
	b.song.ID = objID
	return b
}

// WithTitle sets the song title
func (b *SongBuilder) WithTitle(title string) *SongBuilder {
	b.song.Title = title
	return b
}

// WithArtist sets the song artist
func (b *SongBuilder) WithArtist(artist string) *SongBuilder {
	b.song.Artist = artist
	return b
}

// WithAlbum sets the song album
func (b *SongBuilder) WithAlbum(album string) *SongBuilder {
	b.song.Album = album
	return b
}

// WithISRC sets the ISRC code
func (b *SongBuilder) WithISRC(isrc string) *SongBuilder {
	b.song.ISRC = isrc
	return b
}

// WithSpotifyLink adds a Spotify platform link
func (b *SongBuilder) WithSpotifyLink(trackID, url string) *SongBuilder {
	b.song.AddPlatformLink("spotify", trackID, url, 1.0)
	return b
}

// WithAppleMusicLink adds an Apple Music platform link
func (b *SongBuilder) WithAppleMusicLink(trackID, url string) *SongBuilder {
	b.song.AddPlatformLink("apple_music", trackID, url, 1.0)
	return b
}

// WithDuration sets the song duration in milliseconds
func (b *SongBuilder) WithDuration(durationMs int) *SongBuilder {
	b.song.Metadata.Duration = durationMs
	return b
}

// WithPopularity sets the song popularity score
func (b *SongBuilder) WithPopularity(popularity int) *SongBuilder {
	b.song.Metadata.Popularity = popularity
	return b
}

// WithImageURL sets the album art image URL
func (b *SongBuilder) WithImageURL(imageURL string) *SongBuilder {
	b.song.Metadata.ImageURL = imageURL
	return b
}

// WithReleaseDate sets the release date
func (b *SongBuilder) WithReleaseDate(date time.Time) *SongBuilder {
	b.song.Metadata.ReleaseDate = date
	return b
}

// Build returns the constructed song
func (b *SongBuilder) Build() *models.Song {
	return b.song
}

// TrackInfoBuilder provides a fluent interface for creating test TrackInfo
type TrackInfoBuilder struct {
	track *services.TrackInfo
}

// NewTrackInfoBuilder creates a new TrackInfo builder with default values
func NewTrackInfoBuilder() *TrackInfoBuilder {
	return &TrackInfoBuilder{
		track: &services.TrackInfo{
			Platform:   "spotify",
			ExternalID: "test-track-id",
			URL:        "https://open.spotify.com/track/test-track-id",
			Title:      "Test Song",
			Artists:    []string{"Test Artist"},
			Available:  true,
		},
	}
}

// WithPlatform sets the platform
func (b *TrackInfoBuilder) WithPlatform(platform string) *TrackInfoBuilder {
	b.track.Platform = platform
	return b
}

// WithExternalID sets the external ID
func (b *TrackInfoBuilder) WithExternalID(id string) *TrackInfoBuilder {
	b.track.ExternalID = id
	return b
}

// WithURL sets the URL
func (b *TrackInfoBuilder) WithURL(url string) *TrackInfoBuilder {
	b.track.URL = url
	return b
}

// WithTitle sets the title
func (b *TrackInfoBuilder) WithTitle(title string) *TrackInfoBuilder {
	b.track.Title = title
	return b
}

// WithArtists sets the artists
func (b *TrackInfoBuilder) WithArtists(artists ...string) *TrackInfoBuilder {
	b.track.Artists = artists
	return b
}

// WithAlbum sets the album
func (b *TrackInfoBuilder) WithAlbum(album string) *TrackInfoBuilder {
	b.track.Album = album
	return b
}

// WithISRC sets the ISRC
func (b *TrackInfoBuilder) WithISRC(isrc string) *TrackInfoBuilder {
	b.track.ISRC = isrc
	return b
}

// WithDuration sets the duration in milliseconds
func (b *TrackInfoBuilder) WithDuration(durationMs int) *TrackInfoBuilder {
	b.track.Duration = durationMs
	return b
}

// WithPopularity sets the popularity score
func (b *TrackInfoBuilder) WithPopularity(popularity int) *TrackInfoBuilder {
	b.track.Popularity = popularity
	return b
}

// WithImageURL sets the image URL
func (b *TrackInfoBuilder) WithImageURL(imageURL string) *TrackInfoBuilder {
	b.track.ImageURL = imageURL
	return b
}

// WithReleaseDate sets the release date
func (b *TrackInfoBuilder) WithReleaseDate(releaseDate string) *TrackInfoBuilder {
	b.track.ReleaseDate = releaseDate
	return b
}

// WithAvailable sets the availability
func (b *TrackInfoBuilder) WithAvailable(available bool) *TrackInfoBuilder {
	b.track.Available = available
	return b
}

// Build returns the constructed TrackInfo
func (b *TrackInfoBuilder) Build() *services.TrackInfo {
	return b.track
}

// Common test data
var (
	// Sample ISRC codes
	TestISRC1 = "USUM71703861"
	TestISRC2 = "USRC17607839"
	TestISRC3 = "GBUM71505078"

	// Sample Spotify track IDs
	SpotifyTrackID1 = "4iV5W9uYEdYUVa79Axb7Rh"
	SpotifyTrackID2 = "1YLJVMUFYjwdAF4lDPqH7G"

	// Sample Apple Music track IDs
	AppleMusicTrackID1 = "1440857781"
	AppleMusicTrackID2 = "1440857782"

	// Sample URLs
	SpotifyURL1 = "https://open.spotify.com/track/" + SpotifyTrackID1
	SpotifyURL2 = "https://open.spotify.com/track/" + SpotifyTrackID2
	AppleMusicURL1 = "https://music.apple.com/us/song/test-song/" + AppleMusicTrackID1
	AppleMusicURL2 = "https://music.apple.com/us/album/test-album/123456789?i=" + AppleMusicTrackID2
)

// CreateTestSong creates a basic test song with default values
func CreateTestSong() *models.Song {
	return NewSongBuilder().
		WithISRC(TestISRC1).
		WithSpotifyLink(SpotifyTrackID1, SpotifyURL1).
		WithDuration(240000).
		WithPopularity(75).
		Build()
}

// CreateTestSongWithPlatforms creates a test song with multiple platform links
func CreateTestSongWithPlatforms() *models.Song {
	return NewSongBuilder().
		WithTitle("Bohemian Rhapsody").
		WithArtist("Queen").
		WithAlbum("A Night at the Opera").
		WithISRC(TestISRC2).
		WithSpotifyLink(SpotifyTrackID1, SpotifyURL1).
		WithAppleMusicLink(AppleMusicTrackID1, AppleMusicURL1).
		WithDuration(355000).
		WithPopularity(90).
		WithReleaseDate(time.Date(1975, 10, 31, 0, 0, 0, 0, time.UTC)).
		Build()
}

// CreateTestTrackInfo creates a basic test TrackInfo
func CreateTestTrackInfo() *services.TrackInfo {
	return NewTrackInfoBuilder().
		WithTitle("Test Song").
		WithArtists("Test Artist").
		WithISRC(TestISRC1).
		WithDuration(240000).
		WithPopularity(75).
		Build()
}