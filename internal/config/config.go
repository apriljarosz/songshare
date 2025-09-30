package config

import (
	"os"
)

type Config struct {
	Environment string
	MongoURI    string
	Port        string

	// Platform API credentials
	SpotifyClientID        string
	SpotifyClientSecret    string
	AppleMusicTeamID       string
	AppleMusicKeyID        string
	AppleMusicPrivateKey   string
}

func Load() *Config {
	return &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		MongoURI:    getEnv("MONGO_URI", "mongodb://localhost:27017/songshare"),
		Port:        getEnv("PORT", "8080"),

		SpotifyClientID:        getEnv("SPOTIFY_CLIENT_ID", ""),
		SpotifyClientSecret:    getEnv("SPOTIFY_CLIENT_SECRET", ""),
		AppleMusicTeamID:       getEnv("APPLE_MUSIC_TEAM_ID", ""),
		AppleMusicKeyID:        getEnv("APPLE_MUSIC_KEY_ID", ""),
		AppleMusicPrivateKey:   getEnv("APPLE_MUSIC_PRIVATE_KEY", "AuthKey_TR264K37UX.p8"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
