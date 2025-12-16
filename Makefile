.PHONY: help build test test-unit test-integration test-coverage lint fmt vet clean run docker-build docker-up docker-down migrate-up migrate-down deps tidy

# Variables
APP_NAME=minion
BINARY_NAME=minion
DOCKER_IMAGE=minion-agent:latest
GO=go
GOTEST=$(GO) test
GOVET=$(GO) vet
GOFMT=gofmt
GOLINT=golangci-lint

# Colors for output
COLOR_RESET=\033[0m
COLOR_BOLD=\033[1m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m

## help: Display this help message
help:
	@echo "$(COLOR_BOLD)Minion Agent Framework - Makefile Commands$(COLOR_RESET)"
	@echo ""
	@sed -n 's/^##//p' Makefile | column -t -s ':' | sed -e 's/^/ /'

## build: Build the application binary
build:
	@echo "$(COLOR_GREEN)Building $(APP_NAME)...$(COLOR_RESET)"
	$(GO) build -o bin/$(BINARY_NAME) ./cmd/minion
	@echo "$(COLOR_GREEN)Build complete: bin/$(BINARY_NAME)$(COLOR_RESET)"

## run: Run the application
run:
	@echo "$(COLOR_GREEN)Running $(APP_NAME)...$(COLOR_RESET)"
	$(GO) run ./cmd/minion

## test: Run all tests
test: test-unit test-integration

## test-unit: Run unit tests
test-unit:
	@echo "$(COLOR_GREEN)Running unit tests...$(COLOR_RESET)"
	$(GOTEST) -v -race -short ./...

## test-integration: Run integration tests
test-integration:
	@echo "$(COLOR_GREEN)Running integration tests...$(COLOR_RESET)"
	$(GOTEST) -v -race -run Integration ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "$(COLOR_GREEN)Running tests with coverage...$(COLOR_RESET)"
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(COLOR_GREEN)Coverage report: coverage.html$(COLOR_RESET)"

## bench: Run benchmarks
bench:
	@echo "$(COLOR_GREEN)Running benchmarks...$(COLOR_RESET)"
	$(GOTEST) -bench=. -benchmem ./...

## lint: Run linter
lint:
	@echo "$(COLOR_GREEN)Running linter...$(COLOR_RESET)"
	$(GOLINT) run --timeout=5m ./...

## fmt: Format code
fmt:
	@echo "$(COLOR_GREEN)Formatting code...$(COLOR_RESET)"
	$(GOFMT) -s -w .

## vet: Run go vet
vet:
	@echo "$(COLOR_GREEN)Running go vet...$(COLOR_RESET)"
	$(GOVET) ./...

## clean: Clean build artifacts
clean:
	@echo "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	rm -rf bin/
	rm -f coverage.out coverage.html
	$(GO) clean

## deps: Download dependencies
deps:
	@echo "$(COLOR_GREEN)Downloading dependencies...$(COLOR_RESET)"
	$(GO) mod download

## tidy: Tidy dependencies
tidy:
	@echo "$(COLOR_GREEN)Tidying dependencies...$(COLOR_RESET)"
	$(GO) mod tidy

## docker-build: Build Docker image
docker-build:
	@echo "$(COLOR_GREEN)Building Docker image...$(COLOR_RESET)"
	docker build -t $(DOCKER_IMAGE) .

## docker-up: Start services with docker-compose
docker-up:
	@echo "$(COLOR_GREEN)Starting services with docker-compose...$(COLOR_RESET)"
	docker-compose up -d

## docker-down: Stop services with docker-compose
docker-down:
	@echo "$(COLOR_YELLOW)Stopping services with docker-compose...$(COLOR_RESET)"
	docker-compose down

## docker-logs: View docker-compose logs
docker-logs:
	docker-compose logs -f

## migrate-up: Run database migrations (up)
migrate-up:
	@echo "$(COLOR_GREEN)Running database migrations (up)...$(COLOR_RESET)"
	$(GO) run ./cmd/migrate up

## migrate-down: Run database migrations (down)
migrate-down:
	@echo "$(COLOR_YELLOW)Running database migrations (down)...$(COLOR_RESET)"
	$(GO) run ./cmd/migrate down

## migrate-create: Create a new migration (usage: make migrate-create NAME=migration_name)
migrate-create:
	@echo "$(COLOR_GREEN)Creating new migration: $(NAME)...$(COLOR_RESET)"
	$(GO) run ./cmd/migrate create $(NAME)

## install-tools: Install development tools
install-tools:
	@echo "$(COLOR_GREEN)Installing development tools...$(COLOR_RESET)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/golang/mock/mockgen@latest
	go install github.com/swaggo/swag/cmd/swag@latest

## generate: Generate code (mocks, etc.)
generate:
	@echo "$(COLOR_GREEN)Generating code...$(COLOR_RESET)"
	$(GO) generate ./...

## check: Run all checks (fmt, vet, lint, test)
check: fmt vet lint test
	@echo "$(COLOR_GREEN)All checks passed!$(COLOR_RESET)"

## dev: Start development environment
dev: docker-up
	@echo "$(COLOR_GREEN)Development environment ready$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)PostgreSQL: localhost:5432$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Jaeger UI: http://localhost:16686$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Prometheus: http://localhost:9090$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Grafana: http://localhost:3000$(COLOR_RESET)"
