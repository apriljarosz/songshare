package config

import (
	"github.com/kelseyhightower/envconfig"
)

// Config holds all configuration for the application
type Config struct {
	Port       string `envconfig:"PORT" default:"8080"`
	GinMode    string `envconfig:"GIN_MODE" default:"debug"`
	MongodbURL string `envconfig:"MONGODB_URL" required:"true"`
	ValkeyURL  string `envconfig:"VALKEY_URL" required:"true"`
	
	// Spotify API credentials
	SpotifyClientID     string `envconfig:"SPOTIFY_CLIENT_ID"`
	SpotifyClientSecret string `envconfig:"SPOTIFY_CLIENT_SECRET"`
	
	// Apple Music API credentials  
	AppleMusicKeyID   string `envconfig:"APPLE_MUSIC_KEY_ID"`
	AppleMusicTeamID  string `envconfig:"APPLE_MUSIC_TEAM_ID"`
	AppleMusicKeyFile string `envconfig:"APPLE_MUSIC_KEY_FILE"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}