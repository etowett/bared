# Release Uniformity Validation

> **Historical document.** This recorded a point-in-time check that the Docker image,
> the release binaries and a local `make build` produce the same application. Details
> below have drifted: the web toolchain is Bun (not Node/npm), the coverage gate is a
> ratchet in the `Makefile` (34% today, not 75%), and the version numbers used in
> examples are illustrative. For the release process as it actually runs, see
> [release-process.md](release-process.md) and
> [docs/operations/versioning.md](../operations/versioning.md).

## Overview

This document validates that all distribution methods (Docker images, binary releases, local builds) produce **uniform, identical applications** with the same features and version information.

## Distribution Methods

### 1. Docker Images (`ektowett/bared`)

- Built via `.github/workflows/docker.yml`
- Multi-platform: linux/amd64, linux/arm64
- Triggered by: git tags (`v*.*.*`) and main branch pushes

### 2. Binary Releases (GitHub Releases)

- Built via `.github/workflows/release.yml`
- Platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- Triggered by: git tags (`v*.*.*`)

### 3. Local Builds (Makefile)

- Built via `make build`, `make build-all`, etc.
- Platforms: Any (depends on local environment)
- Triggered by: Developer manually

## Uniformity Guarantees

### ✅ Web Frontend Inclusion

**Status**: **FIXED** - All methods now include web frontend

| Method | Web Frontend | Verification |
|--------|--------------|--------------|
| Docker | ✅ Included | Multi-stage build: Stage 1 builds React, Stage 2 embeds |
| Binary Release | ✅ Included | Workflow builds web with `bun install --frozen-lockfile && bun run build` before Go build |
| Local Build | ✅ Included | Use `make build-with-web` or manual `cd apps/web && bun run build` |

**Implementation**:

- Docker: Dockerfile stages 1-2 build and embed frontend
- Release: Added Node.js setup and web build step before binary compilation
- Local: `make build-with-web` target builds web then Go binary

### ✅ Version Information

**Status**: **UNIFORM** - All methods use identical version injection

| Method | VERSION | COMMIT | BUILD_DATE | Format |
|--------|---------|--------|------------|--------|
| Docker | `v1.0.0` or `main-abc1234` | Short SHA | RFC3339 | ✅ Uniform |
| Binary Release | `v1.0.0` | Short SHA | RFC3339 | ✅ Uniform |
| Local Build | Git tag or `dev` | Short SHA | RFC3339 | ✅ Uniform |

**Standardized Format**:

```bash
VERSION: v1.0.0 (or dev, or main-abc1234)
COMMIT: abc1234 (short SHA)
BUILD_DATE: 2025-12-04T10:30:45Z (RFC3339)
```

**LDFLAGS** (identical across all methods):

```bash
-X github.com/etowett/bared/apps/api/internal/version.Version=${VERSION}
-X github.com/etowett/bared/apps/api/internal/version.Commit=${COMMIT}
-X github.com/etowett/bared/apps/api/internal/version.BuildDate=${BUILD_DATE}
```

### ✅ Static Compilation (CGO)

**Status**: **UNIFORM** - All methods disable CGO for static binaries

| Method | CGO_ENABLED | Static Binary | Portable |
|--------|-------------|---------------|----------|
| Docker | `0` | ✅ Yes | ✅ Yes |
| Binary Release | `0` | ✅ Yes | ✅ Yes |
| Local Build | `0` | ✅ Yes | ✅ Yes |

**Benefits**:

- No external C dependencies required
- Works on any Linux distribution (musl, glibc, etc.)
- Smaller binary size
- Better cross-compilation support

### ✅ Build Flags

**Status**: **UNIFORM** - All methods use consistent build settings

Docker Dockerfile:

```bash
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
  -ldflags "-extldflags '-static' -X github.com/etowett/bared/apps/api/internal/version.Version=... " \
  -o brd ./cmd/brd
```

Binary Release (.github/workflows/release.yml):

```bash
CGO_ENABLED=0 GOOS=<platform> GOARCH=<arch> go build \
  -ldflags "-X github.com/etowett/bared/apps/api/internal/version.Version=..." \
  -o dist/brd-<platform>-<arch> ./cmd/brd
```

Local Build (Makefile):

```bash
CGO_ENABLED=0 go build \
  -ldflags "-X github.com/etowett/bared/apps/api/internal/version.Version=..." \
  -o bin/brd ./cmd/brd
```

**Note**: Docker adds `-a -installsuffix cgo -extldflags '-static'` for extra static linking guarantees on Alpine Linux. This doesn't affect functionality.

### ✅ Feature Parity

**Status**: **COMPLETE** - All methods have identical features

| Feature | Docker | Binary | Local | Notes |
|---------|--------|--------|-------|-------|
| Database Backup (MySQL, PostgreSQL, Redis) | ✅ | ✅ | ✅ | Core functionality |
| Storage Backends (Local, S3, SFTP) | ✅ | ✅ | ✅ | All included |
| Compression (tar.gz) | ✅ | ✅ | ✅ | Built-in |
| Scheduling (Cron) | ✅ | ✅ | ✅ | Daemon mode |
| HTTP API | ✅ | ✅ | ✅ | Port 8080 |
| Web UI | ✅ | ✅ | ✅ | **Now uniform!** |
| WebSocket Logs | ✅ | ✅ | ✅ | Real-time updates |
| Notifications (Slack) | ✅ | ✅ | ✅ | All methods |
| Retention Policies | ✅ | ✅ | ✅ | All methods |
| CLI Commands | ✅ | ✅ | ✅ | backup, restore, list, daemon |

### ✅ Platform Support

**Status**: **MAXIMIZED** - Broad platform coverage

| Platform | Docker | Binary Release | Notes |
|----------|--------|----------------|-------|
| Linux amd64 | ✅ | ✅ | Universal support |
| Linux arm64 | ✅ | ✅ | ARM servers, Graviton, Raspberry Pi |
| macOS amd64 (Intel) | ❌ | ✅ | Docker not needed on macOS |
| macOS arm64 (Apple Silicon) | ❌ | ✅ | Docker not needed on macOS |
| Windows amd64 | ❌ | ✅ | Docker not common on Windows |

**Rationale**:

- Docker: Focus on Linux (primary server OS)
- Binaries: Cover all platforms for flexibility

## Validation Tests

### Test 1: Version Information Consistency

```bash
# Docker
docker run --rm ektowett/bared:latest --version
# Expected: v1.0.0 (commit: abc1234, built: 2025-12-04T10:30:45Z)

# Binary
./brd-linux-amd64 --version
# Expected: v1.0.0 (commit: abc1234, built: 2025-12-04T10:30:45Z)

# Local
./bin/brd --version
# Expected: v1.0.0 (commit: abc1234, built: 2025-12-04T10:30:45Z)
```

### Test 2: Web UI Accessibility

```bash
# Docker
docker run -p 8080:8080 ektowett/bared:latest daemon --http :8080 --http-user admin --http-pass test
curl http://localhost:8080/
# Expected: HTML with React app

# Binary
./brd-linux-amd64 daemon --http :8080 --http-user admin --http-pass test
curl http://localhost:8080/
# Expected: HTML with React app

# Local
./bin/brd daemon --http :8080 --http-user admin --http-pass test
curl http://localhost:8080/
# Expected: HTML with React app
```

### Test 3: Feature Parity

```bash
# All methods should support identical commands:
<binary> --help
<binary> backup --target <name>
<binary> restore --target <name> --file <path>
<binary> list --target <name>
<binary> daemon --config <path>
<binary> validate-config --config <path>
```

### Test 4: Static Binary Verification

```bash
# Verify no dynamic dependencies (except system libraries)
ldd ./brd-linux-amd64
# Expected: "not a dynamic executable" (fully static)

# Or for Docker:
docker run --rm ektowett/bared:latest sh -c "ldd /usr/local/bin/brd"
# Expected: Should show minimal dependencies
```

## Changes Made for Uniformity

### 1. Binary Release Workflow (`.github/workflows/release.yml`)

**Before**:

- ❌ No web frontend
- ❌ CGO not explicitly disabled
- ❌ Different date format (`%Y-%m-%d_%H:%M:%S`)
- ❌ Missing web build step

**After**:

- ✅ Web frontend built and embedded
- ✅ `CGO_ENABLED=0` for all platforms
- ✅ RFC3339 date format (`%Y-%m-%dT%H:%M:%SZ`)
- ✅ Web toolchain setup and build step added (Bun; this said Node/npm when written)

### 2. Makefile (Local Builds)

**Before**:

- ❌ CGO not explicitly disabled
- ❌ Different date format
- ❌ Missing linux/arm64 in build-all

**After**:

- ✅ `CGO_ENABLED=0` for all builds
- ✅ RFC3339 date format
- ✅ linux/arm64 added to build-all

### 3. Docker Workflow (`.github/workflows/docker.yml`)

**Before**:

- ✅ Already had web frontend
- ✅ Already had CGO_ENABLED=0
- ✅ Already had RFC3339 format

**After**:

- ✅ No changes needed (already correct)

## CI/CD Integration

### Validation Flow

```
Git Tag (v1.0.0) Push
    |
    ├─> CI Workflow
    |   ├─> Go Tests (test, lint)
    |   ├─> Web Frontend (type-check, lint, format, build)
    |   └─> Build Verification
    |
    ├─> Release Workflow (waits for CI)
    |   ├─> Build Web Frontend
    |   ├─> Build 5 Platform Binaries (with embedded web)
    |   └─> Create GitHub Release
    |
    └─> Docker Workflow (waits for CI)
        ├─> Build Multi-Platform Images (with embedded web)
        └─> Push to Docker Hub
```

### Quality Gates

All distribution methods must pass:

1. ✅ Go unit tests (coverage ratchet — see `COVERAGE_THRESHOLD` in the `Makefile`)
2. ✅ Go linting (golangci-lint)
3. ✅ Web type checking (TypeScript strict)
4. ✅ Web linting (ESLint)
5. ✅ Web formatting (Prettier)
6. ✅ Web build (Vite production build)
7. ✅ Config validation (example config)

## User Experience Consistency

### Installation Experience

**Docker**:

```bash
docker pull ektowett/bared:latest
docker run -v /backups:/backups -v ./config.yml:/etc/bared/bared.yml \
  -p 8080:8080 ektowett/bared:latest daemon --http :8080
# Web UI: http://localhost:8080
```

**Binary (Linux)**:

```bash
wget https://github.com/etowett/bared/releases/download/v0.4.0/brd-linux-amd64.tar.gz
tar xzf brd-linux-amd64.tar.gz
sudo mv brd-linux-amd64 /usr/local/bin/brd
brd daemon --config config.yml --http :8080
# Web UI: http://localhost:8080
```

**Local Build**:

```bash
make build-with-web
./bin/brd daemon --config config.yml --http :8080
# Web UI: http://localhost:8080
```

### Runtime Experience

All methods provide:

- ✅ Identical CLI interface
- ✅ Identical web UI
- ✅ Identical API endpoints
- ✅ Identical feature set
- ✅ Identical configuration format
- ✅ Identical version reporting

## Summary

### Before Fixes

| Aspect | Uniform? | Issue |
|--------|----------|-------|
| Web Frontend | ❌ No | Binary releases missing web UI |
| Version Format | ❌ No | Different date formats |
| CGO Handling | ❌ No | Inconsistent static compilation |
| Build Flags | ⚠️ Mostly | Minor differences |

### After Fixes

| Aspect | Uniform? | Status |
|--------|----------|--------|
| Web Frontend | ✅ Yes | All methods include embedded web UI |
| Version Format | ✅ Yes | RFC3339 format everywhere |
| CGO Handling | ✅ Yes | All methods use CGO_ENABLED=0 |
| Build Flags | ✅ Yes | Consistent across all methods |
| Features | ✅ Yes | Complete parity |
| Quality | ✅ Yes | All pass same CI checks |

## Recommendations for Users

### When to Use Docker

- Production deployments
- Containerized environments (Kubernetes, Docker Compose)
- Consistent runtime environment needed
- ARM64 servers (AWS Graviton, etc.)

### When to Use Binary Releases

- Traditional server deployments
- Systemd service installation
- Windows/macOS development
- Offline environments (air-gapped)

### When to Build Locally

- Development
- Custom modifications
- Testing unreleased features
- Learning the codebase

**Note**: All methods produce functionally identical applications!

## Maintenance

To maintain uniformity:

1. Always build web frontend before Go binary
2. Use `CGO_ENABLED=0` for all builds
3. Use RFC3339 date format (`date -u +'%Y-%m-%dT%H:%M:%SZ'`)
4. Use identical ldflags across all build methods
5. Test all distribution methods after changes
6. Keep Dockerfile, release.yml, and Makefile in sync

## Verification Checklist

Before release, verify:

- [ ] Docker image includes web UI (test at localhost:8080)
- [ ] Binary releases include web UI (test at localhost:8080)
- [ ] Version info is identical across methods
- [ ] All binaries are static (no dynamic dependencies)
- [ ] Date format is RFC3339 everywhere
- [ ] CI passes for all distribution methods
- [ ] Web frontend validation passes
- [ ] Go tests pass
- [ ] All platforms build successfully
