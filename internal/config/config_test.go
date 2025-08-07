package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Set required environment variables
	os.Setenv("MONGODB_URL", "mongodb://test:test@localhost:27017/test")
	os.Setenv("VALKEY_URL", "valkey://localhost:6379")
	defer func() {
		os.Unsetenv("MONGODB_URL")
		os.Unsetenv("VALKEY_URL")
	}()

	cfg, err := Load()
	require.NoError(t, err)
	
	assert.Equal(t, "8080", cfg.Port) // default value
	assert.Equal(t, "debug", cfg.GinMode) // default value
	assert.Equal(t, "mongodb://test:test@localhost:27017/test", cfg.MongodbURL)
	assert.Equal(t, "valkey://localhost:6379", cfg.ValkeyURL)
	assert.NotNil(t, cfg.Platforms)
}

func TestLoadBuiltinPlatforms(t *testing.T) {
	tests := []struct {
		name       string
		setupEnv   func()
		cleanupEnv func()
		verify     func(t *testing.T, cfg *Config)
	}{
		{
			name: "Spotify configuration",
			setupEnv: func() {
				os.Setenv("SPOTIFY_CLIENT_ID", "test-client-id")
				os.Setenv("SPOTIFY_CLIENT_SECRET", "test-client-secret")
				os.Setenv("MONGODB_URL", "mongodb://test:test@localhost:27017/test")
				os.Setenv("VALKEY_URL", "valkey://localhost:6379")
			},
			cleanupEnv: func() {
				os.Unsetenv("SPOTIFY_CLIENT_ID")
				os.Unsetenv("SPOTIFY_CLIENT_SECRET")
				os.Unsetenv("MONGODB_URL")
				os.Unsetenv("VALKEY_URL")
			},
			verify: func(t *testing.T, cfg *Config) {
				spotifyConfig, exists := cfg.GetPlatformConfig("spotify")
				require.True(t, exists)
				assert.Equal(t, "spotify", spotifyConfig.Name)
				assert.True(t, spotifyConfig.Enabled)
				assert.Equal(t, AuthMethodOAuth2, spotifyConfig.AuthMethod)
				assert.Equal(t, "test-client-id", spotifyConfig.ClientID)
				assert.Equal(t, "test-client-secret", spotifyConfig.ClientSecret)
				assert.Equal(t, "https://accounts.spotify.com/api/token", spotifyConfig.TokenURL)
			},
		},
		{
			name: "Apple Music configuration",
			setupEnv: func() {
				os.Setenv("APPLE_MUSIC_KEY_ID", "test-key-id")
				os.Setenv("APPLE_MUSIC_TEAM_ID", "test-team-id")
				os.Setenv("APPLE_MUSIC_KEY_FILE", "/path/to/key.p8")
				os.Setenv("MONGODB_URL", "mongodb://test:test@localhost:27017/test")
				os.Setenv("VALKEY_URL", "valkey://localhost:6379")
			},
			cleanupEnv: func() {
				os.Unsetenv("APPLE_MUSIC_KEY_ID")
				os.Unsetenv("APPLE_MUSIC_TEAM_ID")
				os.Unsetenv("APPLE_MUSIC_KEY_FILE")
				os.Unsetenv("MONGODB_URL")
				os.Unsetenv("VALKEY_URL")
			},
			verify: func(t *testing.T, cfg *Config) {
				appleConfig, exists := cfg.GetPlatformConfig("apple_music")
				require.True(t, exists)
				assert.Equal(t, "apple_music", appleConfig.Name)
				assert.True(t, appleConfig.Enabled)
				assert.Equal(t, AuthMethodJWT, appleConfig.AuthMethod)
				assert.Equal(t, "test-key-id", appleConfig.KeyID)
				assert.Equal(t, "test-team-id", appleConfig.TeamID)
				assert.Equal(t, "/path/to/key.p8", appleConfig.KeyFile)
			},
		},
		{
			name: "No platform configuration",
			setupEnv: func() {
				os.Setenv("MONGODB_URL", "mongodb://test:test@localhost:27017/test")
				os.Setenv("VALKEY_URL", "valkey://localhost:6379")
			},
			cleanupEnv: func() {
				os.Unsetenv("MONGODB_URL")
				os.Unsetenv("VALKEY_URL")
			},
			verify: func(t *testing.T, cfg *Config) {
				assert.Empty(t, cfg.Platforms)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer tt.cleanupEnv()

			cfg, err := Load()
			require.NoError(t, err)
			tt.verify(t, cfg)
		})
	}
}

func TestValidatePlatformConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *PlatformConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid OAuth2 config",
			config: &PlatformConfig{
				Name:         "test_oauth2",
				AuthMethod:   AuthMethodOAuth2,
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				BaseURL:      "https://api.test.com",
			},
			wantErr: false,
		},
		{
			name: "valid JWT config",
			config: &PlatformConfig{
				Name:       "test_jwt",
				AuthMethod: AuthMethodJWT,
				KeyID:      "key-id",
				TeamID:     "team-id",
				BaseURL:    "https://api.test.com",
			},
			wantErr: false,
		},
		{
			name: "valid API key config",
			config: &PlatformConfig{
				Name:       "test_apikey",
				AuthMethod: AuthMethodAPIKey,
				APIKey:     "api-key",
				BaseURL:    "https://api.test.com",
			},
			wantErr: false,
		},
		{
			name: "valid basic auth config",
			config: &PlatformConfig{
				Name:       "test_basic",
				AuthMethod: AuthMethodBasicAuth,
				Username:   "username",
				Password:   "password",
				BaseURL:    "https://api.test.com",
			},
			wantErr: false,
		},
		{
			name: "missing platform name",
			config: &PlatformConfig{
				AuthMethod: AuthMethodAPIKey,
				APIKey:     "api-key",
				BaseURL:    "https://api.test.com",
			},
			wantErr: true,
			errMsg:  "platform name cannot be empty",
		},
		{
			name: "OAuth2 missing client_id",
			config: &PlatformConfig{
				Name:         "test_oauth2",
				AuthMethod:   AuthMethodOAuth2,
				ClientSecret: "client-secret",
				BaseURL:      "https://api.test.com",
			},
			wantErr: true,
			errMsg:  "OAuth2 requires client_id and client_secret",
		},
		{
			name: "JWT missing team_id",
			config: &PlatformConfig{
				Name:       "test_jwt",
				AuthMethod: AuthMethodJWT,
				KeyID:      "key-id",
				BaseURL:    "https://api.test.com",
			},
			wantErr: true,
			errMsg:  "JWT requires key_id and team_id",
		},
		{
			name: "API key missing key",
			config: &PlatformConfig{
				Name:       "test_apikey",
				AuthMethod: AuthMethodAPIKey,
				BaseURL:    "https://api.test.com",
			},
			wantErr: true,
			errMsg:  "API key authentication requires api_key",
		},
		{
			name: "Basic auth missing password",
			config: &PlatformConfig{
				Name:       "test_basic",
				AuthMethod: AuthMethodBasicAuth,
				Username:   "username",
				BaseURL:    "https://api.test.com",
			},
			wantErr: true,
			errMsg:  "Basic auth requires username and password",
		},
		{
			name: "unsupported auth method",
			config: &PlatformConfig{
				Name:       "test_unknown",
				AuthMethod: "unknown_method",
				BaseURL:    "https://api.test.com",
			},
			wantErr: true,
			errMsg:  "unsupported auth method",
		},
		{
			name: "missing base URL",
			config: &PlatformConfig{
				Name:       "test_no_url",
				AuthMethod: AuthMethodAPIKey,
				APIKey:     "api-key",
			},
			wantErr: true,
			errMsg:  "base_url is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePlatformConfig(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_RegisterPlatformConfig(t *testing.T) {
	cfg := &Config{
		Platforms: make(map[string]*PlatformConfig),
	}

	// Test successful registration
	config := &PlatformConfig{
		Name:       "test_platform",
		AuthMethod: AuthMethodAPIKey,
		APIKey:     "test-key",
		BaseURL:    "https://api.test.com",
	}

	err := cfg.RegisterPlatformConfig("test_platform", config)
	assert.NoError(t, err)

	registered, exists := cfg.GetPlatformConfig("test_platform")
	assert.True(t, exists)
	assert.Equal(t, "test_platform", registered.Name)

	// Test registration with invalid config
	invalidConfig := &PlatformConfig{
		Name:       "invalid_platform",
		AuthMethod: AuthMethodOAuth2,
		// Missing required fields
		BaseURL: "https://api.test.com",
	}

	err = cfg.RegisterPlatformConfig("invalid_platform", invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid platform config")
}

func TestConfig_GetEnabledPlatforms(t *testing.T) {
	cfg := &Config{
		Platforms: map[string]*PlatformConfig{
			"enabled_platform": {
				Name:    "enabled_platform",
				Enabled: true,
			},
			"disabled_platform": {
				Name:    "disabled_platform",
				Enabled: false,
			},
			"another_enabled": {
				Name:    "another_enabled",
				Enabled: true,
			},
		},
	}

	enabled := cfg.GetEnabledPlatforms()
	assert.Len(t, enabled, 2)
	assert.Contains(t, enabled, "enabled_platform")
	assert.Contains(t, enabled, "another_enabled")
	assert.NotContains(t, enabled, "disabled_platform")
}

func TestConfig_IsEnabled(t *testing.T) {
	cfg := &Config{
		Platforms: map[string]*PlatformConfig{
			"enabled_platform": {
				Name:    "enabled_platform",
				Enabled: true,
			},
			"disabled_platform": {
				Name:    "disabled_platform",
				Enabled: false,
			},
		},
	}

	assert.True(t, cfg.IsEnabled("enabled_platform"))
	assert.False(t, cfg.IsEnabled("disabled_platform"))
	assert.False(t, cfg.IsEnabled("nonexistent_platform"))
}

func TestConfig_GetAuthConfig(t *testing.T) {
	cfg := &Config{
		Platforms: map[string]*PlatformConfig{
			"oauth2_platform": {
				Name:         "oauth2_platform",
				AuthMethod:   AuthMethodOAuth2,
				ClientID:     "client-id",
				ClientSecret: "client-secret",
				TokenURL:     "https://auth.test.com/token",
			},
			"apikey_platform": {
				Name:       "apikey_platform",
				AuthMethod: AuthMethodAPIKey,
				APIKey:     "api-key",
				APISecret:  "api-secret",
			},
		},
	}

	// Test OAuth2 config
	auth, err := cfg.GetAuthConfig("oauth2_platform")
	require.NoError(t, err)
	assert.Equal(t, "oauth2", auth["method"])
	assert.Equal(t, "client-id", auth["client_id"])
	assert.Equal(t, "client-secret", auth["client_secret"])
	assert.Equal(t, "https://auth.test.com/token", auth["token_url"])

	// Test API key config
	auth, err = cfg.GetAuthConfig("apikey_platform")
	require.NoError(t, err)
	assert.Equal(t, "api_key", auth["method"])
	assert.Equal(t, "api-key", auth["api_key"])
	assert.Equal(t, "api-secret", auth["api_secret"])

	// Test nonexistent platform
	_, err = cfg.GetAuthConfig("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestConfigFromEnvironment(t *testing.T) {
	tests := []struct {
		name         string
		platformName string
		setupEnv     func()
		cleanupEnv   func()
		expectConfig bool
		verify       func(t *testing.T, config *PlatformConfig)
	}{
		{
			name:         "disabled platform",
			platformName: "test_platform",
			setupEnv: func() {
				os.Setenv("PLATFORM_TEST_PLATFORM_ENABLED", "false")
			},
			cleanupEnv: func() {
				os.Unsetenv("PLATFORM_TEST_PLATFORM_ENABLED")
			},
			expectConfig: false,
		},
		{
			name:         "enabled API key platform",
			platformName: "youtube_music",
			setupEnv: func() {
				os.Setenv("PLATFORM_YOUTUBE_MUSIC_ENABLED", "true")
				os.Setenv("PLATFORM_YOUTUBE_MUSIC_AUTH_METHOD", "api_key")
				os.Setenv("PLATFORM_YOUTUBE_MUSIC_API_KEY", "test-api-key")
				os.Setenv("PLATFORM_YOUTUBE_MUSIC_BASE_URL", "https://api.youtube.com/v3")
				os.Setenv("PLATFORM_YOUTUBE_MUSIC_RATE_LIMIT", "100")
				os.Setenv("PLATFORM_YOUTUBE_MUSIC_TIMEOUT", "15")
			},
			cleanupEnv: func() {
				os.Unsetenv("PLATFORM_YOUTUBE_MUSIC_ENABLED")
				os.Unsetenv("PLATFORM_YOUTUBE_MUSIC_AUTH_METHOD")
				os.Unsetenv("PLATFORM_YOUTUBE_MUSIC_API_KEY")
				os.Unsetenv("PLATFORM_YOUTUBE_MUSIC_BASE_URL")
				os.Unsetenv("PLATFORM_YOUTUBE_MUSIC_RATE_LIMIT")
				os.Unsetenv("PLATFORM_YOUTUBE_MUSIC_TIMEOUT")
			},
			expectConfig: true,
			verify: func(t *testing.T, config *PlatformConfig) {
				assert.Equal(t, "youtube_music", config.Name)
				assert.True(t, config.Enabled)
				assert.Equal(t, AuthMethodAPIKey, config.AuthMethod)
				assert.Equal(t, "test-api-key", config.APIKey)
				assert.Equal(t, "https://api.youtube.com/v3", config.BaseURL)
				assert.Equal(t, 100, config.RateLimit)
				assert.Equal(t, 15, config.Timeout)
			},
		},
		{
			name:         "OAuth2 platform",
			platformName: "deezer",
			setupEnv: func() {
				os.Setenv("PLATFORM_DEEZER_ENABLED", "true")
				os.Setenv("PLATFORM_DEEZER_AUTH_METHOD", "oauth2")
				os.Setenv("PLATFORM_DEEZER_CLIENT_ID", "deezer-client-id")
				os.Setenv("PLATFORM_DEEZER_CLIENT_SECRET", "deezer-client-secret")
				os.Setenv("PLATFORM_DEEZER_TOKEN_URL", "https://auth.deezer.com/token")
				os.Setenv("PLATFORM_DEEZER_BASE_URL", "https://api.deezer.com")
			},
			cleanupEnv: func() {
				os.Unsetenv("PLATFORM_DEEZER_ENABLED")
				os.Unsetenv("PLATFORM_DEEZER_AUTH_METHOD")
				os.Unsetenv("PLATFORM_DEEZER_CLIENT_ID")
				os.Unsetenv("PLATFORM_DEEZER_CLIENT_SECRET")
				os.Unsetenv("PLATFORM_DEEZER_TOKEN_URL")
				os.Unsetenv("PLATFORM_DEEZER_BASE_URL")
			},
			expectConfig: true,
			verify: func(t *testing.T, config *PlatformConfig) {
				assert.Equal(t, "deezer", config.Name)
				assert.Equal(t, AuthMethodOAuth2, config.AuthMethod)
				assert.Equal(t, "deezer-client-id", config.ClientID)
				assert.Equal(t, "deezer-client-secret", config.ClientSecret)
				assert.Equal(t, "https://auth.deezer.com/token", config.TokenURL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv()
			defer tt.cleanupEnv()

			config, err := ConfigFromEnvironment(tt.platformName)
			
			if !tt.expectConfig {
				assert.NoError(t, err)
				assert.Nil(t, config)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, config)
			tt.verify(t, config)
		})
	}
}

// Test missing required environment variables
func TestLoad_MissingRequiredEnv(t *testing.T) {
	// Ensure no required env vars are set
	originalMongoDB := os.Getenv("MONGODB_URL")
	originalValkey := os.Getenv("VALKEY_URL")
	
	os.Unsetenv("MONGODB_URL")
	os.Unsetenv("VALKEY_URL")
	
	defer func() {
		if originalMongoDB != "" {
			os.Setenv("MONGODB_URL", originalMongoDB)
		}
		if originalValkey != "" {
			os.Setenv("VALKEY_URL", originalValkey)
		}
	}()

	_, err := Load()
	assert.Error(t, err)
}