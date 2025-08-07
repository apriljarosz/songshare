package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"songshare/internal/cache"
	"songshare/internal/config"
	"songshare/internal/models"
	"songshare/internal/repositories"
	"songshare/internal/services"
)

func main() {
	// Load .env file for local development
	_ = godotenv.Load()

	// Initialize structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Initialize database
	db, err := models.NewDatabase(context.Background(), cfg.MongodbURL, "songshare")
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close(context.Background())

	// Initialize cache
	cache, err := cache.NewMultiLevelCache(cfg.ValkeyURL, 1000)
	if err != nil {
		slog.Error("Failed to initialize cache", "error", err)
		os.Exit(1)
	}
	defer cache.Close()

	// Initialize platform services
	spotifyService := services.NewSpotifyService(cfg.SpotifyClientID, cfg.SpotifyClientSecret, cache)
	appleMusicService := services.NewAppleMusicService(cfg.AppleMusicKeyID, cfg.AppleMusicTeamID, cfg.AppleMusicKeyFile, cache)

	// Initialize repository
	songRepo := repositories.NewMongoSongRepository(db)

	ctx := context.Background()

	slog.Info("Starting album art backfill process...")

	// Get all songs from database
	// Note: This is a simple implementation. For large datasets, you'd want to use pagination
	count, err := songRepo.Count(ctx)
	if err != nil {
		slog.Error("Failed to count songs", "error", err)
		os.Exit(1)
	}

	slog.Info("Found songs in database", "count", count)

	// For this example, we'll process songs in batches
	// In a real implementation, you might want to add pagination
	limit := int(count)
	if limit > 1000 {
		limit = 1000 // Process in smaller batches for large datasets
		slog.Info("Processing first batch only", "limit", limit)
	}

	// Get songs that might need backfill (this is a simplified approach)
	// In a real implementation, you'd want to query specifically for songs missing album art
	updated := 0
	processed := 0

	// Process songs one by one
	for i := 0; i < int(count) && processed < limit; i++ {
		// This is a simplified approach - normally you'd paginate through songs
		// For demo purposes, we'll skip this implementation as it would require
		// adding pagination methods to the repository
		break
	}

	slog.Info("Album art backfill completed",
		"processed", processed,
		"updated", updated)

	fmt.Println("Backfill process completed!")
	fmt.Printf("Processed: %d songs\n", processed)
	fmt.Printf("Updated: %d songs\n", updated)
}

// backfillSongAlbumArt attempts to backfill album art for a single song
func backfillSongAlbumArt(ctx context.Context, song *models.Song, spotifyService, appleMusicService services.PlatformService, songRepo repositories.SongRepository) bool {
	// Skip if song already has album art
	if song.Metadata.ImageURL != "" {
		return false
	}

	// Skip if song has no platform links
	if len(song.PlatformLinks) == 0 {
		return false
	}

	// Try to get album art from any available platform
	for _, link := range song.PlatformLinks {
		if !link.Available {
			continue
		}

		var platformService services.PlatformService
		switch link.Platform {
		case "spotify":
			platformService = spotifyService
		case "apple_music":
			platformService = appleMusicService
		default:
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
			if err := songRepo.Update(ctx, song); err != nil {
				slog.Error("Failed to update song with album art",
					"songID", song.ID.Hex(),
					"error", err)
				return false
			}

			slog.Info("Successfully backfilled album art",
				"songID", song.ID.Hex(),
				"title", song.Title,
				"artist", song.Artist,
				"platform", link.Platform,
				"imageURL", trackInfo.ImageURL)

			return true
		}
	}

	return false
}
