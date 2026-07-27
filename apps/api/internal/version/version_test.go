package version

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

// stubBuildInfo replaces the debug.ReadBuildInfo seam for one test. Passing a
// nil info models a binary with no readable build info at all.
func stubBuildInfo(t *testing.T, info *debug.BuildInfo) {
	t.Helper()
	original := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		if info == nil {
			return nil, false
		}
		return info, true
	}
	t.Cleanup(func() { readBuildInfo = original })
}

// stubLdflags sets the ldflags-stamped variables for one test.
func stubLdflags(t *testing.T, version, commit, buildDate string) {
	t.Helper()
	origVersion, origCommit, origBuildDate := Version, Commit, BuildDate
	Version, Commit, BuildDate = version, commit, buildDate
	t.Cleanup(func() {
		Version, Commit, BuildDate = origVersion, origCommit, origBuildDate
	})
}

// buildInfoWith assembles a *debug.BuildInfo carrying a module version and the
// vcs settings the toolchain stamps for checkout builds.
func buildInfoWith(modVersion, revision, vcsTime string) *debug.BuildInfo {
	info := &debug.BuildInfo{}
	info.Main.Path = "github.com/etowett/bared/apps/api"
	info.Main.Version = modVersion
	if revision != "" {
		info.Settings = append(info.Settings, debug.BuildSetting{Key: "vcs.revision", Value: revision})
	}
	if vcsTime != "" {
		info.Settings = append(info.Settings, debug.BuildSetting{Key: "vcs.time", Value: vcsTime})
	}
	return info
}

// --- ldflags-stamped builds (release binaries, `make build`) -----------------

func TestGetVersion_PrefersLdflags(t *testing.T) {
	// A release build must report exactly what was stamped, even though build
	// info is also present and says something different. This is the guard on
	// #79's fix: the fallback must never override a real stamped version.
	stubLdflags(t, "v0.4.0", "abc1234", "2026-07-27T00:00:00Z")
	stubBuildInfo(t, buildInfoWith("v0.0.0-20260727065336-4b503be846d6", "deadbeefcafe", "2020-01-01T00:00:00Z"))

	assert.Equal(t, "v0.4.0", GetVersion())
	assert.Equal(t, "v0.4.0 (commit: abc1234, built: 2026-07-27T00:00:00Z)", GetFullVersion())
}

// --- go install builds (no ldflags, module version available) ---------------

func TestGetVersion_FallsBackToModulePseudoVersion(t *testing.T) {
	// The regression test for #106: `go install ...@latest` produces a binary
	// with no ldflags. It must report the module pseudo-version rather than the
	// literal string "dev".
	stubLdflags(t, devVersion, devCommit, devBuildDate)
	stubBuildInfo(t, buildInfoWith("v0.0.0-20260727065336-4b503be846d6", "", ""))

	assert.Equal(t, "v0.0.0-20260727065336-4b503be846d6", GetVersion())
	assert.NotEqual(t, devVersion, GetVersion())
}

func TestGetVersion_FallsBackToTaggedModuleVersion(t *testing.T) {
	// Once apps/api/vX.Y.Z tags exist, `go install ...@latest` resolves to a
	// real release version rather than a pseudo-version.
	stubLdflags(t, devVersion, devCommit, devBuildDate)
	stubBuildInfo(t, buildInfoWith("v0.4.0", "", ""))

	assert.Equal(t, "v0.4.0", GetVersion())
}

func TestGetFullVersion_FallsBackToVCSStamps(t *testing.T) {
	// A plain `go build` from a checkout has no module version but does carry
	// vcs.revision and vcs.time.
	stubLdflags(t, devVersion, devCommit, devBuildDate)
	stubBuildInfo(t, buildInfoWith("(devel)", "4b503be846d6f00dc0ffee", "2026-07-27T06:53:36Z"))

	// Revision is trimmed to the same width GoReleaser's .ShortCommit uses.
	assert.Equal(t, "dev (commit: 4b503be, built: 2026-07-27T06:53:36Z)", GetFullVersion())
}

func TestResolve_PartialLdflags(t *testing.T) {
	// Only the version was stamped; commit and date still fall back.
	stubLdflags(t, "v1.2.3", devCommit, devBuildDate)
	stubBuildInfo(t, buildInfoWith("v0.9.9", "abcdef0123456", "2026-01-02T03:04:05Z"))

	version, commit, buildDate := resolve()
	assert.Equal(t, "v1.2.3", version)
	assert.Equal(t, "abcdef0", commit)
	assert.Equal(t, "2026-01-02T03:04:05Z", buildDate)
}

func TestResolve_EmptyLdflagsValueCountsAsUnstamped(t *testing.T) {
	// `-X ...Version=` yields "" rather than the default, and must still fall back.
	stubLdflags(t, "", "", "")
	stubBuildInfo(t, buildInfoWith("v0.5.0", "1234567890", "2026-02-03T00:00:00Z"))

	version, commit, buildDate := resolve()
	assert.Equal(t, "v0.5.0", version)
	assert.Equal(t, "1234567", commit)
	assert.Equal(t, "2026-02-03T00:00:00Z", buildDate)
}

// --- degenerate builds: nothing better than the placeholders ----------------

func TestGetVersion_DevelModuleVersionStaysDev(t *testing.T) {
	// "(devel)" carries no more information than "dev", so it is not used.
	stubLdflags(t, devVersion, devCommit, devBuildDate)
	stubBuildInfo(t, buildInfoWith("(devel)", "", ""))

	assert.Equal(t, devVersion, GetVersion())
}

func TestGetVersion_NoBuildInfoStaysDev(t *testing.T) {
	stubLdflags(t, devVersion, devCommit, devBuildDate)
	stubBuildInfo(t, nil)

	assert.Equal(t, devVersion, GetVersion())
	assert.Equal(t, "dev (commit: none, built: unknown)", GetFullVersion())
}

func TestGetVersion_EmptyModuleVersionStaysDev(t *testing.T) {
	stubLdflags(t, devVersion, devCommit, devBuildDate)
	stubBuildInfo(t, buildInfoWith("", "", ""))

	assert.Equal(t, devVersion, GetVersion())
}

// --- helpers ----------------------------------------------------------------

func TestIsUnstamped(t *testing.T) {
	assert.True(t, isUnstamped("", devVersion))
	assert.True(t, isUnstamped(devVersion, devVersion))
	assert.True(t, isUnstamped(devCommit, devCommit))
	assert.False(t, isUnstamped("v0.4.0", devVersion))
	assert.False(t, isUnstamped("dev", devCommit)) // placeholder is per-field
}

func TestShortenCommit(t *testing.T) {
	assert.Equal(t, "abcdef0", shortenCommit("abcdef0123456789"))
	assert.Equal(t, "abcdef0", shortenCommit("abcdef0"))
	assert.Equal(t, "abc", shortenCommit("abc"), "short revisions are returned unchanged")
	assert.Equal(t, "", shortenCommit(""))
}

func TestBuildInfo_ReadsVCSSettings(t *testing.T) {
	stubBuildInfo(t, buildInfoWith("v1.0.0", "cafebabe", "2026-03-04T05:06:07Z"))

	modVersion, revision, vcsTime := buildInfo()
	assert.Equal(t, "v1.0.0", modVersion)
	assert.Equal(t, "cafebabe", revision, "buildInfo returns the raw revision; shortening happens in resolve")
	assert.Equal(t, "2026-03-04T05:06:07Z", vcsTime)
}

func TestBuildInfo_Unavailable(t *testing.T) {
	stubBuildInfo(t, nil)

	modVersion, revision, vcsTime := buildInfo()
	assert.Empty(t, modVersion)
	assert.Empty(t, revision)
	assert.Empty(t, vcsTime)
}

// --- format and defaults ----------------------------------------------------

func TestGetFullVersion_Format(t *testing.T) {
	stubLdflags(t, "v1.2.3", "abc1234", "2026-07-27T00:00:00Z")
	stubBuildInfo(t, nil)

	fullVersion := GetFullVersion()
	assert.Contains(t, fullVersion, "(")
	assert.Contains(t, fullVersion, ")")
	assert.Contains(t, fullVersion, "commit:")
	assert.Contains(t, fullVersion, ", built:")
	assert.Greater(t, len(fullVersion), len(GetVersion()))
}

func TestDefaultVariables(t *testing.T) {
	// The package-level defaults are what an unstamped build starts from; the
	// test binary itself is unstamped, so they are still in place here.
	assert.Equal(t, devVersion, Version)
	assert.Equal(t, devCommit, Commit)
	assert.Equal(t, devBuildDate, BuildDate)
}

func TestGetVersion_TestBinaryReportsDev(t *testing.T) {
	// The test binary has no ldflags and records Main.Version="(devel)" with no
	// vcs settings, so the real (unstubbed) path still yields the placeholders.
	assert.Equal(t, devVersion, GetVersion())
	assert.Equal(t, "dev (commit: none, built: unknown)", GetFullVersion())
}
