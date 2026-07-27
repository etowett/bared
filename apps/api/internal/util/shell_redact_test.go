package util

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExecuteCommand_RedactsCredentials is the reproduction from issue #133,
// inverted: no credential-carrying argv may survive into a returned error.
// Those errors end up in jobs.Job.Error, the jobs table, and /api/jobs.
func TestExecuteCommand_RedactsCredentials(t *testing.T) {
	const password = "Xk9pQ2mZvR7t" // deliberately contains none of the redactor keywords

	tests := []struct {
		name        string
		args        []string
		wantMasked  string
		wantVisible []string
	}{
		{
			name:        "mysqldump attached password",
			args:        []string{"--host=db", "--user=root", "--password=" + password},
			wantMasked:  "--password=" + Redacted,
			wantVisible: []string{"--host=db", "--user=root"},
		},
		{
			name:       "separate token password",
			args:       []string{"--host", "db", "--password", password},
			wantMasked: "--password " + Redacted,
		},
		{
			name:        "redis-cli auth short flag",
			args:        []string{"-h", "db", "-p", "6379", "-a", password},
			wantMasked:  "-a " + Redacted,
			wantVisible: []string{"-p 6379"},
		},
		{
			name:       "redis-cli long pass",
			args:       []string{"--pass", password},
			wantMasked: "--pass " + Redacted,
		},
		{
			name:       "postgres env-style token",
			args:       []string{"PGPASSWORD=" + password},
			wantMasked: "PGPASSWORD=" + Redacted,
		},
		{
			name:       "generic secret flag",
			args:       []string{"--client-secret=" + password},
			wantMasked: "--client-secret=" + Redacted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			assertRedacted := func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				assert.NotContains(t, err.Error(), password, "credential leaked into error: %v", err)
				assert.Contains(t, err.Error(), tt.wantMasked)
				for _, visible := range tt.wantVisible {
					assert.Contains(t, err.Error(), visible, "non-secret argv should stay debuggable")
				}
			}

			t.Run("ExecuteCommand", func(t *testing.T) {
				assertRedacted(t, ExecuteCommand(ctx, io.Discard, getFalseCommand(), tt.args...))
			})

			t.Run("ExecuteCommandWithStderr", func(t *testing.T) {
				var stderr bytes.Buffer
				assertRedacted(t, ExecuteCommandWithStderr(ctx, io.Discard, &stderr, getFalseCommand(), tt.args...))
			})

			t.Run("ExecuteCommandWithStdin", func(t *testing.T) {
				assertRedacted(t, ExecuteCommandWithStdin(ctx, strings.NewReader(""), getFalseCommand(), tt.args...))
			})

			t.Run("ExecuteCommandOutput", func(t *testing.T) {
				_, err := ExecuteCommandOutput(ctx, getFalseCommand(), tt.args...)
				assertRedacted(t, err)
			})
		})
	}
}

// TestExecuteCommand_RedactsStderr covers the second leak route: an engine that
// echoes its own command line on failure leaks through the stderr text
// interpolated into the same error.
func TestExecuteCommand_RedactsStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX shell")
	}

	const password = "Xk9pQ2mZvR7t" // deliberately contains none of the redactor keywords
	script := "echo 'usage: mysqldump --host=db --password=" + password + "' >&2; exit 1"
	ctx := context.Background()

	t.Run("ExecuteCommand", func(t *testing.T) {
		err := ExecuteCommand(ctx, io.Discard, "sh", "-c", script)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), password)
		assert.Contains(t, err.Error(), "--password="+Redacted)
	})

	t.Run("ExecuteCommandWithStdin", func(t *testing.T) {
		err := ExecuteCommandWithStdin(ctx, strings.NewReader(""), "sh", "-c", script)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), password)
	})

	t.Run("ExecuteCommandOutput", func(t *testing.T) {
		_, err := ExecuteCommandOutput(ctx, "sh", "-c", script)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), password)
	})
}
