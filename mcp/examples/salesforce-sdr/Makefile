.PHONY: help build run test docker-build docker-run docker-compose-up docker-compose-down k8s-deploy k8s-delete clean

# Variables
APP_NAME := virtual-sdr
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
DOCKER_IMAGE := $(APP_NAME):$(VERSION)
DOCKER_LATEST := $(APP_NAME):latest

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: ## Build the application
	@echo "Building $(APP_NAME)..."
	go build -o $(APP_NAME) main.go
	@echo "✅ Build complete: ./$(APP_NAME)"

run: ## Run the application locally
	@echo "Starting $(APP_NAME)..."
	@if [ ! -f .env ]; then \
		echo "⚠️  .env file not found. Copy .env.example to .env and configure it."; \
		exit 1; \
	fi
	@source .env && ./$(APP_NAME)

test: ## Run tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "✅ Tests complete"

coverage: test ## Generate test coverage report
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report: coverage.html"

lint: ## Run linters
	@echo "Running linters..."
	golangci-lint run ./...
	@echo "✅ Lint complete"

docker-build: ## Build Docker image
	@echo "Building Docker image: $(DOCKER_IMAGE)"
	docker build -t $(DOCKER_IMAGE) -t $(DOCKER_LATEST) .
	@echo "✅ Docker image built: $(DOCKER_IMAGE)"

docker-run: docker-build ## Run in Docker
	@echo "Running $(APP_NAME) in Docker..."
	@if [ ! -f .env ]; then \
		echo "⚠️  .env file not found. Copy .env.example to .env and configure it."; \
		exit 1; \
	fi
	docker run --rm \
		--name $(APP_NAME) \
		--env-file .env \
		-p 9090:9090 \
		-v $(PWD)/credentials:/credentials:ro \
		$(DOCKER_LATEST)

docker-compose-up: ## Start all services with docker-compose
	@echo "Starting services with docker-compose..."
	@if [ ! -f .env ]; then \
		echo "⚠️  .env file not found. Copy .env.example to .env and configure it."; \
		exit 1; \
	fi
	docker-compose up -d
	@echo "✅ Services started:"
	@echo "  • Virtual SDR:  http://localhost:9090"
	@echo "  • Prometheus:   http://localhost:9091"
	@echo "  • Grafana:      http://localhost:3000 (admin/admin)"

docker-compose-down: ## Stop all services
	@echo "Stopping services..."
	docker-compose down
	@echo "✅ Services stopped"

docker-compose-logs: ## View logs
	docker-compose logs -f virtual-sdr

k8s-deploy: ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes..."
	kubectl apply -f k8s-deployment.yaml
	@echo "✅ Deployed to Kubernetes"
	@echo ""
	@echo "Check status:"
	@echo "  kubectl get pods -n virtual-sdr"
	@echo "  kubectl logs -n virtual-sdr -l app=virtual-sdr -f"

k8s-delete: ## Delete from Kubernetes
	@echo "Deleting from Kubernetes..."
	kubectl delete -f k8s-deployment.yaml
	@echo "✅ Deleted from Kubernetes"

k8s-status: ## Check Kubernetes status
	kubectl get all -n virtual-sdr

k8s-logs: ## View Kubernetes logs
	kubectl logs -n virtual-sdr -l app=virtual-sdr -f --tail=100

metrics: ## View metrics
	@echo "Fetching metrics from http://localhost:9090/metrics"
	@curl -s http://localhost:9090/metrics | grep -E "^mcp_"

status: ## View status
	@echo "Fetching status from http://localhost:9090/status"
	@curl -s http://localhost:9090/status | jq

health: ## Check health
	@echo "Checking health..."
	@curl -s http://localhost:9090/health
	@echo ""

demo: ## Run a quick demo workflow
	@echo "Running demo workflow..."
	@echo "This will:"
	@echo "  1. Start the agent"
	@echo "  2. Connect to MCP servers"
	@echo "  3. Run a sample lead qualification"
	@echo ""
	@$(MAKE) run

install-deps: ## Install development dependencies
	@echo "Installing dependencies..."
	go mod download
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✅ Dependencies installed"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -f $(APP_NAME)
	rm -f coverage.out coverage.html
	rm -rf vendor/
	@echo "✅ Cleaned"

generate-creds: ## Generate sample credentials template
	@echo "Generating sample credentials..."
	@mkdir -p credentials
	@echo '{"type": "service_account"}' > credentials/gmail-credentials.json.example
	@echo '{"type": "service_account"}' > credentials/calendar-credentials.json.example
	@echo "✅ Sample credentials created in credentials/"
	@echo "   Edit these files with your actual credentials"

validate-env: ## Validate environment configuration
	@echo "Validating environment configuration..."
	@if [ ! -f .env ]; then \
		echo "❌ .env file not found"; \
		exit 1; \
	fi
	@source .env && \
	if [ -z "$$SALESFORCE_INSTANCE_URL" ]; then \
		echo "❌ SALESFORCE_INSTANCE_URL not set"; \
		exit 1; \
	fi
	@echo "✅ Environment configuration valid"

# Development workflow shortcuts
dev: build run ## Build and run (development)

prod: docker-compose-up ## Start production stack

stop: docker-compose-down ## Stop production stack

restart: docker-compose-down docker-compose-up ## Restart production stack

all: clean install-deps lint test build ## Run all checks and build
