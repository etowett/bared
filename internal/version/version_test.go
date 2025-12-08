package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVersion(t *testing.T) {
	// Default value (not set by ldflags)
	version := GetVersion()
	assert.NotEmpty(t, version)
	assert.Equal(t, Version, version)
}

func TestGetVersion_DefaultValue(t *testing.T) {
	// When not built with ldflags, should have default value
	version := GetVersion()
	assert.Equal(t, "dev", version)
}

func TestGetFullVersion(t *testing.T) {
	// Default values (not set by ldflags)
	fullVersion := GetFullVersion()
	assert.NotEmpty(t, fullVersion)
	assert.Contains(t, fullVersion, Version)
	assert.Contains(t, fullVersion, Commit)
	assert.Contains(t, fullVersion, BuildDate)
	assert.Contains(t, fullVersion, "commit:")
	assert.Contains(t, fullVersion, "built:")
}

func TestGetFullVersion_DefaultValues(t *testing.T) {
	// When not built with ldflags, should have default values
	fullVersion := GetFullVersion()
	assert.Contains(t, fullVersion, "dev")
	assert.Contains(t, fullVersion, "none")
	assert.Contains(t, fullVersion, "unknown")
}

func TestGetFullVersion_Format(t *testing.T) {
	// Verify format: "Version (commit: Commit, built: BuildDate)"
	fullVersion := GetFullVersion()

	// Should contain parentheses
	assert.Contains(t, fullVersion, "(")
	assert.Contains(t, fullVersion, ")")

	// Should contain labels
	assert.Contains(t, fullVersion, "commit:")
	assert.Contains(t, fullVersion, "built:")

	// Should have comma separator
	assert.Contains(t, fullVersion, ", built:")
}

func TestVersionVariables(t *testing.T) {
	// Test that version variables are accessible
	assert.NotNil(t, Version)
	assert.NotNil(t, Commit)
	assert.NotNil(t, BuildDate)

	// Test default values
	assert.Equal(t, "dev", Version)
	assert.Equal(t, "none", Commit)
	assert.Equal(t, "unknown", BuildDate)
}

func TestVersionVariables_Types(t *testing.T) {
	// Verify all variables are strings
	var _ string = Version
	var _ string = Commit
	var _ string = BuildDate
}

func TestGetVersion_ConsistentWithVariable(t *testing.T) {
	// GetVersion() should always return Version variable
	assert.Equal(t, Version, GetVersion())
}

func TestGetFullVersion_IncludesAllComponents(t *testing.T) {
	fullVersion := GetFullVersion()

	// Should include version
	assert.Contains(t, fullVersion, Version)

	// Should include commit
	assert.Contains(t, fullVersion, Commit)

	// Should include build date
	assert.Contains(t, fullVersion, BuildDate)
}

func TestGetFullVersion_NotEmpty(t *testing.T) {
	fullVersion := GetFullVersion()
	assert.NotEmpty(t, fullVersion)
	assert.Greater(t, len(fullVersion), len(Version))
}

// Note: Testing with actual ldflags values would require build-time injection
// These tests verify the default behavior when ldflags are not set
