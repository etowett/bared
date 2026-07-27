package daemon

import (
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3" // SQLite driver for the test DB
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/util"
)

// newKeyTestDB returns an empty in-memory DB with just the encryption_keys
// table, mirroring the schema persistence.SQLStore creates.
func newKeyTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() {
		//nolint:errcheck // closing an in-memory test DB
		_ = db.Close()
	})

	_, err = db.Exec(`
		CREATE TABLE encryption_keys (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_data TEXT NOT NULL,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`)
	require.NoError(t, err)

	return db
}

func TestInitializeEncryptionKey_FromEnv(t *testing.T) {
	validKey := make([]byte, encryptionKeyBytes)
	for i := range validKey {
		validKey[i] = byte(i)
	}

	tests := []struct {
		name    string
		envKey  string
		wantErr bool
		// wantErrContains are substrings the failure must name so the operator
		// can act on it without reading the source.
		wantErrContains []string
	}{
		{
			name:   "valid base64 32-byte key",
			envKey: base64.StdEncoding.EncodeToString(validKey),
		},
		{
			name:            "64 hex chars, the units the old message wrongly advertised",
			envKey:          hex.EncodeToString(validKey),
			wantErr:         true,
			wantErrContains: []string{"base64", "openssl rand -base64 32"},
		},
		{
			name:            "not base64 at all",
			envKey:          "not a key!!",
			wantErr:         true,
			wantErrContains: []string{"base64", "openssl rand -base64 32"},
		},
		{
			name:            "base64 but too short",
			envKey:          base64.StdEncoding.EncodeToString([]byte("too short")),
			wantErr:         true,
			wantErrContains: []string{"9 bytes", "want 32", "openssl rand -base64 32"},
		},
		{
			name:            "base64 but too long",
			envKey:          base64.StdEncoding.EncodeToString(make([]byte, 64)),
			wantErr:         true,
			wantErrContains: []string{"64 bytes", "want 32", "openssl rand -base64 32"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BARED_ENCRYPTION_KEY", tt.envKey)

			key, err := initializeEncryptionKey(newKeyTestDB(t), util.GetLogger())

			if tt.wantErr {
				require.Error(t, err)
				for _, want := range tt.wantErrContains {
					assert.Contains(t, err.Error(), want)
				}
				// The message must never suggest hex, which is what sent
				// operators into a second failure.
				assert.NotContains(t, strings.ToLower(err.Error()), "hex")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, validKey, key)
		})
	}
}

// With no env var set, a key is generated and persisted; a second start reuses
// it rather than generating a new one (which would orphan every credential).
func TestInitializeEncryptionKey_GeneratesAndReusesDBKey(t *testing.T) {
	t.Setenv("BARED_ENCRYPTION_KEY", "")
	db := newKeyTestDB(t)

	first, err := initializeEncryptionKey(db, util.GetLogger())
	require.NoError(t, err)
	assert.Len(t, first, encryptionKeyBytes)

	second, err := initializeEncryptionKey(db, util.GetLogger())
	require.NoError(t, err)
	assert.Equal(t, first, second, "a restart must reuse the stored key")

	var rows int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM encryption_keys`).Scan(&rows))
	assert.Equal(t, 1, rows, "a second start must not store a second key")
}

// The env var wins over the stored key. This is the ordering trap: setting it
// after a key already exists silently switches to a different key, and anything
// encrypted with the stored one can no longer be decrypted.
func TestInitializeEncryptionKey_EnvOverridesStoredKey(t *testing.T) {
	t.Setenv("BARED_ENCRYPTION_KEY", "")
	db := newKeyTestDB(t)

	stored, err := initializeEncryptionKey(db, util.GetLogger())
	require.NoError(t, err)

	envKey := make([]byte, encryptionKeyBytes)
	for i := range envKey {
		envKey[i] = 0xAB
	}
	t.Setenv("BARED_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(envKey))

	got, err := initializeEncryptionKey(db, util.GetLogger())
	require.NoError(t, err)
	assert.Equal(t, envKey, got)
	assert.NotEqual(t, stored, got, "the env var takes precedence over the stored key")
}
