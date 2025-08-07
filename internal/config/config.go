package config

import (
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

// AuthMethod represents different authentication methods platforms can use
type AuthMethod string

const (
	AuthMethodOAuth2    AuthMethod = "oauth2"
	AuthMethodJWT       AuthMethod = "jwt"
	AuthMethodAPIKey    AuthMethod = "api_key"
	AuthMethodBasicAuth AuthMethod = "basic_auth"
)

// PlatformConfig represents configuration for a single platform
type PlatformConfig struct {
	Name       string     `json:"name"`
	Enabled    bool       `json:"enabled"`
	AuthMethod AuthMethod `json:"auth_method"`
	
	// OAuth2 credentials (Spotify-style)
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	TokenURL     string `json:"token_url,omitempty"`
	
	// JWT credentials (Apple Music-style)
	KeyID    string `json:"key_id,omitempty"`
	TeamID   string `json:"team_id,omitempty"`
	KeyFile  string `json:"key_file,omitempty"`
	
	// API Key credentials (simple)
	APIKey    string `json:"api_key,omitempty"`
	APISecret string `json:"api_secret,omitempty"`
	
	// Basic Auth credentials
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	
	// Additional configuration
	BaseURL     string            `json:"base_url,omitempty"`
	RateLimit   int               `json:"rate_limit,omitempty"`  // requests per minute
	Timeout     int               `json:"timeout,omitempty"`     // seconds
	ExtraConfig map[string]string `json:"extra_config,omitempty"`
}

// Config holds all configuration for the application
type Config struct {
	// Application settings
	Port       string `envconfig:"PORT" default:"8080"`
	GinMode    string `envconfig:"GIN_MODE" default:"debug"`
	BaseURL    string `envconfig:"BASE_URL" default:"http://localhost:8080"`
	MongodbURL string `envconfig:"MONGODB_URL" required:"true"`
	ValkeyURL  string `envconfig:"VALKEY_URL" required:"true"`
	
	// Legacy platform credentials (for backward compatibility)
	SpotifyClientID     string `envconfig:"SPOTIFY_CLIENT_ID"`
	SpotifyClientSecret string `envconfig:"SPOTIFY_CLIENT_SECRET"`
	AppleMusicKeyID     string `envconfig:"APPLE_MUSIC_KEY_ID"`
	AppleMusicTeamID    string `envconfig:"APPLE_MUSIC_TEAM_ID"`
	AppleMusicKeyFile   string `envconfig:"APPLE_MUSIC_KEY_FILE"`
	
	// Platform configurations (dynamically loaded)
	Platforms map[string]*PlatformConfig `json:"-"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	var cfg Config
	err := envconfig.Process("", &cfg)
	if err != nil {
		return nil, err
	}
	
	// Initialize platform configurations
	cfg.Platforms = make(map[string]*PlatformConfig)
	
	// Load built-in platform configurations
	if err := cfg.loadBuiltinPlatforms(); err != nil {
		return nil, fmt.Errorf("failed to load builtin platforms: %w", err)
	}
	
	// Load additional platforms from environment
	if err := cfg.loadDynamicPlatforms(); err != nil {
		return nil, fmt.Errorf("failed to load dynamic platforms: %w", err)
	}
	
	return &cfg, nil
}

// loadBuiltinPlatforms loads configuration for known platforms
func (c *Config) loadBuiltinPlatforms() error {
	// Spotify configuration
	if c.SpotifyClientID != "" && c.SpotifyClientSecret != "" {
		c.Platforms["spotify"] = &PlatformConfig{
			Name:         "spotify",
			Enabled:      true,
			AuthMethod:   AuthMethodOAuth2,
			ClientID:     c.SpotifyClientID,
			ClientSecret: c.SpotifyClientSecret,
			TokenURL:     "https://accounts.spotify.com/api/token",
			BaseURL:      "https://api.spotify.com/v1",
			RateLimit:    100, // requests per minute
			Timeout:      10,  // seconds
		}
	}
	
	// Apple Music configuration
	if c.AppleMusicKeyID != "" && c.AppleMusicTeamID != "" && c.AppleMusicKeyFile != "" {
		c.Platforms["apple_music"] = &PlatformConfig{
			Name:       "apple_music",
			Enabled:    true,
			AuthMethod: AuthMethodJWT,
			KeyID:      c.AppleMusicKeyID,
			TeamID:     c.AppleMusicTeamID,
			KeyFile:    c.AppleMusicKeyFile,
			BaseURL:    "https://api.music.apple.com/v1",
			RateLimit:  120, // requests per minute
			Timeout:    10,  // seconds
		}
	}
	
	return nil
}

// loadDynamicPlatforms loads platform configurations from environment variables
// Format: PLATFORM_<PLATFORM_NAME>_<CONFIG_KEY>=value
func (c *Config) loadDynamicPlatforms() error {
	// This would be implemented to scan environment variables for platform configs
	// For now, it's a placeholder for future extensibility
	return nil
}

// GetPlatformConfig returns configuration for a specific platform
func (c *Config) GetPlatformConfig(platform string) (*PlatformConfig, bool) {
	config, exists := c.Platforms[platform]
	return config, exists
}

// GetEnabledPlatforms returns a list of enabled platform names
func (c *Config) GetEnabledPlatforms() []string {
	var platforms []string
	for name, config := range c.Platforms {
		if config.Enabled {
			platforms = append(platforms, name)
		}
	}
	return platforms
}

// ValidatePlatformConfig validates a platform configuration
func ValidatePlatformConfig(config *PlatformConfig) error {
	if config.Name == "" {
		return fmt.Errorf("platform name cannot be empty")
	}
	
	switch config.AuthMethod {
	case AuthMethodOAuth2:
		if config.ClientID == "" || config.ClientSecret == "" {
			return fmt.Errorf("OAuth2 requires client_id and client_secret")
		}
	case AuthMethodJWT:
		if config.KeyID == "" || config.TeamID == "" {
			return fmt.Errorf("JWT requires key_id and team_id")
		}
	case AuthMethodAPIKey:
		if config.APIKey == "" {
			return fmt.Errorf("API key authentication requires api_key")
		}
	case AuthMethodBasicAuth:
		if config.Username == "" || config.Password == "" {
			return fmt.Errorf("Basic auth requires username and password")
		}
	default:
		return fmt.Errorf("unsupported auth method: %s", config.AuthMethod)
	}
	
	if config.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	
	return nil
}

// RegisterPlatformConfig registers a new platform configuration
func (c *Config) RegisterPlatformConfig(platform string, config *PlatformConfig) error {
	if err := ValidatePlatformConfig(config); err != nil {
		return fmt.Errorf("invalid platform config for %s: %w", platform, err)
	}
	
	config.Name = platform
	c.Platforms[platform] = config
	return nil
}

// IsEnabled checks if a platform is enabled
func (c *Config) IsEnabled(platform string) bool {
	config, exists := c.GetPlatformConfig(platform)
	return exists && config.Enabled
}

// GetAuthConfig returns authentication configuration for a platform
func (c *Config) GetAuthConfig(platform string) (map[string]string, error) {
	config, exists := c.GetPlatformConfig(platform)
	if !exists {
		return nil, fmt.Errorf("platform %s not configured", platform)
	}
	
	auth := make(map[string]string)
	auth["method"] = string(config.AuthMethod)
	
	switch config.AuthMethod {
	case AuthMethodOAuth2:
		auth["client_id"] = config.ClientID
		auth["client_secret"] = config.ClientSecret
		auth["token_url"] = config.TokenURL
	case AuthMethodJWT:
		auth["key_id"] = config.KeyID
		auth["team_id"] = config.TeamID
		auth["key_file"] = config.KeyFile
	case AuthMethodAPIKey:
		auth["api_key"] = config.APIKey
		if config.APISecret != "" {
			auth["api_secret"] = config.APISecret
		}
	case AuthMethodBasicAuth:
		auth["username"] = config.Username
		auth["password"] = config.Password
	}
	
	return auth, nil
}

// ConfigFromEnvironment creates a platform config from environment variables
// using a standardized naming convention: PLATFORM_<NAME>_<KEY>
func ConfigFromEnvironment(platformName string) (*PlatformConfig, error) {
	prefix := fmt.Sprintf("PLATFORM_%s", strings.ToUpper(platformName))
	
	var envConfig struct {
		Enabled    bool   `envconfig:"ENABLED" default:"false"`
		AuthMethod string `envconfig:"AUTH_METHOD" default:"api_key"`
		
		ClientID     string `envconfig:"CLIENT_ID"`
		ClientSecret string `envconfig:"CLIENT_SECRET"`
		TokenURL     string `envconfig:"TOKEN_URL"`
		
		KeyID   string `envconfig:"KEY_ID"`
		TeamID  string `envconfig:"TEAM_ID"`
		KeyFile string `envconfig:"KEY_FILE"`
		
		APIKey    string `envconfig:"API_KEY"`
		APISecret string `envconfig:"API_SECRET"`
		
		Username string `envconfig:"USERNAME"`
		Password string `envconfig:"PASSWORD"`
		
		BaseURL   string `envconfig:"BASE_URL"`
		RateLimit int    `envconfig:"RATE_LIMIT" default:"60"`
		Timeout   int    `envconfig:"TIMEOUT" default:"10"`
	}
	
	if err := envconfig.Process(prefix, &envConfig); err != nil {
		return nil, err
	}
	
	if !envConfig.Enabled {
		return nil, nil // Platform not enabled
	}
	
	config := &PlatformConfig{
		Name:         platformName,
		Enabled:      envConfig.Enabled,
		AuthMethod:   AuthMethod(envConfig.AuthMethod),
		ClientID:     envConfig.ClientID,
		ClientSecret: envConfig.ClientSecret,
		TokenURL:     envConfig.TokenURL,
		KeyID:        envConfig.KeyID,
		TeamID:       envConfig.TeamID,
		KeyFile:      envConfig.KeyFile,
		APIKey:       envConfig.APIKey,
		APISecret:    envConfig.APISecret,
		Username:     envConfig.Username,
		Password:     envConfig.Password,
		BaseURL:      envConfig.BaseURL,
		RateLimit:    envConfig.RateLimit,
		Timeout:      envConfig.Timeout,
	}
	
	return config, ValidatePlatformConfig(config)
}