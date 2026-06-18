---
name: release
description: Cut a BareD release — tag a new version so GitHub Actions runs GoReleaser to publish a GitHub release (binaries + checksums) and Docker images. Use when the user wants to "cut a release", "publish a new version", "tag a release", "ship vX.Y.Z", "do a release", or asks how releases work / how to release manually.
---

# Cut a BareD release

BareD releases are **automated**: merging a PR to `main` creates a version tag, which triggers the build/publish workflows. Releasing is normally about labeling the PR correctly — manual tagging is the emergency/fallback path.

## When to use
- "Cut a release" / "publish a new version" / "tag a release" / "ship vX.Y.Z"
- "How do I do a manual release?" / a release failed and needs re-running

## How the automation works (the normal path)
1. Every PR carries **one** release label: `release:major` / `release:minor` / `release:patch` / `release:skip`. No label → defaults to **patch**.
2. On merge to `main`, `.github/workflows/auto-release.yml` finds the merged PR, reads its label, computes the next `vMAJOR.MINOR.PATCH` from the latest tag, and pushes a `vX.Y.Z` annotated tag (skipped if labeled `release:skip` or no PR found).
3. The pushed `v*.*.*` tag triggers `.github/workflows/release.yml`: builds the web UI, embeds it, then runs **GoReleaser** (`release --clean`) to publish a GitHub release — binaries for linux/darwin (amd64+arm64) + windows/amd64, `.tar.gz`/`.zip` archives, `checksums.txt`, and a github-native changelog.
4. On the Release workflow's success, `.github/workflows/docker.yml` builds and pushes the `ektowett/bared` image (tags: `vX.Y.Z` + `latest`) to Docker Hub.

Total time is roughly 15–20 min (tag ~1 min, binaries ~5–10 min, Docker ~10–15 min).

## Steps to release
- **Normal:** ensure the merged PR had the right `release:*` label — that's the whole release. Confirm `auto-release` created the tag, then watch the downstream workflows.
- **Manual / emergency** (if automation fails), from CONTRIBUTING.md:
  ```sh
  git tag -a v1.2.3 -m "Release v1.2.3"
  git push origin v1.2.3   # triggers the Release + Docker workflows
  ```
- **Re-run after a failure:** fix the issue, delete the bad tag and retry:
  ```sh
  git tag -d v1.2.3 && git push origin :refs/tags/v1.2.3
  ```

## Dry-run locally before tagging (from CONTRIBUTING.md)
```sh
brew install goreleaser        # one-time
goreleaser check               # validate .goreleaser.yml
goreleaser release --snapshot --clean   # full build, does NOT publish
ls -lh dist/                   # inspect built archives + checksums
```

## Verify
```sh
gh run list --workflow=release.yml        # Release workflow status
gh run list --workflow=docker.yml         # Docker build status
gh release view vX.Y.Z                     # confirm assets + changelog
docker pull ektowett/bared:vX.Y.Z          # confirm the image was pushed
```
Keep to the real workflow — do not invent extra steps. Tag format is always `vX.Y.Z`.
