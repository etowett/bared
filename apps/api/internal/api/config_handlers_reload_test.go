package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/etowett/bared/apps/api/internal/configservice"
	"github.com/etowett/bared/apps/api/internal/encryption"
	"github.com/etowett/bared/apps/api/internal/testutil/configdb"
)

// Regression for #153: only PATCH /targets/{name}/schedule signalled the daemon,
// so a target created, updated, imported, or deleted through the API never
// reached the running scheduler. In production this meant 18 imported targets
// with valid cron expressions sat in the database for days without a single
// backup running — the daemon had started against an empty database, found no
// schedules, and never started its cron at all.
//
// The reload channel is buffered with capacity 1 and triggerReload is
// non-blocking, so "did the handler signal?" is exactly "is the channel
// non-empty afterwards?".

func newReloadTestServer(t *testing.T) (*Server, chan struct{}) {
	t.Helper()

	enc, err := encryption.NewService(configdb.TestKey)
	require.NoError(t, err)

	reloadChan := make(chan struct{}, 1)
	return &Server{
		configService: configservice.NewService(configdb.New(t), enc),
		reloadChan:    reloadChan,
	}, reloadChan
}

// reloadSignalled drains the channel, reporting whether a reload was pending.
func reloadSignalled(reloadChan chan struct{}) bool {
	select {
	case <-reloadChan:
		return true
	default:
		return false
	}
}

func targetRequestBody(t *testing.T, name, schedule string) *bytes.Reader {
	t.Helper()

	body, err := json.Marshal(TargetRequest{
		Name:     name,
		Schedule: schedule,
		Connection: ConnectionRequest{
			Type:     "postgres",
			Host:     "db.internal",
			Port:     5432,
			User:     "backup",
			Password: "hunter2",
			Database: name,
		},
	})
	require.NoError(t, err)

	return bytes.NewReader(body)
}

// withURLParam attaches a chi route parameter, which the handlers read via
// chi.URLParam rather than from the path.
func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestHandleCreateTarget_TriggersReload(t *testing.T) {
	s, reloadChan := newReloadTestServer(t)

	w := httptest.NewRecorder()
	s.handleCreateTarget(w, httptest.NewRequest(http.MethodPost, "/api/config/targets",
		targetRequestBody(t, "stage_billing", "29 21 * * *")))

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.True(t, reloadSignalled(reloadChan),
		"a created target's schedule never reaches the scheduler without a reload")
}

// A rejected create must not signal: nothing changed, and a reload stops and
// rebuilds the daemon's cron.
func TestHandleCreateTarget_NoReloadWhenValidationFails(t *testing.T) {
	s, reloadChan := newReloadTestServer(t)

	body, err := json.Marshal(TargetRequest{Name: "", Schedule: "@daily"})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	s.handleCreateTarget(w, httptest.NewRequest(http.MethodPost, "/api/config/targets", bytes.NewReader(body)))

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.False(t, reloadSignalled(reloadChan))
}

func TestHandleUpdateTarget_TriggersReload(t *testing.T) {
	s, reloadChan := newReloadTestServer(t)

	w := httptest.NewRecorder()
	s.handleCreateTarget(w, httptest.NewRequest(http.MethodPost, "/api/config/targets",
		targetRequestBody(t, "stage_billing", "29 21 * * *")))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.True(t, reloadSignalled(reloadChan))

	// PUT can move the cron expression just as PATCH /schedule can.
	w = httptest.NewRecorder()
	req := withURLParam(httptest.NewRequest(http.MethodPut, "/api/config/targets/stage_billing",
		targetRequestBody(t, "stage_billing", "5 3 * * *")), "name", "stage_billing")
	s.handleUpdateTarget(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, reloadSignalled(reloadChan))
}

func TestHandleDeleteTarget_TriggersReload(t *testing.T) {
	s, reloadChan := newReloadTestServer(t)

	w := httptest.NewRecorder()
	s.handleCreateTarget(w, httptest.NewRequest(http.MethodPost, "/api/config/targets",
		targetRequestBody(t, "stage_billing", "29 21 * * *")))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.True(t, reloadSignalled(reloadChan))

	// Without a reload the cron entry outlives the target and keeps firing.
	w = httptest.NewRecorder()
	req := withURLParam(httptest.NewRequest(http.MethodDelete, "/api/config/targets/stage_billing", nil),
		"name", "stage_billing")
	s.handleDeleteTarget(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, reloadSignalled(reloadChan))
}

func TestHandleDeleteTarget_NoReloadWhenTargetMissing(t *testing.T) {
	s, reloadChan := newReloadTestServer(t)

	w := httptest.NewRecorder()
	req := withURLParam(httptest.NewRequest(http.MethodDelete, "/api/config/targets/absent", nil),
		"name", "absent")
	s.handleDeleteTarget(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.False(t, reloadSignalled(reloadChan))
}

const importYAML = `
targets:
  - name: stage_billing
    schedule: "29 21 * * *"
    conn:
      type: postgres
      host: db.internal
      port: 5432
      user: backup
      password: hunter2
      database: stage_billing
`

func importRequest(t *testing.T, dryRun bool, yaml string) *http.Request {
	t.Helper()

	body, err := json.Marshal(ConfigImportRequest{
		YAMLContent:  yaml,
		ConflictMode: "override",
		DryRun:       dryRun,
	})
	require.NoError(t, err)

	return httptest.NewRequest(http.MethodPost, "/api/config/import", bytes.NewReader(body))
}

// The path that actually bit us: `brd config import` is how a fleet of targets
// first reaches a daemon.
func TestHandleImportConfig_TriggersReload(t *testing.T) {
	s, reloadChan := newReloadTestServer(t)

	w := httptest.NewRecorder()
	s.handleImportConfig(w, importRequest(t, false, importYAML))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp ConfigImportResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, []string{"stage_billing"}, resp.Targets.Created)
	require.True(t, reloadSignalled(reloadChan),
		"imported targets sit in the database unscheduled until the daemon reloads")
}

func TestHandleImportConfig_NoReloadOnDryRun(t *testing.T) {
	s, reloadChan := newReloadTestServer(t)

	w := httptest.NewRecorder()
	s.handleImportConfig(w, importRequest(t, true, importYAML))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.False(t, reloadSignalled(reloadChan), "a dry run changes nothing to reload")
}

// Reloading rebuilds the scheduler and nothing else — jobs.Manager resolves
// storages from the database when a job runs — so a storage-only import has no
// scheduler state to refresh.
func TestHandleImportConfig_NoReloadWhenNoTargetsChanged(t *testing.T) {
	s, reloadChan := newReloadTestServer(t)

	w := httptest.NewRecorder()
	s.handleImportConfig(w, importRequest(t, false, `
storages:
  archive:
    type: local
    path: /srv/backups
    keep: 7
`))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp ConfigImportResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, []string{"archive"}, resp.Storages.Created)
	require.Empty(t, resp.Targets.Created)
	require.False(t, reloadSignalled(reloadChan))
}

// triggerReload must never block the request, even when a reload is already
// queued and nothing is draining the channel.
func TestTriggerReload_DoesNotBlockWhenReloadPending(t *testing.T) {
	s, reloadChan := newReloadTestServer(t)

	for i := range 3 {
		w := httptest.NewRecorder()
		s.handleCreateTarget(w, httptest.NewRequest(http.MethodPost, "/api/config/targets",
			targetRequestBody(t, fmt.Sprintf("target_%d", i), "29 21 * * *")))
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	}

	require.Len(t, reloadChan, 1, "the pending signal coalesces rather than queueing or blocking")
}
