# AI Agent Guide for BareD

This document is specifically designed for AI coding assistants (like Claude) working with the BareD codebase. It provides patterns, conventions, and mental models necessary to effectively contribute to both the backend (Go) and frontend (React/TypeScript) components.

## Table of Contents

1. [Project Overview](#project-overview)
2. [Architecture Mental Model](#architecture-mental-model)
3. [Backend Development (Go)](#backend-development-go)
4. [Frontend Development (ReactTypeScript)](#frontend-development-reacttypescript)
5. [Common Development Tasks](#common-development-tasks)
6. [Testing Patterns](#testing-patterns)
7. [Code Patterns and Conventions](#code-patterns-and-conventions)
8. [Integration Points](#integration-points)
9. [Safety and Security](#safety-and-security)

---

## Project Overview

**BareD** is a backup and restore daemon for databases with both CLI and web interfaces.

### Core Purpose

- **Backup**: Stream database dumps → compress → upload to storage → track/cleanup
- **Restore**: Retrieve from storage → decompress → restore to database
- **Schedule**: Run backups on cron schedules via daemon mode
- **Monitor**: Web UI for real-time job monitoring and management

### Technology Stack

**Backend (Go 1.25.4)**:

- Pure Go with minimal dependencies
- Stdlib-first approach
- CLI via Cobra
- HTTP server with WebSocket support
- SQLite for job persistence

**Frontend (React 19 + TypeScript)**:

- Vite build tool
- Zustand for state management
- React Query for server state
- Radix UI components + Tailwind CSS
- WebSocket for real-time updates

### Key Files to Know

```
cmd/brd/main.go                 # CLI entry point
internal/app/                   # High-level backup/restore orchestration
internal/api/server.go          # HTTP API and WebSocket server
internal/daemon/daemon.go       # Cron scheduler and signal handling
internal/jobs/manager.go        # Job queue and worker pool
internal/config/config.go       # Configuration structure
web/src/App.tsx                 # Frontend entry point
web/src/api/client.ts          # API client
web/src/hooks/useWebSocket.ts  # WebSocket integration
```

---

## Architecture Mental Model

### Backend: Streaming Pipeline Architecture

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

- `internal/jobs/manager.go` - Job lifecycle, queue, persistence
- `internal/jobs/worker.go` - Worker pool execution
- `internal/jobs/job.go` - Job structure and state

### Frontend: Event-Driven Architecture

```
┌─────────────┐
│   App.tsx   │ ← Root: Theme + Auth + QueryClient
└──────┬──────┘
       │
       ▼
┌──────────────┐
│ Dashboard    │ ← Overview, stats
└──────────────┘
       │
       ├─▶ JobList ────────┐
       │                   │
       ├─▶ RestoreForm     ├─▶ useJobs (React Query)
       │                   │
       └─▶ JobDetail ──────┴─▶ useWebSocket (real-time)
                           │
                           └─▶ API Client (axios)
```

**State Management**:

- **Server State**: React Query (`@tanstack/react-query`)
- **Global UI State**: Zustand (`useAuthStore`, `useJobStore`)
- **Theme State**: React Context (`ThemeContext`)
- **Real-time Updates**: WebSocket custom hook

---

## Backend Development (Go)

### Project Structure

```
internal/
├── app/           # Orchestration layer (backup, restore, list operations)
├── api/           # HTTP server, WebSocket, REST endpoints
├── config/        # YAML parsing, validation, env var expansion
├── database/      # Database dump/restore implementations
│   ├── database.go    # Interfaces
│   ├── mysql.go       # MySQL implementation
│   ├── postgres.go    # PostgreSQL implementation
│   ├── redis.go       # Redis implementation
│   └── factory.go     # Type dispatch
├── storage/       # Storage backend implementations
│   ├── storage.go     # Interface
│   ├── local.go       # Local filesystem
│   ├── s3.go          # S3/S3-compatible
│   ├── sftp.go        # SFTP
│   └── factory.go     # Type dispatch
├── compress/      # Compression implementations
│   ├── compress.go    # Interface
│   ├── tgz.go         # tar.gz implementation
│   └── factory.go     # Type dispatch
├── daemon/        # Cron scheduling, signal handling
├── jobs/          # Job queue, worker pool, persistence
├── notify/        # Notification system (Slack, Email, Webhook)
├── persistence/   # Job history (SQLite, PostgreSQL, MySQL)
├── retention/     # Backup retention policies and cleanup
├── progress/      # Progress tracking and estimation
├── util/          # Utilities (logging, retry, shell, paths)
└── web/           # Embedded frontend assets (via go:embed)
```

### Adding a New Database Type

**Example**: Adding MongoDB support

1. **Create implementation** (`internal/database/mongodb.go`):

```go
package database

import (
    "context"
    "fmt"
    "io"
    "bared/internal/config"
    "bared/internal/util"
)

type MongoDB struct {
    conn *config.Connection
}

func NewMongoDB(conn *config.Connection) *MongoDB {
    return &MongoDB{conn: conn}
}

// Dump implements Dumper interface
func (m *MongoDB) Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error) {
    // Build mongodump command
    args := []string{
        "--host", m.conn.Host,
        "--port", fmt.Sprintf("%d", m.conn.Port),
        "--archive",  // Stream to stdout
    }

    if m.conn.User != "" {
        args = append(args, "--username", m.conn.User)
    }
    if m.conn.Password != "" {
        args = append(args, "--password", m.conn.Password)
    }
    if m.conn.Database != "" {
        args = append(args, "--db", m.conn.Database)
    }

    // Execute with streaming output
    executor := util.NewShellExecutor("mongodump", args...)
    size, err := executor.Execute(ctx, w)
    if err != nil {
        return nil, fmt.Errorf("mongodump failed: %w", err)
    }

    return &DumpMetadata{
        DatabaseType: "mongodb",
        DatabaseName: m.conn.Database,
        Size:         size,
    }, nil
}

// Restore implements Restorer interface
func (m *MongoDB) Restore(ctx context.Context, r io.Reader) error {
    args := []string{
        "--host", m.conn.Host,
        "--port", fmt.Sprintf("%d", m.conn.Port),
        "--archive",  // Read from stdin
    }

    // Add auth if provided
    if m.conn.User != "" {
        args = append(args, "--username", m.conn.User)
    }
    if m.conn.Password != "" {
        args = append(args, "--password", m.conn.Password)
    }

    executor := util.NewShellExecutor("mongorestore", args...)
    _, err := executor.ExecuteWithInput(ctx, r)
    return err
}

func (m *MongoDB) Name() string {
    return "mongodb"
}

func (m *MongoDB) Validate(ctx context.Context) error {
    // Check if mongodump is available
    if !util.CommandExists("mongodump") {
        return fmt.Errorf("mongodump command not found")
    }
    if !util.CommandExists("mongorestore") {
        return fmt.Errorf("mongorestore command not found")
    }
    return nil
}

func (m *MongoDB) ValidateConnection(ctx context.Context) error {
    // Test connection (could use mongo client library)
    return nil
}
```

2. **Register in factory** (`internal/database/factory.go`):

```go
func New(target *config.Target) (Dumper, error) {
    switch target.Conn.Type {
    case "mysql":
        return NewMySQL(target.Conn), nil
    case "postgres":
        return NewPostgres(target.Conn), nil
    case "redis":
        return NewRedis(target.Conn), nil
    case "mongodb":  // ADD THIS
        return NewMongoDB(target.Conn), nil
    default:
        return nil, fmt.Errorf("unsupported database type: %s", target.Conn.Type)
    }
}

func NewRestorer(target *config.Target) (Restorer, error) {
    switch target.Conn.Type {
    case "mysql":
        return NewMySQL(target.Conn), nil
    case "postgres":
        return NewPostgres(target.Conn), nil
    case "redis":
        return NewRedis(target.Conn), nil
    case "mongodb":  // ADD THIS
        return NewMongoDB(target.Conn), nil
    default:
        return nil, fmt.Errorf("unsupported database type: %s", target.Conn.Type)
    }
}
```

3. **Update validator** (`internal/config/validator.go`):

```go
func (v *Validator) validateConnection(conn *Connection, fieldPath string) error {
    // ... existing validation

    // Validate type
    validTypes := []string{"mysql", "postgres", "redis", "mongodb"}  // ADD mongodb
    if !contains(validTypes, conn.Type) {
        return fmt.Errorf("%s.type: must be one of %v", fieldPath, validTypes)
    }

    // ... rest of validation
}
```

4. **Add tests** (`internal/database/mongodb_test.go`):

```go
package database

import (
    "testing"
    "context"
    "bytes"
    "bared/internal/config"
    "github.com/stretchr/testify/assert"
)

func TestMongoDBDump(t *testing.T) {
    // Skip if mongodump not available
    if !util.CommandExists("mongodump") {
        t.Skip("mongodump not available")
    }

    conn := &config.Connection{
        Type:     "mongodb",
        Host:     "localhost",
        Port:     27017,
        Database: "testdb",
        User:     "testuser",
        Password: "testpass",
    }

    db := NewMongoDB(conn)

    var buf bytes.Buffer
    meta, err := db.Dump(context.Background(), &buf)

    assert.NoError(t, err)
    assert.NotNil(t, meta)
    assert.Equal(t, "mongodb", meta.DatabaseType)
    assert.Greater(t, buf.Len(), 0)
}
```

5. **Update documentation**:
   - Add example to `examples/config.example.yml`
   - Update README.md features list
   - Update `docs/user-guide/configuration.md`

### Adding a New Storage Backend

**Example**: Adding Azure Blob Storage

1. **Create implementation** (`internal/storage/azure.go`):

```go
package storage

import (
    "context"
    "fmt"
    "io"
    "path/filepath"
    "bared/internal/config"
    // Import Azure SDK
)

type AzureStorage struct {
    cfg         *config.Storage
    client      *azblob.Client
    container   string
    retryPolicy *util.RetryPolicy
}

func NewAzureStorage(cfg *config.Storage) (*AzureStorage, error) {
    // Initialize Azure client
    client, err := azblob.NewClient(cfg.AccountURL, credential, nil)
    if err != nil {
        return nil, err
    }

    return &AzureStorage{
        cfg:         cfg,
        client:      client,
        container:   cfg.Container,
        retryPolicy: util.NewRetryPolicy(3, time.Second),
    }, nil
}

func (a *AzureStorage) Store(ctx context.Context, path string, r io.Reader, size int64) error {
    // Use retry policy for network operations
    return a.retryPolicy.Do(ctx, func() error {
        blobClient := a.client.ServiceClient().NewContainerClient(a.container).NewBlockBlobClient(path)
        _, err := blobClient.UploadStream(ctx, r, &azblob.UploadStreamOptions{})
        return err
    })
}

func (a *AzureStorage) Retrieve(ctx context.Context, path string, w io.Writer) error {
    return a.retryPolicy.Do(ctx, func() error {
        blobClient := a.client.ServiceClient().NewContainerClient(a.container).NewBlockBlobClient(path)
        downloadResponse, err := blobClient.DownloadStream(ctx, nil)
        if err != nil {
            return err
        }
        defer downloadResponse.Body.Close()

        _, err = io.Copy(w, downloadResponse.Body)
        return err
    })
}

// Implement remaining Storage interface methods...
```

2. **Update config** (`internal/config/config.go`):

```go
type Storage struct {
    Type     string `yaml:"type"`
    // ... existing fields

    // Azure-specific fields
    AccountURL  string `yaml:"account_url,omitempty"`
    AccountKey  string `yaml:"account_key,omitempty"`
    Container   string `yaml:"container,omitempty"`
}
```

3. **Register in factory** (`internal/storage/factory.go`):

```go
func New(cfg *config.Storage) (Storage, error) {
    switch cfg.Type {
    case "local":
        return NewLocalStorage(cfg), nil
    case "s3":
        return NewS3Storage(cfg)
    case "sftp":
        return NewSFTPStorage(cfg)
    case "azure":  // ADD THIS
        return NewAzureStorage(cfg)
    default:
        return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
    }
}
```

### Adding a Notification Channel

**Example**: Adding Discord webhook support

1. **Create implementation** (`internal/notify/discord.go`):

```go
package notify

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "bared/internal/config"
)

type Discord struct {
    webhookURL string
    onSuccess  bool
    onFailure  bool
}

type discordPayload struct {
    Content string        `json:"content"`
    Embeds  []discordEmbed `json:"embeds,omitempty"`
}

type discordEmbed struct {
    Title       string `json:"title"`
    Description string `json:"description"`
    Color       int    `json:"color"`
}

func NewDiscord(cfg *config.Notifier) *Discord {
    return &Discord{
        webhookURL: cfg.WebhookURL,
        onSuccess:  cfg.OnSuccess,
        onFailure:  cfg.OnFailure,
    }
}

func (d *Discord) Notify(ctx context.Context, event *Event) error {
    // Check if we should notify for this event type
    if (event.Success && !d.onSuccess) || (!event.Success && !d.onFailure) {
        return nil
    }

    color := 3066993 // Green
    if !event.Success {
        color = 15158332 // Red
    }

    payload := discordPayload{
        Embeds: []discordEmbed{
            {
                Title:       fmt.Sprintf("Backup %s: %s", statusString(event.Success), event.Target),
                Description: event.Message,
                Color:       color,
            },
        },
    }

    body, _ := json.Marshal(payload)
    req, err := http.NewRequestWithContext(ctx, "POST", d.webhookURL, bytes.NewReader(body))
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
    }

    return nil
}

func (d *Discord) Name() string {
    return "discord"
}

func statusString(success bool) string {
    if success {
        return "Success"
    }
    return "Failed"
}
```

2. **Update config** (`internal/config/config.go`):

```go
type Notifier struct {
    Type   string `yaml:"type"`
    Name   string `yaml:"name,omitempty"`

    // ... existing fields

    // Discord-specific (reuses WebhookURL)
    WebhookURL string `yaml:"webhook_url,omitempty"`
    OnSuccess  bool   `yaml:"on_success,omitempty"`
    OnFailure  bool   `yaml:"on_failure,omitempty"`
}
```

3. **Register in factory** (`internal/notify/factory.go`):

```go
func New(cfg *config.Notifier) (Notifier, error) {
    switch cfg.Type {
    case "slack":
        return NewSlack(cfg), nil
    case "email":
        return NewEmail(cfg), nil
    case "webhook":
        return NewWebhook(cfg), nil
    case "discord":  // ADD THIS
        return NewDiscord(cfg), nil
    default:
        return nil, fmt.Errorf("unsupported notifier type: %s", cfg.Type)
    }
}
```

### HTTP API Development

**Pattern**: The API server in `internal/api/server.go` follows a simple handler pattern.

**Adding a new endpoint**:

1. **Add handler method to Server**:

```go
// internal/api/server.go

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

### WebSocket Communication

**Pattern**: WebSocket in `internal/api/websocket.go` broadcasts job logs in real-time.

**Key concepts**:

- Clients connect to `/api/ws`
- Server broadcasts `LogMessage` events to all connected clients
- Frontend subscribes to job-specific logs by filtering `job_id`

**Adding new WebSocket message types**:

```go
// internal/api/websocket.go

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
make validate           # Run all checks
make web-build          # Build frontend only
make web-validate       # Frontend validation (type-check + lint + format + tests)
```

**Version injection** (happens automatically in Makefile):

```go
// internal/version/version.go
var (
    Version   = "dev"         // Injected at link time
    Commit    = "unknown"     // Injected at link time
    BuildDate = "unknown"     // Injected at link time
)
```

---

## Frontend Development (React/TypeScript)

### Project Structure

```
web/
├── src/
│   ├── api/                  # API client and types
│   │   └── client.ts         # Axios instance, auth, endpoints
│   ├── components/           # React components
│   │   ├── Dashboard.tsx     # Main dashboard view
│   │   ├── JobList.tsx       # Job listing with filters
│   │   ├── JobDetail.tsx     # Job details with real-time logs
│   │   ├── RestoreForm.tsx   # Backup restore interface
│   │   ├── TargetList.tsx    # Target configuration display
│   │   ├── Login.tsx         # Authentication form
│   │   └── ui/               # Reusable UI components (Radix)
│   ├── hooks/                # Custom React hooks
│   │   ├── useJobs.ts        # Job fetching with React Query
│   │   ├── useWebSocket.ts   # WebSocket connection
│   │   ├── useAuth.ts        # Authentication state
│   │   └── useDashboard.ts   # Dashboard data
│   ├── contexts/             # React contexts
│   │   └── ThemeContext.tsx  # Dark mode management
│   ├── types/                # TypeScript type definitions
│   │   └── index.ts          # Shared types (Job, Target, etc.)
│   ├── lib/                  # Utility libraries
│   │   └── utils.ts          # Helper functions
│   ├── styles/               # Global CSS
│   │   └── index.css         # Tailwind imports + globals
│   ├── App.tsx               # Root component
│   ├── main.tsx              # Entry point
│   └── vite-env.d.ts         # Vite type declarations
├── public/                   # Static assets
├── dist/                     # Build output (copied to internal/web/dist/)
├── package.json              # Dependencies
├── tsconfig.json             # TypeScript configuration
├── vite.config.ts            # Vite configuration
├── tailwind.config.js        # Tailwind CSS configuration
└── postcss.config.js         # PostCSS configuration
```

### State Management Strategy

**Three-tier state architecture**:

1. **Server State** (React Query):
   - Job lists, dashboard data, targets
   - Automatic caching, refetching, invalidation
   - Loading and error states handled automatically

2. **Real-time State** (WebSocket):
   - Job logs, progress updates
   - Direct WebSocket connection, no caching
   - Falls back to polling if WebSocket fails

3. **Client State** (Zustand):
   - Authentication (token, user)
   - UI preferences (theme, filters)
   - Global app state

**Example - Using React Query**:

```typescript
// hooks/useJobs.ts
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '../api/client';
import type { Job } from '../types';

export function useJobs(filters?: { status?: string; target?: string }) {
  return useQuery({
    queryKey: ['jobs', filters],
    queryFn: async () => {
      const { data } = await apiClient.get<Job[]>('/api/jobs', {
        params: filters,
      });
      return data;
    },
    refetchInterval: 5000, // Poll every 5 seconds
    staleTime: 3000,
  });
}
```

**Example - Using Zustand**:

```typescript
// stores/authStore.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface AuthState {
  token: string | null;
  user: string | null;
  setAuth: (token: string, user: string) => void;
  clearAuth: () => void;
  isAuthenticated: boolean;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      user: null,
      setAuth: (token, user) => set({ token, user }),
      clearAuth: () => set({ token: null, user: null }),
      isAuthenticated: () => !!get().token,
    }),
    {
      name: 'auth-storage',
    }
  )
);
```

### WebSocket Integration

**Pattern**: Custom hook manages WebSocket connection lifecycle and message subscriptions.

```typescript
// hooks/useWebSocket.ts
import { useEffect, useState, useCallback, useRef } from 'react';

interface LogMessage {
  job_id: string;
  timestamp: string;
  level: string;
  message: string;
}

interface UseWebSocketOptions {
  jobId?: string;
  onMessage?: (message: LogMessage) => void;
  enabled?: boolean;
}

export function useWebSocket({ jobId, onMessage, enabled = true }: UseWebSocketOptions) {
  const [connected, setConnected] = useState(false);
  const [logs, setLogs] = useState<LogMessage[]>([]);
  const wsRef = useRef<WebSocket | null>(null);

  const connect = useCallback(() => {
    if (!enabled) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/ws`;

    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      console.log('WebSocket connected');
      setConnected(true);
    };

    ws.onmessage = (event) => {
      const message = JSON.parse(event.data) as { type: string; payload: LogMessage };

      if (message.type === 'log') {
        const log = message.payload;

        // Filter by job ID if specified
        if (!jobId || log.job_id === jobId) {
          setLogs((prev) => [...prev, log]);
          onMessage?.(log);
        }
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    ws.onclose = () => {
      console.log('WebSocket disconnected');
      setConnected(false);

      // Reconnect after 3 seconds
      setTimeout(() => {
        connect();
      }, 3000);
    };

    wsRef.current = ws;
  }, [jobId, onMessage, enabled]);

  useEffect(() => {
    connect();

    return () => {
      wsRef.current?.close();
    };
  }, [connect]);

  const clearLogs = useCallback(() => {
    setLogs([]);
  }, []);

  return { connected, logs, clearLogs };
}
```

**Usage in component**:

```typescript
function JobDetail({ jobId }: { jobId: string }) {
  const { logs, connected } = useWebSocket({ jobId });

  return (
    <div>
      <div>Status: {connected ? 'Connected' : 'Disconnected'}</div>
      <div className="logs">
        {logs.map((log, i) => (
          <div key={i} className={`log-${log.level}`}>
            {log.timestamp} [{log.level}] {log.message}
          </div>
        ))}
      </div>
    </div>
  );
}
```

### API Client Pattern

**Centralized axios instance** with authentication interceptor:

```typescript
// api/client.ts
import axios from 'axios';
import { useAuthStore } from '../stores/authStore';

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '',
  headers: {
    'Content-Type': 'application/json',
  },
});

// Add auth token to requests
apiClient.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Handle auth errors
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      useAuthStore.getState().clearAuth();
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

// Type-safe API methods
export const api = {
  dashboard: {
    get: () => apiClient.get('/api/dashboard'),
  },
  jobs: {
    list: (params?: { status?: string; target?: string }) =>
      apiClient.get('/api/jobs', { params }),
    get: (id: string) => apiClient.get(`/api/jobs/${id}`),
    triggerBackup: (data: { target: string }) =>
      apiClient.post('/api/jobs/backup', data),
    triggerRestore: (data: { target: string; backup: string }) =>
      apiClient.post('/api/jobs/restore', data),
    cancel: (id: string) => apiClient.delete(`/api/jobs/${id}`),
    logs: (id: string) => apiClient.get(`/api/jobs/${id}/logs`),
  },
  targets: {
    list: () => apiClient.get('/api/targets'),
  },
  restoreTargets: {
    list: () => apiClient.get('/api/restore-targets'),
  },
};
```

### Component Patterns

**Pattern 1: Data-fetching component with React Query**:

```typescript
// components/JobList.tsx
import { useJobs } from '../hooks/useJobs';
import { Card } from './ui/card';
import { Loader } from './ui/loader';

export function JobList() {
  const { data: jobs, isLoading, error, refetch } = useJobs();

  if (isLoading) return <Loader />;
  if (error) return <div>Error: {error.message}</div>;

  return (
    <div className="space-y-4">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold">Jobs</h2>
        <button onClick={() => refetch()}>Refresh</button>
      </div>

      {jobs?.map((job) => (
        <Card key={job.id}>
          <div className="flex justify-between">
            <div>
              <h3>{job.target}</h3>
              <p className="text-sm text-gray-500">{job.type}</p>
            </div>
            <div>
              <span className={`badge badge-${job.status}`}>
                {job.status}
              </span>
            </div>
          </div>
        </Card>
      ))}
    </div>
  );
}
```

**Pattern 2: Form component with mutations**:

```typescript
// components/RestoreForm.tsx
import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '../api/client';
import { useToast } from '../hooks/useToast';

export function RestoreForm({ target }: { target: string }) {
  const [backup, setBackup] = useState('latest');
  const { toast } = useToast();
  const queryClient = useQueryClient();

  const restoreMutation = useMutation({
    mutationFn: (data: { target: string; backup: string }) =>
      api.jobs.triggerRestore(data),
    onSuccess: () => {
      toast({ title: 'Restore job started', variant: 'success' });
      queryClient.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: (error) => {
      toast({ title: 'Failed to start restore', description: error.message, variant: 'error' });
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    restoreMutation.mutate({ target, backup });
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label>Backup</label>
        <input
          type="text"
          value={backup}
          onChange={(e) => setBackup(e.target.value)}
          placeholder="latest or path/to/backup.tar.gz"
        />
      </div>

      <button
        type="submit"
        disabled={restoreMutation.isPending}
      >
        {restoreMutation.isPending ? 'Starting...' : 'Start Restore'}
      </button>
    </form>
  );
}
```

### Styling with Tailwind + Radix UI

**Pattern**: Use Radix UI primitives for accessible components, styled with Tailwind:

```typescript
// components/ui/button.tsx
import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '../../lib/utils';

const buttonVariants = cva(
  'inline-flex items-center justify-center rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
  {
    variants: {
      variant: {
        default: 'bg-primary text-primary-foreground hover:bg-primary/90',
        destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90',
        outline: 'border border-input bg-background hover:bg-accent hover:text-accent-foreground',
        ghost: 'hover:bg-accent hover:text-accent-foreground',
        link: 'text-primary underline-offset-4 hover:underline',
      },
      size: {
        default: 'h-10 px-4 py-2',
        sm: 'h-9 rounded-md px-3',
        lg: 'h-11 rounded-md px-8',
        icon: 'h-10 w-10',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : 'button';
    return (
      <Comp
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        {...props}
      />
    );
  }
);
Button.displayName = 'Button';

export { Button, buttonVariants };
```

### Build and Development

**Development**:

```bash
cd web
npm install
npm run dev          # Starts dev server on http://localhost:5173
```

**Vite proxy configuration** (in `vite.config.ts`):

```typescript
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/api/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
});
```

**Production build**:

```bash
npm run build        # Outputs to web/dist/
```

**Integration with Go**:

```bash
make build-with-web  # Builds frontend, then embeds in Go binary
```

The frontend is embedded in the Go binary via `go:embed` directive in `internal/web/handler.go`:

```go
//go:embed dist/*
var distFS embed.FS

func Handler() http.Handler {
    sub, _ := fs.Sub(distFS, "dist")
    return http.FileServer(http.FS(sub))
}
```

---

## Common Development Tasks

### Task 1: Add a New Configuration Field

**Scenario**: Add a `timeout` field to backup targets.

1. **Update config struct** (`internal/config/config.go`):

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

2. **Add validation** (`internal/config/validator.go`):

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

3. **Use in backup logic** (`internal/app/backup.go`):

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

1. **Define progress callback type** (`internal/progress/progress.go`):

```go
type Callback func(bytesTransferred, totalBytes int64)
```

2. **Create progress reader wrapper** (`internal/progress/reader.go`):

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

3. **Use in S3 storage** (`internal/storage/s3.go`):

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
// internal/jobs/job.go
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

**Backend** (`internal/api/server.go`):

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

**Frontend Hook** (`web/src/hooks/useStats.ts`):

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

**Frontend Component** (`web/src/components/StatsCard.tsx`):

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

**Create test file** (`internal/app/backup_test.go`):

```go
//go:build integration
// +build integration

package app

import (
    "context"
    "testing"
    "bared/internal/config"
    "bared/internal/testutil"
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

---

## Testing Patterns

### Backend Testing

**Unit test structure**:

```go
package storage

import (
    "bytes"
    "context"
    "testing"
    "bared/internal/config"
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
// internal/storage/mock.go
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

### Frontend Testing

**Component test with React Testing Library**:

```typescript
// components/JobList.test.tsx
import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { JobList } from './JobList';
import { api } from '../api/client';
import { vi } from 'vitest';

vi.mock('../api/client');

describe('JobList', () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  const wrapper = ({ children }) => (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  );

  it('renders loading state', () => {
    render(<JobList />, { wrapper });
    expect(screen.getByText(/loading/i)).toBeInTheDocument();
  });

  it('renders jobs list', async () => {
    const mockJobs = [
      { id: '1', target: 'mysql_prod', status: 'completed', type: 'backup' },
      { id: '2', target: 'postgres_dev', status: 'running', type: 'backup' },
    ];

    vi.mocked(api.jobs.list).mockResolvedValue({ data: mockJobs });

    render(<JobList />, { wrapper });

    await waitFor(() => {
      expect(screen.getByText('mysql_prod')).toBeInTheDocument();
      expect(screen.getByText('postgres_dev')).toBeInTheDocument();
    });
  });

  it('handles error state', async () => {
    vi.mocked(api.jobs.list).mockRejectedValue(new Error('API error'));

    render(<JobList />, { wrapper });

    await waitFor(() => {
      expect(screen.getByText(/error/i)).toBeInTheDocument();
    });
  });
});
```

**Hook testing**:

```typescript
// hooks/useWebSocket.test.ts
import { renderHook, waitFor } from '@testing-library/react';
import { useWebSocket } from './useWebSocket';
import WS from 'vitest-websocket-mock';

describe('useWebSocket', () => {
  let server: WS;

  beforeEach(() => {
    server = new WS('ws://localhost/api/ws');
  });

  afterEach(() => {
    WS.clean();
  });

  it('connects to WebSocket', async () => {
    const { result } = renderHook(() => useWebSocket({ enabled: true }));

    await waitFor(() => {
      expect(result.current.connected).toBe(true);
    });
  });

  it('receives log messages', async () => {
    const { result } = renderHook(() =>
      useWebSocket({ jobId: 'job-1', enabled: true })
    );

    await server.connected;

    server.send(JSON.stringify({
      type: 'log',
      payload: {
        job_id: 'job-1',
        timestamp: '2024-01-01T00:00:00Z',
        level: 'info',
        message: 'Test log',
      },
    }));

    await waitFor(() => {
      expect(result.current.logs).toHaveLength(1);
      expect(result.current.logs[0].message).toBe('Test log');
    });
  });
});
```

---

## Code Patterns and Conventions

### Backend Conventions

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

### Frontend Conventions

**Component naming**:

```
- PascalCase for components: `JobList.tsx`, `RestoreForm.tsx`
- camelCase for hooks: `useJobs.ts`, `useWebSocket.ts`
- kebab-case for CSS files: `job-list.css`
```

**Type definitions**:

```typescript
// Use interfaces for object shapes
interface Job {
  id: string;
  target: string;
  type: 'backup' | 'restore';
  status: 'pending' | 'running' | 'completed' | 'failed';
  created_at: string;
  updated_at: string;
}

// Use type aliases for unions and utilities
type JobStatus = 'pending' | 'running' | 'completed' | 'failed';
type JobType = 'backup' | 'restore';

// Partial utility for optional updates
type JobUpdate = Partial<Job>;
```

**Async/await patterns**:

```typescript
// Always handle errors
async function triggerBackup(target: string) {
  try {
    const response = await api.jobs.triggerBackup({ target });
    toast.success('Backup started');
    return response.data;
  } catch (error) {
    if (axios.isAxiosError(error)) {
      toast.error(error.response?.data?.error || 'Failed to start backup');
    }
    throw error;
  }
}

// Use React Query for automatic error handling
const mutation = useMutation({
  mutationFn: api.jobs.triggerBackup,
  onSuccess: () => {
    toast.success('Backup started');
  },
  onError: (error) => {
    toast.error(error.message);
  },
});
```

**Component organization**:

```typescript
// 1. Imports
import React from 'react';
import { useJobs } from '../hooks/useJobs';

// 2. Types
interface Props {
  target: string;
}

// 3. Component
export function JobList({ target }: Props) {
  // 4. Hooks
  const { data, isLoading } = useJobs({ target });
  const [filter, setFilter] = React.useState('all');

  // 5. Derived state
  const filteredJobs = React.useMemo(
    () => data?.filter(job => filter === 'all' || job.status === filter),
    [data, filter]
  );

  // 6. Event handlers
  const handleFilterChange = (value: string) => {
    setFilter(value);
  };

  // 7. Render
  return (
    <div>
      {/* JSX */}
    </div>
  );
}
```

---

## Integration Points

### Backend-Frontend Integration

**API Contract**:

Backend types (`internal/api/types.go`):

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

Frontend types (`web/src/types/index.ts`):

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

---

## Safety and Security

### Security Considerations

**1. Never log sensitive data**:

```go
// ❌ BAD
slog.Info("connecting to database", "password", conn.Password)

// ✅ GOOD
slog.Info("connecting to database", "host", conn.Host, "database", conn.Database)
```

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

**5. SQL injection prevention** (for future direct DB queries):

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

---

## Development Workflow Summary

### Backend Workflow

1. **Make changes** in `internal/` packages
2. **Format**: `make fmt` or `gofmt -w .`
3. **Run tests**: `make test` (unit) or `make test-integration`
4. **Lint**: `make lint`
5. **Build**: `make build`
6. **Test manually**: `./bin/brd <command>`

### Frontend Workflow

1. **Make changes** in `web/src/`
2. **Run dev server**: `cd web && npm run dev`
3. **Test**: `npm run test` or `npm run test:ui`
4. **Type-check**: `npm run type-check`
5. **Lint**: `npm run lint`
6. **Format**: `npm run format`
7. **Build**: `npm run build`
8. **Validate all**: `npm run validate`

### Full-Stack Workflow

1. **Start backend**: `make build && ./bin/brd daemon --config config.yml --http :8080`
2. **Start frontend dev server**: `cd web && npm run dev` (proxies to :8080)
3. **Make changes** to both backend and frontend
4. **Test integration**: Frontend at <http://localhost:5173> talks to backend at :8080
5. **Build for production**: `make build-with-web` (builds both, embeds frontend in binary)

### Release Workflow

1. **Update version**: Git tag `v1.x.x`
2. **Push tag**: `git push origin v1.x.x`
3. **GitHub Actions**: Automatically builds and releases
4. **Manual**: `make build-all` produces binaries for all platforms

---

## Key Mental Models

### 1. Streaming is Sacred

Never introduce buffering that loads entire datasets into memory. Always use `io.Reader` and `io.Writer` with `io.Pipe` for stage connections.

### 2. Interfaces Enable Extension

To add new functionality:

- Database type → implement `Dumper`/`Restorer`
- Storage backend → implement `Storage`
- Notification → implement `Notifier`

### 3. Context Propagation

Always pass `context.Context` as the first parameter and check for cancellation in long-running operations.

### 4. Frontend State Layers

- **Server state** (React Query) for API data
- **Real-time state** (WebSocket) for live updates
- **Client state** (Zustand) for UI preferences

### 5. Type Safety Everywhere

Both Go and TypeScript enforce strong typing. Use it to prevent bugs at compile time.

---

## Quick Reference

### File Locations

| Component | Location |
|-----------|----------|
| CLI entry | `cmd/brd/main.go` |
| Config parsing | `internal/config/` |
| Backup logic | `internal/app/backup.go` |
| API server | `internal/api/server.go` |
| Job queue | `internal/jobs/` |
| Database impls | `internal/database/*.go` |
| Storage impls | `internal/storage/*.go` |
| Frontend entry | `web/src/main.tsx` |
| API client | `web/src/api/client.ts` |
| WebSocket hook | `web/src/hooks/useWebSocket.ts` |

### Commands

| Task | Command |
|------|---------|
| Build backend | `make build` |
| Build frontend | `cd web && npm run build` |
| Build both | `make build-with-web` |
| Run backend tests | `make test` |
| Run frontend tests | `cd web && npm run test` |
| Run integration tests | `make test-integration` |
| Lint backend | `make lint` |
| Lint frontend | `cd web && npm run lint` |
| Format backend | `make fmt` |
| Format frontend | `cd web && npm run format` |
| Dev frontend | `cd web && npm run dev` |
| Validate all backend | `make validate` |
| Validate all frontend | `cd web && npm run validate` |

---

This guide should provide you with a comprehensive understanding of the BareD codebase and enable you to contribute effectively to both the backend and frontend. Remember: when in doubt, follow existing patterns in the codebase, and always prioritize streaming architectures and type safety.
