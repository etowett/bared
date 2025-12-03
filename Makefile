.PHONY: all build clean install uninstall test test-unit test-integration test-e2e test-all coverage coverage-check bench pre-commit validate fmt lint vet run-daemon dev help web-install web-build web-dev web-clean web-lint web-validate web-format build-with-web validate-all

# Default target
all: build

# Build variables
BINARY_NAME=brd
BIN_DIR=bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-X bared/internal/version.Version=${VERSION} -X bared/internal/version.Commit=${COMMIT} -X bared/internal/version.BuildDate=${BUILD_TIME}"

# Build the binary
build:
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p ${BIN_DIR}
	CGO_ENABLED=0 go build ${LDFLAGS} -o ${BIN_DIR}/${BINARY_NAME} ./cmd/brd
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
	go test -v -race -short -coverprofile=coverage.out ./...

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
run-daemon: build
	@echo "Starting daemon in development mode..."
	./${BIN_DIR}/${BINARY_NAME} daemon --config examples/config.example.yml

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
	docker build -t bared:latest \
		--build-arg VERSION=${VERSION} \
		--build-arg COMMIT=${COMMIT} \
		--build-arg BUILD_DATE=${BUILD_TIME} \
		.
	@echo "Docker image built: bared:latest"

# Docker build with version tag
docker-build-version:
	@echo "Building Docker image with version ${VERSION}..."
	docker build -t bared:${VERSION} -t bared:latest \
		--build-arg VERSION=${VERSION} \
		--build-arg COMMIT=${COMMIT} \
		--build-arg BUILD_DATE=${BUILD_TIME} \
		.
	@echo "Docker image built: bared:${VERSION}, bared:latest"

# Docker build multi-platform (requires buildx)
docker-buildx:
	@echo "Building multi-platform Docker image..."
	docker buildx build --platform linux/amd64,linux/arm64 \
		-t bared:latest \
		--build-arg VERSION=${VERSION} \
		--build-arg COMMIT=${COMMIT} \
		--build-arg BUILD_DATE=${BUILD_TIME} \
		--load \
		.
	@echo "Multi-platform Docker image built: bared:latest"

# Docker push to registry (ektowett/bared)
docker-push:
	@echo "Tagging and pushing Docker image to ektowett/bared..."
	docker tag bared:latest ektowett/bared:latest
	docker tag bared:latest ektowett/bared:${VERSION}
	docker push ektowett/bared:latest
	docker push ektowett/bared:${VERSION}
	@echo "Docker images pushed: ektowett/bared:latest, ektowett/bared:${VERSION}"

# Docker push latest only
docker-push-latest:
	@echo "Tagging and pushing Docker image to ektowett/bared:latest..."
	docker tag bared:latest ektowett/bared:latest
	docker push ektowett/bared:latest
	@echo "Docker image pushed: ektowett/bared:latest"

# Docker build and push (complete workflow)
docker-release: docker-build-version docker-push
	@echo "Docker release complete!"

# Docker build multi-platform and push
docker-release-multiplatform:
	@echo "Building and pushing multi-platform Docker image..."
	docker buildx build --platform linux/amd64,linux/arm64 \
		-t ektowett/bared:latest \
		-t ektowett/bared:${VERSION} \
		--build-arg VERSION=${VERSION} \
		--build-arg COMMIT=${COMMIT} \
		--build-arg BUILD_DATE=${BUILD_TIME} \
		--push \
		.
	@echo "Multi-platform Docker images pushed: ektowett/bared:latest, ektowett/bared:${VERSION}"

# Create release archive
release: build
	@echo "Creating release archive..."
	mkdir -p dist
	tar -czf dist/${BINARY_NAME}-${VERSION}.tar.gz -C ${BIN_DIR} ${BINARY_NAME} -C .. README.md examples/ plan.md
	@echo "Release archive created: dist/${BINARY_NAME}-${VERSION}.tar.gz"

# Show Go environment
env:
	@go env

# Show help
help:
	@echo "BareD Makefile Commands:"
	@echo ""
	@echo "Build Commands:"
	@echo "  make build       - Build the brd binary"
	@echo "  make build-all   - Build for multiple platforms (Linux, macOS, Windows)"
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

# Build with web frontend
build-with-web: web-build build
	@echo "Build complete with embedded web UI"

# Validate everything (Go + Web)
validate-all: validate web-validate
	@echo "✅ All validation passed (Go + Web)!"
