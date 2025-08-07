# AGENTS.md - Development Guide for AI Coding Agents

## Build/Test/Lint Commands
```bash
# Run all tests
go test ./...

# Run single test file
go test ./internal/services/song_resolution_service_test.go

# Run specific test function
go test -run TestFunctionName ./path/to/package

# Run with coverage
go test -cover ./...

# Lint code (required before commits)
golangci-lint run

# Format code
go fmt ./... && gofmt -s -w .

# Build application
go build -o songshare ./cmd/server

# Run with hot reload (development)
air
```

## Code Style Guidelines

**Imports**: Use `goimports` formatting. Group stdlib, external, then internal packages with blank lines between groups.

**Naming**: Use camelCase for variables/functions, PascalCase for exported types. Acronyms like ISRC, URL, API stay uppercase.

**Error Handling**: Always wrap errors with context using `fmt.Errorf("description: %w", err)`. Use structured logging with `slog`.

**Types**: Use explicit types for struct fields with proper JSON/BSON tags. Prefer interfaces for dependencies (e.g., `repositories.SongRepository`).

**Testing**: Use testify for assertions (`assert`, `require`). Mock interfaces with testify/mock. Test files end with `_test.go`.

**Comments**: Document exported functions/types. Use `//` for single line, avoid block comments unless necessary.