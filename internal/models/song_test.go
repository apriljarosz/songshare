package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSong(t *testing.T) {
	title := "Bohemian Rhapsody"
	artist := "Queen"

	song := NewSong(title, artist)

	assert.Equal(t, CurrentSchemaVersion, song.SchemaVersion)
	assert.Equal(t, title, song.Title)
	assert.Equal(t, artist, song.Artist)
	assert.Empty(t, song.PlatformLinks)
	assert.NotZero(t, song.CreatedAt)
	assert.NotZero(t, song.UpdatedAt)
	assert.Equal(t, song.CreatedAt, song.UpdatedAt)
}

func TestSong_AddPlatformLink(t *testing.T) {
	song := NewSong("Test Song", "Test Artist")
	originalUpdatedAt := song.UpdatedAt

	// Sleep briefly to ensure timestamp difference
	time.Sleep(1 * time.Millisecond)

	// Add first platform link
	song.AddPlatformLink("spotify", "track123", "https://open.spotify.com/track/track123", 0.95)

	require.Len(t, song.PlatformLinks, 1)
	link := song.PlatformLinks[0]

	assert.Equal(t, "spotify", link.Platform)
	assert.Equal(t, "track123", link.ExternalID)
	assert.Equal(t, "https://open.spotify.com/track/track123", link.URL)
	assert.True(t, link.Available)
	assert.Equal(t, 0.95, link.Confidence)
	assert.True(t, song.UpdatedAt.After(originalUpdatedAt))
	assert.NotZero(t, link.LastVerified)
}

func TestSong_AddPlatformLink_UpdateExisting(t *testing.T) {
	song := NewSong("Test Song", "Test Artist")

	// Add initial link
	song.AddPlatformLink("spotify", "track123", "https://open.spotify.com/track/track123", 0.8)

	require.Len(t, song.PlatformLinks, 1)
	originalLastVerified := song.PlatformLinks[0].LastVerified

	// Sleep briefly to ensure timestamp difference
	time.Sleep(1 * time.Millisecond)

	// Update the same platform
	song.AddPlatformLink("spotify", "track456", "https://open.spotify.com/track/track456", 0.95)

	// Should still have only one link, but updated
	require.Len(t, song.PlatformLinks, 1)
	link := song.PlatformLinks[0]

	assert.Equal(t, "spotify", link.Platform)
	assert.Equal(t, "track456", link.ExternalID)
	assert.Equal(t, "https://open.spotify.com/track/track456", link.URL)
	assert.Equal(t, 0.95, link.Confidence)
	assert.True(t, link.LastVerified.After(originalLastVerified))
}

func TestSong_AddMultiplePlatformLinks(t *testing.T) {
	song := NewSong("Test Song", "Test Artist")

	song.AddPlatformLink("spotify", "track123", "https://open.spotify.com/track/track123", 0.9)
	song.AddPlatformLink("apple_music", "456789", "https://music.apple.com/us/song/test-song/456789", 0.85)

	require.Len(t, song.PlatformLinks, 2)

	spotifyLink := song.GetPlatformLink("spotify")
	require.NotNil(t, spotifyLink)
	assert.Equal(t, "track123", spotifyLink.ExternalID)

	appleMusicLink := song.GetPlatformLink("apple_music")
	require.NotNil(t, appleMusicLink)
	assert.Equal(t, "456789", appleMusicLink.ExternalID)
}

func TestSong_GetPlatformLink(t *testing.T) {
	song := NewSong("Test Song", "Test Artist")
	song.AddPlatformLink("spotify", "track123", "https://open.spotify.com/track/track123", 0.9)

	// Test existing platform
	link := song.GetPlatformLink("spotify")
	require.NotNil(t, link)
	assert.Equal(t, "spotify", link.Platform)
	assert.Equal(t, "track123", link.ExternalID)

	// Test non-existing platform
	nonExistentLink := song.GetPlatformLink("youtube")
	assert.Nil(t, nonExistentLink)
}

func TestSong_HasPlatform(t *testing.T) {
	song := NewSong("Test Song", "Test Artist")
	song.AddPlatformLink("spotify", "track123", "https://open.spotify.com/track/track123", 0.9)

	assert.True(t, song.HasPlatform("spotify"))
	assert.False(t, song.HasPlatform("apple_music"))
	assert.False(t, song.HasPlatform("youtube"))
}

func TestSong_GetAvailablePlatforms(t *testing.T) {
	song := NewSong("Test Song", "Test Artist")

	// Initially no platforms
	platforms := song.GetAvailablePlatforms()
	assert.Empty(t, platforms)

	// Add available platform
	song.AddPlatformLink("spotify", "track123", "https://open.spotify.com/track/track123", 0.9)
	platforms = song.GetAvailablePlatforms()
	assert.Equal(t, []string{"spotify"}, platforms)

	// Add another available platform
	song.AddPlatformLink("apple_music", "456789", "https://music.apple.com/us/song/test-song/456789", 0.85)
	platforms = song.GetAvailablePlatforms()
	assert.Len(t, platforms, 2)
	assert.Contains(t, platforms, "spotify")
	assert.Contains(t, platforms, "apple_music")

	// Mark one platform as unavailable
	song.PlatformLinks[0].Available = false
	platforms = song.GetAvailablePlatforms()
	assert.Equal(t, []string{"apple_music"}, platforms)
}

func TestSong_SchemaVersion(t *testing.T) {
	song := NewSong("Test Song", "Test Artist")
	assert.Equal(t, CurrentSchemaVersion, song.SchemaVersion)

	// Verify that CurrentSchemaVersion is set to expected value
	assert.Equal(t, 1, CurrentSchemaVersion)
}

func TestPlatformLink_DefaultValues(t *testing.T) {
	song := NewSong("Test Song", "Test Artist")
	song.AddPlatformLink("spotify", "track123", "https://open.spotify.com/track/track123", 0.9)

	link := song.PlatformLinks[0]
	assert.True(t, link.Available, "Platform link should be available by default")
	assert.NotZero(t, link.LastVerified, "LastVerified should be set")
	assert.Equal(t, 0.9, link.Confidence, "Confidence should match provided value")
}

func TestSongMetadata_DefaultValues(t *testing.T) {
	song := NewSong("Test Song", "Test Artist")

	// Test default values for metadata
	assert.Empty(t, song.Metadata.Genre)
	assert.Zero(t, song.Metadata.Duration)
	assert.True(t, song.Metadata.ReleaseDate.IsZero())
	assert.Empty(t, song.Metadata.Language)
	assert.Zero(t, song.Metadata.Popularity)
	assert.False(t, song.Metadata.Explicit)
	assert.Empty(t, song.Metadata.ImageURL)
}

func TestSong_UpdatedAtChanges(t *testing.T) {
	song := NewSong("Test Song", "Test Artist")
	originalUpdatedAt := song.UpdatedAt

	// Sleep to ensure timestamp difference
	time.Sleep(1 * time.Millisecond)

	// Adding platform link should update UpdatedAt
	song.AddPlatformLink("spotify", "track123", "https://open.spotify.com/track/track123", 0.9)
	assert.True(t, song.UpdatedAt.After(originalUpdatedAt))

	// Updating existing platform link should also update UpdatedAt
	updatedAt1 := song.UpdatedAt
	time.Sleep(1 * time.Millisecond)

	song.AddPlatformLink("spotify", "track456", "https://open.spotify.com/track/track456", 0.95)
	assert.True(t, song.UpdatedAt.After(updatedAt1))
}

func TestSong_EmptyValues(t *testing.T) {
	// Test with empty strings
	song := NewSong("", "")
	assert.Empty(t, song.Title)
	assert.Empty(t, song.Artist)
	assert.Empty(t, song.Album)
	assert.Empty(t, song.ISRC)
}

func TestSong_LongValues(t *testing.T) {
	// Test with very long strings (realistic edge cases)
	longTitle := "This Is An Extremely Long Song Title That Might Be Used In Some Musical Compositions Or Experimental Music"
	longArtist := "An Artist With A Very Long Name Including Multiple Words And Perhaps Some Special Characters"

	song := NewSong(longTitle, longArtist)
	assert.Equal(t, longTitle, song.Title)
	assert.Equal(t, longArtist, song.Artist)
}
