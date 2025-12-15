# Development Guide

Resources for contributors and developers working on or extending BareD.

## Contents

### [Development Setup](setup.md)
Complete guide to setting up your development environment, including prerequisites, cloning, building, and running tests.

### [Tooling Guide](tooling.md)
Development tools, CI/CD pipelines, linters, formatters, and the full development workflow.

### [Testing Strategy](testing.md)
Testing philosophy, unit tests, integration tests, and test coverage practices.

### [System Architecture](architecture.md)
High-level overview of BareD's architecture, key components, and how they interact.

### [Contributing](../../CONTRIBUTING.md)
Contribution guidelines, code style, pull request process, and how to get started contributing.

## Quick Start for Contributors

### 1. Set Up Development Environment
```bash
# Clone repository
git clone <repository-url>
cd bared

# Install development tools
make dev

# Build
make build

# Run tests
make test
```

**See**: [Development Setup](setup.md)

### 2. Make Your Changes
- Follow [code style guidelines](../../CONTRIBUTING.md#code-style)
- Add tests for new features
- Update documentation
- Run `make check` before committing

**See**: [Contributing Guide](../../CONTRIBUTING.md)

### 3. Submit Pull Request
- Ensure all tests pass
- Update relevant documentation
- Add clear description
- Link related issues

## Adding New Features

### Adding a New Database Type
1. Implement `Dumper` and `Restorer` interfaces in `internal/database/`
2. Register in `internal/database/factory.go`
3. Add validation in `internal/config/validator.go`
4. Update example configuration
5. Add tests

**See**: [Contributing - Adding Database Types](../../CONTRIBUTING.md#adding-a-new-database-type)

### Adding a New Storage Backend
1. Implement `Storage` interface in `internal/storage/`
2. Register in `internal/storage/factory.go`
3. Add config options to `internal/config/config.go`
4. Update example configuration
5. Add tests

**See**: [Contributing - Adding Storage](../../CONTRIBUTING.md#adding-a-new-storage-backend)

### Adding a New Notifier
1. Implement `Notifier` interface in `internal/notify/`
2. Register in `internal/notify/factory.go`
3. Add config options
4. Update documentation
5. Add tests

**See**: [Contributing - Adding Notifier](../../CONTRIBUTING.md#adding-a-new-notifier)

## Development Workflow

### Daily Development
```bash
# Start Docker services for testing
docker-compose up -d

# Make changes...
vim internal/database/mongodb.go

# Format code
make fmt

# Run tests
make test

# Lint
make lint

# Full check
make check
```

### Before Committing
```bash
make check    # Run all checks
make test     # Ensure tests pass
make validate # Validate example config
```

### Creating a Release
```bash
# Tag release
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# GitHub Actions handles the rest
```

**See**: [Tooling Guide](tooling.md#release-process)

## Architecture Overview

BareD uses a streaming pipeline architecture to avoid temporary files:

```
Backup:  [DB Dump] → [Compress] → [Upload] → [Cleanup]
Restore: [Retrieve] → [Decompress] → [Restore] → [Cleanup]
                ↓ (via io.Pipe, no temp files)
```

**Key Design Principles**:
- **Streaming**: No temp files between stages
- **Interfaces**: Easy extensibility
- **Minimal dependencies**: Stdlib-first
- **Error handling**: Graceful failures with retries

**See**: [System Architecture](architecture.md)

## Testing

### Unit Tests
```bash
# Run all unit tests
make test

# Run specific package
go test -v ./internal/config/

# With coverage
make coverage
open coverage.html
```

### Integration Tests
```bash
# Start test services
docker-compose up -d

# Run integration tests
go test -tags=integration -v ./...
```

**See**: [Testing Strategy](testing.md)

## Code Quality

### Linting
```bash
make lint          # Run linter
make lint-fix      # Auto-fix issues
```

### Formatting
```bash
make fmt           # Format code
```

### Static Analysis
```bash
make vet           # Run go vet
```

## Common Development Tasks

**Adding a configuration option**:
1. Add field to struct in `internal/config/config.go`
2. Add YAML tag
3. Add validation
4. Update example config
5. Document in README

**Debugging**:
```bash
# Enable verbose logging
./brd backup --config config.yml --target mydb 2>&1 | tee backup.log

# Run with race detector
go test -race ./...

# Profile
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof
```

**See**: [Contributing - Common Tasks](../../CONTRIBUTING.md#common-tasks)

## Resources

- **[Architecture Docs](../architecture/)** - Deep dives into system design
- **[API Reference](../api/)** - REST and WebSocket APIs
- **[Contributing Guide](../../CONTRIBUTING.md)** - Full contribution guidelines

## Getting Help

- Review existing code and patterns
- Check [Architecture Docs](../architecture/)
- Ask questions in GitHub issues
- Join discussions

---

[← Back to Documentation](../README.md)
