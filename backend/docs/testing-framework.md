# Platform Service Testing Framework

This document describes the comprehensive testing framework for platform services in the songshare application.

## Overview

The testing framework provides standardized test suites for platform services, ensuring consistent testing across all music streaming platform integrations. It supports both mock testing (for development) and integration testing (with real API credentials).

## Components

### 1. PlatformServiceTestSuite

The main test suite that provides comprehensive testing for platform services.

```go
type PlatformServiceTestSuite struct {
    Service      services.PlatformService
    PlatformName string
    TestTrackID  string
    TestURL      string
    TestISRC     string
    TestQueries  []TestQuery
    URLPatterns  []URLTestCase
    SkipISRC     bool
    SkipSearch   bool
}
```

#### Key Features:
- **Full API Testing**: Tests all PlatformService interface methods
- **Error Handling**: Verifies proper error handling and error types
- **Caching Verification**: Tests that caching is working properly
- **Flexible Configuration**: Skip features not supported by specific platforms

### 2. MockPlatformServiceTestSuite

Lightweight test suite that runs without requiring API credentials.

```go
type MockPlatformServiceTestSuite struct {
    Service      services.PlatformService
    PlatformName string
    URLPatterns  []URLTestCase
}
```

Perfect for:
- CI/CD pipelines without API credentials
- Local development
- Quick validation of URL parsing logic

### 3. PlatformServiceBenchmarkSuite

Performance benchmarking suite for platform services.

```go
type PlatformServiceBenchmarkSuite struct {
    Service     services.PlatformService
    TestTrackID string
    TestURL     string
    TestQuery   services.SearchQuery
}
```

## Usage Examples

### Basic Integration Test

```go
func TestMyPlatformServiceIntegration(t *testing.T) {
    // Check for credentials
    apiKey := os.Getenv("TEST_MYPLATFORM_API_KEY")
    if apiKey == "" {
        // Fall back to mock testing
        cache := testutil.CreateTestCache()
        service := NewMyPlatformService("fake-key", cache)
        
        mockSuite := &testutil.MockPlatformServiceTestSuite{
            Service:      service,
            PlatformName: "myplatform",
            URLPatterns: testutil.GenerateCommonURLTests(
                "myplatform",
                "https://myplatform.com/track",
                "abc123",
            ),
        }
        
        mockSuite.RunMockTestSuite(t)
        return
    }
    
    // Run full integration tests
    cache := testutil.CreateTestCache()
    service := NewMyPlatformService(apiKey, cache)
    
    suite := &testutil.PlatformServiceTestSuite{
        Service:      service,
        PlatformName: "myplatform",
        TestTrackID:  "abc123",
        TestURL:      "https://myplatform.com/track/abc123",
        TestISRC:     "USMYP0123456",
        TestQueries:  testutil.GenerateCommonSearchTests(),
        URLPatterns:  testutil.GenerateCommonURLTests(
            "myplatform",
            "https://myplatform.com/track",
            "abc123",
        ),
        SkipISRC:   false, // Set to true if platform doesn't support ISRC
        SkipSearch: false, // Set to true if search isn't implemented
    }
    
    suite.RunFullTestSuite(t)
}
```

### Custom URL Pattern Testing

```go
urlPatterns := []testutil.URLTestCase{
    {
        Name:        "Standard platform URL",
        URL:         "https://myplatform.com/track/abc123",
        ShouldMatch: true,
        ExpectedID:  "abc123",
    },
    {
        Name:        "URL with query parameters",
        URL:         "https://myplatform.com/track/abc123?ref=share",
        ShouldMatch: true,
        ExpectedID:  "abc123",
    },
    {
        Name:        "Invalid URL format",
        URL:         "https://myplatform.com/album/xyz789",
        ShouldMatch: false,
    },
}
```

### Custom Search Queries

```go
testQueries := []testutil.TestQuery{
    {
        Name: "Search by artist and title",
        Query: services.SearchQuery{
            Title:  "Bohemian Rhapsody",
            Artist: "Queen",
            Limit:  10,
        },
        Expected: testutil.ExpectedResult{
            ShouldFind:  true,
            MinResults:  1,
            TrackTitle:  "Bohemian Rhapsody",
            TrackArtist: "Queen",
        },
    },
    {
        Name: "Search by ISRC",
        Query: services.SearchQuery{
            ISRC:  "GBUM71507208",
            Limit: 1,
        },
        Expected: testutil.ExpectedResult{
            ShouldFind: true,
            MinResults: 1,
        },
    },
}
```

### Performance Benchmarking

```go
func BenchmarkMyPlatformService(b *testing.B) {
    apiKey := os.Getenv("TEST_MYPLATFORM_API_KEY")
    if apiKey == "" {
        b.Skip("Skipping benchmarks - credentials not provided")
    }
    
    cache := testutil.CreateTestCache()
    service := NewMyPlatformService(apiKey, cache)
    
    suite := &testutil.PlatformServiceBenchmarkSuite{
        Service:     service,
        TestTrackID: "abc123",
        TestURL:     "https://myplatform.com/track/abc123",
        TestQuery: services.SearchQuery{
            Title:  "Bohemian Rhapsody",
            Artist: "Queen",
            Limit:  5,
        },
    }
    
    suite.RunBenchmarks(b)
}
```

## Test Categories

### 1. Interface Compliance Tests
- `TestGetPlatformName`: Verifies platform name is correct
- `TestHealth`: Tests health check functionality
- `TestParseURL`: Tests URL parsing with various formats
- `TestBuildURL`: Tests URL construction

### 2. API Integration Tests
- `TestGetTrackByID`: Tests fetching track by platform ID
- `TestGetTrackByISRC`: Tests ISRC-based track lookup
- `TestSearchTrack`: Tests search functionality

### 3. Error Handling Tests
- Invalid track IDs
- Invalid ISRCs
- Empty searches
- Network error simulation

### 4. Performance Tests
- URL parsing performance
- API call performance
- Cache hit/miss ratios

## Environment Variables

For integration testing, set these environment variables:

```bash
# Spotify
TEST_SPOTIFY_CLIENT_ID=your_client_id
TEST_SPOTIFY_CLIENT_SECRET=your_client_secret

# Apple Music
TEST_APPLE_MUSIC_KEY_ID=your_key_id
TEST_APPLE_MUSIC_TEAM_ID=your_team_id
TEST_APPLE_MUSIC_KEY_FILE=path/to/AuthKey_KEYID.p8

# Your Platform
TEST_MYPLATFORM_API_KEY=your_api_key
```

## Running Tests

### Run All Platform Tests
```bash
go test ./internal/services -v
```

### Run Only Mock Tests (No API Calls)
```bash
go test ./internal/services -v -short
```

### Run Integration Tests with API Calls
```bash
# Set environment variables first
export TEST_SPOTIFY_CLIENT_ID=...
export TEST_SPOTIFY_CLIENT_SECRET=...

go test ./internal/services -v -run="Integration"
```

### Run Benchmarks
```bash
# Set environment variables first
go test ./internal/services -bench=. -benchmem
```

## Best Practices

### 1. Test Data Selection
- Use well-known tracks that are unlikely to be removed (e.g., "Bohemian Rhapsody" by Queen)
- Use tracks with known ISRCs for ISRC testing
- Include edge cases in URL patterns

### 2. Error Handling
- Always test invalid inputs
- Verify error types match expected PlatformError
- Test network failure scenarios where possible

### 3. Platform-Specific Considerations
- Some platforms may not support ISRC lookup
- Search functionality may vary between platforms
- URL patterns can be complex (multiple formats, country codes, etc.)

### 4. CI/CD Integration
- Mock tests should always pass in CI
- Integration tests should only run when credentials are available
- Use separate credentials for testing vs production

## Common Patterns

### Graceful Degradation
```go
if !hasCredentials {
    t.Log("Running mock tests - no credentials provided")
    runMockTests()
    return
}
runIntegrationTests()
```

### Skip Unsupported Features
```go
suite := &testutil.PlatformServiceTestSuite{
    // ... other config
    SkipISRC:   true,  // Platform doesn't support ISRC
    SkipSearch: false, // Platform supports search
}
```

### Custom Test Helpers
```go
// Create platform-specific URL patterns
func generateSpotifyURLTests() []testutil.URLTestCase {
    base := testutil.GenerateCommonURLTests("spotify", "https://open.spotify.com/track", "4iV5W9uYEdYUVa79Axb7Rh")
    
    // Add Spotify-specific patterns
    base = append(base, testutil.URLTestCase{
        Name:        "Spotify with si parameter",
        URL:         "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh?si=abc123",
        ShouldMatch: true,
        ExpectedID:  "4iV5W9uYEdYUVa79Axb7Rh",
    })
    
    return base
}
```

## Extending the Framework

### Adding New Test Types

1. **Create new test method in PlatformServiceTestSuite:**
```go
func (suite *PlatformServiceTestSuite) TestNewFeature(t *testing.T) {
    // Test implementation
}
```

2. **Add to RunFullTestSuite:**
```go
func (suite *PlatformServiceTestSuite) RunFullTestSuite(t *testing.T) {
    // ... existing tests
    t.Run("NewFeature", suite.TestNewFeature)
}
```

### Platform-Specific Extensions

Create platform-specific test suites that embed the base suite:

```go
type SpotifyTestSuite struct {
    *testutil.PlatformServiceTestSuite
}

func (s *SpotifyTestSuite) TestSpotifySpecificFeature(t *testing.T) {
    // Spotify-specific tests
}
```

This framework ensures consistent, comprehensive testing across all platform services while being flexible enough to accommodate platform-specific requirements.