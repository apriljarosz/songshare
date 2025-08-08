.PHONY: help build test bench clean docker-up docker-down

# Default target
help:
	@echo "Available targets:"
	@echo "  make build          - Build the application binary"
	@echo "  make test           - Run all tests"
	@echo "  make test-short     - Run short tests only"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make bench          - Run all benchmarks"
	@echo "  make bench-cpu      - Run benchmarks with CPU profiling"
	@echo "  make bench-mem      - Run benchmarks with memory profiling"
	@echo "  make bench-compare  - Compare benchmark results"
	@echo "  make bench-service  - Run service-specific benchmarks"
	@echo "  make bench-cache    - Run cache-specific benchmarks"
	@echo "  make bench-handler  - Run handler-specific benchmarks"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make docker-up      - Start Docker services"
	@echo "  make docker-down    - Stop Docker services"
	@echo "  make lint           - Run linters"
	@echo "  make fmt            - Format code"

# Build targets
build:
	go build -o songshare ./cmd/server

build-race:
	go build -race -o songshare ./cmd/server

# Test targets
test:
	go test -v ./...

test-short:
	go test -short ./...

test-race:
	go test -race ./...

test-coverage:
	go test -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-integration:
	go test -tags=integration ./test/integration/...

# Benchmark targets
bench:
	@echo "Running all benchmarks..."
	@mkdir -p benchmarks
	go test -bench=. -benchmem -run=^$$ ./... | tee benchmarks/bench_$(shell date +%Y%m%d_%H%M%S).txt

bench-cpu:
	@echo "Running benchmarks with CPU profiling..."
	@mkdir -p benchmarks/profiles
	go test -bench=. -benchmem -cpuprofile=benchmarks/profiles/cpu.prof -run=^$$ ./...
	@echo "CPU profile saved to benchmarks/profiles/cpu.prof"
	@echo "View with: go tool pprof benchmarks/profiles/cpu.prof"

bench-mem:
	@echo "Running benchmarks with memory profiling..."
	@mkdir -p benchmarks/profiles
	go test -bench=. -benchmem -memprofile=benchmarks/profiles/mem.prof -run=^$$ ./...
	@echo "Memory profile saved to benchmarks/profiles/mem.prof"
	@echo "View with: go tool pprof benchmarks/profiles/mem.prof"

bench-service:
	@echo "Running service benchmarks..."
	@mkdir -p benchmarks
	go test -bench=. -benchmem -run=^$$ ./internal/services/... | tee benchmarks/bench_services_$(shell date +%Y%m%d_%H%M%S).txt

bench-cache:
	@echo "Running cache and repository benchmarks..."
	@mkdir -p benchmarks
	go test -bench=. -benchmem -run=^$$ ./internal/cache/... ./internal/repositories/... | tee benchmarks/bench_cache_$(shell date +%Y%m%d_%H%M%S).txt

bench-handler:
	@echo "Running handler benchmarks..."
	@mkdir -p benchmarks
	go test -bench=. -benchmem -run=^$$ ./internal/handlers/... | tee benchmarks/bench_handlers_$(shell date +%Y%m%d_%H%M%S).txt

bench-compare:
	@echo "Comparing benchmark results..."
	@if [ ! -f benchmarks/bench_base.txt ]; then \
		echo "No base benchmark found. Run 'make bench-save-base' first."; \
		exit 1; \
	fi
	@echo "Running current benchmarks..."
	@go test -bench=. -benchmem -run=^$$ ./... > benchmarks/bench_current.txt
	@echo "Comparison results:"
	@go install golang.org/x/perf/cmd/benchstat@latest 2>/dev/null || true
	@if command -v benchstat >/dev/null 2>&1; then \
		benchstat benchmarks/bench_base.txt benchmarks/bench_current.txt; \
	else \
		echo "benchstat not found. Install with: go install golang.org/x/perf/cmd/benchstat@latest"; \
		echo "Showing raw diff instead:"; \
		diff benchmarks/bench_base.txt benchmarks/bench_current.txt || true; \
	fi

bench-save-base:
	@echo "Saving base benchmark..."
	@mkdir -p benchmarks
	go test -bench=. -benchmem -run=^$$ ./... > benchmarks/bench_base.txt
	@echo "Base benchmark saved to benchmarks/bench_base.txt"

# Specific benchmark patterns
bench-resolution:
	go test -bench=BenchmarkSongResolutionService -benchmem -run=^$$ ./internal/services/...

bench-platform:
	go test -bench=BenchmarkPlatformService -benchmem -run=^$$ ./internal/services/...

bench-cached:
	go test -bench=BenchmarkCachedSongRepository -benchmem -run=^$$ ./internal/repositories/...

# Code quality targets
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		go vet ./...; \
	fi

fmt:
	go fmt ./...
	gofmt -s -w .

fmt-check:
	@if [ -n "$$(gofmt -s -l .)" ]; then \
		echo "Code needs formatting. Run 'make fmt'"; \
		gofmt -s -l .; \
		exit 1; \
	fi

vet:
	go vet ./...

security:
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not found. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "govulncheck not found. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

# Docker targets
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-build:
	docker-compose build

docker-logs:
	docker-compose logs -f

docker-reset:
	./reset-db.sh

# Development targets
dev:
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "Air not found. Install with: go install github.com/air-verse/air@latest"; \
		go run cmd/server/main.go; \
	fi

run:
	go run cmd/server/main.go

# Clean targets
clean:
	rm -f songshare
	rm -f coverage.out coverage.html
	rm -rf benchmarks/profiles
	go clean -cache

clean-all: clean
	rm -rf benchmarks/
	docker-compose down -v

# Installation targets
install-tools:
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/perf/cmd/benchstat@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest

# Dependencies
deps:
	go mod download
	go mod verify

tidy:
	go mod tidy

# Default target
all: fmt lint test build