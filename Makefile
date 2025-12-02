.PHONY: all build clean install uninstall test coverage validate fmt lint vet run-daemon dev help

# Default target
all: build

# Build variables
BINARY_NAME=brd
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

# Build the binary
build:
	@echo "Building ${BINARY_NAME}..."
	go build ${LDFLAGS} -o ${BINARY_NAME} ./cmd/brd
	@echo "Build complete: ./${BINARY_NAME}"

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-linux-amd64 ./cmd/brd
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-amd64 ./cmd/brd
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-darwin-arm64 ./cmd/brd
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o dist/${BINARY_NAME}-windows-amd64.exe ./cmd/brd
	@echo "Cross-platform builds complete in ./dist/"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f ${BINARY_NAME}
	rm -rf dist/
	rm -f coverage.out coverage.html
	@echo "Clean complete"

# Install to /usr/local/bin
install: build
	@echo "Installing ${BINARY_NAME} to /usr/local/bin..."
	sudo cp ${BINARY_NAME} /usr/local/bin/
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

# Run tests
test:
	@echo "Running tests..."
	go test -v -race ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Validate the example configuration
validate: build
	@echo "Validating example configuration..."
	./${BINARY_NAME} validate-config --config examples/config.example.yml

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
	./${BINARY_NAME} daemon --config examples/config.example.yml

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

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t bared:latest .
	@echo "Docker image built: bared:latest"

# Create release archive
release: build
	@echo "Creating release archive..."
	mkdir -p dist
	tar -czf dist/${BINARY_NAME}-${VERSION}.tar.gz ${BINARY_NAME} README.md examples/ plan.md
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
	@echo "  make test        - Run tests"
	@echo "  make coverage    - Run tests with coverage report"
	@echo "  make bench       - Run benchmarks"
	@echo "  make validate    - Validate example configuration"
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
	@echo "  make docker-build - Build Docker image"
	@echo ""
	@echo "Info Commands:"
	@echo "  make env         - Show Go environment"
	@echo "  make mod-info    - Show module information"
	@echo "  make help        - Show this help message"
