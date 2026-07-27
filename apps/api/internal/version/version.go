// Package version provides version information for the application.
package version

import "runtime/debug"

// Placeholder values for the ldflags-stamped variables below. A build that did
// not pass -ldflags leaves the variables on these, which is the signal to fall
// back to the build info the Go toolchain embeds.
const (
	devVersion   = "dev"
	devCommit    = "none"
	devBuildDate = "unknown"
)

// Version information (set by ldflags during build).
//
// Release builds (GoReleaser) and `make build` stamp these. Builds that go
// through the module proxy — `go install github.com/etowett/bared/apps/api/cmd/brd@latest`
// — cannot, because the ldflags live in .goreleaser.yml and the Makefile, not
// in the source. For those, the accessors below recover the module version and
// VCS stamps from debug.ReadBuildInfo() rather than reporting a bare "dev".
var (
	Version   = devVersion
	Commit    = devCommit
	BuildDate = devBuildDate
)

// readBuildInfo is a seam for tests. Production always reads the real build info.
var readBuildInfo = debug.ReadBuildInfo

// shortCommitLen matches the width GoReleaser's .ShortCommit stamps, so a
// go-install build and a release build print commits of the same shape.
const shortCommitLen = 7

// GetVersion returns the full version string.
func GetVersion() string {
	version, _, _ := resolve()
	return version
}

// GetFullVersion returns the full version string with commit and build date.
func GetFullVersion() string {
	version, commit, buildDate := resolve()
	return version + " (commit: " + commit + ", built: " + buildDate + ")"
}

// resolve returns the effective version, commit and build date. Values stamped
// via ldflags always win; anything still on its placeholder is filled in from
// the embedded build info when that carries something better.
func resolve() (version, commit, buildDate string) {
	version, commit, buildDate = Version, Commit, BuildDate

	// Nothing to recover if ldflags supplied everything.
	if !isUnstamped(version, devVersion) &&
		!isUnstamped(commit, devCommit) &&
		!isUnstamped(buildDate, devBuildDate) {
		return version, commit, buildDate
	}

	modVersion, revision, vcsTime := buildInfo()

	if isUnstamped(version, devVersion) && modVersion != "" {
		version = modVersion
	}
	if isUnstamped(commit, devCommit) && revision != "" {
		commit = shortenCommit(revision)
	}
	if isUnstamped(buildDate, devBuildDate) && vcsTime != "" {
		buildDate = vcsTime
	}

	return version, commit, buildDate
}

// isUnstamped reports whether value still carries its placeholder, meaning no
// ldflags value was supplied. An empty string counts: `-X ...Version=` sets it
// to "" rather than leaving the default.
func isUnstamped(value, placeholder string) bool {
	return value == "" || value == placeholder
}

// buildInfo extracts the module version and VCS stamps the Go toolchain records
// in the binary. Any of the three may come back empty.
//
// The module version is populated for builds resolved through the module cache
// (`go install pkg@version`) and is a real semver tag or a pseudo-version such
// as v0.0.0-20260727065336-4b503be846d6. The vcs.* settings are the opposite:
// they are stamped only when building from a checkout, and are absent for
// proxy builds.
func buildInfo() (modVersion, revision, vcsTime string) {
	info, ok := readBuildInfo()
	if !ok || info == nil {
		return "", "", ""
	}

	// "(devel)" is what the toolchain records for a build from a working tree
	// with no version selected — no more informative than "dev".
	if v := info.Main.Version; v != "(devel)" {
		modVersion = v
	}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			vcsTime = setting.Value
		}
	}

	return modVersion, revision, vcsTime
}

// shortenCommit trims a full VCS revision to the short form used elsewhere.
func shortenCommit(revision string) string {
	if len(revision) > shortCommitLen {
		return revision[:shortCommitLen]
	}
	return revision
}
