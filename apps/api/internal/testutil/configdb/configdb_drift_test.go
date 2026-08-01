// This test lives in the external test package so it can import persistence.
// configdb itself must not: persistence imports jobs, which imports
// configservice, and configservice's own tests use configdb.
package configdb_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/persistence"
	"github.com/etowett/bared/apps/api/internal/testutil/configdb"
)

// tableDDL maps table name to its CREATE statement as SQLite recorded it.
func tableDDL(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()

	rows, err := db.Query(`SELECT name, sql FROM sqlite_master WHERE type = 'table' AND sql IS NOT NULL`)
	require.NoError(t, err)
	defer rows.Close() //nolint:errcheck // read-only query on a throwaway database

	ddl := make(map[string]string)
	for rows.Next() {
		var name, stmt string
		require.NoError(t, rows.Scan(&name, &stmt))
		ddl[name] = normalizeDDL(stmt)
	}
	require.NoError(t, rows.Err())

	return ddl
}

// normalizeDDL makes the two sources comparable: persistence guards every table
// with IF NOT EXISTS and the two files indent differently. Neither difference
// changes the resulting table.
func normalizeDDL(stmt string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(stmt, "IF NOT EXISTS ", "")), " ")
}

// configdb.Schema is a hand-copy of the config half of persistence.initSchema,
// because the import graph rules out sharing the real one. A copy with only a
// "keep it in step" comment guarding it drifts silently, and every test built on
// it would then be asserting against a schema production never had. This fails
// the moment the two diverge.
func TestSchema_MatchesPersistence(t *testing.T) {
	store, err := persistence.NewSQLStore("sqlite3", filepath.Join(t.TempDir(), "persistence.db"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close())
	})

	production := tableDDL(t, store.DB())
	helper := tableDDL(t, configdb.New(t))

	require.NotEmpty(t, helper, "configdb applied no tables")

	for table, want := range helper {
		got, ok := production[table]
		require.True(t, ok, "configdb defines table %q that persistence.initSchema does not", table)
		require.Equal(t, got, want,
			"configdb.Schema has drifted from persistence.initSchema for table %q", table)
	}
}
