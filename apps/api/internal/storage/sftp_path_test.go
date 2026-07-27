package storage

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/etowett/bared/apps/api/internal/config"
	"github.com/etowett/bared/apps/api/internal/util"
)

// captureLogs collects log messages emitted while fn runs.
func captureLogs(t *testing.T, fn func()) []string {
	t.Helper()

	var messages []string
	util.SetLogHook(func(_ util.LogLevel, message string) {
		messages = append(messages, message)
	})
	t.Cleanup(func() { util.SetLogHook(nil) })

	fn()

	return messages
}

// An SFTP backend with no path writes to the SSH login directory. That is the
// symptom #104 produced, and the reason it went unnoticed was that nothing
// said so — so say so.
func TestNewSFTP_WarnsWhenRemotePathIsEmpty(t *testing.T) {
	messages := captureLogs(t, func() {
		NewSFTP(&config.Storage{
			Name: "offsite", Type: "sftp",
			Host: "backup.example.com", Port: 22, Username: "backup", Password: "shhh",
		})
	})

	joined := strings.Join(messages, "\n")
	assert.Contains(t, joined, "SSH login directory")
	assert.NotContains(t, joined, "shhh", "credentials must never be logged")
}

func TestNewSFTP_SilentWhenRemotePathIsSet(t *testing.T) {
	messages := captureLogs(t, func() {
		NewSFTP(&config.Storage{
			Name: "offsite", Type: "sftp",
			Host: "backup.example.com", Port: 22, Username: "backup", Password: "shhh",
			Path: "/srv/backups",
		})
	})

	assert.Empty(t, messages)
}
