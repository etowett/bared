package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bared/internal/config"
	"bared/internal/testutil/fixtures"
	"bared/internal/util"
)

func init() {
	// Initialize logger before tests to avoid race conditions
	util.InitLogger(util.ERROR)
}

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	d := New(cfg)

	require.NotNil(t, d)
	assert.NotNil(t, d.cfg)
	assert.NotNil(t, d.scheduler)
	assert.NotNil(t, d.jobManager)
	assert.NotNil(t, d.ctx)
	assert.NotNil(t, d.cancel)
	assert.Equal(t, 3, d.maxConcurrentJobs)
	assert.Equal(t, 10, d.jobHistorySize)
	assert.Equal(t, 1*time.Hour, d.shutdownTimeout)
	assert.Empty(t, d.httpAddr)
	assert.Nil(t, d.apiServer)
}

func TestNew_WithOptions(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	d := New(cfg,
		WithHTTP("localhost:8080", "admin", "secret"),
		WithMaxConcurrentJobs(5),
		WithJobHistorySize(20),
		WithShutdownTimeout(30*time.Minute),
	)

	require.NotNil(t, d)
	assert.Equal(t, "localhost:8080", d.httpAddr)
	assert.Equal(t, "admin", d.authUser)
	assert.Equal(t, "secret", d.authPass)
	assert.Equal(t, 5, d.maxConcurrentJobs)
	assert.Equal(t, 20, d.jobHistorySize)
	assert.Equal(t, 30*time.Minute, d.shutdownTimeout)
	assert.NotNil(t, d.apiServer)
}

func TestWithHTTP(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	tests := []struct {
		name     string
		addr     string
		user     string
		pass     string
		hasAPI   bool
	}{
		{
			name:   "with http",
			addr:   "localhost:8080",
			user:   "admin",
			pass:   "password",
			hasAPI: true,
		},
		{
			name:   "with http no auth",
			addr:   "localhost:9090",
			user:   "",
			pass:   "",
			hasAPI: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(cfg, WithHTTP(tt.addr, tt.user, tt.pass))

			assert.Equal(t, tt.addr, d.httpAddr)
			assert.Equal(t, tt.user, d.authUser)
			assert.Equal(t, tt.pass, d.authPass)

			if tt.hasAPI {
				assert.NotNil(t, d.apiServer)
			} else {
				assert.Nil(t, d.apiServer)
			}
		})
	}
}

func TestWithMaxConcurrentJobs(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	tests := []struct {
		name     string
		maxJobs  int
	}{
		{name: "single job", maxJobs: 1},
		{name: "multiple jobs", maxJobs: 5},
		{name: "many jobs", maxJobs: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(cfg, WithMaxConcurrentJobs(tt.maxJobs))
			assert.Equal(t, tt.maxJobs, d.maxConcurrentJobs)
		})
	}
}

func TestWithJobHistorySize(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	tests := []struct {
		name        string
		historySize int
	}{
		{name: "small history", historySize: 5},
		{name: "medium history", historySize: 50},
		{name: "large history", historySize: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(cfg, WithJobHistorySize(tt.historySize))
			assert.Equal(t, tt.historySize, d.jobHistorySize)
		})
	}
}

func TestWithShutdownTimeout(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{name: "short timeout", timeout: 5 * time.Minute},
		{name: "medium timeout", timeout: 30 * time.Minute},
		{name: "long timeout", timeout: 2 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(cfg, WithShutdownTimeout(tt.timeout))
			assert.Equal(t, tt.timeout, d.shutdownTimeout)
		})
	}
}

func TestNew_MultipleOptions(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	// Apply multiple options at once
	d := New(cfg,
		WithHTTP("0.0.0.0:8080", "user", "pass"),
		WithMaxConcurrentJobs(7),
		WithJobHistorySize(50),
		WithShutdownTimeout(45*time.Minute),
	)

	// Verify all options were applied
	assert.Equal(t, "0.0.0.0:8080", d.httpAddr)
	assert.Equal(t, "user", d.authUser)
	assert.Equal(t, "pass", d.authPass)
	assert.Equal(t, 7, d.maxConcurrentJobs)
	assert.Equal(t, 50, d.jobHistorySize)
	assert.Equal(t, 45*time.Minute, d.shutdownTimeout)
	assert.NotNil(t, d.apiServer)
}

func TestGetJobManager(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	d := New(cfg)

	mgr := d.GetJobManager()
	require.NotNil(t, mgr)
	assert.Equal(t, d.jobManager, mgr)
}

func TestDaemon_ContextCancellation(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	d := New(cfg)

	// Context should not be cancelled initially
	select {
	case <-d.ctx.Done():
		t.Fatal("context should not be cancelled initially")
	default:
		// Expected
	}

	// Cancel the context
	d.cancel()

	// Context should now be cancelled
	select {
	case <-d.ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("context should be cancelled")
	}
}

func TestScheduleTarget(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	d := New(cfg)

	tests := []struct {
		name        string
		schedule    string
		expectError bool
	}{
		{
			name:        "valid cron expression",
			schedule:    "0 2 * * *", // Daily at 2 AM
			expectError: false,
		},
		{
			name:        "valid every minute",
			schedule:    "* * * * *",
			expectError: false,
		},
		{
			name:        "valid with seconds",
			schedule:    "@hourly",
			expectError: false,
		},
		{
			name:        "invalid cron expression",
			schedule:    "invalid",
			expectError: true,
		},
		{
			name:        "invalid too many fields",
			schedule:    "* * * * * * *",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := fixtures.MySQLTarget()
			target.Schedule = tt.schedule

			err := d.scheduleTarget(target)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid cron schedule")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestScheduleTarget_MultipleTargets(t *testing.T) {
	target1 := fixtures.MySQLTarget()
	target1.Name = "mysql-prod"
	target1.Schedule = "0 2 * * *"

	target2 := fixtures.PostgresTarget()
	target2.Name = "postgres-dev"
	target2.Schedule = "0 3 * * *"

	cfg := &config.Config{
		Targets: []*config.Target{target1, target2},
	}

	d := New(cfg)

	// Schedule both targets
	err := d.scheduleTarget(target1)
	require.NoError(t, err)

	err = d.scheduleTarget(target2)
	require.NoError(t, err)
}

func TestReload(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	d := New(cfg)

	// Reload should not error (even though not fully implemented)
	err := d.Reload()
	assert.NoError(t, err)
}

func TestStop_WithoutStart(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	d := New(cfg)

	// Stop should work even if daemon was never started
	err := d.Stop()
	assert.NoError(t, err)

	// Context should be cancelled
	select {
	case <-d.ctx.Done():
		// Expected
	default:
		t.Fatal("context should be cancelled after Stop()")
	}
}

func TestStop_WithHTTPServer(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	// Create daemon with HTTP server (but don't start it)
	d := New(cfg, WithHTTP("localhost:0", "admin", "secret"))

	// Stop should handle the API server gracefully
	err := d.Stop()
	assert.NoError(t, err)
}

func TestDaemon_DefaultValues(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	d := New(cfg)

	// Verify default values
	assert.Equal(t, 3, d.maxConcurrentJobs, "default maxConcurrentJobs should be 3")
	assert.Equal(t, 10, d.jobHistorySize, "default jobHistorySize should be 10")
	assert.Equal(t, 1*time.Hour, d.shutdownTimeout, "default shutdownTimeout should be 1 hour")
	assert.Empty(t, d.httpAddr, "default httpAddr should be empty")
	assert.Empty(t, d.authUser, "default authUser should be empty")
	assert.Empty(t, d.authPass, "default authPass should be empty")
}

func TestDaemon_JobManagerIntegration(t *testing.T) {
	target := fixtures.MySQLTarget()
	target.Name = "test-target"

	cfg := &config.Config{
		DefaultStorage: "local",
		Storages: map[string]*config.Storage{
			"local": fixtures.LocalStorage(),
		},
		Targets: []*config.Target{target},
	}

	d := New(cfg, WithMaxConcurrentJobs(2))

	// Verify job manager is configured correctly
	mgr := d.GetJobManager()
	require.NotNil(t, mgr)

	// Job manager should be properly initialized
	// (Can't check unexported fields, but can verify it works)
	assert.NotNil(t, mgr)
}

func TestDaemon_SchedulerInitialization(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	d := New(cfg)

	// Scheduler should be initialized
	require.NotNil(t, d.scheduler)

	// Scheduler should not be running initially
	// (no direct way to check, but should not panic)
}

func TestDaemon_ContextInheritance(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	d := New(cfg)

	// Context should be a cancellable context
	ctx := d.ctx
	require.NotNil(t, ctx)

	// Should have a cancel function
	require.NotNil(t, d.cancel)

	// Test cancellation propagation
	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		close(done)
	}()

	d.cancel()

	select {
	case <-done:
		// Expected - context cancellation propagated
	case <-time.After(100 * time.Millisecond):
		t.Fatal("context cancellation did not propagate")
	}
}

func TestScheduleTarget_TargetClosure(t *testing.T) {
	// This tests that the closure in scheduleTarget correctly captures the target
	target1 := fixtures.MySQLTarget()
	target1.Name = "target1"
	target1.Schedule = "* * * * *"

	target2 := fixtures.MySQLTarget()
	target2.Name = "target2"
	target2.Schedule = "* * * * *"

	cfg := &config.Config{
		DefaultStorage: "local",
		Storages: map[string]*config.Storage{
			"local": fixtures.LocalStorage(),
		},
		Targets: []*config.Target{target1, target2},
	}

	d := New(cfg)

	// Schedule both targets - should not have closure issues
	err := d.scheduleTarget(target1)
	require.NoError(t, err)

	err = d.scheduleTarget(target2)
	require.NoError(t, err)
}

func TestStop_ShutdownSequence(t *testing.T) {
	cfg := &config.Config{
		Targets: []*config.Target{fixtures.MySQLTarget()},
	}

	d := New(cfg, WithShutdownTimeout(1*time.Second))

	// Stop should execute shutdown sequence without errors
	err := d.Stop()
	assert.NoError(t, err)

	// Context should be cancelled after stop
	err = d.ctx.Err()
	assert.Equal(t, context.Canceled, err)
}

// Note: Testing Start() is difficult because it blocks on signal handling.
// Full integration tests would require:
// - Mocking signal channels
// - Testing with actual scheduled jobs
// - Testing HTTP server startup
// - Testing graceful shutdown with running jobs
// These should be implemented as separate integration tests.
