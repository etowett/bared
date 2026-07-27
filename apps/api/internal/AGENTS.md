# Backend (Go) — Agent Guide

> Scope: the Go daemon — all packages under `apps/api/internal/` (config, daemon, jobs, api, web, persistence, database, storage, notify, compress, encryption, retention, progress, …). Part of the BareD AGENTS.md tree — see the root [`AGENTS.md`](../../../AGENTS.md) for project-wide workflow. **The innermost guide wins** when instructions conflict.

## Architecture

### Streaming Pipeline Architecture

**Critical Concept**: BareD uses streaming with `io.Pipe` to avoid temporary files. Data flows through stages without hitting disk between operations.

```
Backup Flow:
┌──────────┐    ┌──────────┐    ┌─────────┐    ┌─────────┐
│ Database │───▶│ Compress │───▶│ Storage │───▶│ Tracker │
│  Dump    │    │  (tgz)   │    │ Upload  │    │ Update  │
└──────────┘    └──────────┘    └─────────┘    └─────────┘
     │               │                │              │
     └───────────────┴────────────────┴──────────────┘
              io.Pipe - NO temp files

Restore Flow:
┌─────────┐    ┌────────────┐    ┌──────────┐
│ Storage │───▶│ Decompress │───▶│ Database │
│Retrieve │    │    (tgz)   │    │ Restore  │
└─────────┘    └────────────┘    └──────────┘
     │              │                  │
     └──────────────┴──────────────────┘
              io.Pipe
```

**Why this matters**: Never introduce code that buffers entire dumps in memory or creates unnecessary temporary files. Always use `io.Reader`/`io.Writer` interfaces and `io.Pipe` for stage connections.

### Interface-Driven Design

All major components are abstracted behind interfaces:

```go
// Database abstraction
type Dumper interface {
    Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error)
    Name() string
    Validate(ctx context.Context) error
}

type Restorer interface {
    Restore(ctx context.Context, r io.Reader) error
    Name() string
    ValidateConnection(ctx context.Context) error
}

// Storage abstraction
type Storage interface {
    Store(ctx context.Context, path string, r io.Reader, size int64) error
    Retrieve(ctx context.Context, path string, w io.Writer) error
    List(ctx context.Context) ([]*BackupInfo, error)
    Delete(ctx context.Context, path string) error
    // ... more methods
}

// Compression abstraction
type Compressor interface {
    Compress(ctx context.Context, r io.Reader, w io.Writer) error
    Decompress(ctx context.Context, r io.Reader, w io.Writer) error
    Extension() string
}
```

**Pattern**: New database types, storage backends, or compression formats implement these interfaces and register in factory functions.

> **Extending a subsystem?** The three pluggable subsystems have their own dedicated guides — read them instead of duplicating their setup here:
> - Adding a new database engine → [`apps/api/internal/database/AGENTS.md`](database/AGENTS.md)
> - Adding a new storage backend → [`apps/api/internal/storage/AGENTS.md`](storage/AGENTS.md)
> - Adding a notification channel → [`apps/api/internal/notify/AGENTS.md`](notify/AGENTS.md)

### Job Queue Architecture

```
┌────────────┐
│ Job Submit │ (via API or CLI)
└─────┬──────┘
      │
      ▼
┌──────────────┐
│ Job Manager  │ ← Manages lifecycle, persistence
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Worker Pool  │ ← Concurrent execution (configurable workers)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ Job Executor │ ← Runs backup/restore, reports progress
└──────┬───────┘
       │
       ▼
┌──────────────┐
│ WebSocket    │ ← Real-time log streaming to frontend
└──────────────┘
```

**Key Files**:

- `apps/api/internal/jobs/manager.go` - Job lifecycle, queue, persistence
- `apps/api/internal/jobs/worker.go` - Worker pool execution
- `apps/api/internal/jobs/job.go` - Job structure and state

## Package Layout

```
apps/api/internal/
├── api/           # HTTP server, WebSocket, REST endpoints, middleware
├── app/           # Orchestration layer (backup, restore, list operations)
├── client/        # Config import client (import_client.go, import_types.go)
├── compress/      # Compression implementations
│   ├── compress.go    # Interface
│   ├── gzip.go        # gzip implementation
│   ├── tgz.go         # tar.gz implementation
│   └── factory.go     # Type dispatch
├── config/        # YAML parsing, validation, env var expansion
│   ├── config.go      # Structs
│   ├── parser.go      # YAML parsing + env expansion
│   └── validator.go   # Validation rules
├── configservice/ # Config import/load/secrets service (importer, loader, secrets, validator)
├── daemon/        # Cron scheduling, signal handling
├── database/      # Database dump/restore implementations  (see database/AGENTS.md)
│   ├── database.go    # Interfaces
│   ├── mysql.go       # MySQL implementation
│   ├── postgres.go    # PostgreSQL implementation
│   ├── redis.go       # Redis implementation
│   └── factory.go     # Type dispatch
├── encryption/    # Backup encryption (encryption.go)
├── jobs/          # Job queue, worker pool, persistence (job, logs, manager, store)
├── notify/        # Notification system (Slack, Email, Webhook)  (see notify/AGENTS.md)
│   ├── notifier.go    # Interface
│   ├── slack.go       # Slack channel
│   ├── email.go       # Email channel
│   ├── webhook.go     # Webhook channel
│   └── factory.go     # Type dispatch
├── persistence/   # Job history (SQLite, PostgreSQL, MySQL) — sql.go, store.go
├── progress/      # Progress tracking and estimation (estimator.go)
├── retention/     # Backup retention policies and cleanup (tracker.go)
├── storage/       # Storage backend implementations  (see storage/AGENTS.md)
│   ├── storage.go     # Interface
│   ├── local.go       # Local filesystem
│   ├── s3.go          # S3/S3-compatible
│   ├── sftp.go        # SFTP
│   └── factory.go     # Type dispatch
├── testutil/      # Test helpers (fixtures, integration, mocks)
├── util/          # Utilities (logging, retry, shell, paths, heartbeat, stage, tempfile)
├── version/       # Build-time version metadata (version.go)
└── apps/web/           # Embedded frontend assets via go:embed (embed.go, dist/)
```

## HTTP API

### HTTP API Development

**Pattern**: The API server in `apps/api/internal/api/server.go` follows a simple handler pattern.

**Adding a new endpoint**:

1. **Add handler method to Server**:

```go
// apps/api/internal/api/server.go

func (s *Server) handleGetStorages(w http.ResponseWriter, r *http.Request) {
    storages := make([]map[string]interface{}, 0, len(s.cfg.Storages))
    for name, storage := range s.cfg.Storages {
        storages = append(storages, map[string]interface{}{
            "name": name,
            "type": storage.Type,
            "keep": storage.Keep,
        })
    }

    s.respondJSON(w, http.StatusOK, map[string]interface{}{
        "storages": storages,
    })
}
```

2. **Register route in setupRoutes**:

```go
func (s *Server) setupRoutes() {
    // ... existing routes
    s.mux.HandleFunc("/api/storages", s.handleGetStorages)
}
```

3. **Add authentication if needed**:

```go
func (s *Server) setupRoutes() {
    // Authenticated route
    s.mux.HandleFunc("/api/storages", s.withAuth(s.handleGetStorages))
}
```

## WebSocket

### WebSocket Communication

**Pattern**: WebSocket in `apps/api/internal/api/websocket.go` broadcasts job logs in real-time.

**Key concepts**:

- Clients connect to `/api/ws`
- Server broadcasts `LogMessage` events to all connected clients
- Frontend subscribes to job-specific logs by filtering `job_id`

**Adding new WebSocket message types**:

```go
// apps/api/internal/api/websocket.go

type MessageType string

const (
    MessageTypeLog    MessageType = "log"
    MessageTypeStatus MessageType = "status"  // NEW
)

type Message struct {
    Type    MessageType     `json:"type"`
    Payload json.RawMessage `json:"payload"`
}

type StatusUpdate struct {
    JobID   string `json:"job_id"`
    Status  string `json:"status"`
    Progress int   `json:"progress"`
}

// Broadcast status update
func (h *Hub) BroadcastStatus(jobID, status string, progress int) {
    payload, _ := json.Marshal(StatusUpdate{
        JobID:    jobID,
        Status:   status,
        Progress: progress,
    })

    msg := Message{
        Type:    MessageTypeStatus,
        Payload: payload,
    }

    h.broadcast <- msg
}
```

## Backend Conventions

**Error handling**:

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to connect to database: %w", err)
}

// Check specific errors
if errors.Is(err, os.ErrNotExist) {
    // Handle file not found
}

// Type assertion for custom errors
var storageErr *storage.NotFoundError
if errors.As(err, &storageErr) {
    // Handle storage-specific error
}
```

**Context usage**:

```go
func (b *BackupOrchestrator) Backup(ctx context.Context, targetName string) error {
    // Always check context cancellation in long operations
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // Pass context to all downstream calls
    dumper, err := database.New(target)
    if err != nil {
        return err
    }

    return dumper.Dump(ctx, writer)
}
```

**Logging**:

```go
import "log/slog"

// Structured logging
slog.Info("backup started",
    "target", targetName,
    "storage", storageName,
)

slog.Error("backup failed",
    "target", targetName,
    "error", err,
)

// Set log level from config
var level slog.Level
switch cfg.LogLevel {
case "debug":
    level = slog.LevelDebug
case "info":
    level = slog.LevelInfo
case "warn":
    level = slog.LevelWarn
case "error":
    level = slog.LevelError
}
```

**Resource cleanup**:

```go
func (s *S3Storage) Store(ctx context.Context, path string, r io.Reader, size int64) error {
    // Use defer for cleanup
    pr, pw := io.Pipe()
    defer pr.Close()

    // Ensure cleanup even on error
    var uploadErr error
    go func() {
        defer pw.Close()
        _, uploadErr = io.Copy(pw, r)
    }()

    // ... upload logic
}
```

### Build System

**Makefile targets** (most commonly used):

```makefile
make build              # Build backend (bin/brd)
make build-with-web     # Build frontend, then backend with embedded assets
make test               # Unit tests only
make test-integration   # Integration tests (requires Docker)
make test-unit          # Explicitly unit tests
make coverage           # Generate coverage report
make lint               # Run golangci-lint
make fmt                # Format Go code
make pre-commit         # Full backend gate: fmt + vet + lint + unit tests + coverage
make validate           # Build, then validate examples/config.example.yml
make web-build          # Build frontend only
make web-validate       # Frontend validation (type-check + lint + format + tests)
```

**Version injection** (happens automatically in Makefile):

```go
// apps/api/internal/version/version.go
var (
    Version   = "dev"         // Injected at link time
    Commit    = "unknown"     // Injected at link time
    BuildDate = "unknown"     // Injected at link time
)
```

For CLI/binary build detail (the `brd` command, entrypoints, flags), see [`../cmd/AGENTS.md`](../cmd/AGENTS.md).

## Testing (Backend)

**Unit test structure**:

```go
package storage

import (
    "bytes"
    "context"
    "testing"
    "github.com/etowett/bared/apps/api/internal/config"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLocalStorage_Store(t *testing.T) {
    // Arrange
    tmpDir := t.TempDir()
    cfg := &config.Storage{
        Type: "local",
        Path: tmpDir,
    }
    storage := NewLocalStorage(cfg)

    data := []byte("test backup data")
    reader := bytes.NewReader(data)

    // Act
    err := storage.Store(context.Background(), "backup.tar.gz", reader, int64(len(data)))

    // Assert
    require.NoError(t, err)

    // Verify file was created
    var buf bytes.Buffer
    err = storage.Retrieve(context.Background(), "backup.tar.gz", &buf)
    require.NoError(t, err)
    assert.Equal(t, data, buf.Bytes())
}

func TestLocalStorage_Store_InvalidPath(t *testing.T) {
    cfg := &config.Storage{
        Type: "local",
        Path: "/nonexistent/path",
    }
    storage := NewLocalStorage(cfg)

    err := storage.Store(context.Background(), "backup.tar.gz", bytes.NewReader([]byte("data")), 4)

    assert.Error(t, err)
}
```

**Table-driven tests**:

```go
func TestValidateConnection(t *testing.T) {
    tests := []struct {
        name    string
        conn    *config.Connection
        wantErr bool
    }{
        {
            name: "valid mysql connection",
            conn: &config.Connection{
                Type:     "mysql",
                Host:     "localhost",
                Port:     3306,
                User:     "root",
                Password: "pass",
                Database: "testdb",
            },
            wantErr: false,
        },
        {
            name: "missing host",
            conn: &config.Connection{
                Type:     "mysql",
                Host:     "",
                Port:     3306,
                Database: "testdb",
            },
            wantErr: true,
        },
        {
            name: "invalid port",
            conn: &config.Connection{
                Type:     "mysql",
                Host:     "localhost",
                Port:     -1,
                Database: "testdb",
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateConnection(tt.conn)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

**Mock interfaces** (for testing with mocks):

```go
// apps/api/internal/storage/mock.go
type MockStorage struct {
    StoreFunc    func(ctx context.Context, path string, r io.Reader, size int64) error
    RetrieveFunc func(ctx context.Context, path string, w io.Writer) error
    ListFunc     func(ctx context.Context) ([]*BackupInfo, error)
}

func (m *MockStorage) Store(ctx context.Context, path string, r io.Reader, size int64) error {
    if m.StoreFunc != nil {
        return m.StoreFunc(ctx, path, r, size)
    }
    return nil
}

// ... implement other methods

// Usage in tests
func TestBackupWithMockStorage(t *testing.T) {
    mockStorage := &MockStorage{
        StoreFunc: func(ctx context.Context, path string, r io.Reader, size int64) error {
            // Verify expected arguments
            assert.Contains(t, path, "test_target")
            return nil
        },
    }

    // Use mockStorage in test...
}
```

## Full-Stack Recipes

> The React side of these recipes is detailed in [`../web/AGENTS.md`](../../web/AGENTS.md). The steps below cover the Go backend work; frontend hooks/components are shown for context where the source recipe spans both sides.

### Task 1: Add a New Configuration Field

**Scenario**: Add a `timeout` field to backup targets.

1. **Update config struct** (`apps/api/internal/config/config.go`):

```go
type Target struct {
    Name     string       `yaml:"name"`
    Conn     *Connection  `yaml:"conn"`
    Compress *Compress    `yaml:"compress,omitempty"`
    Storage  *Storage     `yaml:"storage,omitempty"`
    Schedule string       `yaml:"schedule,omitempty"`
    Timeout  int          `yaml:"timeout,omitempty"`  // NEW: timeout in seconds
}
```

2. **Add validation** (`apps/api/internal/config/validator.go`):

```go
func (v *Validator) validateTarget(target *Target, idx int) error {
    // ... existing validation

    // Validate timeout
    if target.Timeout != 0 && target.Timeout < 0 {
        return fmt.Errorf("targets[%d].timeout: must be positive", idx)
    }

    return nil
}
```

3. **Use in backup logic** (`apps/api/internal/app/backup.go`):

```go
func (b *BackupOrchestrator) Backup(ctx context.Context, targetName string) error {
    // ... find target

    // Apply timeout if specified
    if target.Timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, time.Duration(target.Timeout)*time.Second)
        defer cancel()
    }

    // ... rest of backup logic
}
```

4. **Update example config** (`examples/config.example.yml`):

```yaml
targets:
  - name: my_mysql_db
    timeout: 300  # 5 minutes
    conn:
      type: mysql
      # ...
```

5. **Update documentation**:
   - Add to `docs/user-guide/configuration.md`
   - Mention in README.md if it's a major feature

### Task 2: Add Progress Tracking to an Operation

**Scenario**: Add progress tracking to S3 uploads.

1. **Define progress callback type** (`apps/api/internal/progress/progress.go`):

```go
type Callback func(bytesTransferred, totalBytes int64)
```

2. **Create progress reader wrapper** (`apps/api/internal/progress/reader.go`):

```go
type Reader struct {
    r        io.Reader
    total    int64
    current  int64
    callback Callback
}

func NewReader(r io.Reader, total int64, callback Callback) *Reader {
    return &Reader{
        r:        r,
        total:    total,
        callback: callback,
    }
}

func (pr *Reader) Read(p []byte) (n int, err error) {
    n, err = pr.r.Read(p)
    pr.current += int64(n)

    if pr.callback != nil {
        pr.callback(pr.current, pr.total)
    }

    return
}
```

3. **Use in S3 storage** (`apps/api/internal/storage/s3.go`):

```go
func (s *S3Storage) Store(ctx context.Context, path string, r io.Reader, size int64) error {
    // Wrap reader with progress tracking
    progressReader := progress.NewReader(r, size, func(transferred, total int64) {
        percent := float64(transferred) / float64(total) * 100
        log.Printf("Upload progress: %.2f%% (%d/%d bytes)", percent, transferred, total)
    })

    _, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
        Bucket: aws.String(s.bucket),
        Key:    aws.String(path),
        Body:   progressReader,
    })

    return err
}
```

4. **Expose progress via WebSocket** (if needed):

```go
// apps/api/internal/jobs/job.go
type Job struct {
    // ... existing fields
    Progress     int    `json:"progress"`      // 0-100
    BytesWritten int64  `json:"bytes_written"`
}

// Update progress
func (j *Job) UpdateProgress(transferred, total int64) {
    j.BytesWritten = transferred
    if total > 0 {
        j.Progress = int(float64(transferred) / float64(total) * 100)
    }

    // Broadcast to WebSocket clients
    wsHub.BroadcastStatus(j.ID, j.Status, j.Progress)
}
```

### Task 3: Add a New API Endpoint

**Scenario**: Add endpoint to get backup statistics.

**Backend** (`apps/api/internal/api/server.go`):

```go
type BackupStats struct {
    TotalBackups     int64 `json:"total_backups"`
    TotalSize        int64 `json:"total_size"`
    SuccessfulBackups int64 `json:"successful_backups"`
    FailedBackups    int64 `json:"failed_backups"`
}

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
    // Query persistence layer
    stats, err := s.persistence.GetStats(r.Context())
    if err != nil {
        s.respondError(w, http.StatusInternalServerError, "Failed to get stats")
        return
    }

    s.respondJSON(w, http.StatusOK, stats)
}

// Register route
func (s *Server) setupRoutes() {
    // ... existing routes
    s.mux.HandleFunc("/api/stats", s.withAuth(s.handleGetStats))
}
```

**Frontend Hook** (`apps/web/src/hooks/useStats.ts`):

```typescript
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../api/client';

interface BackupStats {
  total_backups: number;
  total_size: number;
  successful_backups: number;
  failed_backups: number;
}

export function useStats() {
  return useQuery({
    queryKey: ['stats'],
    queryFn: async () => {
      const { data } = await apiClient.get<BackupStats>('/api/stats');
      return data;
    },
    refetchInterval: 30000, // Refetch every 30 seconds
  });
}
```

**Frontend Component** (`apps/web/src/components/StatsCard.tsx`):

```typescript
import { useStats } from '../hooks/useStats';
import { Card } from './ui/card';

export function StatsCard() {
  const { data: stats, isLoading } = useStats();

  if (isLoading) return <div>Loading...</div>;

  const successRate = stats
    ? (stats.successful_backups / stats.total_backups * 100).toFixed(1)
    : 0;

  return (
    <Card className="p-6">
      <h3 className="text-lg font-semibold mb-4">Backup Statistics</h3>
      <div className="grid grid-cols-2 gap-4">
        <div>
          <p className="text-sm text-muted-foreground">Total Backups</p>
          <p className="text-2xl font-bold">{stats?.total_backups}</p>
        </div>
        <div>
          <p className="text-sm text-muted-foreground">Success Rate</p>
          <p className="text-2xl font-bold">{successRate}%</p>
        </div>
        <div>
          <p className="text-sm text-muted-foreground">Total Size</p>
          <p className="text-2xl font-bold">
            {formatBytes(stats?.total_size || 0)}
          </p>
        </div>
        <div>
          <p className="text-sm text-muted-foreground">Failed</p>
          <p className="text-2xl font-bold text-red-500">
            {stats?.failed_backups}
          </p>
        </div>
      </div>
    </Card>
  );
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
}
```

### Task 4: Add Integration Test

**Scenario**: Test MySQL backup and restore flow.

**Create test file** (`apps/api/internal/app/backup_test.go`):

```go
//go:build integration
// +build integration

package app

import (
    "context"
    "testing"
    "github.com/etowett/bared/apps/api/internal/config"
    "github.com/etowett/bared/apps/api/internal/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMySQLBackupRestore(t *testing.T) {
    // Setup test database
    db := testutil.SetupMySQL(t)
    defer testutil.TeardownMySQL(t, db)

    // Insert test data
    _, err := db.Exec(`
        CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(100));
        INSERT INTO users VALUES (1, 'Alice'), (2, 'Bob');
    `)
    require.NoError(t, err)

    // Create config
    cfg := &config.Config{
        DefaultStorage: "test_local",
        Storages: map[string]*config.Storage{
            "test_local": {
                Type: "local",
                Path: t.TempDir(),
                Keep: 5,
            },
        },
        Targets: []*config.Target{
            {
                Name: "test_mysql",
                Conn: &config.Connection{
                    Type:     "mysql",
                    Host:     "localhost",
                    Port:     3306,
                    User:     "root",
                    Password: "testpass",
                    Database: "testdb",
                },
                Compress: &config.Compress{
                    Enabled: true,
                    Type:    "tgz",
                },
                Storage: &config.TargetStorage{
                    Enabled: true,
                    Name:    "test_local",
                },
            },
        },
    }

    // Test backup
    orchestrator := NewBackupOrchestrator(cfg)
    err = orchestrator.Backup(context.Background(), "test_mysql")
    require.NoError(t, err)

    // Verify backup file exists
    lister := NewLister(cfg)
    backups, err := lister.List(context.Background(), "test_mysql")
    require.NoError(t, err)
    assert.Len(t, backups, 1)

    // Drop table
    _, err = db.Exec("DROP TABLE users")
    require.NoError(t, err)

    // Test restore
    restorer := NewRestoreOrchestrator(cfg)
    err = restorer.Restore(context.Background(), "test_mysql", "latest")
    require.NoError(t, err)

    // Verify data restored
    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
    require.NoError(t, err)
    assert.Equal(t, 2, count)
}
```

**Run integration tests**:

```bash
# Start test services
docker-compose up -d

# Run integration tests
make test-integration

# Or manually
go test -tags=integration -v ./...
```

## Backend↔Frontend Integration

### Backend-Frontend Integration

**API Contract**:

Backend types (`apps/api/internal/api/types.go`):

```go
type Job struct {
    ID        string    `json:"id"`
    Target    string    `json:"target"`
    Type      string    `json:"type"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

Frontend types (`apps/web/src/types/index.ts`):

```typescript
interface Job {
  id: string;
  target: string;
  type: string;
  status: string;
  created_at: string;  // ISO 8601 string from JSON
  updated_at: string;
}
```

**Date handling**:

```typescript
// Parse dates from API
const job: Job = await api.jobs.get(id);
const createdDate = new Date(job.created_at);

// Format for display
import { format } from 'date-fns';
const formatted = format(createdDate, 'yyyy-MM-dd HH:mm:ss');
```

### Configuration to UI Mapping

**Backend config** → **Frontend display**:

```yaml
# config.yml
targets:
  - name: mysql_prod
    conn:
      type: mysql
      host: db.example.com
      database: production
    schedule: "0 2 * * *"
```

**Frontend representation**:

```typescript
interface Target {
  name: string;
  conn: {
    type: 'mysql' | 'postgres' | 'redis';
    host: string;
    database?: string;
  };
  schedule?: string;
}

// Display in UI
function TargetCard({ target }: { target: Target }) {
  const scheduleDescription = cronToHuman(target.schedule); // "Daily at 2:00 AM"

  return (
    <Card>
      <h3>{target.name}</h3>
      <p>{target.conn.type} - {target.conn.host}</p>
      {target.schedule && <p>Schedule: {scheduleDescription}</p>}
    </Card>
  );
}
```

## Safety & Security

### Security Considerations

**1. Never log sensitive data**:

```go
// ❌ BAD
slog.Info("connecting to database", "password", conn.Password)

// ✅ GOOD
slog.Info("connecting to database", "host", conn.Host, "database", conn.Database)
```

**1a. Never format a raw argv slice or command output into an error or a log line.**
`internal/util/redact.go` holds the single redaction rule:

- `util.RedactArgs(args)` — masks `--password=…`, `--password …`, redis-cli `-a …`,
  attached `-p…`, `PGPASSWORD=…` and any flag whose name contains
  `pass`/`pwd`/`secret`/`token`/`key`/`auth`, keeping the flag name so failures stay
  debuggable. The token after a secret flag is masked whatever it looks like — a password
  can start with `-`.
- `util.RedactSecrets(text)` — the same rule over free-form text (stderr, an error string).
- `util.RedactErr(err)` — masks an error's message, keeping the chain for `errors.Is/As`.

Every error path in `util/shell.go` already runs both, `Job.MarkFailed` scrubs before the
error is persisted, and `persistence.SQLStore` scrubs on read plus rewrites legacy rows at
startup. Add to the keyword list rather than writing a second redactor — the per-engine
`sanitizeArgs` helpers delegate here. A `jobs.Job.Error` is persisted **and** served by
`/api/jobs`; treat it as public. (Issue #133.)

**2. Validate user input**:

```go
func (s *Server) handleTriggerBackup(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Target string `json:"target"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.respondError(w, http.StatusBadRequest, "Invalid request body")
        return
    }

    // Validate target exists
    if !s.cfg.HasTarget(req.Target) {
        s.respondError(w, http.StatusNotFound, "Target not found")
        return
    }

    // Continue...
}
```

**3. Prevent path traversal**:

```go
func (l *LocalStorage) Store(ctx context.Context, path string, r io.Reader, size int64) error {
    // Clean path to prevent traversal
    cleanPath := filepath.Clean(path)
    if strings.Contains(cleanPath, "..") {
        return fmt.Errorf("invalid path: contains parent directory reference")
    }

    fullPath := filepath.Join(l.basePath, cleanPath)

    // Ensure path is within base directory
    if !strings.HasPrefix(fullPath, l.basePath) {
        return fmt.Errorf("invalid path: outside base directory")
    }

    // Continue...
}
```

**4. Rate limiting** (if implementing public API):

```go
import "golang.org/x/time/rate"

type Server struct {
    // ... existing fields
    limiter *rate.Limiter
}

func (s *Server) withRateLimit(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if !s.limiter.Allow() {
            s.respondError(w, http.StatusTooManyRequests, "Rate limit exceeded")
            return
        }
        next(w, r)
    }
}
```

**5. HTTP API authentication** (`internal/api`):

Two mechanisms, both handled by `authMiddleware` (cookie first, then Basic):

- **Session cookie** — the dashboard. `POST /api/login` issues an opaque token
  from `crypto/rand`, stored in `sessionStore` (in-memory, `session.go`), and
  returns it in an `httpOnly; SameSite=Strict; Path=/` cookie. Nothing is
  derived from the password, so the cookie is not password-recoverable — but it
  *is* a bearer credential until it expires or is revoked. **Never log a session
  token.**
- **HTTP Basic** — CLI and API clients (`internal/client`). Credentials are
  compared with `crypto/subtle.ConstantTimeCompare`; a plain `==` leaks length
  and prefix information through timing, and previously let a server with no
  credentials configured authenticate `Basic ":"`.

Rules when touching this area:

- **`Secure` on cookies is conditional** (`isSecureRequest`) — real TLS or the
  explicit `--http-secure-cookies`. Never infer it from `X-Forwarded-Proto`
  (client-controlled), and never hardcode it: the daemon is normally run on
  plain HTTP over a LAN, where a `Secure` cookie is silently dropped.
- **Cookies mean CSRF.** Unsafe methods authenticated by cookie go through
  `csrfMiddleware`, which requires a same-origin or allowlisted `Origin`. CORS
  headers are *not* access control — `corsMiddleware` only advertises, it never
  rejects.
- **Origin comparisons go through `canonicalOrigin`** (`origin.go`). Never
  string-prefix an origin: `evil-localhost` matches `localhost` under a prefix
  check.
- **Auth is checked once per connection, not per message.** A WebSocket
  authenticated at the handshake would otherwise outlive its session, so
  `handleStreamJobLogs` selects on the session's `Done()` channel and logout /
  TTL expiry closes it. Any future long-lived connection must do the same.

**6. SQL injection prevention** (for future direct DB queries):

```go
// ❌ BAD - Never concatenate SQL
query := fmt.Sprintf("SELECT * FROM backups WHERE name = '%s'", name)

// ✅ GOOD - Use parameterized queries
query := "SELECT * FROM backups WHERE name = ?"
rows, err := db.QueryContext(ctx, query, name)
```

### Error Handling Best Practices

**Don't expose internal errors to users**:

```go
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
    job, err := s.jobManager.Get(jobID)
    if err != nil {
        // Log internal error
        slog.Error("failed to get job", "job_id", jobID, "error", err)

        // Return generic error to user
        s.respondError(w, http.StatusInternalServerError, "Failed to retrieve job")
        return
    }

    s.respondJSON(w, http.StatusOK, job)
}
```

**Frontend error handling**:

```typescript
// Don't show raw error objects to users
try {
  await api.jobs.triggerBackup({ target });
} catch (error) {
  if (axios.isAxiosError(error)) {
    // Show user-friendly message
    const message = error.response?.data?.error || 'An unexpected error occurred';
    toast.error(message);

    // Log full error for debugging
    console.error('Backup trigger failed:', error);
  }
}
```

## See also

- [`../AGENTS.md`](../../../AGENTS.md) — root project guide and project-wide workflow
- [`../cmd/AGENTS.md`](../cmd/AGENTS.md) — CLI / `brd` binary and entrypoints
- [`../web/AGENTS.md`](../../web/AGENTS.md) — frontend (React) guide
- [`database/AGENTS.md`](database/AGENTS.md) — adding a new database engine
- [`storage/AGENTS.md`](storage/AGENTS.md) — adding a new storage backend
- [`notify/AGENTS.md`](notify/AGENTS.md) — adding a notification channel
