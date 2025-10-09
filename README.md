# SongShare

A universal song catalog that indexes music and provides links to songs across all platforms (Spotify, Apple Music, etc.). Search for any song or paste a platform URL to get unified results with links to all available platforms.

## Features

- **Universal Search**: Search for songs by title, artist, or album
- **URL Resolution**: Paste a Spotify or Apple Music URL to find the song on all platforms
- **ISRC Matching**: Accurate cross-platform song matching using ISRC codes
- **Universal Share Links**: Generate shareable links that work across all platforms (`/s/{id}`)
- **Persistent Catalog**: MongoDB-backed database that grows over time
- **Fast Performance**: In-memory caching to reduce API calls

## Tech Stack

### Backend
- **Go** with Gin web framework
- **MongoDB** for persistent storage
- **Platform APIs**: Spotify Web API, Apple Music API

### Frontend
- **SvelteKit** with TypeScript
- **Vite** for fast development and building
- **pnpm** for package management
- **Tygo** for automatic Go → TypeScript type generation

## Project Structure

```
songshare/
├── cmd/server/          # Go backend entry point
├── internal/
│   ├── cache/          # In-memory caching
│   ├── config/         # Configuration management
│   ├── handlers/       # HTTP request handlers
│   ├── middleware/     # HTTP middleware
│   ├── models/         # Data models (Go structs)
│   ├── repository/     # Database operations
│   └── services/       # Platform API clients
├── frontend/
│   ├── src/
│   │   ├── routes/            # SvelteKit pages
│   │   ├── lib/
│   │   │   ├── api/          # API client
│   │   │   ├── components/   # Svelte components
│   │   │   └── types/        # TypeScript types (auto-generated)
│   │   └── app.html
│   └── static/               # Static assets
├── docker-compose.yml   # Multi-service orchestration
├── Makefile            # Common commands
└── tygo.yaml           # Type generation config
```

## Getting Started

### Prerequisites

- Go 1.21+
- Node.js 20+
- pnpm (install with `npm install -g pnpm`)
- MongoDB 7.0+
- Docker & Docker Compose (for containerized deployment)

### Environment Setup

1. Clone the repository:
```bash
git clone https://github.com/apriljarosz/songshare.git
cd songshare
```

2. Copy environment files:
```bash
cp .env.example .env
```

3. Add your API credentials to `.env`:
```env
SPOTIFY_CLIENT_ID=your_spotify_client_id
SPOTIFY_CLIENT_SECRET=your_spotify_client_secret
APPLE_MUSIC_TEAM_ID=your_team_id
APPLE_MUSIC_KEY_ID=your_key_id
APPLE_MUSIC_PRIVATE_KEY=path_to_your_key.p8
```

### Development

#### Option 1: Docker (Recommended)

Start all services with Docker Compose:
```bash
make start
# or
docker-compose up
```

Services will be available at:
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080
- MongoDB: localhost:27017

#### Option 2: Local Development

1. Install dependencies:
```bash
make install
```

2. Start MongoDB:
```bash
docker-compose up mongodb
```

3. Run backend and frontend in separate terminals:
```bash
# Terminal 1: Backend
make backend

# Terminal 2: Frontend
make frontend
```

### Makefile Commands

```bash
make help          # Show all available commands
make dev           # Run backend + frontend in parallel
make types         # Generate TypeScript types from Go models
make build         # Build Docker images
make start         # Start all services with Docker
make stop          # Stop all Docker services
make clean         # Clean up Docker containers and volumes
make logs          # Show logs from all services
make test          # Run tests
```

## API Endpoints

### Search for Songs
```bash
POST /api/search
Content-Type: application/json

{
  "query": "bohemian rhapsody queen"
}
```

### Resolve Platform URL
```bash
POST /api/resolve
Content-Type: application/json

{
  "url": "https://open.spotify.com/track/..."
}
```

### Get Song by ID
```bash
GET /api/songs/:id
```

### Universal Share Link
```bash
GET /s/:id
```

### List Supported Platforms
```bash
GET /api/platforms
```

### Health Check
```bash
GET /health
```

## Type Generation

This project uses `tygo` to automatically generate TypeScript types from Go structs:

```bash
# Generate types manually
make types

# Or run tygo directly
tygo generate
```

Generated types are output to `frontend/src/lib/types/generated.ts` and should not be edited manually.

## How It Works

1. **User searches** for a song or pastes a platform URL
2. **Backend queries** Spotify and Apple Music APIs in parallel
3. **Songs are matched** by ISRC (International Standard Recording Code)
4. **Results are cached** in memory and saved to MongoDB
5. **Frontend displays** unified results with links to all platforms
6. **Database grows** organically as users search for songs

## Platform Support

### Currently Supported
- ✅ Spotify
- ✅ Apple Music

### Planned
- YouTube Music
- Tidal
- Deezer

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details
