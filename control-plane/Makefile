.PHONY: all build run test clean docker-up docker-down migrate-up migrate-down lint fmt help templ templ-watch css css-watch dev-web build-web

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Binary name
BINARY_NAME=server
BINARY_PATH=./cmd/server

# Docker
DOCKER_COMPOSE=docker compose -f docker/docker-compose.yml

all: build

## Build the application
build:
	$(GOBUILD) -o $(BINARY_NAME) $(BINARY_PATH)

## Run the application locally
run: build
	./$(BINARY_NAME)

## Run with hot reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

## Run tests
test:
	$(GOTEST) -v -race -cover ./...

## Run tests with coverage
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

## Start Docker services
docker-up:
	$(DOCKER_COMPOSE) up -d

## Stop Docker services
docker-down:
	$(DOCKER_COMPOSE) down

## Build Docker image
docker-build:
	$(DOCKER_COMPOSE) build

## View Docker logs
docker-logs:
	$(DOCKER_COMPOSE) logs -f

## Run database migrations up
migrate-up:
	$(GOCMD) run $(BINARY_PATH) migrate up

## Run database migrations down
migrate-down:
	$(GOCMD) run $(BINARY_PATH) migrate down

## Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

## Run linter
lint:
	$(GOLINT) run ./...

## Format code
fmt:
	$(GOFMT) -s -w .

## Generate code (mocks, etc.)
generate:
	$(GOCMD) generate ./...

## Check if build works
check: fmt lint test build

## Start all services for local development
start: docker-up
	@echo "Waiting for services to be ready..."
	@sleep 5
	@curl -s http://localhost:8080/health || echo "API not yet ready"

## Stop all services
stop: docker-down

## Reset database (drop and recreate)
db-reset:
	$(DOCKER_COMPOSE) down -v
	$(DOCKER_COMPOSE) up -d postgres redis
	@echo "Waiting for database..."
	@sleep 3

## Show help
help:
	@echo "BanhBaoRing Control Plane - Available commands:"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
	@echo ""
	@echo "Usage: make <target>"

# ============================================
# Dashboard / Web UI Commands
# ============================================

## Generate templ templates
templ:
	templ generate

## Watch templ templates for changes
templ-watch:
	templ generate --watch

## Build Tailwind CSS
css:
	npx tailwindcss -i static/css/input.css -o static/css/output.css --minify

## Watch Tailwind CSS for changes
css-watch:
	npx tailwindcss -i static/css/input.css -o static/css/output.css --watch

## Run web development (templ + css + server with hot reload)
dev-web: templ css
	@echo "Starting development server..."
	@echo "Run 'make templ-watch' and 'make css-watch' in separate terminals for live reload"
	air

## Production build for web dashboard
build-web: templ css build
	@echo "Production build complete"

## Install web development dependencies
install-web-deps:
	@echo "Installing templ CLI..."
	go install github.com/a-h/templ/cmd/templ@latest
	@echo "Installing air for hot reload..."
	go install github.com/cosmtrek/air@latest
	@echo "Done! Make sure you have Node.js installed for Tailwind CSS"
