# Contributing to BareD

Thank you for your interest in contributing to BareD! This document provides guidelines and information for contributors.

## Getting Started

### Prerequisites

- Go 1.26.5
- Bun 1.3.14 (for the web dashboard — the version is pinned in `apps/web/.bun-version`)
- Make (optional but recommended)
- Docker and Docker Compose (for testing)
- golangci-lint (for code quality checks)

### Development Setup

1. Clone the repository:

```bash
git clone https://github.com/etowett/bared.git
cd bared
```

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

Each deployable app lives under `apps/`; the repo root holds project-level concerns only.

```
bared/
├── apps/
│   ├── api/                  # Go backend — module root (go.mod lives here)
│   │   ├── cmd/brd/          # CLI entry point
│   │   └── internal/
│   │       ├── app/          # High-level orchestration
│   │       ├── config/       # Configuration parsing & validation
│   │       ├── database/     # Database dumpers (MySQL, Postgres, Redis)
│   │       ├── storage/      # Storage backends (Local, S3, SFTP)
│   │       ├── compress/     # Compression implementations
│   │       ├── notify/       # Notification implementations
│   │       ├── daemon/       # Daemon and scheduler
│   │       ├── retention/    # Backup tracking and cleanup
│   │       ├── web/          # go:embed of the built dashboard
│   │       └── util/         # Utility functions
│   └── web/                  # React 19 + TypeScript dashboard
├── docs/                     # Long-form documentation
├── examples/                 # Example configurations
└── specs/                    # Spec-driven feature work
```

> **Where to run commands.** The `Makefile` at the repo root drives everything and already
> knows where each app lives — prefer it. If you invoke Go directly, run it from `apps/api/`
> (or use `go -C apps/api …`), and Bun from `apps/web/`. The Go module path is
> `github.com/etowett/bared/apps/api`, so imports look like
> `github.com/etowett/bared/apps/api/internal/storage` and `github.com/etowett/bared/apps/api/cmd/brd`.

## Development Workflow

### Making Changes

1. Create a new branch:

```bash
git checkout -b feature/your-feature-name
```

2. Make your changes following the code style guidelines below

3. Run the verify gate — formatting, vet, lint, unit tests, and the coverage threshold:

```bash
make pre-commit
```

> The final `coverage-check` step is a **ratchet**, not an aspiration: `COVERAGE_THRESHOLD` in the
> `Makefile` is set just below the repo's real coverage (test helpers under `internal/testutil/` are
> excluded, since they aren't production statements), and CI enforces the same number. It passes on
> a clean checkout, so if it goes red your change lowered coverage — add tests rather than lowering
> the threshold. If your change lifts coverage well clear of the ratchet, raise `COVERAGE_THRESHOLD`
> in the same PR so it can never slide back.

4. If you touched the web UI, run its gate too:

```bash
make web-validate
```

5. Sanity-check the example config still loads (`make validate` does only this — it is not the gate):

```bash
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

1. Create a new file in `apps/api/internal/database/` (e.g., `mongodb.go`)
2. Implement the `Dumper` and `Restorer` interfaces
3. Add the new type to `apps/api/internal/database/factory.go`
4. Update `apps/api/internal/config/validator.go` to recognize the new type
5. Add example configuration to `examples/config.example.yml`
6. Update documentation

#### Adding a New Storage Backend

1. Create a new file in `apps/api/internal/storage/` (e.g., `gcs.go`)
2. Implement the `Storage` interface
3. Add the new type to `apps/api/internal/storage/factory.go`
4. Update `apps/api/internal/config/validator.go` to recognize the new type
5. Add configuration options to `apps/api/internal/config/config.go`
6. Add example configuration to `examples/config.example.yml`
7. Update documentation

#### Adding a New Notifier

1. Create a new file in `apps/api/internal/notify/` (e.g., `discord.go`)
2. Implement the `Notifier` interface
3. Add the new type to `apps/api/internal/notify/factory.go`
4. Update `apps/api/internal/config/validator.go` to recognize the new type
5. Add configuration options as needed
6. Update documentation

### Testing

#### Unit Tests

Write unit tests for new functionality:

```go
// apps/api/internal/database/mydb_test.go
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
5. Indicate the release type in the PR template — a **maintainer** applies the actual
   label, since contributors working from a fork cannot set labels
6. Request review from maintainers

### Pull Request Checklist

- [ ] Code follows project style guidelines
- [ ] Tests added for new functionality
- [ ] All tests pass (`make test`)
- [ ] Code is properly formatted (`make fmt`)
- [ ] Linter passes (`make lint`)
- [ ] Documentation updated
- [ ] Example configuration updated (if needed)
- [ ] Release type indicated in the PR description (a maintainer applies the label)

## Release Process

BareD uses an automated release system that creates new versions when PRs are merged to `main`. The version bump type is determined by PR labels.

### Release Labels

Every PR gets ONE of these labels. **Maintainers apply it** — a contributor working
from a fork has no permission to set labels, so just indicate the intended bump in
the PR description:

| Label | Version Bump | Use Case | Example |
|-------|--------------|----------|---------|
| `release:major` | x.0.0 | Breaking changes, major refactors | 1.5.2 → 2.0.0 |
| `release:minor` | 0.x.0 | New features, enhancements | 1.5.2 → 1.6.0 |
| `release:patch` | 0.0.x | Bug fixes, docs, small improvements | 1.5.2 → 1.5.3 |
| `release:skip` | No release | Internal changes, no user impact | N/A |

**Default Behavior:** If no label is added, a `patch` release will be created (safe default).

### How It Works

1. **Create PR** with your changes
2. **A maintainer adds the release label** based on the impact:
   - Breaking API changes? → `release:major`
   - New feature? → `release:minor`
   - Bug fix or docs? → `release:patch`
   - Internal only? → `release:skip`
3. **Merge to main** (after review and approval)
4. **Automated workflow**:
   - Detects the label
   - Calculates next version
   - Creates two git tags on the same commit (e.g., `v1.2.3` **and** `apps/api/v1.2.3`)
   - Builds binaries for all platforms
   - Creates GitHub release with changelog
   - Builds and pushes Docker images

### Why every release pushes two tags

`go.mod` lives at `apps/api/`, so the Go module is `github.com/etowett/bared/apps/api`. Go
only recognises tags prefixed with the module's subdirectory as versions of that module, so a
root `v1.2.3` tag alone is invisible to it — `go install .../cmd/brd@latest` would quietly
install a pseudo-version of `main`'s tip, and `@v1.2.3` would fail with
`unknown revision apps/api/v1.2.3`. Each release therefore pushes:

| Tag | Purpose |
|-----|---------|
| `v1.2.3` | The release. Triggers `release.yml` → GoReleaser → GitHub release → `docker.yml`. |
| `apps/api/v1.2.3` | The Go module version, so `go install .../cmd/brd@latest` resolves to the release. |

Things worth knowing:

- **They must never drift.** `auto-release.yml` pushes both with `git push --atomic`, so git
  updates both refs or neither. A collision on one tag can't leave the other half-published.
- **Only `v1.2.3` triggers a build.** `release.yml` filters on `tags: v*.*.*`, and GitHub's
  ref-filter `*` does not match `/`, so `apps/api/v1.2.3` never matches. No double-fire.
- **`git describe` needs `--match 'v[0-9]*'`.** Two tags sit on the release commit and a bare
  `git describe` returns `apps/api/v1.2.3`. Both `auto-release.yml`'s version maths and the
  `Makefile`'s `VERSION` filter for root tags; anything new that derives a version must too.
- **GoReleaser needs the same filter.** It has its own tag lookup, so `.goreleaser.yml` sets
  `git.ignore_tags: ["apps/api/*"]`; `release.yml` additionally pins
  `GORELEASER_CURRENT_TAG: ${{ github.ref_name }}`. Without the filter, a run from any commit
  *after* the tag resolved `{{ .Version }}` to `apps/api/v1.2.3`, and the `/` turned the archive
  `name_template` into a nested directory (`dist/brd-apps/api/…`) — see #120.
- **Users still write `@v1.2.3`.** Go adds the `apps/api/` prefix when resolving the tag;
  `@apps/api/v1.2.3` is rejected as a disallowed version string.

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
# Create both tags: the release tag and the Go module tag
git tag -a v1.2.3 -m "Release v1.2.3"
git tag -a apps/api/v1.2.3 -m "Release v1.2.3"

# Push atomically so the two can never drift apart
git push --atomic origin v1.2.3 apps/api/v1.2.3

# Only v1.2.3 matches release.yml's `tags: v*.*.*`, so this triggers
# the release and Docker workflows exactly once.
```

If you need to undo a bad release, delete both tags together:

```bash
git tag -d v1.2.3 apps/api/v1.2.3
git push --atomic origin :refs/tags/v1.2.3 :refs/tags/apps/api/v1.2.3
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

The dry run works from any commit, not just one sitting exactly on a tag: `.goreleaser.yml`
ignores the `apps/api/*` module tags, so the snapshot version comes from the latest root
`vX.Y.Z` tag. Expect flat, release-shaped names —
`brd-1.2.3-SNAPSHOT-<sha>-linux-amd64.tar.gz` — plus `checksums.txt`. A nested
`dist/brd-apps/` directory means the module tag leaked back in (#120).

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
   - Web build failed: Check `apps/web/` directory, run `bun run build` locally
   - Binary build failed: Check Go version, dependencies
   - Docker push failed: Check Docker Hub credentials
3. **Recovery:**
   - Fix the issue
   - Delete the tag: `git tag -d v1.2.3 && git push origin :refs/tags/v1.2.3`
   - Create PR with fix
   - Merge and let automation retry

For more details, see [docs/architecture/release-process.md](docs/architecture/release-process.md)
and [CHANGELOG.md](CHANGELOG.md), which explains why release notes are generated
rather than hand-written.

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

1. Add field to appropriate struct in `apps/api/internal/config/config.go`
2. Add YAML tag for parsing
3. Add validation in `apps/api/internal/config/validator.go`
4. Update example configuration
5. Document in README.md

### Debugging

Enable verbose logging:

```bash
./brd backup --config config.yml --target mydb 2>&1 | tee backup.log
```

Use Go's race detector:

```bash
go -C apps/api test -race ./...
```

Profile the application:

```bash
go -C apps/api test -cpuprofile=cpu.prof -memprofile=mem.prof -bench=.
go tool pprof cpu.prof
```

### Troubleshooting the Verify Gate

**`make lint` reports dozens of errors on lines that already have `//nolint`.**

Look at the paths in the output. If they read `../../internal/...` rather than
`apps/api/internal/...`, and the log is peppered with `no such file or directory`, you are
looking at a stale golangci-lint cache left over from before the module moved under `apps/`.
The findings are not real. Clear the cache:

```bash
golangci-lint cache clean
make lint
```

**`make pre-commit` fails at the end, on `coverage-check`.**

Read the last few lines before assuming it is about coverage. `coverage-check` runs the suite
first, so it fails for two different reasons and says which:

- **"Coverage (x%) is below the ratchet"** — your change added statements without tests. Run
  `make coverage`, open `coverage.html`, and add tests for your uncovered lines. Do not lower
  `COVERAGE_THRESHOLD`; CI checks the same value, so a local edit only moves the failure.
- **"Tests failed — a test failed, coverage was not even measured"** — a test broke. The step
  ends with the failing test names, the path to the full log, and the `-count`/`-shuffle`
  re-runs that expose an order-dependent flake. Fix the test; never paper over it with a retry.

## Working with AI Coding Agents

The repository ships shared configuration for Claude Code (`.claude/`) and the Codex
CLI (`.codex/`) — hooks, subagents, skills and slash commands. See
[`.claude/README.md`](.claude/README.md) for what's available.

**The tracked config deliberately auto-approves nothing that executes project code.**
`.claude/settings.json` carries only the `ask` and `deny` lists and the hook
registrations; the command allow-list is not committed. To opt in:

```bash
cp .claude/settings.local.json.example .claude/settings.local.json
```

`.claude/settings.local.json` is gitignored. Codex users can relax the equivalent
rules in their personal `~/.codex/config.toml`; the tracked
`.codex/rules/allowlist.rules` prompts for the same commands.

Why: an allow-list entry like `Bash(make:*)` or `Bash(go run:*)` silently approves a
command whose behaviour is defined by files in the working tree — `Makefile`,
`scripts/`, `apps/web/package.json` scripts, `go:generate` directives. On a public
repository, checking out a fork's pull request and opening an agent session to review
it would give that fork arbitrary code execution on your machine, with no prompt.

**When reviewing a pull request from a fork, review the diff before you run anything** —
especially changes to `Makefile`, `scripts/`, `apps/web/package.json`, `Dockerfile`,
and any `go:generate` line. Consider moving your local settings aside first.

## Reporting Security Issues

**Do not open a public issue for a security vulnerability.** BareD handles database
passwords, storage credentials and an encryption key — report privately through
GitHub's private vulnerability reporting instead. See [SECURITY.md](SECURITY.md) for
the process, response expectations, and the project's known security limitations.

The same applies to pull requests: if a fix would reveal an unreported vulnerability,
report it privately first and let the advisory drive the timing.

## Getting Help

- Check existing issues and pull requests
- Review the documentation in `README.md` and `docs/`
- Ask questions in issues with the "question" label

## Code of Conduct

This project follows the [Contributor Covenant](CODE_OF_CONDUCT.md). In short: be
respectful and inclusive, give constructive feedback, focus on the code rather than
the person, and help others learn. Read
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for the full text and how to report a
problem.

## License

BareD is released under the MIT License. By contributing, you agree that your
contributions are licensed under the MIT License — see [LICENSE](LICENSE).

Thank you for contributing to BareD!
