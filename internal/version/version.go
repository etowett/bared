// Package version provides version information for the application.
package version

// Version information (set by ldflags during build)
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// GetVersion returns the full version string
func GetVersion() string {
	return Version
}

// GetFullVersion returns the full version string with commit and build date
func GetFullVersion() string {
	return Version + " (commit: " + Commit + ", built: " + BuildDate + ")"
}
