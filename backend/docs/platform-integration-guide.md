# Platform Integration Guide

This guide walks you through adding a new music streaming platform to the songshare application.

## Overview

The songshare platform system is designed around the `PlatformService` interface, which provides a standardized way to integrate with different music streaming services. Each platform service handles URL parsing, API communication, search functionality, and metadata extraction.

## Prerequisites

Before adding a new platform, ensure you have:
- API credentials for the target platform
- Documentation for their REST API
- Understanding of their URL structure
- Knowledge of their authentication method (OAuth2, JWT, API Key, etc.)

## Step-by-Step Integration

### 1. Create the Service File

Create a new file: `internal/services/{platform}_service.go`

```go
package services

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/go-resty/resty/v2"
    "songshare/internal/cache"
)

// {platform}Service implements PlatformService for {Platform Name}
type {platform}Service struct {
    client      *resty.Client
    apiKey      string  // or other auth credentials
    cache       cache.Cache
    mu          sync.RWMutex
}

const (
    {platform}APIURL = "https://api.{platform}.com/v1"
)

// Cache TTL constants
const (
    {platform}TrackCacheTTL  = 4 * time.Hour
    {platform}SearchCacheTTL = 2 * time.Hour
    {platform}ISRCCacheTTL   = 24 * time.Hour
)

// New{Platform}Service creates a new {Platform} service
func New{Platform}Service(apiKey string, cache cache.Cache) PlatformService {
    client := resty.New().
        SetTimeout(10*time.Second).
        SetRetryCount(3).
        SetRetryWaitTime(1*time.Second).
        SetRetryMaxWaitTime(5*time.Second)

    return &{platform}Service{
        client: client,
        apiKey: apiKey,
        cache:  cache,
    }
}

// Implement all PlatformService interface methods...
```

### 2. Implement Required Methods

#### GetPlatformName()
```go
func (s *{platform}Service) GetPlatformName() string {
    return "{platform}"
}
```

#### ParseURL()
```go
func (s *{platform}Service) ParseURL(url string) (*TrackInfo, error) {
    // Extract track ID using regex
    matches := {Platform}URLPattern.Regex.FindStringSubmatch(url)
    if len(matches) <= {Platform}URLPattern.TrackIDIndex {
        return nil, &PlatformError{
            Platform:  "{platform}",
            Operation: "parse_url",
            Message:   "invalid {Platform} URL format",
            URL:       url,
        }
    }

    trackID := matches[{Platform}URLPattern.TrackIDIndex]
    
    return &TrackInfo{
        Platform:   "{platform}",
        ExternalID: trackID,
        URL:        s.BuildURL(trackID),
        Available:  true,
    }, nil
}
```

#### GetTrackByID()
```go
func (s *{platform}Service) GetTrackByID(ctx context.Context, trackID string) (*TrackInfo, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("api:{platform}:track:%s", trackID)
    if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
        var trackInfo TrackInfo
        if err := json.Unmarshal(cached, &trackInfo); err == nil {
            return &trackInfo, nil
        }
    }

    // Make API request
    var apiTrack {Platform}Track
    resp, err := s.client.R().
        SetContext(ctx).
        SetHeader("Authorization", "Bearer "+s.apiKey).
        SetResult(&apiTrack).
        Get(fmt.Sprintf("%s/tracks/%s", {platform}APIURL, trackID))

    if err != nil {
        return nil, &PlatformError{
            Platform:  "{platform}",
            Operation: "get_track",
            Message:   "request failed",
            Err:       err,
        }
    }

    if resp.StatusCode() == http.StatusNotFound {
        return nil, &PlatformError{
            Platform:  "{platform}",
            Operation: "get_track",
            Message:   "track not found",
        }
    }

    if resp.StatusCode() != http.StatusOK {
        return nil, &PlatformError{
            Platform:  "{platform}",
            Operation: "get_track",
            Message:   fmt.Sprintf("API returned status %d", resp.StatusCode()),
        }
    }

    trackInfo := s.convert{Platform}Track(&apiTrack)
    
    // Cache the result
    if data, err := json.Marshal(trackInfo); err == nil {
        if err := s.cache.Set(ctx, cacheKey, data, {platform}TrackCacheTTL); err != nil {
            slog.Error("Failed to cache {platform} track", "trackID", trackID, "error", err)
        }
    }
    
    return trackInfo, nil
}
```

#### SearchTrack()
```go
func (s *{platform}Service) SearchTrack(ctx context.Context, query SearchQuery) ([]*TrackInfo, error) {
    searchQuery := s.buildSearchQuery(query)
    limit := query.Limit
    if limit == 0 {
        limit = 10
    }
    if limit > 50 { // Adjust based on platform limits
        limit = 50
    }

    // Check cache first
    cacheKey := fmt.Sprintf("api:{platform}:search:%s:limit:%d", searchQuery, limit)
    if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
        var tracks []*TrackInfo
        if err := json.Unmarshal(cached, &tracks); err == nil {
            return tracks, nil
        }
    }

    // Make search request
    var searchResult {Platform}SearchResult
    resp, err := s.client.R().
        SetContext(ctx).
        SetHeader("Authorization", "Bearer "+s.apiKey).
        SetQueryParams(map[string]string{
            "q":     searchQuery,
            "type":  "track",
            "limit": fmt.Sprintf("%d", limit),
        }).
        SetResult(&searchResult).
        Get(fmt.Sprintf("%s/search", {platform}APIURL))

    // Handle response and convert to TrackInfo...
    // Cache results...
    
    return tracks, nil
}
```

#### GetTrackByISRC()
```go
func (s *{platform}Service) GetTrackByISRC(ctx context.Context, isrc string) (*TrackInfo, error) {
    query := SearchQuery{
        ISRC:  isrc,
        Limit: 1,
    }
    
    tracks, err := s.SearchTrack(ctx, query)
    if err != nil {
        return nil, err
    }

    if len(tracks) == 0 {
        return nil, &PlatformError{
            Platform:  "{platform}",
            Operation: "get_by_isrc",
            Message:   "no tracks found with ISRC " + isrc,
        }
    }

    return tracks[0], nil
}
```

#### BuildURL()
```go
func (s *{platform}Service) BuildURL(trackID string) string {
    return fmt.Sprintf("https://{platform}.com/track/%s", trackID)
}
```

#### Health()
```go
func (s *{platform}Service) Health(ctx context.Context) error {
    if s.apiKey == "" {
        return &PlatformError{
            Platform:  "{platform}",
            Operation: "health",
            Message:   "missing {Platform} API credentials",
        }
    }

    // Test API connectivity with a simple request
    _, err := s.client.R().
        SetContext(ctx).
        SetHeader("Authorization", "Bearer "+s.apiKey).
        Get(fmt.Sprintf("%s/me", {platform}APIURL)) // or similar endpoint

    if err != nil {
        return &PlatformError{
            Platform:  "{platform}",
            Operation: "health",
            Message:   "API health check failed",
            Err:       err,
        }
    }

    return nil
}
```

### 3. Add URL Pattern

In `internal/services/platform_service.go`, add your platform's URL pattern:

```go
var (
    // ... existing patterns ...
    
    {Platform}URLPattern = URLPattern{
        Regex:        regexp.MustCompile(`(?:https?://)?(?:www\.)?{platform}\.com/track/([a-zA-Z0-9]+)`),
        Platform:     "{platform}",
        TrackIDIndex: 1,
    }
)
```

Update the `ParsePlatformURL` function to include your pattern:

```go
func ParsePlatformURL(url string) (platform string, trackID string, err error) {
    patterns := []URLPattern{SpotifyURLPattern, AppleMusicURLPattern, {Platform}URLPattern}
    // ... rest of function
}
```

### 4. Add Configuration

In `internal/config/config.go`, add environment variables for your platform:

```go
type Config struct {
    // ... existing fields ...
    {Platform}APIKey string `envconfig:"{PLATFORM}_API_KEY"`
}
```

Update `.env.example`:

```bash
# {Platform} API Credentials
{PLATFORM}_API_KEY=your_{platform}_api_key
```

### 5. Register the Service

In `cmd/server/main.go`, add your service registration:

```go
// Initialize platform services
spotifyService := services.NewSpotifyService(cfg.SpotifyClientID, cfg.SpotifyClientSecret, cache)
appleMusicService := services.NewAppleMusicService(cfg.AppleMusicKeyID, cfg.AppleMusicTeamID, cfg.AppleMusicKeyFile, cache)
{platform}Service := services.New{Platform}Service(cfg.{Platform}APIKey, cache)

// Register platforms
resolutionService.RegisterPlatform(spotifyService)
resolutionService.RegisterPlatform(appleMusicService)
resolutionService.RegisterPlatform({platform}Service)
```

### 6. Add UI Support

#### Add Platform Icon

In `internal/handlers/songs.go`, update the badge rendering logic:

```go
// Around line 1366 in search results badge logic
if platform.Platform == "apple_music" {
    platformIcon = `<img src="https://upload.wikimedia.org/wikipedia/commons/5/5f/Apple_Music_icon.svg" alt="" class="platform-badge-icon" aria-hidden="true">`
    platformName = "Apple Music"
} else if platform.Platform == "spotify" {
    platformIcon = `<img src="https://upload.wikimedia.org/wikipedia/commons/8/84/Spotify_icon.svg" alt="" class="platform-badge-icon" aria-hidden="true">`
    platformName = "Spotify"
} else if platform.Platform == "{platform}" {
    platformIcon = `<img src="URL_TO_{PLATFORM}_ICON" alt="" class="platform-badge-icon" aria-hidden="true">`
    platformName = "{Platform Name}"
}
```

#### Add Share Page Button

In the share page template (around line 384):

```go
{{if eq $platform "{platform}"}}
    <img src="URL_TO_{PLATFORM}_ICON" alt="" class="platform-icon" aria-hidden="true">
    Open in {Platform Name}
{{end}}
```

### 7. Create Tests

Create `internal/services/{platform}_service_test.go`:

```go
package services

import (
    "context"
    "testing"

    "songshare/internal/cache"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func Test{Platform}Service_GetPlatformName(t *testing.T) {
    cache := cache.NewInMemoryCache(100)
    service := New{Platform}Service("test-key", cache)
    
    assert.Equal(t, "{platform}", service.GetPlatformName())
}

func Test{Platform}Service_ParseURL(t *testing.T) {
    cache := cache.NewInMemoryCache(100)
    service := New{Platform}Service("test-key", cache)
    
    tests := []struct {
        name     string
        url      string
        expected string
        wantErr  bool
    }{
        {
            name:     "valid {platform} URL",
            url:      "https://{platform}.com/track/abc123",
            expected: "abc123",
            wantErr:  false,
        },
        {
            name:    "invalid URL",
            url:     "https://example.com/song/123",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            track, err := service.ParseURL(tt.url)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.expected, track.ExternalID)
                assert.Equal(t, "{platform}", track.Platform)
            }
        })
    }
}

// Add more tests for other methods...
```

## Authentication Patterns

### OAuth2 (like Spotify)
```go
import "golang.org/x/oauth2/clientcredentials"

tokenSource := &clientcredentials.Config{
    ClientID:     clientID,
    ClientSecret: clientSecret,
    TokenURL:     tokenURL,
}
```

### JWT (like Apple Music)
```go
import "github.com/golang-jwt/jwt/v5"

claims := jwt.MapClaims{
    "iss": teamID,
    "iat": now.Unix(),
    "exp": now.Add(60 * time.Minute).Unix(),
}

token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
```

### API Key
```go
.SetHeader("Authorization", "Bearer " + apiKey)
// or
.SetHeader("X-API-Key", apiKey)
```

## Best Practices

### 1. Error Handling
Always use `PlatformError` for consistent error reporting:

```go
return nil, &PlatformError{
    Platform:  "your_platform",
    Operation: "operation_name",
    Message:   "descriptive error message",
    URL:       url,      // if URL-related
    Err:       err,      // if wrapping another error
}
```

### 2. Caching Strategy
- Use different TTLs for different operations
- ISRC searches should have longer cache TTL (24h)
- Regular searches can be shorter (2h)
- Individual track lookups moderate (4h)

### 3. Rate Limiting
- Implement backoff strategies using resty's retry mechanism
- Respect platform API rate limits
- Use appropriate timeouts

### 4. Data Conversion
- Always populate ISRC when available (critical for cross-platform matching)
- Handle missing fields gracefully
- Normalize artist names consistently
- Use medium-quality images (300-640px) when available

### 5. Testing
- Mock external API calls in tests
- Test URL parsing edge cases  
- Test error conditions
- Verify caching behavior

## Common Pitfalls

1. **Missing ISRC Support**: This breaks cross-platform song matching
2. **Inconsistent Artist Names**: Can cause duplicate entries
3. **Poor Error Handling**: Makes debugging difficult
4. **No Caching**: Causes unnecessary API calls and slower response times
5. **Hardcoded Values**: Makes configuration management difficult

## Platform-Specific Considerations

### YouTube Music
- Uses video IDs for tracks
- Has complex URL patterns (video vs music)
- Good ISRC support
- Rich metadata available

### Deezer
- Straightforward REST API
- Good ISRC support
- Album artwork available
- Clear rate limits

### Tidal
- High-quality focus
- Good metadata
- ISRC support varies
- More complex authentication

### SoundCloud
- User-uploaded content model
- Limited ISRC availability
- Different URL patterns for users vs official tracks
- OAuth2 authentication required

## Getting Help

1. Check existing platform implementations in `internal/services/`
2. Review the `PlatformService` interface documentation
3. Look at test files for usage examples
4. Check the main application setup in `cmd/server/main.go`

Remember: The goal is to provide users with a seamless experience to find their music across all supported platforms!