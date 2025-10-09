.PHONY: help dev build start stop clean types backend frontend test

help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

dev: ## Run development environment (backend + frontend)
	@echo "Starting development servers..."
	@make -j2 backend frontend

backend: ## Run backend development server
	@echo "Starting Go backend..."
	@./run.sh

frontend: ## Run frontend development server
	@echo "Starting frontend dev server..."
	@cd frontend && pnpm run dev

types: ## Generate TypeScript types from Go models
	@echo "Generating TypeScript types..."
	@tygo generate
	@echo "✓ Types generated successfully"

build: ## Build all services with Docker
	@echo "Building all services..."
	@docker-compose build

start: ## Start all services with Docker
	@echo "Starting all services..."
	@docker-compose up -d

stop: ## Stop all Docker services
	@echo "Stopping all services..."
	@docker-compose down

clean: ## Clean up Docker containers and volumes
	@echo "Cleaning up..."
	@docker-compose down -v
	@rm -rf frontend/node_modules frontend/.svelte-kit

install: ## Install dependencies
	@echo "Installing backend dependencies..."
	@go mod download
	@echo "Installing frontend dependencies..."
	@cd frontend && pnpm install

test: ## Run tests
	@echo "Running Go tests..."
	@go test ./...
	@echo "Running frontend checks..."
	@cd frontend && pnpm run check

logs: ## Show logs from all services
	@docker-compose logs -f

logs-backend: ## Show backend logs
	@docker-compose logs -f backend

logs-frontend: ## Show frontend logs
	@docker-compose logs -f frontend

reset-db: ## Reset MongoDB database (development only)
	@./scripts/reset-db.sh
