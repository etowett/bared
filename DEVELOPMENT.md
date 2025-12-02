# Development Guide

This guide provides detailed information for developers working on BareD.

## Quick Start

```bash
# Clone and setup
git clone <repository-url>
cd bared
make dev

# Build and test
make build
make test
make validate
```

## Development Tools

### Required Tools

- **Go 1.21+**: Primary language
- **Make**: Build automation
- **Git**: Version control

### Recommended Tools

- **golangci-lint**: Code quality checks (`make dev` installs this)
- **Docker & Docker Compose**: Testing with real databases
- **VS Code**: IDE with Go extension (config included)

### Optional Tools

- **k9s**: Kubernetes cluster management
- **kubectl**: Kubernetes CLI
- **jq**: JSON processing for tracker files

## Project Layout

```
bared/
├── cmd/brd/                   # Application entry point
│   └── main.go               # CLI commands and flags
│
├── internal/                  # Private application code
│   ├── app/                  # Business logic layer
│   │   ├── backup.go        # Backup orchestration
│   │   ├── restore.go       # Restore orchestration
│   │   └── list.go          # Backup listing
│   │
│   ├── config/               # Configuration management
│   │   ├── config.go        # Data structures
│   │   ├── parser.go        # YAML parsing
│   │   └── validator.go     # Validation rules
│   │
│   ├── database/             # Database implementations
│   │   ├── database.go      # Interfaces
│   │   ├── mysql.go         # MySQL/MariaDB
│   │   ├── postgres.go      # PostgreSQL
│   │   ├── redis.go         # Redis
│   │   └── factory.go       # Type dispatch
│   │
│   ├── storage/              # Storage backends
│   │   ├── storage.go       # Interface
│   │   ├── local.go         # Local filesystem
│   │   ├── s3.go            # S3/S3-compatible
│   │   ├── sftp.go          # SFTP
│   │   └── factory.go       # Type dispatch
│   │
│   ├── compress/             # Compression
│   │   ├── compress.go      # Interface
│   │   ├── tgz.go           # tar.gz implementation
│   │   └── factory.go       # Type dispatch
│   │
│   ├── notify/               # Notifications
│   │   ├── notifier.go      # Interface
│   │   ├── slack.go         # Slack webhooks
│   │   └── factory.go       # Type dispatch
│   │
│   ├── daemon/               # Scheduler
│   │   └── daemon.go        # Cron scheduling & signals
│   │
│   ├── retention/            # Backup management
│   │   └── tracker.go       # JSON-based tracking
│   │
│   └── util/                 # Utilities
│       ├── paths.go         # Path generation
│       ├── retry.go         # Retry logic
│       ├── shell.go         # Command execution
│       └── stream.go        # Streaming helpers
│
├── examples/                  # Example files
│   ├── config.example.yml   # Full configuration
│   └── bared.service        # systemd service
│
├── .github/workflows/        # CI/CD
│   ├── ci.yml               # Continuous integration
│   └── release.yml          # Release automation
│
├── .vscode/                  # VS Code configuration
│   ├── settings.json        # Editor settings
│   └── launch.json          # Debug configurations
│
├── Makefile                  # Build automation
├── Dockerfile                # Container image
├── docker-compose.yml        # Development stack
├── .golangci.yml            # Linter configuration
└── .gitignore               # Git ignore rules
```

## Building

### Local Build

```bash
# Simple build
make build

# Build with version info
VERSION=v1.0.0 make build

# Cross-platform builds
make build-all
```

### Docker Build

```bash
# Build image
make docker-build

# Or manually
docker build -t bared:latest .

# Run in container
docker run --rm -v $(pwd)/examples:/etc/bared bared:latest validate-config --config /etc/bared/config.example.yml
```

## Testing

### Unit Tests

```bash
# Run all tests
make test

# Run specific package
go test -v ./internal/config/

# Run with race detector
go test -race ./...

# Run with coverage
make coverage
```

### Integration Tests

Use Docker Compose to spin up test services:

```bash
# Start services
docker-compose up -d

# Wait for services to be ready
sleep 10

# Create test database
docker-compose exec mysql mysql -uroot -ptestpass -e "CREATE DATABASE IF NOT EXISTS testdb;"

# Run backup test
./brd backup --config test-config.yml --target test_mysql

# Verify backup exists
./brd list --config test-config.yml --target test_mysql

# Test restore
./brd restore --config test-config.yml --target test_mysql --backup latest

# Cleanup
docker-compose down -v
```

### Manual Testing

```bash
# Setup test environment
make setup-test-env

# Create test config
cat > test-config.yml << 'EOF'
default_storage: local_disk

storages:
  local_disk:
    type: local
    path: ./backups/test
    keep: 5

targets:
  - name: test_mysql
    conn:
      type: mysql
      user: root
      password: testpass
      database: testdb
      host: localhost
      port: 3306
    compress:
      enabled: true
      type: tgz
    storage:
      enabled: true
      name: local_disk
EOF

# Test backup
./brd backup --config test-config.yml --target test_mysql

# Check results
ls -lh backups/test/test_mysql/
```

## Debugging

### VS Code

The repository includes VS Code debug configurations. Press F5 to:

- Run BareD with different commands
- Set breakpoints
- Inspect variables
- Step through code

### Command Line Debugging

```bash
# Use Delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug a test
dlv test ./internal/config/

# Debug the application
dlv debug ./cmd/brd -- backup --config config.yml --target mydb
```

### Verbose Logging

Add logging statements:

```go
log.Printf("[DEBUG] Variable value: %+v", myVar)
```

### Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof

# Live profiling (add to code)
import _ "net/http/pprof"
go http.ListenAndServe("localhost:6060", nil)
# Then visit http://localhost:6060/debug/pprof/
```

## Code Quality

### Formatting

```bash
# Format code
make fmt

# Check formatting without modifying
gofmt -l .
```

### Linting

```bash
# Run linter
make lint

# Run specific linters
golangci-lint run --disable-all --enable=errcheck

# Fix auto-fixable issues
golangci-lint run --fix
```

### Static Analysis

```bash
# Run go vet
make vet

# Check for suspicious constructs
go vet -composites=false ./...
```

### Pre-commit Checks

Run before committing:

```bash
make check  # Runs fmt, vet, and lint
```

## Making Changes

### Adding a Database Type

1. **Create implementation file**: `internal/database/mongodb.go`

```go
package database

import (
    "context"
    "io"
    "bared/internal/config"
)

type MongoDB struct {
    conn *config.Connection
}

func NewMongoDB(conn *config.Connection) *MongoDB {
    return &MongoDB{conn: conn}
}

func (m *MongoDB) Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error) {
    // Implementation
}

func (m *MongoDB) Restore(ctx context.Context, r io.Reader) error {
    // Implementation
}
```

2. **Update factory**: `internal/database/factory.go`

```go
case "mongodb":
    return NewMongoDB(target.Conn), nil
```

3. **Update validator**: `internal/config/validator.go`

```go
case "mysql", "postgres", "redis", "mongodb":
    // existing validation
```

4. **Add tests**: `internal/database/mongodb_test.go`

5. **Update docs**: Add example to `examples/config.example.yml`

### Adding a Storage Backend

Similar process to adding a database, but in `internal/storage/`.

### Adding Configuration Options

1. **Update struct**: `internal/config/config.go`

```go
type Storage struct {
    // ... existing fields
    NewOption string `yaml:"new_option,omitempty"`
}
```

2. **Add validation**: `internal/config/validator.go`

```go
if storage.NewOption == "" {
    return fmt.Errorf("new_option is required")
}
```

3. **Update example**: `examples/config.example.yml`

4. **Update docs**: Document in README.md

## Release Process

### Creating a Release

1. **Update version**:

```bash
# Tag the release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

2. **GitHub Actions will**:
   - Build binaries for all platforms
   - Generate checksums
   - Create GitHub release
   - Upload artifacts

### Manual Release

```bash
# Build for all platforms
make build-all

# Create release archive
make release

# Upload to GitHub manually
```

## Troubleshooting

### Build Issues

```bash
# Clean and rebuild
make clean
make build

# Update dependencies
make update-deps

# Verify Go installation
go version
go env
```

### Test Failures

```bash
# Run specific test with verbose output
go test -v -run TestFunctionName ./internal/package/

# Check for race conditions
go test -race ./...

# Clean test cache
go clean -testcache
```

### Docker Issues

```bash
# Check service logs
docker-compose logs mysql

# Restart services
docker-compose restart

# Clean everything
docker-compose down -v
docker system prune -af
```

## Performance Optimization

### Memory Usage

- Use streaming (`io.Pipe`) for large data
- Avoid buffering entire dumps in memory
- Close resources promptly with `defer`

### Concurrency

- Use goroutines for parallel operations
- Implement worker pools for bounded concurrency
- Always check context cancellation

### Network Efficiency

- Implement retry logic with backoff
- Use connection pooling
- Compress data before transmission

## Documentation

### Code Comments

```go
// Package database provides database dump and restore functionality.
package database

// Dumper represents a database that can be backed up.
// Implementations must be safe for concurrent use.
type Dumper interface {
    // Dump writes the database dump to w.
    // It returns metadata about the dump and any error encountered.
    Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error)
}
```

### Updating Documentation

When making changes, update:

- Code comments
- README.md (user-facing changes)
- CONTRIBUTING.md (developer changes)
- plan.md (architectural changes)
- Example configurations

## Resources

### Go Resources

- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Go Proverbs](https://go-proverbs.github.io/)

### Project Resources

- Architecture: See `plan.md`
- Contributing: See `CONTRIBUTING.md`
- Examples: See `examples/` directory

## Getting Help

- Check existing issues on GitHub
- Review the plan.md for architecture decisions
- Ask questions in GitHub Discussions
- Join the community chat (if available)
