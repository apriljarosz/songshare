# Universal Song Catalog - Implementation Plan

## Project Overview
A universal song catalog that indexes music and provides links to songs across all platforms (Spotify, Apple Music, Tidal, etc.). Users can search for songs or paste platform URLs to get unified results with links to all available platforms.

## Confirmed Decisions
- ✅ Build persistent catalog with MongoDB
- ✅ Generate universal shareable links (`/s/{id}`)
- ✅ In-memory caching only (no Redis initially)
- ✅ Start with Spotify + Apple Music
- ✅ ISRC-only matching (no fuzzy matching to avoid false positives)
- ✅ Support text search AND URL resolution
- ✅ No user accounts (public search)
- ✅ **Gin framework** for backend routing

## Tech Stack
- **Backend**: Go with Gin web framework
- **Frontend**: SvelteKit + TypeScript
- **Database**: MongoDB
- **Caching**: In-memory (Go `sync.Map` or custom implementation)
- **Deployment**: Docker Compose

## Implementation Phases

### Phase 1: Backend Core
1. Initialize Go module and install Gin + MongoDB driver
2. Setup project structure (cmd/main.go, internal/handlers, internal/services, internal/models)
3. Configure Gin router with CORS and basic middleware
4. MongoDB connection setup and health check endpoint
5. Create Song model with ISRC, metadata, and platform links
6. Implement simple in-memory cache with TTL

### Phase 2: Platform Integrations
7. Spotify service: search, get track by ID/URL, extract ISRC
8. Apple Music service: search, get song by ID/URL, extract ISRC
9. URL parser to detect platform and extract IDs from URLs
10. Song resolver: query platforms in parallel, match by ISRC, save to MongoDB

### Phase 3: API Endpoints (Gin handlers)
11. `POST /api/search` - Text search across platforms
12. `POST /api/resolve` - Resolve song from platform URL
13. `GET /s/:id` - Universal link (redirect or JSON response)
14. `GET /api/songs/:id` - Get song details by internal ID
15. `GET /api/platforms` - List supported platforms
16. `GET /health` - Health check

### Phase 4: Frontend (SvelteKit)
17. Setup SvelteKit project with TypeScript
18. Create API client service with typed responses
19. Search page with input and real-time results
20. Song card component (album art, title, artist, platform badges)
21. Platform badge component with links
22. Share button for universal links
23. Basic responsive styling

### Phase 5: Deployment & Testing
24. Docker setup (multi-stage build for Go, MongoDB container)
25. docker-compose.yml for local development
26. Environment variable configuration (.env.example)
27. Basic integration tests for API endpoints
28. README with setup instructions

## Key Technical Details

### ISRC Matching Strategy
- Only create unified song records when ISRC matches across platforms
- If ISRC is unavailable from a platform, that platform link won't be included
- This ensures high accuracy and avoids false positives from fuzzy matching

### Caching Strategy
- Cache platform API responses for 24 hours to reduce rate limits
- In-memory cache with TTL (time-to-live)
- Cache keys: search queries and platform-specific IDs

### URL Pattern Detection
- Spotify: `spotify.com/track/{id}`, `open.spotify.com/track/{id}`
- Apple Music: `music.apple.com/{country}/album/{album-id}?i={track-id}`
- Extract IDs and fetch track details to get ISRC

### Database Schema (MongoDB)
```go
type Song struct {
    ID          string                 `bson:"_id" json:"id"`
    ISRC        string                 `bson:"isrc" json:"isrc"`
    Title       string                 `bson:"title" json:"title"`
    Artists     []string               `bson:"artists" json:"artists"`
    Album       string                 `bson:"album" json:"album"`
    AlbumArt    string                 `bson:"album_art" json:"albumArt"`
    Duration    int                    `bson:"duration" json:"duration"` // milliseconds
    ReleaseDate string                 `bson:"release_date" json:"releaseDate"`
    Platforms   []PlatformLink         `bson:"platforms" json:"platforms"`
    CreatedAt   time.Time              `bson:"created_at" json:"createdAt"`
    UpdatedAt   time.Time              `bson:"updated_at" json:"updatedAt"`
}

type PlatformLink struct {
    Platform string `bson:"platform" json:"platform"` // "spotify", "apple_music"
    ID       string `bson:"id" json:"id"`
    URL      string `bson:"url" json:"url"`
}
```

## Project Structure
```
songshare/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── cache/
│   │   └── memory.go
│   ├── config/
│   │   └── config.go
│   ├── handlers/
│   │   ├── health.go
│   │   ├── search.go
│   │   ├── resolve.go
│   │   └── universal.go
│   ├── models/
│   │   └── song.go
│   ├── repository/
│   │   └── song_repository.go
│   ├── services/
│   │   ├── spotify.go
│   │   ├── apple_music.go
│   │   ├── resolver.go
│   │   └── url_parser.go
│   └── middleware/
│       └── cors.go
├── frontend/
│   ├── src/
│   │   ├── routes/
│   │   ├── lib/
│   │   └── components/
│   └── package.json
├── docker-compose.yml
├── Dockerfile
├── .env.example
├── go.mod
├── go.sum
└── README.md
```

## API Design

### Search Endpoint
```
POST /api/search
Body: { "query": "Song Name Artist" }
Response: {
  "songs": [
    {
      "id": "abc123",
      "isrc": "USRC12345678",
      "title": "Song Name",
      "artists": ["Artist Name"],
      "album": "Album Name",
      "albumArt": "https://...",
      "platforms": [
        { "platform": "spotify", "url": "https://..." },
        { "platform": "apple_music", "url": "https://..." }
      ]
    }
  ]
}
```

### Resolve Endpoint
```
POST /api/resolve
Body: { "url": "https://open.spotify.com/track/..." }
Response: {
  "song": { /* same structure as search */ }
}
```

### Universal Link
```
GET /s/:id
Response: Song details or redirect to preferred platform
```

## Future Enhancements (Post-MVP)
- Add YouTube Music, Tidal, Deezer support
- User preferences for default platform
- Playlist conversion across platforms
- Song metadata enrichment (genres, popularity)
- Rate limiting per IP
- Analytics on most searched songs
- Redis for distributed caching
- API rate limit pooling across multiple API keys