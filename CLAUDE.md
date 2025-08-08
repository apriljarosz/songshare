# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Running the Application
```bash
# Run locally (requires .env file setup)
go run cmd/server/main.go

# Run with hot reload (recommended for development)
air

# Using Docker Compose (includes MongoDB and Valkey)
docker-compose up

# Build Docker image
docker build -t songshare .
```

### Development Setup
```bash
# Install Air for hot reloading (one-time setup)
go install github.com/air-verse/air@latest

# Add Go bin to PATH if not already (add to your shell profile)
export PATH=$PATH:$(go env GOPATH)/bin

# Start development server with hot reload
air

# Alternative: run air directly with full path
$(go env GOPATH)/bin/air
```

### Go Commands
```bash
# Install/update dependencies
go mod download

# Verify dependencies
go mod verify

# Build binary
go build -o songshare ./cmd/server

# Format code
go fmt ./...

# Run linting
golangci-lint run

# Check code formatting
gofmt -s -l .

# Fix code formatting
gofmt -s -w .
```

### Testing Commands
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...

# Run specific package tests
go test ./internal/services

# Generate HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run only short/fast tests
go test -short ./...

# Run integration tests (requires MongoDB & Valkey)
go test -tags=integration ./test/integration/...

# Run benchmarks (see Benchmarking section below for more options)
go test -bench=. -benchmem ./...
```

### Code Quality & Security
```bash
# Install security and quality tools (one-time setup)
go install golang.org/x/vuln/cmd/govulncheck@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
go install github.com/securecodewarrior/gosec/cmd/gosec@latest

# Run security and quality checks
govulncheck ./...    # Check for known vulnerabilities
staticcheck ./...    # Static analysis
gosec ./...          # Security scanning
go vet ./...         # Go's built-in analyzer
```

### Utility Scripts
```bash
# Database management
./scripts/db-stats.sh          # View database statistics
./reset-db.sh                  # Reset database and cache for testing

# Maintenance tasks
go run cmd/backfill-album-art/main.go  # Backfill missing album artwork
```

### Benchmarking Commands

The project includes comprehensive benchmarking for performance monitoring and optimization:

```bash
# Run all benchmarks
make bench

# Run specific benchmark categories
make bench-service    # Service layer benchmarks
make bench-cache      # Cache and repository benchmarks
make bench-handler    # HTTP handler benchmarks

# Run benchmarks with profiling
make bench-cpu        # CPU profiling
make bench-mem        # Memory profiling

# Benchmark comparison and analysis
make bench-save-base  # Save baseline for comparison
make bench-compare    # Compare current run with baseline

# Specific benchmark patterns
make bench-resolution      # Song resolution service benchmarks
make bench-platform       # Platform service benchmarks
make bench-cached         # Cached repository benchmarks

# Performance profiling and analysis
./scripts/profile-analyze.sh cpu                    # CPU profiling
./scripts/profile-analyze.sh memory                 # Memory profiling
./scripts/profile-analyze.sh package internal/services  # Package-specific profiling
./scripts/profile-analyze.sh comprehensive         # Full profiling suite

# Benchmark monitoring over time
./scripts/bench-monitor.sh run        # Run and save benchmarks
./scripts/bench-monitor.sh compare    # Run and compare with previous
./scripts/bench-monitor.sh trends     # Show performance trends
./scripts/bench-monitor.sh report     # Generate performance report

# Install benchmarking tools (one-time setup)
go install golang.org/x/perf/cmd/benchstat@latest  # For benchmark comparison
go install github.com/uber/go-torch@latest         # For flame graphs (optional)
```

#### Benchmark Coverage

The benchmarking suite covers:

- **Song Resolution Service**: Cross-platform resolution performance, search operations, multi-platform handling
- **Cache Layer**: In-memory LRU cache, Valkey operations, cache hit/miss scenarios
- **Repository Layer**: MongoDB operations, cached vs uncached access patterns
- **Platform Services**: API call simulation, concurrent operations, latency handling
- **HTTP Handlers**: Request/response throughput, JSON serialization, content negotiation

#### Performance Profiling

Use the profiling tools to identify bottlenecks:

```bash
# Generate CPU profile and analyze
make bench-cpu
go tool pprof benchmarks/profiles/cpu.prof

# Generate memory profile  
make bench-mem
go tool pprof benchmarks/profiles/mem.prof

# Comprehensive profiling with reports
./scripts/profile-analyze.sh comprehensive
```

#### Benchmark Results Storage

Benchmark results are stored in:
- `benchmarks/` - Timestamped benchmark outputs
- `benchmarks/profiles/` - CPU and memory profiles
- `benchmarks/profiles/flamegraphs/` - Flame graph visualizations

Use `make bench-compare` to track performance regressions between runs.

## Architecture Overview

### Core Components

**Entry Point**: `cmd/server/main.go`
- Application initialization and dependency injection
- Graceful shutdown with 30-second timeout
- Structured logging with slog
- Health check endpoint at `/health`

**Configuration**: `internal/config/config.go`
- Environment-based configuration using envconfig
- Required: MONGODB_URL, VALKEY_URL
- Optional: Spotify/Apple Music API credentials

**Data Flow Architecture**:
1. **Handlers** (`internal/handlers/`) - HTTP request handling
2. **Services** (`internal/services/`) - Business logic and platform integration
3. **Repositories** (`internal/repositories/`) - Data access with caching layer
4. **Models** (`internal/models/`) - Data structures and database schemas

### Platform Integration

**Song Resolution Service** (`internal/services/song_resolution_service.go`):
- Centralized service that coordinates multiple platform services
- Registers platform services (Spotify, Apple Music) dynamically
- Handles cross-platform song resolution and search

**Platform Services**:
- **Spotify Service**: OAuth2-based API integration
- **Apple Music Service**: JWT-based API with private key authentication

### Caching Strategy

**Multi-Level Caching** (`internal/cache/`):
- In-memory LRU cache (1000 items by default)
- Valkey/Redis for distributed caching
- Repository-level caching decorator pattern

### Database Design

**MongoDB** with connection pooling and indexes
- Song documents with platform links
- Metadata includes duration, release dates, ISRC codes
- Short ID generation for universal links (8-character hex)

## API Endpoints

### Core Endpoints
- `POST /api/v1/songs/resolve` - Resolve song from platform URL
- `POST /api/v1/songs/search` - Search songs across platforms
- `GET /s/:id` - Universal link redirects (dual JSON/HTML response)
- `GET /health` - Health check

### Content Negotiation
The `/s/:id` endpoint supports dual-mode responses:
- JSON for API clients
- HTML with HTMX for browsers

## Environment Setup

Copy `.env.example` to `.env` and configure:
- MongoDB connection string
- Valkey/Redis connection string
- Platform API credentials (Spotify, Apple Music)

### Required External Services
- MongoDB 8.0+
- Valkey/Redis for caching
- Spotify Developer Account (optional)
- Apple Music API access (optional)

### Go Version Requirements
- Go 1.24+ (application built and tested with Go 1.24.5)

## Key Architecture Patterns

### Service Registration Pattern
Dynamic platform service registration in `SongResolutionService` allows for plugin-like platform integrations.

### Repository Caching Pattern
Multi-level caching with decorator pattern provides both in-memory and distributed caching layers.

### Schema Versioning
Database schema versioning system (`CurrentSchemaVersion = 1`) enables safe migrations.

### Content Negotiation
The `/s/:id` endpoint provides dual JSON/HTML responses based on Accept headers.

## Security Notes

- Application runs as non-root user in containers
- API keys stored in `keys/` directory (Apple Music private key)
- Distroless container image for minimal attack surface
- Environment variable validation on startup
- Security scanning configured in CI/CD pipeline

## Development Memories
- The go server is running in Air, which auto hot-reloads, so you don't need to restart anything