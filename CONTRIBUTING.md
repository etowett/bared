# Contributing to BareD

Thank you for your interest in contributing to BareD! This document provides guidelines and information for contributors.

## Getting Started

### Prerequisites

- Go 1.26.4
- Make (optional but recommended)
- Docker and Docker Compose (for testing)
- golangci-lint (for code quality checks)

### Development Setup

1. Clone the repository:

```bash
git clone <repository-url>
cd bared
```s

2. Set up the development environment:

```bash
make dev
```

3. Build the project:

```bash
make build
```

4. Run tests:

```bash
make test
```

## Project Structure

```
bared/
├── cmd/brd/              # CLI entry point
├── internal/
│   ├── app/              # High-level orchestration
│   ├── config/           # Configuration parsing & validation
│   ├── database/         # Database dumpers (MySQL, Postgres, Redis)
│   ├── storage/          # Storage backends (Local, S3, SFTP)
│   ├── compress/         # Compression implementations
│   ├── notify/           # Notification implementations
│   ├── daemon/           # Daemon and scheduler
│   ├── retention/        # Backup tracking and cleanup
│   └── util/             # Utility functions
└── examples/             # Example configurations
```

## Development Workflow

### Making Changes

1. Create a new branch:

```bash
git checkout -b feature/your-feature-name
```

2. Make your changes following the code style guidelines below

3. Format your code:

```bash
make fmt
```

4. Run all checks:

```bash
make check
```

5. Test your changes:

```bash
make test
make validate
```

6. Commit your changes with a clear commit message

### Code Style Guidelines

- Follow standard Go conventions and idioms
- Use meaningful variable and function names
- Add comments for exported functions and types
- Keep functions small and focused
- Use interfaces for extensibility
- Handle errors explicitly (no silent failures)

### Adding New Features

#### Adding a New Database Type

1. Create a new file in `internal/database/` (e.g., `mongodb.go`)
2. Implement the `Dumper` and `Restorer` interfaces
3. Add the new type to `internal/database/factory.go`
4. Update `internal/config/validator.go` to recognize the new type
5. Add example configuration to `examples/config.example.yml`
6. Update documentation

#### Adding a New Storage Backend

1. Create a new file in `internal/storage/` (e.g., `gcs.go`)
2. Implement the `Storage` interface
3. Add the new type to `internal/storage/factory.go`
4. Update `internal/config/validator.go` to recognize the new type
5. Add configuration options to `internal/config/config.go`
6. Add example configuration to `examples/config.example.yml`
7. Update documentation

#### Adding a New Notifier

1. Create a new file in `internal/notify/` (e.g., `discord.go`)
2. Implement the `Notifier` interface
3. Add the new type to `internal/notify/factory.go`
4. Update `internal/config/validator.go` to recognize the new type
5. Add configuration options as needed
6. Update documentation

### Testing

#### Unit Tests

Write unit tests for new functionality:

```go
// internal/database/mydb_test.go
package database

import (
    "testing"
)

func TestMyDB(t *testing.T) {
    // Test implementation
}
```

Run tests:

```bash
make test
```

#### Integration Tests

For integration tests that require external services, use Docker:

```bash
# Start test services
docker-compose up -d mysql postgres redis

# Run integration tests
go test -tags=integration ./...

# Clean up
docker-compose down
```

### Documentation

- Update README.md for user-facing changes
- Update docs/architecture/original-plan.md for architectural changes
- Add inline comments for complex logic
- Update example configurations

## Pull Request Process

1. Ensure all tests pass
2. Update documentation as needed
3. Add a clear description of your changes
4. Link any related issues
5. **Add appropriate release label** (see Release Process below)
6. Request review from maintainers

### Pull Request Checklist

- [ ] Code follows project style guidelines
- [ ] Tests added for new functionality
- [ ] All tests pass (`make test`)
- [ ] Code is properly formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated
- [ ] Example configuration updated (if needed)
- [ ] **Appropriate release label added** (`release:major`, `release:minor`, `release:patch`, or `release:skip`)

## Release Process

BareD uses an automated release system that creates new versions when PRs are merged to `main`. The version bump type is determined by PR labels.

### Release Labels

Every PR should have ONE of these labels:

| Label | Version Bump | Use Case | Example |
|-------|--------------|----------|---------|
| `release:major` | x.0.0 | Breaking changes, major refactors | 1.5.2 → 2.0.0 |
| `release:minor` | 0.x.0 | New features, enhancements | 1.5.2 → 1.6.0 |
| `release:patch` | 0.0.x | Bug fixes, docs, small improvements | 1.5.2 → 1.5.3 |
| `release:skip` | No release | Internal changes, no user impact | N/A |

**Default Behavior:** If no label is added, a `patch` release will be created (safe default).

### How It Works

1. **Create PR** with your changes
2. **Add release label** based on the impact:
   - Breaking API changes? → `release:major`
   - New feature? → `release:minor`
   - Bug fix or docs? → `release:patch`
   - Internal only? → `release:skip`
3. **Merge to main** (after review and approval)
4. **Automated workflow**:
   - Detects the label
   - Calculates next version
   - Creates git tag (e.g., `v1.2.3`)
   - Builds binaries for all platforms
   - Creates GitHub release with changelog
   - Builds and pushes Docker images

### Release Timeline

After merging:
- **Auto-release workflow:** Creates tag (~1 minute)
- **Binary release workflow:** Builds all binaries (~5-10 minutes)
- **Docker workflow:** Builds and pushes images (~10-15 minutes)

Total time: ~15-20 minutes for a complete release.

### What Gets Released

Each release includes:
- **Binaries:** Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64)
- **Archives:** `.tar.gz` for Unix, `.zip` for Windows
- **Checksums:** SHA256 for all artifacts
- **Docker Images:** Multi-platform (linux/amd64, linux/arm64)
- **Changelog:** Auto-generated from PR titles and commits

### Manual Release (Emergency)

If automation fails, maintainers can create a manual release:

```bash
# Create and push tag
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3

# This triggers the release and Docker workflows
```

### Testing Releases Locally

Before merging, you can test the release process locally:

```bash
# Install GoReleaser
brew install goreleaser

# Test configuration
goreleaser check

# Dry run (doesn't publish)
goreleaser release --snapshot --clean

# Check built binaries
ls -lh dist/
```

### Version Numbering Guidelines

Follow [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR (x.0.0):** Incompatible API changes
  - Removed commands or flags
  - Changed configuration format
  - Removed database/storage support
  - Breaking behavior changes

- **MINOR (0.x.0):** Backwards-compatible functionality
  - New database types
  - New storage backends
  - New features
  - Enhancements to existing features

- **PATCH (0.0.x):** Backwards-compatible bug fixes
  - Bug fixes
  - Documentation updates
  - Performance improvements (no API changes)
  - Dependency updates

When in doubt, choose the lower version bump (prefer `minor` over `major`, `patch` over `minor`).

### Changelog Best Practices

To generate useful changelogs:
- **PR titles** should be clear and descriptive (they appear in the changelog)
- Use conventional commit prefixes when possible:
  - `feat:` for new features
  - `fix:` for bug fixes
  - `docs:` for documentation
  - `chore:` for maintenance
- Link related issues in PR description

Example good PR titles:
- ✅ "Add PostgreSQL 15 support"
- ✅ "Fix MySQL backup timeout on large databases"
- ✅ "Update documentation for S3 configuration"

Example poor PR titles:
- ❌ "Update stuff"
- ❌ "Fix bug"
- ❌ "Changes"

### Troubleshooting Releases

If a release fails:

1. **Check workflow logs:** GitHub Actions → Failed workflow → View logs
2. **Common issues:**
   - Web build failed: Check `web/` directory, run `npm run build` locally
   - Binary build failed: Check Go version, dependencies
   - Docker push failed: Check Docker Hub credentials
3. **Recovery:**
   - Fix the issue
   - Delete the tag: `git tag -d v1.2.3 && git push origin :refs/tags/v1.2.3`
   - Create PR with fix
   - Merge and let automation retry

For more details, see [docs/architecture/release-process.md](docs/architecture/release-process.md).

## Makefile Commands

```bash
# Build and clean
make build          # Build the binary
make build-all      # Build for multiple platforms
make clean          # Remove build artifacts

# Testing
make test           # Run tests
make coverage       # Generate coverage report
make bench          # Run benchmarks
make validate       # Validate example config

# Code quality
make fmt            # Format code
make vet            # Run go vet
make lint           # Run linter
make check          # Run all checks

# Development
make dev            # Setup dev environment
make run-daemon     # Run daemon locally
make deps           # Download dependencies
```

## Architecture Decisions

### Streaming Architecture

BareD uses a streaming architecture with `io.Pipe()` to avoid creating temporary files. When adding new features, maintain this pattern:

```go
// Example: Streaming pipeline
pr, pw := io.Pipe()

go func() {
    defer pw.Close()
    // Write data to pw
}()

// Read from pr in main goroutine
```

### Error Handling

Always wrap errors with context:

```go
if err != nil {
    return fmt.Errorf("failed to connect to database: %w", err)
}
```

### Interface Design

Use interfaces for extensibility:

```go
type MyInterface interface {
    DoSomething(ctx context.Context) error
}
```

## Common Tasks

### Adding a Configuration Option

1. Add field to appropriate struct in `internal/config/config.go`
2. Add YAML tag for parsing
3. Add validation in `internal/config/validator.go`
4. Update example configuration
5. Document in README.md

### Debugging

Enable verbose logging:

```bash
./brd backup --config config.yml --target mydb 2>&1 | tee backup.log
```

Use Go's race detector:

```bash
go test -race ./...
```

Profile the application:

```bash
go test -cpuprofile=cpu.prof -memprofile=mem.prof -bench=.
go tool pprof cpu.prof
```

## Getting Help

- Check existing issues and pull requests
- Review the documentation in `README.md` and `docs/`
- Ask questions in issues with the "question" label

## Code of Conduct

- Be respectful and inclusive
- Provide constructive feedback
- Focus on the code, not the person
- Help others learn and grow

## License

By contributing to BareD, you agree that your contributions will be licensed under the same license as the project.

Thank you for contributing to BareD!
