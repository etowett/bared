# Release Process

This document describes the automated release process for BareD, including the workflow architecture, version management, and operational procedures.

## Overview

BareD uses a fully automated release system that:
- Creates releases on every merge to `main` (unless skipped)
- Uses PR labels to control semantic versioning
- Builds multi-platform binaries with GoReleaser
- Creates GitHub releases with changelogs
- Publishes Docker images to Docker Hub
- Completes the entire process in ~15-20 minutes

## Architecture

### Three-Workflow System

```
┌─────────────────────────────────────────────────────────────┐
│                     Developer Workflow                       │
├─────────────────────────────────────────────────────────────┤
│  1. Create PR with changes                                   │
│  2. Add release label (major/minor/patch/skip)               │
│  3. Review & merge to main                                   │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────┐
│         Workflow 1: Auto Release (.github/workflows/        │
│                     auto-release.yml)                        │
├─────────────────────────────────────────────────────────────┤
│  Trigger: Push to main                                       │
│  Actions:                                                    │
│    1. Detect merged PR number                                │
│    2. Check for release:skip label → exit if present         │
│    3. Get latest git tag (or v0.0.0)                         │
│    4. Detect version bump from PR labels                     │
│    5. Calculate next version (semver)                        │
│    6. Create annotated git tag                               │
│    7. Push tag to origin                                     │
│  Duration: ~1 minute                                         │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  │ (tag push triggers)
                  ▼
┌─────────────────────────────────────────────────────────────┐
│      Workflow 2: Binary Release (.github/workflows/         │
│                   release.yml)                               │
├─────────────────────────────────────────────────────────────┤
│  Trigger: Tag push (v*.*.*)                                  │
│  Actions:                                                    │
│    1. Checkout with full history                             │
│    2. Setup Go 1.26.1 & Node.js                              │
│    3. Build web frontend (npm ci && npm run build)           │
│    4. Copy web/dist to internal/web/dist                     │
│    5. Run GoReleaser:                                        │
│       - Cross-compile for 5 platforms                        │
│       - Create archives (tar.gz, zip)                        │
│       - Generate SHA256 checksums                            │
│       - Generate changelog                                   │
│       - Create GitHub release                                │
│       - Upload all artifacts                                 │
│  Duration: ~5-10 minutes                                     │
└─────────────────┬───────────────────────────────────────────┘
                  │
                  │ (on success triggers)
                  ▼
┌─────────────────────────────────────────────────────────────┐
│       Workflow 3: Docker Release (.github/workflows/        │
│                    docker.yml)                               │
├─────────────────────────────────────────────────────────────┤
│  Trigger: Release workflow completion (workflow_run)         │
│  Condition: Only if release workflow succeeded               │
│  Actions:                                                    │
│    1. Extract version from git tag                           │
│    2. Setup QEMU & Docker Buildx                             │
│    3. Login to Docker Hub                                    │
│    4. Build multi-platform images:                           │
│       - linux/amd64                                          │
│       - linux/arm64                                          │
│    5. Tag with semantic versions:                            │
│       - ektowett/bared:1.2.3                                 │
│       - ektowett/bared:1.2                                   │
│       - ektowett/bared:1                                     │
│       - ektowett/bared:latest                                │
│    6. Push images to Docker Hub                              │
│    7. Update Docker Hub description                          │
│  Duration: ~10-15 minutes                                    │
└─────────────────────────────────────────────────────────────┘
```

### Version Calculation

**Semantic Versioning Rules:**

```
Current version: v1.5.2

PR label: release:major
Next version: v2.0.0 (breaking changes)

PR label: release:minor
Next version: v1.6.0 (new features)

PR label: release:patch (or no label)
Next version: v1.5.3 (bug fixes)

PR label: release:skip
Next version: none (no release created)
```

**Implementation:** `.github/workflows/auto-release.yml` lines 69-97

```bash
# Parse current version
CURRENT="v1.5.2"
MAJOR=1, MINOR=5, PATCH=2

# Apply bump
case "$BUMP_TYPE" in
  major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
  minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
  patch) PATCH=$((PATCH + 1)) ;;
esac

# Result: v2.0.0, v1.6.0, or v1.5.3
```

## Configuration Files

### GoReleaser Configuration

**File:** `.goreleaser.yml`

Key sections:

1. **Before Hooks:** Build web frontend
   ```yaml
   before:
     hooks:
       - sh -c "cd web && npm ci && npm run build"
       - sh -c "mkdir -p internal/web && cp -r web/dist internal/web/dist"
       - go mod tidy
   ```

2. **Build Matrix:** 5 platform combinations
   - Linux: amd64, arm64
   - macOS: amd64, arm64
   - Windows: amd64

3. **LDFLAGS:** Version injection
   ```yaml
   ldflags:
     - -X bared/internal/version.Version={{.Version}}
     - -X bared/internal/version.Commit={{.ShortCommit}}
     - -X bared/internal/version.BuildDate={{.Date}}
   ```

4. **Archives:** Platform-specific formats
   - Unix: `.tar.gz`
   - Windows: `.zip`

5. **Changelog:** Auto-generated with categorization
   - 🚀 Features
   - 💥 Breaking Changes
   - 🐛 Bug Fixes
   - 📚 Documentation
   - 🔧 Maintenance

### PR Labels

**File:** `.github/release-labels.yml`

```yaml
- name: "release:major"
  color: "d73a4a"  # red
  description: "Breaking changes - triggers major version bump (x.0.0)"

- name: "release:minor"
  color: "0e8a16"  # green
  description: "New features - triggers minor version bump (0.x.0)"

- name: "release:patch"
  color: "0075ca"  # blue
  description: "Bug fixes - triggers patch version bump (0.0.x)"

- name: "release:skip"
  color: "ffffff"  # white
  description: "Skip release - no version bump or release"
```

Labels are synced via `.github/workflows/sync-labels.yml` on changes to main.

## Operational Procedures

### Standard Release (Automated)

**Prerequisites:**
- PR is ready to merge
- All CI checks pass
- Code review approved

**Steps:**

1. **Add Release Label**
   ```
   In GitHub PR UI:
   - Click "Labels" on right sidebar
   - Select ONE of: release:major, release:minor, release:patch, release:skip
   ```

2. **Merge PR**
   ```
   - Click "Merge pull request"
   - Confirm merge
   ```

3. **Monitor Automation** (optional)
   ```
   GitHub Actions tab:
   - Watch "Auto Release" workflow (~1 min)
   - Watch "Release" workflow (~5-10 min)
   - Watch "Docker" workflow (~10-15 min)
   ```

4. **Verify Release**
   ```
   Check GitHub Releases page:
   - New version appears
   - All 5 binaries + checksums present
   - Changelog is accurate

   Check Docker Hub:
   - New tags appear (version, latest)
   - Multi-platform manifest exists
   ```

### Manual Release (Emergency)

Use when automation fails or for hotfixes.

**Prerequisites:**
- Maintainer access
- Local git repository up to date

**Steps:**

1. **Determine Next Version**
   ```bash
   # Get latest tag
   git describe --tags --abbrev=0
   # Example output: v1.5.2

   # Decide next version based on changes
   # Bug fix: v1.5.3
   # Feature: v1.6.0
   # Breaking: v2.0.0
   ```

2. **Create Tag**
   ```bash
   git tag -a v1.5.3 -m "Release v1.5.3" -m "Emergency hotfix for critical bug"
   ```

3. **Push Tag**
   ```bash
   git push origin v1.5.3
   ```

4. **Monitor Workflows**
   - Release workflow triggers automatically
   - Docker workflow triggers after release succeeds

### Rollback Release

If a bad release is published:

**Option 1: Delete Release & Tag**

```bash
# Delete local tag
git tag -d v1.5.3

# Delete remote tag
git push origin :refs/tags/v1.5.3

# Manually delete GitHub release in UI:
# Releases → v1.5.3 → Delete release
```

**Option 2: Quick Hotfix**

```bash
# Create hotfix branch from bad release
git checkout -b hotfix/v1.5.4 v1.5.3

# Fix the issue
git commit -am "fix: critical bug in v1.5.3"

# Push and create PR with release:patch label
git push origin hotfix/v1.5.4

# Merge PR → automation creates v1.5.4
```

### Testing Release Process

**Local Testing (Before Merge):**

```bash
# Install GoReleaser
brew install goreleaser

# Validate configuration
goreleaser check

# Dry run (doesn't publish)
goreleaser release --snapshot --clean

# Inspect artifacts
ls -lh dist/
tree dist/

# Test a binary
./dist/brd-darwin-arm64/brd --version
```

**Staging Test (On Feature Branch):**

Cannot fully test without merging, but you can:
- Verify CI passes
- Verify web build succeeds
- Validate GoReleaser config in CI

## Troubleshooting

### Auto-Release Workflow Fails

**Symptom:** Tag not created after merge

**Debugging:**

1. Check workflow run in GitHub Actions
2. Look for error messages in logs
3. Common causes:
   - PR not detected: Check commit is on main
   - Permission denied: Check GITHUB_TOKEN permissions
   - Git operations fail: Check repository state

**Resolution:**
```bash
# Manual tag creation (see Manual Release above)
git tag -a v1.5.3 -m "Manual release after automation failure"
git push origin v1.5.3
```

### Binary Release Workflow Fails

**Symptom:** GitHub release not created

**Debugging:**

1. Check release workflow logs
2. Common causes:
   - Web build failed: Missing node_modules, npm errors
   - Go build failed: Dependency issues, syntax errors
   - GoReleaser error: Config validation, platform issues
   - Permission denied: GitHub token insufficient

**Resolution:**

If web build failed:
```bash
cd web
npm ci
npm run build
# Fix any errors, commit, create new release
```

If Go build failed:
```bash
make build-all
# Fix compilation errors, commit, create new release
```

If GoReleaser config issue:
```bash
goreleaser check
# Fix .goreleaser.yml, commit, create new release
```

### Docker Workflow Fails

**Symptom:** Docker images not on Docker Hub

**Debugging:**

1. Check docker workflow logs
2. Common causes:
   - Docker Hub credentials: Expired token, wrong credentials
   - Buildx failure: Platform build error
   - Network issues: Transient failures
   - Disk space: Build cache too large

**Resolution:**

Retry workflow:
```
GitHub Actions → Docker workflow → Re-run failed jobs
```

If credentials issue:
```
Settings → Secrets → Update DOCKERHUB_TOKEN
```

If persistent failure, build locally:
```bash
make docker-buildx
docker push ektowett/bared:1.5.3
```

### Version Collision

**Symptom:** Tag already exists

**Cause:** Attempting to create duplicate version

**Resolution:**

Delete existing tag first:
```bash
git tag -d v1.5.3
git push origin :refs/tags/v1.5.3

# Then create new tag
git tag -a v1.5.3 -m "Release v1.5.3 (retry)"
git push origin v1.5.3
```

Or skip to next version:
```bash
# Instead of v1.5.3, use v1.5.4
git tag -a v1.5.4 -m "Release v1.5.4"
git push origin v1.5.4
```

## Monitoring & Metrics

### Success Criteria

A successful release should:
- ✅ Complete in < 20 minutes
- ✅ Create GitHub release with all artifacts
- ✅ Push Docker images with correct tags
- ✅ Generate accurate changelog
- ✅ Pass all checksums

### Key Metrics

Track these over time:
- Release frequency (releases per week)
- Release duration (time from merge to Docker push)
- Failure rate (% of failed releases)
- Manual intervention rate (% requiring manual fix)

### Notifications

Consider setting up GitHub Actions notifications:
- Email on workflow failure
- Slack integration for release announcements
- Discord webhook for team updates

## Security Considerations

### Secrets Management

Required secrets in repository settings:
- `GITHUB_TOKEN` (automatic, provided by GitHub)
- `DOCKERHUB_USERNAME` (set in repo secrets)
- `DOCKERHUB_TOKEN` (set in repo secrets)

Never commit:
- Docker credentials
- API tokens
- Private keys

### Supply Chain Security

The release process ensures:
- **Reproducible builds:** Same inputs → same outputs
- **Integrity verification:** SHA256 checksums for all binaries
- **Provenance tracking:** Git commit → tag → release
- **Signed commits:** (optional) GPG sign tags for verification

Future enhancements:
- **SLSA compliance:** Generate provenance attestations
- **Sigstore signing:** Sign binaries with cosign
- **SBOM generation:** Include software bill of materials

## Future Enhancements

### Planned Improvements

1. **Package Managers**
   - Homebrew tap for macOS
   - Scoop bucket for Windows
   - AUR package for Arch Linux
   - APT/YUM repositories for Linux

2. **Release Candidates**
   - Pre-release versions (v1.2.0-rc.1)
   - Beta channel for testing
   - Automatic promotion to stable

3. **Enhanced Testing**
   - Smoke tests before release
   - Integration tests with real databases
   - Binary execution tests on all platforms

4. **Better Changelogs**
   - Contributor credits
   - Breaking change highlights
   - Migration guides
   - Linked documentation

5. **Notifications**
   - Release announcements to Slack/Discord
   - Email to release mailing list
   - Twitter/social media automation

### Conventional Commits Migration

Currently using PR labels. Future: migrate to conventional commits.

Benefits:
- Version bump from commit messages
- No manual labeling required
- Industry standard

Implementation plan:
1. Add commitlint to CI
2. Update PR template with commit format
3. Run both systems in parallel
4. Gradually enforce conventional commits
5. Remove label requirement

Example commits:
```
feat: add PostgreSQL 15 support
fix: resolve MySQL timeout on large databases
docs: update S3 configuration examples
chore: upgrade dependencies
BREAKING CHANGE: remove deprecated --legacy flag
```

## References

- [Semantic Versioning 2.0.0](https://semver.org/)
- [GoReleaser Documentation](https://goreleaser.com/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Docker Buildx Documentation](https://docs.docker.com/buildx/working-with-buildx/)
- [Conventional Commits](https://www.conventionalcommits.org/)

## Changelog

- **2025-12-15:** Initial automated release system implemented
  - Three-workflow architecture
  - PR label-based versioning
  - GoReleaser integration
  - Multi-platform Docker builds
