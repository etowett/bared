.PHONY: all build build-all clean install install-service uninstall release env check deps update-deps mod-info agents-doctor agents-sync setup-test-env test test-unit test-integration test-e2e test-all coverage coverage-check coverage-filter coverage-verify bench pre-commit validate validate-all fmt lint vet run-daemon dev help check-bun-pin web-install web-build web-dev web-clean web-lint web-validate web-format web-sync-dist build-with-web docker-build docker-build-version docker-buildx docker-buildx-setup docker-buildx-local docker-buildx-push docker-push docker-push-latest docker-release docker-release-multiplatform compose-up compose-up-fg compose-down compose-down-volumes compose-restart compose-stop compose-start compose-ps compose-logs compose-logs-follow compose-logs-service compose-logs-service-follow compose-build compose-pull compose-clean compose-clean-all compose-services-up compose-services-down compose-exec compose-shell

# Default target
all: build

# Application directories. The repo is a monorepo: each deployable app lives
# under apps/. The Go module root is apps/api (module path
# `github.com/etowett/bared/apps/api`); the dashboard is a separate Bun project.
API_DIR=apps/api
WEB_DIR=apps/web

# Every Go invocation runs inside the module directory. Output paths are made
# absolute with $(CURDIR) so artifacts still land at the repo root.
GO=go -C $(API_DIR)

# The dashboard's package manager. Bun's version is pinned by
# $(WEB_DIR)/.bun-version, which CI and the Dockerfile read too. Overridable so
# a caller can point at a specific install (e.g. BUN=~/.bun/bin/bun).
# Note: `bun --cwd` needs an absolute path, so the web targets cd first.
BUN ?= bun

# Build variables
BINARY_NAME=brd
BIN_DIR=bin
# --match is load-bearing: releases also push an apps/api/vX.Y.Z Go module tag
# (see .github/workflows/auto-release.yml), and a bare `git describe` picks that
# one over vX.Y.Z, stamping the binary "apps/api/v0.5.0-2-gabc1234". Restrict to
# root version tags so `brd --version` reports a version and not a module path.
VERSION=$(shell git describe --tags --always --dirty --match 'v[0-9]*' 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-ldflags "-X github.com/etowett/bared/apps/api/internal/version.Version=${VERSION} -X github.com/etowett/bared/apps/api/internal/version.Commit=${COMMIT} -X github.com/etowett/bared/apps/api/internal/version.BuildDate=${BUILD_TIME}"

# Docker build variables
DOCKER_IMAGE ?= ektowett/bared
DOCKER_PLATFORM ?= $(shell uname -m | grep -qiE 'arm64|aarch64' && echo linux/arm64 || echo linux/amd64)
DOCKER_PLATFORMS ?= linux/amd64,linux/arm64
DOCKER_BUILDX_BUILDER ?= bared-multi

# Package list for test/vet/coverage targets. Recursively expanded so `go list`
# only runs when a recipe actually needs it.
#
# node_modules used to leak in here: npm registry packages occasionally ship Go
# sources (web/node_modules/flatted/golang), and back when the module root was
# the repo root, `./...` swept them up once dependencies had been installed.
# With the module rooted at apps/api and the dashboard at apps/web,
# node_modules is outside the module entirely and no filtering is needed.
GO_PKGS = $(shell cd $(API_DIR) && go list ./...)

# Coverage policy (see #53)
# -------------------------
# COVERAGE_OUT  where the profile lands. Absolute, because $(GO) runs with
#               -C $(API_DIR) and everything else expects it at the repo root.
# COVERAGE_EXCLUDE
#               grep -E pattern of profile lines to drop before computing the
#               total. Test-helper packages exist to support tests; their
#               statements are not production code and should not dilute the
#               number in either direction. Matched as a *path fragment*, not an
#               import-path prefix, so it keeps working after the module rename
#               in #79 (`bared/...` -> `github.com/etowett/bared/apps/api/...`).
# COVERAGE_THRESHOLD
#               a ratchet, not an aspiration. It sits just under the real
#               measured number so the gate is green on a clean checkout and
#               goes red the moment coverage slips. Raise it as coverage
#               improves; never lower it to turn a red gate green.
COVERAGE_OUT = $(CURDIR)/coverage.out
COVERAGE_EXCLUDE ?= /testutil/
COVERAGE_THRESHOLD ?= 32.0

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

# Build the binary (with web UI by default)
build: web-build web-sync-dist
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p ${BIN_DIR}
	CGO_ENABLED=$(CGO) $(GO) build ${LDFLAGS} -o $(CURDIR)/${BIN_DIR}/${BINARY_NAME} ./cmd/brd
	@echo "Build complete: ./${BIN_DIR}/${BINARY_NAME}"

# Build for multiple platforms (with web UI by default)
build-all: web-build web-sync-dist
	@echo "Building for multiple platforms..."
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build ${LDFLAGS} -o $(CURDIR)/dist/${BINARY_NAME}-linux-amd64 ./cmd/brd
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build ${LDFLAGS} -o $(CURDIR)/dist/${BINARY_NAME}-linux-arm64 ./cmd/brd
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build ${LDFLAGS} -o $(CURDIR)/dist/${BINARY_NAME}-darwin-amd64 ./cmd/brd
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build ${LDFLAGS} -o $(CURDIR)/dist/${BINARY_NAME}-darwin-arm64 ./cmd/brd
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build ${LDFLAGS} -o $(CURDIR)/dist/${BINARY_NAME}-windows-amd64.exe ./cmd/brd
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
	$(GO) test -v -race -short $(GO_TEST_LDFLAGS) -coverprofile=$(CURDIR)/coverage.out $(GO_PKGS)

# Run integration tests (requires Docker services)
test-integration:
	@echo "Running integration tests (requires Docker)..."
	@echo "Starting services..."
	docker-compose up -d mysql postgres redis minio
	@echo "Waiting for services to be ready..."
	sleep 15
	$(GO) test -v -race -tags=integration $(GO_PKGS)
	@echo "Stopping services..."
	docker-compose down

# Run end-to-end tests
test-e2e:
	@echo "Running end-to-end tests..."
	$(GO) test -v -race ./test/...

# Run all tests (unit + integration + e2e)
test-all: test-unit test-integration test-e2e

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	$(GO) test -v -race -coverprofile=$(COVERAGE_OUT) $(GO_PKGS)
	@$(MAKE) --no-print-directory coverage-filter
	$(GO) tool cover -html=$(COVERAGE_OUT) -o $(CURDIR)/coverage.html
	@echo "Coverage report generated: coverage.html"

# Drop excluded packages from the profile, in place, so every consumer (the HTML
# report, the ratchet, CI) sees the same statements. Skipped when the pattern is
# empty — `grep -v ''` would otherwise delete the whole profile.
coverage-filter:
	@if [ -n "$(COVERAGE_EXCLUDE)" ] && [ -f $(COVERAGE_OUT) ]; then \
		grep -Ev '$(COVERAGE_EXCLUDE)' $(COVERAGE_OUT) > $(COVERAGE_OUT).tmp && mv $(COVERAGE_OUT).tmp $(COVERAGE_OUT); \
	fi

# Run the suite, then check the profile against the ratchet.
coverage-check:
	@log=$$(mktemp); \
	if ! $(GO) test -coverprofile=$(COVERAGE_OUT) $(GO_PKGS) > $$log 2>&1; then \
		echo "❌ Tests failed:"; cat $$log; rm -f $$log; exit 1; \
	fi; \
	rm -f $$log
	@$(MAKE) --no-print-directory coverage-verify

# Check an *existing* profile against the ratchet. Split out from coverage-check
# so CI can reuse the profile its test job already produced instead of running
# the whole suite a second time.
coverage-verify: coverage-filter
	@echo "Checking coverage threshold ($(COVERAGE_THRESHOLD)%, excluding '$(COVERAGE_EXCLUDE)')..."
	@test -f $(COVERAGE_OUT) || { echo "❌ No coverage profile at $(COVERAGE_OUT) — run 'make coverage' first"; exit 1; }
	@$(GO) tool cover -func=$(COVERAGE_OUT) | awk -v min="$(COVERAGE_THRESHOLD)" '\
		/^total:/ { total = $$3; sub(/%/, "", total) } \
		END { \
			if (total == "") { print "❌ Could not read a total from the coverage profile"; exit 1 } \
			if (total + 0 < min + 0) { \
				printf "❌ Coverage (%s%%) is below the ratchet (%s%%)\n", total, min; \
				print "   Add tests for what you changed — do not lower COVERAGE_THRESHOLD."; \
				exit 1 \
			} \
			printf "✅ Coverage (%s%%) meets the ratchet (%s%%)\n", total, min; \
		}'

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem $(GO_PKGS)

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
	$(GO) fmt ./...
	@echo "Format complete"

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		cd $(API_DIR) && golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install from: https://golangci-lint.run/usage/install/"; \
		exit 1; \
	fi

# Run go vet
vet:
	@echo "Running go vet..."
	$(GO) vet $(GO_PKGS)

# Check for common issues
check: fmt vet lint
	@echo "All checks passed!"

# Run the daemon in development mode
run-daemon: CGO=1
run-daemon: web-build web-sync-dist build
	@echo "Starting daemon in development mode with fresh web UI..."
	./${BIN_DIR}/${BINARY_NAME} daemon \
		--http :8080 \
		--http-user admin \
		--http-pass changeme

# Run the daemon without rebuilding the web UI (faster for backend-only development).
# Compiles rather than `go run` so the daemon still starts with the repo root as
# its working directory — config paths and the sqlite file are resolved from there.
run-daemon-fast:
	@echo "Starting daemon in development mode (no web UI rebuild)..."
	@mkdir -p ${BIN_DIR}
	CGO_ENABLED=1 $(GO) build ${LDFLAGS} -o $(CURDIR)/${BIN_DIR}/${BINARY_NAME} ./cmd/brd
	./${BIN_DIR}/${BINARY_NAME} daemon \
		--http :8080 \
		--http-user admin \
		--http-pass changeme

# Development setup
dev:
	@echo "Setting up development environment..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "Installing development tools..."
	@command -v golangci-lint >/dev/null 2>&1 || \
		(echo "Installing golangci-lint..." && \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@echo "Development environment ready!"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "Dependencies updated"

# Show module information
mod-info:
	@echo "Go module information:"
	@$(GO) list -m all

# Update dependencies
update-deps:
	@echo "Updating dependencies..."
	$(GO) get -u ./...
	$(GO) mod tidy
	@echo "Dependencies updated"

# Check that the Claude Code <-> Codex agent config mirrors are in sync
agents-doctor:
	@python3 scripts/agents-doctor.py

# Regenerate the derivable agent config (Codex subagents, hook permissions), then check
agents-sync:
	@python3 scripts/agents-doctor.py --fix

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
		docker buildx create --driver docker-container --use $(DOCKER_BUILDX_BUILDER)
	@docker buildx use $(DOCKER_BUILDX_BUILDER)
	@echo "Registering binfmt/QEMU (best-effort; may require privileged support)..."
	@docker run --privileged --rm tonistiigi/binfmt --install all >/dev/null 2>&1 || true
	@docker buildx inspect --bootstrap >/dev/null
	@echo "✅ buildx is ready:"
	@docker buildx ls | sed -n '1,12p'

# Local multi-platform build (builds locally into buildx cache, no push/load).
# Useful for testing multi-platform builds before pushing.
# Note: Images are stored in the buildx builder cache and can be pushed later.
# Usage:
#   make docker-buildx-local DOCKER_IMAGE=ektowett/bared
docker-buildx-local: docker-buildx-setup
	@echo "Building multi-platform Docker image locally (no push/load)..."
	docker buildx build --platform $(DOCKER_PLATFORMS) \
		-t $(DOCKER_IMAGE):latest \
		-t $(DOCKER_IMAGE):${VERSION} \
		--build-arg VERSION=${VERSION} \
		--build-arg COMMIT=${COMMIT} \
		--build-arg BUILD_DATE=${BUILD_TIME} \
		.
	@echo "Multi-platform Docker image built locally: $(DOCKER_IMAGE):latest, $(DOCKER_IMAGE):${VERSION} (platforms: $(DOCKER_PLATFORMS))"
	@echo "Images are in buildx cache. Use 'make docker-buildx-push' to push them."

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
	docker buildx build --platform $(DOCKER_PLATFORMS) \
		-t $(DOCKER_IMAGE):latest \
		-t $(DOCKER_IMAGE):${VERSION} \
		--build-arg VERSION=${VERSION} \
		--build-arg COMMIT=${COMMIT} \
		--build-arg BUILD_DATE=${BUILD_TIME} \
		--push \
		.
	@echo "Multi-platform Docker images pushed: $(DOCKER_IMAGE):latest, $(DOCKER_IMAGE):${VERSION} (platforms: $(DOCKER_PLATFORMS))"

# Create release archive (current platform only)
release: build
	@echo "Creating release archive for current platform..."
	mkdir -p dist
	tar -czf dist/${BINARY_NAME}-${VERSION}.tar.gz -C ${BIN_DIR} ${BINARY_NAME} -C .. README.md examples/ docs/
	@echo "Release archive created: dist/${BINARY_NAME}-${VERSION}.tar.gz"

# Create release archives for all platforms
release-all: build-all
	@echo "Creating release archives for all platforms..."
	@mkdir -p dist

	# Linux amd64
	@echo "Packaging linux-amd64..."
	@tar -czf dist/${BINARY_NAME}-${VERSION}-linux-amd64.tar.gz \
		-C dist ${BINARY_NAME}-linux-amd64 \
		-C .. README.md examples/ docs/

	# Linux arm64
	@echo "Packaging linux-arm64..."
	@tar -czf dist/${BINARY_NAME}-${VERSION}-linux-arm64.tar.gz \
		-C dist ${BINARY_NAME}-linux-arm64 \
		-C .. README.md examples/ docs/

	# macOS amd64 (Intel)
	@echo "Packaging darwin-amd64..."
	@tar -czf dist/${BINARY_NAME}-${VERSION}-darwin-amd64.tar.gz \
		-C dist ${BINARY_NAME}-darwin-amd64 \
		-C .. README.md examples/ docs/

	# macOS arm64 (Apple Silicon)
	@echo "Packaging darwin-arm64..."
	@tar -czf dist/${BINARY_NAME}-${VERSION}-darwin-arm64.tar.gz \
		-C dist ${BINARY_NAME}-darwin-arm64 \
		-C .. README.md examples/ docs/

	# Windows amd64
	@echo "Packaging windows-amd64..."
	@cd dist && zip -q ${BINARY_NAME}-${VERSION}-windows-amd64.zip \
		${BINARY_NAME}-windows-amd64.exe
	@cd . && tar -czf dist/${BINARY_NAME}-${VERSION}-windows-amd64-extras.tar.gz \
		README.md examples/ docs/

	@echo ""
	@echo "✅ Multi-platform release archives created in ./dist/:"
	@echo "   - ${BINARY_NAME}-${VERSION}-linux-amd64.tar.gz"
	@echo "   - ${BINARY_NAME}-${VERSION}-linux-arm64.tar.gz"
	@echo "   - ${BINARY_NAME}-${VERSION}-darwin-amd64.tar.gz"
	@echo "   - ${BINARY_NAME}-${VERSION}-darwin-arm64.tar.gz"
	@echo "   - ${BINARY_NAME}-${VERSION}-windows-amd64.zip"
	@echo "   - ${BINARY_NAME}-${VERSION}-windows-amd64-extras.tar.gz"

# Show Go environment
env:
	@$(GO) env

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
	@echo "  make build       - Build the brd binary (with embedded web UI)"
	@echo "  make build-all   - Build for multiple platforms (Linux, macOS, Windows) with web UI"
	@echo "  make build-with-web - Alias for build (web UI is now included by default)"
	@echo "  make clean       - Remove build artifacts"
	@echo "  make release     - Create release archive (current platform)"
	@echo "  make release-all - Create release archives for all platforms"
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
	@echo "  make coverage-check   - Run tests and check the $(COVERAGE_THRESHOLD)% coverage ratchet"
	@echo "  make coverage-verify  - Check an existing coverage.out against the ratchet"
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
	@echo "Agent Tooling Commands:"
	@echo "  make agents-doctor - Check the Claude Code <-> Codex config mirrors"
	@echo "  make check-bun-pin - Check the Bun version matches across its three pins"
	@echo "  make agents-sync   - Regenerate the derivable agent config, then check"
	@echo ""
	@echo "Docker Commands:"
	@echo "  make docker-build                - Build Docker image locally (single platform)"
	@echo "  make docker-build-version        - Build Docker image with version tag"
	@echo "  make docker-buildx               - Build single-platform image for local use (auto-detect or set DOCKER_PLATFORM)"
	@echo "  make docker-buildx-setup         - Setup buildx builder for multi-arch builds"
	@echo "  make docker-buildx-local         - Build multi-platform image locally (no push, stored in buildx cache)"
	@echo "  make docker-buildx-push          - Build and push multi-platform image to registry"
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
	@echo "Web Frontend Commands (Bun; version pinned by $(WEB_DIR)/.bun-version):"
	@echo "  make web-install   - Install web frontend dependencies (bun install)"
	@echo "  make web-build     - Build web frontend"
	@echo "  make web-dev       - Start web frontend development server"
	@echo "  make web-lint      - Lint web frontend"
	@echo "  make web-format    - Format web frontend code"
	@echo "  make web-validate  - Validate web frontend (types + lint + format + tests)"
	@echo "  make web-clean     - Clean web frontend (dist + node_modules)"
	@echo "  Override the package manager with BUN=/path/to/bun"
	@echo ""
	@echo "Info Commands:"
	@echo "  make env         - Show Go environment"
	@echo "  make mod-info    - Show module information"
	@echo "  make help        - Show this help message"

# Guard against the Bun version drifting between the THREE places that pin it.
#
#   1. $(WEB_DIR)/.bun-version  — read by oven-sh/setup-bun in CI, and what local
#                                 dev should install
#   2. Dockerfile               — `FROM oven/bun:<tag>`; a Dockerfile cannot read
#                                 .bun-version, so this is an independent pin
#   3. $(WEB_DIR)/package.json  — the "packageManager" field
#
# Dependabot's docker ecosystem bumps only the Dockerfile and cannot know about
# the other two, so the first time it ran (PR #64) all three desynced: the
# Dockerfile went to 1.3.14 while .bun-version and packageManager stayed at
# 1.3.5. This turns that into a hard failure instead of a silent split-brain.
#
# The Go pin needs no equivalent: CI reads go-version-file: apps/api/go.mod
# directly, so only the Dockerfile duplicates it.
check-bun-pin:
	@want="$$(tr -d '[:space:]' < $(WEB_DIR)/.bun-version)"; \
	docker_pin="$$(grep -oE 'oven/bun:[0-9]+\.[0-9]+\.[0-9]+' Dockerfile | head -1 | cut -d: -f2)"; \
	pkg_pin="$$(grep -oE '"packageManager"[[:space:]]*:[[:space:]]*"bun@[0-9]+\.[0-9]+\.[0-9]+"' $(WEB_DIR)/package.json | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')"; \
	rc=0; \
	if [ -z "$$want" ]; then echo "❌ check-bun-pin: $(WEB_DIR)/.bun-version is empty or missing"; exit 1; fi; \
	if [ -z "$$docker_pin" ]; then \
		echo "❌ check-bun-pin: no pinned oven/bun:<x.y.z> tag found in Dockerfile"; rc=1; \
	elif [ "$$want" != "$$docker_pin" ]; then \
		echo "❌ check-bun-pin: Dockerfile pins oven/bun:$$docker_pin, expected $$want"; rc=1; \
	fi; \
	if [ -z "$$pkg_pin" ]; then \
		echo "❌ check-bun-pin: no \"packageManager\": \"bun@<x.y.z>\" found in $(WEB_DIR)/package.json"; rc=1; \
	elif [ "$$want" != "$$pkg_pin" ]; then \
		echo "❌ check-bun-pin: packageManager pins bun@$$pkg_pin, expected $$want"; rc=1; \
	fi; \
	if [ $$rc -ne 0 ]; then \
		echo "   All three must match $(WEB_DIR)/.bun-version ($$want). Update them together."; \
		exit 1; \
	fi; \
	echo "✅ check-bun-pin: Bun $$want pinned consistently in .bun-version, Dockerfile and package.json"

# Web Frontend Commands
# `bun install` (not --frozen-lockfile) so local dev can still add a dependency;
# CI and the Docker build use --frozen-lockfile for reproducibility.
web-install:
	@echo "Installing web frontend dependencies..."
	cd $(WEB_DIR) && $(BUN) install

web-build: web-install
	@echo "Building web frontend..."
	cd $(WEB_DIR) && $(BUN) run build

web-dev:
	@echo "Starting web frontend development server..."
	cd $(WEB_DIR) && $(BUN) run dev

web-lint:
	@echo "Linting web frontend..."
	cd $(WEB_DIR) && $(BUN) run lint

web-format:
	@echo "Formatting web frontend code..."
	cd $(WEB_DIR) && $(BUN) run format

web-validate: web-install
	@echo "Validating web frontend..."
	cd $(WEB_DIR) && $(BUN) run validate

web-clean:
	@echo "Cleaning web frontend..."
	rm -rf $(WEB_DIR)/dist $(WEB_DIR)/node_modules

# Sync built web assets into the Go embed directory ($(API_DIR)/internal/web/dist)
web-sync-dist:
	@echo "Syncing web assets into $(API_DIR)/internal/web/dist..."
	@mkdir -p $(API_DIR)/internal/web/dist
	@if [ -d "$(WEB_DIR)/dist" ]; then \
		rm -rf $(API_DIR)/internal/web/dist/*; \
		cp -R $(WEB_DIR)/dist/. $(API_DIR)/internal/web/dist/; \
	else \
		echo "$(WEB_DIR)/dist not found; leaving $(API_DIR)/internal/web/dist as-is"; \
	fi

# Build with web frontend
build-with-web: web-build web-sync-dist build
	@echo "Build complete with embedded web UI"

# Validate everything (Go + Web)
validate-all: validate web-validate
	@echo "✅ All validation passed (Go + Web)!"
