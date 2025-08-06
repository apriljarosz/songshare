package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"songshare/internal/services"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Get credentials from environment
	spotifyClientID := os.Getenv("SPOTIFY_CLIENT_ID")
	spotifyClientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	appleMusicKeyID := os.Getenv("APPLE_MUSIC_KEY_ID")
	appleMusicTeamID := os.Getenv("APPLE_MUSIC_TEAM_ID")
	appleMusicKeyFile := os.Getenv("APPLE_MUSIC_KEY_FILE")

	// Initialize services
	spotifyService := services.NewSpotifyService(spotifyClientID, spotifyClientSecret)
	appleMusicService := services.NewAppleMusicService(appleMusicKeyID, appleMusicTeamID, appleMusicKeyFile)

	ctx := context.Background()

	// Test search queries
	searches := []services.SearchQuery{
		{
			Title:  "Anti-Hero",
			Artist: "Taylor Swift",
			Limit:  3,
		},
		{
			Title:  "Flowers",
			Artist: "Miley Cyrus",
			Limit:  3,
		},
		{
			Query: "Blinding Lights The Weeknd",
			Limit: 3,
		},
	}

	for _, query := range searches {
		fmt.Printf("\n=== Searching: %+v ===\n", query)

		// Search Spotify
		fmt.Println("\n🎵 Spotify Results:")
		spotifyResults, err := spotifyService.SearchTrack(ctx, query)
		if err != nil {
			fmt.Printf("Spotify search error: %v\n", err)
		} else {
			for i, track := range spotifyResults {
				fmt.Printf("%d. %s by %v (%s)\n", i+1, track.Title, track.Artists, track.URL)
			}
		}

		// Search Apple Music
		fmt.Println("\n🍎 Apple Music Results:")
		appleMusicResults, err := appleMusicService.SearchTrack(ctx, query)
		if err != nil {
			fmt.Printf("Apple Music search error: %v\n", err)
		} else {
			for i, track := range appleMusicResults {
				fmt.Printf("%d. %s by %v (%s)\n", i+1, track.Title, track.Artists, track.URL)
			}
		}

		fmt.Println("\n" + strings.Repeat("-", 60))
	}

	// Test ISRC search
	fmt.Println("\n=== ISRC Search Test ===")
	isrc := "USUG11901472" // Cruel Summer by Taylor Swift
	fmt.Printf("Searching for ISRC: %s\n", isrc)

	spotifyTrack, err := spotifyService.GetTrackByISRC(ctx, isrc)
	if err != nil {
		fmt.Printf("Spotify ISRC search error: %v\n", err)
	} else {
		result, _ := json.MarshalIndent(spotifyTrack, "", "  ")
		fmt.Printf("🎵 Spotify: %s\n", result)
	}

	appleMusicTrack, err := appleMusicService.GetTrackByISRC(ctx, isrc)
	if err != nil {
		fmt.Printf("Apple Music ISRC search error: %v\n", err)
	} else {
		result, _ := json.MarshalIndent(appleMusicTrack, "", "  ")
		fmt.Printf("🍎 Apple Music: %s\n", result)
	}
}