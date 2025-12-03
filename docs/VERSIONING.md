# BareD Versioning Guide

## Version Information

BareD includes version information that can be queried via CLI and API.

## CLI Version

### Display Version

```bash
# Long form
brd --version

# Short form
brd -v

# Output format: brd version X.Y.Z (commit: abc1234, built: 2025-12-03_19:43:22)
```

### Version Format

- **Version**: Git tag or commit hash (e.g., `v1.0.0`, `abc1234`)
- **Commit**: Short git commit hash (7 characters)
- **Build Date**: UTC timestamp when binary was built

## API Version

### Health Endpoint

The health endpoint returns version information:

```bash
curl http://localhost:8080/api/health
```

Response:
```json
{
  "status": "ok",
  "version": "v1.0.0"
}
```

## Building with Version Information

### Using Makefile (Recommended)

The Makefile automatically injects version information:

```bash
make build
```

This automatically sets:
- **VERSION**: From `git describe --tags --always --dirty`
- **COMMIT**: From `git rev-parse --short HEAD`
- **BUILD_DATE**: Current UTC timestamp

### Manual Build with Version

```bash
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')

go build -ldflags "\
  -X bared/internal/version.Version=${VERSION} \
  -X bared/internal/version.Commit=${COMMIT} \
  -X bared/internal/version.BuildDate=${BUILD_DATE}" \
  -o brd ./cmd/brd
```

### Docker Build with Version

Pass version as build args:

```bash
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')

docker build \
  --build-arg VERSION=${VERSION} \
  --build-arg COMMIT=${COMMIT} \
  --build-arg BUILD_DATE=${BUILD_DATE} \
  -t bared:${VERSION} .
```

Or use docker-compose with build args:

```yaml
services:
  bared:
    build:
      context: .
      args:
        VERSION: v1.0.0
        COMMIT: abc1234
        BUILD_DATE: 2025-12-03_19:43:22
```

## Version Package

Version information is stored in `internal/version/version.go`:

```go
package version

var (
    Version   = "dev"      // Set via ldflags
    Commit    = "none"     // Set via ldflags
    BuildDate = "unknown"  // Set via ldflags
)

func GetVersion() string {
    return Version
}

func GetFullVersion() string {
    return Version + " (commit: " + Commit + ", built: " + BuildDate + ")"
}
```

## Semantic Versioning

BareD follows [Semantic Versioning 2.0.0](https://semver.org/):

- **MAJOR**: Incompatible API changes
- **MINOR**: Backwards-compatible functionality
- **PATCH**: Backwards-compatible bug fixes

### Version Format

- Release: `v1.2.3`
- Pre-release: `v1.2.3-beta.1`
- Development: `v1.2.3-5-gabc1234` (5 commits after v1.2.3)
- Dirty: `v1.2.3-dirty` (uncommitted changes)

## Creating Releases

### 1. Tag Release

```bash
# Create annotated tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push tag
git push origin v1.0.0
```

### 2. Build Release Binary

```bash
# Build with Makefile (uses git tag)
make build

# Verify version
./bin/brd --version
# Output: brd version v1.0.0 (commit: abc1234, built: 2025-12-03_19:43:22)
```

### 3. Create Release Archive

```bash
make release
# Creates: dist/brd-v1.0.0.tar.gz
```

### 4. Build Docker Image

```bash
VERSION=v1.0.0
docker build \
  --build-arg VERSION=${VERSION} \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S') \
  -t bared:${VERSION} \
  -t bared:latest .
```

## Version in CI/CD

### GitHub Actions Example

```yaml
name: Build

on:
  push:
    tags:
      - 'v*'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 0  # Fetch all tags

      - name: Get version info
        id: version
        run: |
          echo "VERSION=$(git describe --tags --always)" >> $GITHUB_OUTPUT
          echo "COMMIT=$(git rev-parse --short HEAD)" >> $GITHUB_OUTPUT
          echo "BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')" >> $GITHUB_OUTPUT

      - name: Build binary
        run: |
          make build
          ./bin/brd --version

      - name: Build Docker image
        run: |
          docker build \
            --build-arg VERSION=${{ steps.version.outputs.VERSION }} \
            --build-arg COMMIT=${{ steps.version.outputs.COMMIT }} \
            --build-arg BUILD_DATE=${{ steps.version.outputs.BUILD_DATE }} \
            -t bared:${{ steps.version.outputs.VERSION }} .
```

## Development Builds

For development builds without git:

```bash
# Default development version
go build -o brd ./cmd/brd
./brd --version
# Output: brd version dev (commit: none, built: unknown)
```

## Version Checking

### Check if Version is Development

```go
import "bared/internal/version"

if version.Version == "dev" {
    log.Println("Running development build")
}
```

### Check Minimum Version

```go
import (
    "bared/internal/version"
    "github.com/hashicorp/go-version"
)

minVersion, _ := version.NewVersion("v1.2.0")
currentVersion, _ := version.NewVersion(version.Version)

if currentVersion.LessThan(minVersion) {
    log.Fatalf("Minimum version required: v1.2.0, current: %s", version.Version)
}
```

## Best Practices

1. **Always use annotated tags** for releases:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   ```

2. **Build with Makefile** to ensure version is injected:
   ```bash
   make build
   ```

3. **Verify version** after building:
   ```bash
   ./bin/brd --version
   ```

4. **Include version in logs** for debugging:
   ```go
   log.Printf("Starting BareD %s", version.GetFullVersion())
   ```

5. **Document breaking changes** in release notes

6. **Follow semantic versioning** strictly

## Troubleshooting

### Version shows "dev"

- Not built with Makefile or ldflags
- Git repository not initialized
- No git tags exist

**Solution**: Use `make build` or set ldflags manually

### Version shows "dirty"

- Uncommitted changes in working directory
- Built from dirty git state

**Solution**: Commit changes or use `--always` flag

### Commit shows "none"

- Not a git repository
- Git not installed
- Building outside git context

**Solution**: Build from git repository with git installed

## Related Files

- `internal/version/version.go` - Version package
- `cmd/brd/main.go` - CLI version display
- `internal/api/handlers.go` - API health endpoint
- `Makefile` - Build with version injection
- `Dockerfile` - Docker build with version args

## See Also

- [Semantic Versioning](https://semver.org/)
- [Git Tagging](https://git-scm.com/book/en/v2/Git-Basics-Tagging)
- [Go Build Constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints)
