# Makefile for gcli2apigo

# Variables
BINARY_NAME=gcli2apigo
DOCKER_IMAGE=gcli2apigo
DOCKER_TAG=latest
GO_VERSION=1.21

# Default target
.PHONY: all
all: build

# Build the application
.PHONY: build
build:
	go build -o $(BINARY_NAME) .

# Build for multiple platforms
.PHONY: build-all
build-all:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -o $(BINARY_NAME)-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows-amd64.exe .
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 .

# Run the application
.PHONY: run
run: build
	./$(BINARY_NAME)

# Run tests
.PHONY: test
test:
	go test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	rm -f coverage.out coverage.html

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Run linter
.PHONY: lint
lint:
	golangci-lint run

# Download dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Docker targets
.PHONY: docker-build
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

.PHONY: docker-run
docker-run:
	docker run -d \
		--name $(DOCKER_IMAGE) \
		-p 7860:7860 \
		-e GEMINI_AUTH_PASSWORD=your-secure-password \
		-v $(PWD)/oauth_creds:/app/oauth_creds \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker-stop
docker-stop:
	docker stop $(DOCKER_IMAGE) || true
	docker rm $(DOCKER_IMAGE) || true

.PHONY: docker-logs
docker-logs:
	docker logs -f $(DOCKER_IMAGE)

# Docker Compose targets
.PHONY: compose-up
compose-up:
	docker-compose up -d

.PHONY: compose-down
compose-down:
	docker-compose down

.PHONY: compose-logs
compose-logs:
	docker-compose logs -f

# Development targets
.PHONY: dev
dev:
	go run . &
	echo "Server started in background. Use 'make dev-stop' to stop."

.PHONY: dev-stop
dev-stop:
	pkill -f "go run ." || true

# Health check
.PHONY: health
health:
	curl -f http://localhost:7860/health || echo "Service is not healthy"

# Setup development environment
.PHONY: setup
setup:
	go mod download
	mkdir -p oauth_creds
	cp .env.example .env
	echo "Development environment setup complete!"
	echo "Edit .env file with your configuration before running."

# Release targets
.PHONY: release-build
release-build:
	CGO_ENABLED=0 go build -a -installsuffix cgo -ldflags="-w -s" -o $(BINARY_NAME) .

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  build-all     - Build for multiple platforms"
	@echo "  run           - Build and run the application"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  fmt           - Format code"
	@echo "  lint          - Run linter"
	@echo "  deps          - Download and tidy dependencies"
	@echo ""
	@echo "Docker targets:"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  docker-stop   - Stop and remove Docker container"
	@echo "  docker-logs   - Show Docker container logs"
	@echo ""
	@echo "Docker Compose targets:"
	@echo "  compose-up    - Start services with docker-compose"
	@echo "  compose-down  - Stop services with docker-compose"
	@echo "  compose-logs  - Show docker-compose logs"
	@echo ""
	@echo "Development targets:"
	@echo "  dev           - Run in development mode"
	@echo "  dev-stop      - Stop development server"
	@echo "  health        - Check service health"
	@echo "  setup         - Setup development environment"
	@echo ""
	@echo "Release targets:"
	@echo "  release-build - Build optimized release binary"