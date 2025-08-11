# AGENTS.md - Development Guide for AI Coding Agents

## Build/Test/Lint Commands
```bash
# Run all tests
go test ./... OR make test

# Run single test file  
go test ./internal/services/platform_service_test.go

# Run specific test function
go test -run TestFunctionName ./path/to/package

# Run with coverage
go test -cover ./... OR make test-coverage

# Lint code (required before commits)
golangci-lint run OR make lint

# Format code
go fmt ./... && gofmt -s -w . OR make fmt

# Build application
go build -o songshare ./cmd/server OR make build

# Run with hot reload (development)
air OR make dev
```

## Code Style Guidelines

**Imports**: Use `goimports` formatting. Group stdlib, external, then internal packages with blank lines between groups.

**Naming**: Use descriptive names for exported identifiers. Avoid abbreviations. Acronyms like ISRC, URL, API stay uppercase.

**Error Handling**: Always wrap errors with context using `fmt.Errorf("description: %w", err)`. Return sentinel/wrapped errors; avoid deep logging.

**Context**: Always thread `context.Context` from handlers → services → repositories. Never store contexts in structs.

**Types**: Use explicit types for struct fields with proper JSON/BSON tags. Define small, behavior-focused interfaces near consumers.

**Testing**: Use helpers in `internal/testutil/`. Integration tests in `test/integration/`. Mock external calls via interfaces.

**Control Flow**: Prefer guard clauses and early returns. Handle edge cases first. Avoid deep nesting.