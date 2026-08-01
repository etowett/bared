// Package configdb opens throwaway SQLite databases holding the config tables,
// so tests can exercise configservice and the API config handlers against a real
// database instead of a mock.
//
// The schema is a copy of the config half of persistence.initSchema rather than
// an import of it: internal/persistence imports internal/jobs, which imports
// internal/configservice, so a test helper that both packages can use has to
// stand on its own. It must stay in step with persistence.initSchema —
// TestSchema_MatchesPersistence fails if it does not, so the copy cannot drift
// silently.
package configdb

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3" // SQLite driver for the throwaway test database
)

// Schema is the subset of persistence.initSchema that config rows live in.
const Schema = `
CREATE TABLE storages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT UNIQUE NOT NULL,
	type TEXT NOT NULL,
	config_json TEXT NOT NULL,
	keep INTEGER NOT NULL DEFAULT 7,
	enabled BOOLEAN NOT NULL DEFAULT true,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE notifiers (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT UNIQUE NOT NULL,
	type TEXT NOT NULL,
	config_json TEXT NOT NULL,
	on_success BOOLEAN NOT NULL DEFAULT false,
	enabled BOOLEAN NOT NULL DEFAULT true,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE targets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT UNIQUE NOT NULL,
	type TEXT NOT NULL,
	conn_type TEXT NOT NULL,
	conn_json TEXT NOT NULL,
	storage_name TEXT,
	schedule TEXT,
	compress_enabled BOOLEAN NOT NULL DEFAULT false,
	compress_type TEXT,
	exclude_tables TEXT,
	additional_args TEXT,
	enabled BOOLEAN NOT NULL DEFAULT true,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (storage_name) REFERENCES storages(name) ON DELETE SET NULL
);
CREATE TABLE restore_targets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT UNIQUE NOT NULL,
	conn_type TEXT NOT NULL,
	conn_json TEXT NOT NULL,
	storage_name TEXT,
	source_target TEXT,
	description TEXT,
	enabled BOOLEAN NOT NULL DEFAULT true,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (storage_name) REFERENCES storages(name) ON DELETE SET NULL,
	FOREIGN KEY (source_target) REFERENCES targets(name) ON DELETE SET NULL
);
CREATE TABLE global_config (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE secrets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	ref_type TEXT NOT NULL,
	ref_id INTEGER NOT NULL,
	field_name TEXT NOT NULL,
	encrypted_value TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(ref_type, ref_id, field_name)
);`

// New opens a file-backed SQLite database in the test's temp directory with the
// config schema applied, and closes it when the test ends. It is file-backed
// rather than :memory: because configservice hands the same handle to code that
// opens its own connections.
func New(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "config.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	if _, err := db.Exec(Schema); err != nil {
		t.Fatalf("apply config schema: %v", err)
	}

	return db
}

// TestKey is a 32-byte encryption key for encryption.NewService. It protects
// nothing real — it exists so tests do not each invent their own.
var TestKey = []byte("0123456789abcdef0123456789abcdef")
