# Development Tooling & Repository Setup

This document explains all the development tools, configurations, and supporting files in this repository.

## 📁 Repository Structure

### Core Files

| File | Purpose |
|------|---------|
| `Makefile` | Build automation with 30+ commands |
| `.gitignore` | Git ignore patterns for build artifacts, configs, logs |
| `Dockerfile` | Multi-stage Docker build with database clients |
| `docker-compose.yml` | Development stack (MySQL, PostgreSQL, Redis, MinIO) |
| `.golangci.yml` | Linter configuration with security checks |

### Documentation

| File | Purpose |
|------|---------|
| `README.md` | User documentation and quick start |
| `CONTRIBUTING.md` | Contribution guidelines and process |
| `DEVELOPMENT.md` | Detailed development guide |
| `plan.md` | Original implementation plan (reference) |
| `IMPLEMENTATION_COMPLETE.md` | Project completion summary |

### CI/CD

| File | Purpose |
|------|---------|
| `.github/workflows/ci.yml` | Continuous integration (tests, lint, build) |
| `.github/workflows/release.yml` | Automated releases on tag push |

### IDE Configuration

| File | Purpose |
|------|---------|
| `.vscode/settings.json` | VS Code editor settings for Go |
| `.vscode/launch.json` | Debug configurations for all commands |

## 🛠️ Makefile Commands

### Quick Reference

```bash
# Most common commands
make build          # Build the binary
make test           # Run tests
make validate       # Validate example config
make dev            # Setup development environment
make help           # Show all commands
```

### All Commands

#### Build Commands

```bash
make build          # Build brd binary
make build-all      # Cross-compile for Linux, macOS, Windows
make clean          # Remove build artifacts
make release        # Create release archive
```

#### Installation Commands

```bash
make install        # Install to /usr/local/bin
make uninstall      # Remove from /usr/local/bin
make install-service # Install as systemd service
```

#### Testing Commands

```bash
make test           # Run all tests with race detector
make coverage       # Generate HTML coverage report
make bench          # Run benchmarks
make validate       # Validate example configuration
```

#### Code Quality Commands

```bash
make fmt            # Format Go code with gofmt
make vet            # Run go vet static analysis
make lint           # Run golangci-lint
make check          # Run fmt + vet + lint
```

#### Development Commands

```bash
make dev            # Install dev tools (golangci-lint, etc.)
make run-daemon     # Run daemon with example config
make deps           # Download dependencies
make update-deps    # Update all dependencies to latest
make setup-test-env # Create test directory structure
```

#### Info Commands

```bash
make env            # Show Go environment variables
make mod-info       # Show Go module dependencies
make help           # Show all available commands
```

## 🐳 Docker & Docker Compose

### Dockerfile

Multi-stage build that:

1. **Build stage**: Compiles Go binary with static linking
2. **Runtime stage**: Minimal Alpine image with database clients

Features:

- Non-root user (backup:1000)
- Database client tools pre-installed (mysql-client, postgresql-client, redis)
- Volumes for `/backups` and `/etc/bared`

Usage:

```bash
# Build image
docker build -t bared:latest .

# Run backup
docker run --rm \
  -v $(pwd)/config.yml:/etc/bared/bared.yml:ro \
  -v $(pwd)/backups:/backups \
  bared:latest backup --config /etc/bared/bared.yml --target mydb
```

### Docker Compose

Complete development stack with:

- **BareD daemon**: Main application
- **MySQL 8.0**: Test database
- **PostgreSQL 15**: Test database
- **Redis 7**: Test database
- **MinIO**: S3-compatible storage for testing

Usage:

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f bared

# Connect to MySQL
docker-compose exec mysql mysql -ubackup -ptestpass testdb

# Stop services
docker-compose down

# Clean up volumes
docker-compose down -v
```

## 🔍 Linter Configuration

### .golangci.yml

Enabled linters:

- **errcheck**: Unchecked errors
- **gosimple**: Code simplification
- **govet**: Static analysis
- **ineffassign**: Ineffectual assignments
- **staticcheck**: Advanced static analysis
- **unused**: Unused code detection
- **gofmt**: Formatting
- **goimports**: Import organization
- **misspell**: Spell checking
- **revive**: Fast, configurable linting
- **unconvert**: Unnecessary conversions
- **unparam**: Unused parameters
- **gosec**: Security vulnerabilities

Custom exclusions:

- G204: Subprocess with variable (needed for database commands)
- G401: Weak crypto primitives (not doing cryptography)

Run with:

```bash
make lint

# Or directly
golangci-lint run

# Auto-fix issues
golangci-lint run --fix
```

## 🚀 GitHub Actions

### CI Workflow (.github/workflows/ci.yml)

Runs on: Push and pull requests to `main` and `develop`

Jobs:

1. **Test**: Matrix testing on Ubuntu + macOS with Go 1.24 & 1.25
   - Run tests with race detector
   - Generate coverage report
   - Upload to Codecov
2. **Lint**: Run golangci-lint with full checks
3. **Build**: Build binary and validate example config

### Release Workflow (.github/workflows/release.yml)

Triggers on: Git tags matching `v*.*.*`

Automatically:

1. Build binaries for:
   - Linux (amd64, arm64)
   - macOS (amd64, arm64)
   - Windows (amd64)
2. Generate SHA256 checksums
3. Create release archives (.tar.gz, .zip)
4. Generate changelog from commits
5. Create GitHub release with all artifacts

Create a release:

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
# GitHub Actions handles the rest!
```

## 💻 VS Code Configuration

### Settings (.vscode/settings.json)

Features:

- Go language server enabled
- golangci-lint integration
- Format on save
- Organize imports on save
- Test coverage visualization
- Sensible file exclusions

### Launch Configurations (.vscode/launch.json)

Debug configurations for:

- General BareD execution
- Backup command
- List backups
- Restore backup
- Config validation
- Daemon mode
- Unit tests

Usage: Press F5 in VS Code and select a configuration

## 📝 Git Ignore Patterns

### .gitignore

Ignores:

- **Binaries**: `brd`, `*.exe`, `*.dll`, etc.
- **Build artifacts**: `dist/`, `coverage.out`
- **IDE files**: `.vscode/`, `.idea/`, `.DS_Store`
- **Config files**: `bared.yml`, `*.local.yml` (avoid committing secrets)
- **Generated files**: `.bared/` (tracker files)
- **Logs**: `*.log`, `logs/`
- **Environment**: `.env*`

Safe to commit:

- Example configs (`examples/*.yml`)
- VS Code settings (`.vscode/`)
- Source code (`apps/api/cmd/`, `apps/api/internal/`)

## 🏗️ Project Scaffolding

### Creating New Components

The repository structure follows Go best practices:

**Add a new database:**

```bash
# 1. Create implementation
touch apps/api/internal/database/mongodb.go

# 2. Update factory
vim apps/api/internal/database/factory.go

# 3. Add tests
touch apps/api/internal/database/mongodb_test.go

# 4. Update example
vim examples/config.example.yml
```

**Add a new storage backend:**

```bash
touch apps/api/internal/storage/gcs.go
vim apps/api/internal/storage/factory.go
touch apps/api/internal/storage/gcs_test.go
```

**Add a new notifier:**

```bash
touch apps/api/internal/notify/discord.go
vim apps/api/internal/notify/factory.go
```

## 📦 Dependency Management

### Go Modules

```bash
# Download dependencies
go mod download

# Update dependencies
go get -u ./...
go mod tidy

# View dependency tree
go mod graph

# Why is a package needed?
go mod why github.com/pkg/sftp
```

### Using the Makefile

```bash
make deps          # Download dependencies
make update-deps   # Update to latest versions
make mod-info      # Show all dependencies
```

## 🧪 Testing Infrastructure

### Unit Tests

Run with:

```bash
make test

# Specific package
go test -v ./internal/config/

# With coverage
make coverage
open coverage.html
```

### Integration Tests

Use Docker Compose:

```bash
# Start test services
docker-compose up -d

# Run integration tests
go test -tags=integration -v ./...

# Cleanup
docker-compose down -v
```

### Manual Testing Environment

```bash
# Setup
make setup-test-env

# Creates:
# - backups/test/
# - Example configs
```

## 🔧 Development Workflow

### Typical Day-to-Day

```bash
# Morning: Update and check
git pull
make deps
make build
make test

# Work on feature
vim apps/api/internal/database/mongodb.go
make fmt
make test

# Before committing
make check  # Runs fmt, vet, lint
git add .
git commit -m "feat: add MongoDB support"

# Create PR
git push origin feature/mongodb
# Open PR on GitHub
```

### Release Process

```bash
# 1. Ensure everything works
make clean
make build
make test
make check

# 2. Update version
vim CHANGELOG.md  # Document changes

# 3. Create tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 4. GitHub Actions creates release automatically
```

## 🐛 Debugging Tools

### Delve Debugger

```bash
# Install
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug tests
dlv test ./internal/config/

# Debug application
dlv debug ./cmd/brd -- backup --config config.yml --target mydb
```

### VS Code Debugging

1. Set breakpoints in code
2. Press F5
3. Select debug configuration
4. Step through code with F10/F11

### Printf Debugging

```go
import "log"

log.Printf("[DEBUG] variable: %+v", myVar)
```

## 📊 Profiling

### CPU Profiling

```bash
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof
# Type 'top' to see hot spots
# Type 'web' to see call graph
```

### Memory Profiling

```bash
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof
```

### Live Profiling

Add to main.go:

```go
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

Then visit: <http://localhost:6060/debug/pprof/>

## 🎯 Best Practices

### Before Committing

```bash
make check     # Format, vet, lint
make test      # Run tests
make validate  # Validate config
```

### Before Pushing

```bash
# Ensure CI will pass
make build
make test
make lint

# Check for secrets
git diff --cached | grep -i 'password\|secret\|key'
```

### Before Releasing

```bash
# Full verification
make clean
make build-all
make test
make coverage
make check

# Manual testing
./brd validate-config --config examples/config.example.yml
```

## 🆘 Common Issues

### "golangci-lint not found"

```bash
make dev  # Installs it
```

### "Permission denied" on make install

```bash
# install requires sudo
make install
```

### Docker compose services not starting

```bash
# Check logs
docker-compose logs

# Restart
docker-compose restart

# Clean start
docker-compose down -v
docker-compose up -d
```

### Tests failing

```bash
# Clear cache
go clean -testcache
make test
```

## 📚 Additional Resources

- **Go Documentation**: <https://golang.org/doc/>
- **Make Documentation**: <https://www.gnu.org/software/make/manual/>
- **Docker Docs**: <https://docs.docker.com/>
- **golangci-lint**: <https://golangci-lint.run/>
- **GitHub Actions**: <https://docs.github.com/en/actions>

## Summary

This repository includes comprehensive tooling for:

- ✅ Building (Makefile, Dockerfile, cross-compilation)
- ✅ Testing (unit tests, integration tests, coverage)
- ✅ Code quality (linting, formatting, static analysis)
- ✅ CI/CD (GitHub Actions for tests and releases)
- ✅ Development (VS Code config, Docker Compose stack)
- ✅ Documentation (comprehensive guides)

Everything you need to develop, test, and release BareD efficiently!
