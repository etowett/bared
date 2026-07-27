package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/etowett/bared/apps/api/internal/api"
	"github.com/etowett/bared/apps/api/internal/jobs"
	"github.com/etowett/bared/apps/api/internal/util"

	_ "github.com/mattn/go-sqlite3"
)

const testPassword = "Xk9pQ2mZvR7t" // deliberately contains none of the redactor keywords

func newTestStore(t *testing.T, name string) *SQLStore {
	t.Helper()

	store, err := NewSQLStore("sqlite3", filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Failed to close store: %v", err)
		}
	})

	return store
}

// TestSQLStore_JobErrorNeverReachesAPIInPlaintext walks the whole chain from
// issue #133: a failed job carrying a password, persisted, read back, and
// rendered into the /api/jobs payload.
func TestSQLStore_JobErrorNeverReachesAPIInPlaintext(t *testing.T) {
	store := newTestStore(t, "redact-roundtrip.db")
	ctx := context.Background()

	job := jobs.NewJob(jobs.JobTypeBackup, "leaky_target", true)
	if err := store.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	job.MarkStarted()
	job.MarkFailed(fmt.Errorf(
		"backup failed: mysqldump [--host=db --user=root --password=%s] (exit code 1): exit status 1", testPassword))
	if err := store.UpdateJob(ctx, job); err != nil {
		t.Fatalf("UpdateJob failed: %v", err)
	}

	assertClean := func(t *testing.T, source, errText string) {
		t.Helper()
		if strings.Contains(errText, testPassword) {
			t.Errorf("%s: credential disclosed: %q", source, errText)
		}
		if !strings.Contains(errText, "--password="+util.Redacted) {
			t.Errorf("%s: expected a masked --password, got %q", source, errText)
		}
		if !strings.Contains(errText, "--host=db") {
			t.Errorf("%s: non-secret argv should stay debuggable, got %q", source, errText)
		}
	}

	// The column itself, not just what the API renders.
	var stored string
	if err := store.DB().QueryRowContext(ctx, `SELECT error FROM jobs WHERE id = ?`, job.ID).Scan(&stored); err != nil {
		t.Fatalf("failed to read jobs.error: %v", err)
	}
	assertClean(t, "jobs.error column", stored)

	fetched, err := store.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	assertClean(t, "GetJob → JobToResponse", api.JobToResponse(fetched).Error)

	listed, err := store.ListJobs(ctx, jobs.JobFilter{})
	if err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 job, got %d", len(listed))
	}
	assertClean(t, "ListJobs → JobToResponse", api.JobToResponse(listed[0]).Error)
}

// TestSQLStore_ScrubsPreexistingJobErrors covers rows written by a build from
// before the fix: opening the database must rewrite them in place, and reads
// must be safe regardless.
func TestSQLStore_ScrubsPreexistingJobErrors(t *testing.T) {
	tests := []struct {
		name       string
		errText    string
		wantMasked string
		wantKept   string
		wantRewrit bool
	}{
		{
			name:       "mysqldump attached password",
			errText:    "backup failed: mysqldump [--host=db --user=root --password=" + testPassword + "] (exit code 1)",
			wantMasked: "--password=" + util.Redacted,
			wantKept:   "--host=db",
			wantRewrit: true,
		},
		{
			name:       "redis-cli auth flag",
			errText:    "backup failed: redis-cli [-h db -p 6379 -a " + testPassword + " --rdb /tmp/x.rdb]",
			wantMasked: "-a " + util.Redacted,
			wantKept:   "-p 6379",
			wantRewrit: true,
		},
		{
			name:       "postgres password environment variable",
			errText:    "backup failed: pg_dump (PGPASSWORD=" + testPassword + ")",
			wantMasked: "PGPASSWORD=" + util.Redacted,
			wantKept:   "pg_dump",
			wantRewrit: true,
		},
		{
			name:     "clean error is left alone",
			errText:  "backup failed: pg_dump [--host=db --no-password mydb]: exit status 1",
			wantKept: "--no-password mydb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "legacy.db")

			// Write the contaminated row the way an older build would have.
			seed, err := NewSQLStore("sqlite3", dbPath)
			if err != nil {
				t.Fatalf("Failed to create store: %v", err)
			}
			if _, err := seed.DB().Exec(
				`INSERT INTO jobs (id, type, target_name, status, created_at, error, manual)
				 VALUES ('legacy-1', 'backup', 'leaky_target', 'failed', CURRENT_TIMESTAMP, ?, 1)`,
				tt.errText); err != nil {
				t.Fatalf("failed to seed job row: %v", err)
			}
			if err := seed.Close(); err != nil {
				t.Fatalf("failed to close seed store: %v", err)
			}

			// Reopening runs the scrub.
			store, err := NewSQLStore("sqlite3", dbPath)
			if err != nil {
				t.Fatalf("Failed to reopen store: %v", err)
			}
			defer func() {
				if err := store.Close(); err != nil {
					t.Errorf("Failed to close store: %v", err)
				}
			}()

			var stored string
			if err := store.DB().QueryRow(`SELECT error FROM jobs WHERE id = 'legacy-1'`).Scan(&stored); err != nil {
				t.Fatalf("failed to read jobs.error: %v", err)
			}

			if strings.Contains(stored, testPassword) {
				t.Errorf("credential left at rest in jobs.error: %q", stored)
			}
			if tt.wantRewrit && !strings.Contains(stored, tt.wantMasked) {
				t.Errorf("expected %q in the rewritten row, got %q", tt.wantMasked, stored)
			}
			if !tt.wantRewrit && stored != tt.errText {
				t.Errorf("clean row was rewritten: %q became %q", tt.errText, stored)
			}
			if !strings.Contains(stored, tt.wantKept) {
				t.Errorf("expected %q to survive, got %q", tt.wantKept, stored)
			}
		})
	}
}

func TestSQLStore_ScrubJobErrorsIsIdempotent(t *testing.T) {
	store := newTestStore(t, "idempotent.db")
	ctx := context.Background()

	if _, err := store.DB().ExecContext(ctx,
		`INSERT INTO jobs (id, type, target_name, status, created_at, error, manual)
		 VALUES ('legacy-1', 'backup', 'leaky_target', 'failed', CURRENT_TIMESTAMP, ?, 1)`,
		"mysqldump [--password="+testPassword+"]"); err != nil {
		t.Fatalf("failed to seed job row: %v", err)
	}

	first, err := store.scrubJobErrors(ctx)
	if err != nil {
		t.Fatalf("scrubJobErrors failed: %v", err)
	}
	if first != 1 {
		t.Fatalf("expected 1 row rewritten, got %d", first)
	}

	second, err := store.scrubJobErrors(ctx)
	if err != nil {
		t.Fatalf("second scrubJobErrors failed: %v", err)
	}
	if second != 0 {
		t.Errorf("expected the scrub to be idempotent, rewrote %d rows on the second pass", second)
	}
}

func TestSQLStore_ScrubJobErrorsIgnoresNullAndEmpty(t *testing.T) {
	store := newTestStore(t, "null-errors.db")
	ctx := context.Background()

	if _, err := store.DB().ExecContext(ctx,
		`INSERT INTO jobs (id, type, target_name, status, created_at, error, manual) VALUES
			('ok-1', 'backup', 't', 'completed', CURRENT_TIMESTAMP, NULL, 1),
			('ok-2', 'backup', 't', 'completed', CURRENT_TIMESTAMP, '', 1)`); err != nil {
		t.Fatalf("failed to seed job rows: %v", err)
	}

	scrubbed, err := store.scrubJobErrors(ctx)
	if err != nil {
		t.Fatalf("scrubJobErrors failed: %v", err)
	}
	if scrubbed != 0 {
		t.Errorf("expected no rewrites, got %d", scrubbed)
	}

	var nullable sql.NullString
	if err := store.DB().QueryRowContext(ctx, `SELECT error FROM jobs WHERE id = 'ok-1'`).Scan(&nullable); err != nil {
		t.Fatalf("failed to read jobs.error: %v", err)
	}
	if nullable.Valid {
		t.Errorf("NULL error was rewritten to %q", nullable.String)
	}
}
