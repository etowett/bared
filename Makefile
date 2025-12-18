.PHONY: all build clean install uninstall test test-unit test-integration test-e2e test-all coverage coverage-check bench pre-commit validate fmt lint vet run-daemon dev help web-install web-build web-dev web-clean web-lint web-validate web-format web-sync-dist build-with-web validate-all compose-up compose-up-fg compose-down compose-down-volumes compose-restart compose-stop compose-start compose-ps compose-logs compose-logs-follow compose-logs-service compose-logs-service-follow compose-build compose-pull compose-clean compose-clean-all compose-services-up compose-services-down compose-exec compose-shell

# Default target
all: build

# Build variables
BINARY_NAME=brd
BIN_DIR=bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-X bared/internal/version.Version=${VERSION} -X bared/internal/version.Commit=${COMMIT} -X bared/internal/version.BuildDate=${BUILD_TIME}"

# CGO handling:
# - Default builds are CGO disabled for portability (static-ish binaries).
# - Local dev `run-daemon` enables CGO so sqlite persistence works with github.com/mattn/go-sqlite3.
CGO ?= 0

# On recent macOS versions, `go test -race` can emit noisy (but harmless) ld warnings like:
#   "malformed LC_DYSYMTAB ..."
# This is an Apple ld/toolchain issue; suppress warnings for test binaries on Darwin.
UNAME_S := $(shell uname -s)
GO_TEST_LDFLAGS :=
ifeq ($(UNAME_S),Darwin)
GO_TEST_LDFLAGS := -ldflags=-extldflags=-Wl,-w
endif

# Build the binary
build:
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p ${BIN_DIR}
	CGO_ENABLED=$(CGO) go build ${LDFLAGS} -o ${BIN_DIR}/${BINARY_NAME} ./cmd/brd
	@echo "Build complete: ./${BIN_DIR}/${BINARY_NAME}"

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-amd64 ./cmd/brd
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-arm64 ./cmd/brd
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-amd64 ./cmd/brd
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-arm64 ./cmd/brd
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-windows-amd64.exe ./cmd/brd
	@echo "Cross-platform builds complete in ./dist/"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f ${BINARY_NAME}
	rm -rf ${BIN_DIR}/
	rm -rf dist/
	rm -f coverage.out coverage.html
	@echo "Clean complete"

# Install to /usr/local/bin
install: build
	@echo "Installing ${BINARY_NAME} to /usr/local/bin..."
	sudo cp ${BIN_DIR}/${BINARY_NAME} /usr/local/bin/
	sudo chmod +x /usr/local/bin/${BINARY_NAME}
	@echo "Installation complete. Run 'brd --help' to get started."

# Uninstall from /usr/local/bin
uninstall:
	@echo "Uninstalling ${BINARY_NAME}..."
	sudo rm -f /usr/local/bin/${BINARY_NAME}
	@echo "Uninstall complete"

# Install as systemd service
install-service: install
	@echo "Installing systemd service..."
	sudo cp examples/bared.service /etc/systemd/system/
	sudo systemctl daemon-reload
	@echo "Service installed. Configure /etc/bared/bared.yml and run:"
	@echo "  sudo systemctl enable bared"
	@echo "  sudo systemctl start bared"

# Run tests (default: unit tests only)
test: test-unit

# Run unit tests (fast, no external dependencies)
test-unit:
	@echo "Running unit tests..."
	go test -v -race -short $(GO_TEST_LDFLAGS) -coverprofile=coverage.out ./...

# Run integration tests (requires Docker services)
test-integration:
	@echo "Running integration tests (requires Docker)..."
	@echo "Starting services..."
	docker-compose up -d mysql postgres redis minio
	@echo "Waiting for services to be ready..."
	sleep 15
	go test -v -race -tags=integration ./...
	@echo "Stopping services..."
	docker-compose down

# Run end-to-end tests
test-e2e:
	@echo "Running end-to-end tests..."
	go test -v -race ./test/...

# Run all tests (unit + integration + e2e)
test-all: test-unit test-integration test-e2e

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Check coverage threshold (75%)
coverage-check:
	@echo "Checking coverage threshold (75%)..."
	@go test -coverprofile=coverage.out ./... > /dev/null 2>&1
	@total=$$(go tool cover -func=coverage.out | grep total | awk '{print $$3}' | sed 's/%//'); \
	if [ $$(echo "$$total < 75.0" | bc -l 2>/dev/null || echo "0") -eq 1 ]; then \
		echo "❌ Coverage ($$total%) is below threshold (75%)"; \
		exit 1; \
	else \
		echo "✅ Coverage ($$total%) meets threshold (75%)"; \
	fi

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Pre-commit checks
pre-commit: fmt vet lint test-unit coverage-check
	@echo "✅ All pre-commit checks passed!"

# Validate the example configuration
validate: build
	@echo "Validating example configuration..."
	./${BIN_DIR}/${BINARY_NAME} validate-config --config examples/config.example.yml

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Format complete"

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install from: https://golangci-lint.run/usage/install/"; \
		exit 1; \
	fi

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Check for common issues
check: fmt vet lint
	@echo "All checks passed!"

# Run the daemon in development mode
run-daemon: CGO=1
run-daemon: web-build web-sync-dist build
	@echo "Starting daemon in development mode with fresh web UI..."
	./${BIN_DIR}/${BINARY_NAME} daemon \
		--config config.yml \
		--http :8080 \
		--http-user admin \
		--http-pass changeme

# Development setup
dev:
	@echo "Setting up development environment..."
	go mod download
	go mod tidy
	@echo "Installing development tools..."
	@command -v golangci-lint >/dev/null 2>&1 || \
		(echo "Installing golangci-lint..." && \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@echo "Development environment ready!"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies updated"

# Show module information
mod-info:
	@echo "Go module information:"
	@go list -m all

# Update dependencies
update-deps:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy
	@echo "Dependencies updated"

# Create a test backup directory structure
setup-test-env:
	@echo "Setting up test environment..."
	mkdir -p backups/test
	@echo "Test environment created in ./backups/"

# Docker build (local, single platform)
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):latest \
		--build-arg VERSION=${VERSION} \
		--build-arg COMMIT=${COMMIT} \
		--build-arg BUILD_DATE=${BUILD_TIME} \
		.
	@echo "Docker image built: $(DOCKER_IMAGE):latest"

# Docker build with version tag
docker-build-version:
	@echo "Building Docker image with version ${VERSION}..."
	docker build -t $(DOCKER_IMAGE):${VERSION} -t $(DOCKER_IMAGE):latest \
		--build-arg VERSION=${VERSION} \
		--build-arg COMMIT=${COMMIT} \
		--build-arg BUILD_DATE=${BUILD_TIME} \
		.
	@echo "Docker image built: $(DOCKER_IMAGE):${VERSION}, $(DOCKER_IMAGE):latest"

# Docker build multi-platform (requires buildx)
DOCKER_IMAGE ?= ektowett/bared
DOCKER_PLATFORM ?= $(shell uname -m | grep -qiE 'arm64|aarch64' && echo linux/arm64 || echo linux/amd64)
DOCKER_PLATFORMS ?= linux/amd64,linux/arm64
DOCKER_BUILDX_BUILDER ?= bared-multi

# Local dev: single-platform build that can be loaded into the local Docker engine.
# Note: some Docker backends (e.g. OrbStack) cannot `--load` multi-arch manifest lists.
docker-buildx:
	@echo "Building single-platform Docker image (load into local engine)..."
	docker buildx build --platform $(DOCKER_PLATFORM) \
		-t $(DOCKER_IMAGE):latest \
		--build-arg VERSION=${VERSION} \
		--build-arg COMMIT=${COMMIT} \
		--build-arg BUILD_DATE=${BUILD_TIME} \
		--load \
		.
	@echo "Docker image built and loaded: $(DOCKER_IMAGE):latest (platform: $(DOCKER_PLATFORM))"

# Setup a buildx builder that supports multi-arch builds and register QEMU/binfmt.
# This is required when building linux/amd64 + linux/arm64 on a host that can't natively execute one of them.
docker-buildx-setup:
	@echo "Setting up buildx builder '$(DOCKER_BUILDX_BUILDER)' for multi-arch builds..."
	@docker buildx inspect $(DOCKER_BUILDX_BUILDER) >/dev/null 2>&1 || \
		docker buildx create --name $(DOCKER_BUILDX_BUILDER) --driver docker-container --use
	@docker buildx use $(DOCKER_BUILDX_BUILDER)
	@echo "Registering binfmt/QEMU (best-effort; may require privileged support)..."
	@docker run --privileged --rm tonistiigi/binfmt --install all >/dev/null 2>&1 || true
	@docker buildx inspect --bootstrap >/dev/null
	@echo "✅ buildx is ready:"
	@docker buildx ls | sed -n '1,12p'

# Publishing: multi-platform build that pushes a manifest list to a registry.
# Usage:
#   make docker-buildx-push DOCKER_IMAGE=ektowett/bared
docker-buildx-push: docker-buildx-setup
	@echo "Building multi-platform Docker image (push to registry)..."
	docker buildx build --platform $(DOCKER_PLATFORMS) \
		-t $(DOCKER_IMAGE):latest \
		--build-arg VERSION=${VERSION} \
		--build-arg COMMIT=${COMMIT} \
		--build-arg BUILD_DATE=${BUILD_TIME} \
		--push \
		.
	@echo "Multi-platform Docker image built and pushed: $(DOCKER_IMAGE):latest (platforms: $(DOCKER_PLATFORMS))"

# Docker push to registry
docker-push:
	@echo "Pushing Docker image..."
	docker push $(DOCKER_IMAGE):latest
	docker push $(DOCKER_IMAGE):${VERSION}
	@echo "Docker images pushed: $(DOCKER_IMAGE):latest, $(DOCKER_IMAGE):${VERSION}"

# Docker push latest only
docker-push-latest:
	@echo "Pushing Docker image (latest)..."
	docker push $(DOCKER_IMAGE):latest
	@echo "Docker image pushed: $(DOCKER_IMAGE):latest"

# Docker build and push (complete workflow)
docker-release: docker-build-version docker-push
	@echo "Docker release complete!"

# Docker build multi-platform and push
docker-release-multiplatform:
	@echo "Building and pushing multi-platform Docker image..."
	docker buildx build --platform linux/amd64,linux/arm64 \
		-t $(DOCKER_IMAGE):latest \
		-t $(DOCKER_IMAGE):${VERSION} \
		--build-arg VERSION=${VERSION} \
		--build-arg COMMIT=${COMMIT} \
		--build-arg BUILD_DATE=${BUILD_TIME} \
		--push \
		.
	@echo "Multi-platform Docker images pushed: $(DOCKER_IMAGE):latest, $(DOCKER_IMAGE):${VERSION}"

# Create release archive
release: build
	@echo "Creating release archive..."
	mkdir -p dist
	tar -czf dist/${BINARY_NAME}-${VERSION}.tar.gz -C ${BIN_DIR} ${BINARY_NAME} -C .. README.md examples/ plan.md
	@echo "Release archive created: dist/${BINARY_NAME}-${VERSION}.tar.gz"

# Show Go environment
env:
	@go env

# Docker Compose Commands
# Start all services (detached mode)
compose-up:
	@echo "Starting all services with Docker Compose..."
	docker compose up -d
	@echo "Services started. Use 'make compose-ps' to check status"

# Start all services (foreground mode)
compose-up-fg:
	@echo "Starting all services in foreground..."
	docker compose up

# Stop and remove all containers, networks
compose-down:
	@echo "Stopping and removing all containers..."
	docker compose down
	@echo "All containers stopped and removed"

# Stop and remove all containers, networks, and volumes
compose-down-volumes:
	@echo "Stopping and removing all containers and volumes..."
	docker compose down -v
	@echo "All containers, networks, and volumes removed"

# Restart all services
compose-restart:
	@echo "Restarting all services..."
	docker compose restart
	@echo "All services restarted"

# Stop all services (without removing)
compose-stop:
	@echo "Stopping all services..."
	docker compose stop
	@echo "All services stopped"

# Start all services (that are stopped)
compose-start:
	@echo "Starting all services..."
	docker compose start
	@echo "All services started"

# Show running containers
compose-ps:
	@docker compose ps

# View logs from all services
compose-logs:
	@docker compose logs

# Follow logs from all services
compose-logs-follow:
	@docker compose logs -f

# View logs from specific service (usage: make compose-logs-service SERVICE=bared)
compose-logs-service:
	@docker compose logs $(SERVICE)

# Follow logs from specific service (usage: make compose-logs-service-follow SERVICE=bared)
compose-logs-service-follow:
	@docker compose logs -f $(SERVICE)

# Rebuild and start services
compose-build:
	@echo "Building and starting services..."
	docker compose up -d --build
	@echo "Services built and started"

# Pull latest images
compose-pull:
	@echo "Pulling latest images..."
	docker compose pull
	@echo "Images updated"

# Remove stopped containers
compose-clean:
	@echo "Removing stopped containers..."
	docker compose rm -f
	@echo "Stopped containers removed"

# Remove all containers, volumes, and networks (full cleanup)
compose-clean-all:
	@echo "Performing full cleanup..."
	docker compose down -v --remove-orphans
	@echo "Full cleanup complete"

# Start only database services (mysql, postgres, redis, minio)
compose-services-up:
	@echo "Starting database services only..."
	docker compose up -d mysql postgres redis minio
	@echo "Database services started"

# Stop only database services
compose-services-down:
	@echo "Stopping database services..."
	docker compose stop mysql postgres redis minio
	@echo "Database services stopped"

# Execute command in bared container (usage: make compose-exec CMD="brd list")
compose-exec:
	@docker compose exec bared $(CMD)

# Shell into bared container
compose-shell:
	@docker compose exec bared sh

# Show help
help:
	@echo "BareD Makefile Commands:"
	@echo ""
	@echo "Build Commands:"
	@echo "  make build       - Build the brd binary"
	@echo "  make build-all   - Build for multiple platforms (Linux, macOS, Windows)"
	@echo "  make build-with-web - Build with embedded web UI"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make release     - Create release archive"
	@echo ""
	@echo "Installation Commands:"
	@echo "  make install     - Install brd to /usr/local/bin"
	@echo "  make uninstall   - Remove brd from /usr/local/bin"
	@echo "  make install-service - Install as systemd service"
	@echo ""
	@echo "Testing Commands:"
	@echo "  make test             - Run unit tests (fast, default)"
	@echo "  make test-unit        - Run unit tests only"
	@echo "  make test-integration - Run integration tests (requires Docker)"
	@echo "  make test-e2e         - Run end-to-end tests"
	@echo "  make test-all         - Run all tests (unit + integration + e2e)"
	@echo "  make coverage         - Generate HTML coverage report"
	@echo "  make coverage-check   - Verify coverage meets 75% threshold"
	@echo "  make bench            - Run benchmarks"
	@echo "  make pre-commit       - Run all pre-commit checks"
	@echo "  make validate         - Validate example configuration"
	@echo "  make validate-all     - Validate everything (Go + Web)"
	@echo ""
	@echo "Code Quality Commands:"
	@echo "  make fmt         - Format Go code"
	@echo "  make vet         - Run go vet"
	@echo "  make lint        - Run linter (requires golangci-lint)"
	@echo "  make check       - Run all code quality checks"
	@echo ""
	@echo "Development Commands:"
	@echo "  make dev         - Setup development environment"
	@echo "  make run-daemon  - Run daemon in development mode"
	@echo "  make deps        - Download dependencies"
	@echo "  make update-deps - Update all dependencies"
	@echo "  make setup-test-env - Create test directory structure"
	@echo ""
	@echo "Docker Commands:"
	@echo "  make docker-build                - Build Docker image locally (single platform)"
	@echo "  make docker-build-version        - Build Docker image with version tag"
	@echo "  make docker-buildx               - Build multi-platform image (amd64, arm64)"
	@echo "  make docker-push                 - Push to ektowett/bared (latest + version)"
	@echo "  make docker-push-latest          - Push to ektowett/bared:latest only"
	@echo "  make docker-release              - Build and push (single platform)"
	@echo "  make docker-release-multiplatform - Build and push (multi-platform)"
	@echo ""
	@echo "Docker Compose Commands:"
	@echo "  make compose-up                  - Start all services (detached)"
	@echo "  make compose-up-fg               - Start all services (foreground)"
	@echo "  make compose-down                - Stop and remove all containers"
	@echo "  make compose-down-volumes        - Stop and remove containers + volumes"
	@echo "  make compose-restart             - Restart all services"
	@echo "  make compose-stop                - Stop all services (don't remove)"
	@echo "  make compose-start               - Start stopped services"
	@echo "  make compose-ps                  - Show running containers"
	@echo "  make compose-logs                - View logs from all services"
	@echo "  make compose-logs-follow         - Follow logs from all services"
	@echo "  make compose-logs-service        - View logs from service (SERVICE=name)"
	@echo "  make compose-logs-service-follow - Follow logs from service (SERVICE=name)"
	@echo "  make compose-build               - Rebuild and start services"
	@echo "  make compose-pull                - Pull latest images"
	@echo "  make compose-clean               - Remove stopped containers"
	@echo "  make compose-clean-all           - Full cleanup (containers + volumes)"
	@echo "  make compose-services-up         - Start only database services"
	@echo "  make compose-services-down       - Stop only database services"
	@echo "  make compose-exec                - Execute command (CMD=\"command\")"
	@echo "  make compose-shell               - Open shell in bared container"
	@echo ""
	@echo "Web Frontend Commands:"
	@echo "  make web-install   - Install web frontend dependencies"
	@echo "  make web-build     - Build web frontend"
	@echo "  make web-dev       - Start web frontend development server"
	@echo "  make web-lint      - Lint web frontend"
	@echo "  make web-format    - Format web frontend code"
	@echo "  make web-validate  - Validate web frontend"
	@echo "  make web-clean     - Clean web frontend"
	@echo ""
	@echo "Info Commands:"
	@echo "  make env         - Show Go environment"
	@echo "  make mod-info    - Show module information"
	@echo "  make help        - Show this help message"

# Web Frontend Commands
web-install:
	@echo "Installing web frontend dependencies..."
	cd web && npm install

web-build: web-install
	@echo "Building web frontend..."
	cd web && npm run build

web-dev:
	@echo "Starting web frontend development server..."
	cd web && npm run dev

web-lint:
	@echo "Linting web frontend..."
	cd web && npm run lint

web-format:
	@echo "Formatting web frontend code..."
	cd web && npm run format

web-validate: web-install
	@echo "Validating web frontend..."
	cd web && npm run validate

web-clean:
	@echo "Cleaning web frontend..."
	rm -rf web/dist web/node_modules

# Sync built web assets into the Go embed directory (internal/web/dist)
web-sync-dist:
	@echo "Syncing web assets into internal/web/dist..."
	@mkdir -p internal/web/dist
	@if [ -d "web/dist" ]; then \
		rm -rf internal/web/dist/*; \
		cp -R web/dist/. internal/web/dist/; \
	else \
		echo "web/dist not found; leaving internal/web/dist as-is"; \
	fi

# Build with web frontend
build-with-web: web-build web-sync-dist build
	@echo "Build complete with embedded web UI"

# Validate everything (Go + Web)
validate-all: validate web-validate
	@echo "✅ All validation passed (Go + Web)!"
