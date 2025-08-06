# Songshare

A Go-based web service for sharing and resolving songs across different music platforms.

## Features

- Song resolution across multiple music platforms (Apple Music, Spotify)
- Caching layer with Valkey/Redis
- MongoDB integration for data persistence
- RESTful API with Gin framework
- Docker support

## Getting Started

### Prerequisites

- Go 1.24.5 or later
- MongoDB
- Valkey/Redis (optional, for caching)

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd songshare
```

2. Install dependencies:
```bash
go mod download
```

3. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your configuration
```

4. Run the application:
```bash
go run cmd/server/main.go
```

### Using Docker

```bash
docker-compose up
```

## API Endpoints

The service provides RESTful endpoints for song operations. See the handlers in `internal/handlers/` for available endpoints.

## Project Structure

```
├── cmd/server/          # Application entry point
├── internal/
│   ├── cache/           # Caching layer
│   ├── config/          # Configuration management
│   ├── handlers/        # HTTP handlers
│   ├── middleware/      # HTTP middleware
│   ├── models/          # Data models
│   ├── repositories/    # Data access layer
│   └── services/        # Business logic
└── pkg/                 # Public packages
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License.