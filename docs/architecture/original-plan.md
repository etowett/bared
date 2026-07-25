# BareD (Backup-Restore Daemon) - Implementation Plan

## Executive Summary

BareD is a production-ready backup/restore daemon to be built from scratch in Go, inspired by gobackup but simplified for your specific needs. Target: ~2-3K lines of clean, maintainable code.

**Core Features:**

- Database backup/restore: MySQL/MariaDB, PostgreSQL, Redis
- Storage backends: Local filesystem, S3 (and S3-compatible), SFTP
- Daemon mode with cron scheduling
- Retention policies with automatic cleanup
- Slack notifications
- Streaming architecture (no temp files between stages)

**Design Principles:**

- Minimal dependencies (avoid heavy frameworks)
- Interface-driven architecture (easy extensibility)
- Streaming by default (handle multi-GB dumps)
- Parallel operations (multiple targets, multiple storages)
- Retry logic with exponential backoff
- Graceful error handling (partial failures don't stop everything)

---

## Project Structure

```
bared/
├── apps/api/cmd/brd/
│   └── main.go                    # CLI entry point
│
├── apps/api/internal/
│   ├── app/
│   │   ├── backup.go              # Backup workflow orchestration
│   │   ├── restore.go             # Restore workflow orchestration
│   │   ├── list.go                # List backups logic
│   │   └── pipeline.go            # Pipeline coordination (dump→compress→upload)
│   │
│   ├── config/
│   │   ├── config.go              # Configuration structs
│   │   ├── parser.go              # YAML parsing with env var expansion
│   │   └── validator.go           # Configuration validation
│   │
│   ├── database/
│   │   ├── database.go            # Dumper/Restorer interfaces
│   │   ├── mysql.go               # MySQL implementation
│   │   ├── postgres.go            # PostgreSQL implementation
│   │   ├── redis.go               # Redis implementation
│   │   └── factory.go             # Database factory
│   │
│   ├── storage/
│   │   ├── storage.go             # Storage interface
│   │   ├── local.go               # Local filesystem
│   │   ├── s3.go                  # S3 and S3-compatible services
│   │   ├── sftp.go                # SFTP implementation
│   │   └── factory.go             # Storage factory
│   │
│   ├── compress/
│   │   ├── compress.go            # Compressor interface
│   │   ├── tgz.go                 # tar.gz implementation
│   │   └── factory.go             # Compressor factory
│   │
│   ├── notify/
│   │   ├── notifier.go            # Notifier interface
│   │   ├── slack.go               # Slack implementation
│   │   └── factory.go             # Notifier factory
│   │
│   ├── daemon/
│   │   ├── daemon.go              # Daemon mode with signal handling
│   │   └── scheduler.go           # Cron scheduling
│   │
│   ├── retention/
│   │   ├── tracker.go             # Backup tracking (JSON)
│   │   └── cleaner.go             # Cleanup logic
│   │
│   └── util/
│       ├── paths.go               # Backup path generation
│       ├── shell.go               # Command execution helpers
│       ├── stream.go              # Streaming utilities
│       └── retry.go               # Retry logic with backoff
│
├── examples/
│   ├── config.example.yml         # Example configuration
│   └── bared.service              # systemd service file (Phase 7)
│
├── go.mod
├── go.sum
├── README.md
└── Makefile
```

---

## Technology Stack

### Core Dependencies

```go
require (
    // YAML parsing (lightweight, stdlib-like)
    gopkg.in/yaml.v3 v3.0.1

    // AWS SDK v2 (for S3 and S3-compatible)
    github.com/aws/aws-sdk-go-v2 v1.24.0
    github.com/aws/aws-sdk-go-v2/config v1.26.0
    github.com/aws/aws-sdk-go-v2/service/s3 v1.48.0

    // SFTP client
    github.com/pkg/sftp v1.13.6
    golang.org/x/crypto v0.17.0

    // Cron scheduler
    github.com/robfig/cron/v3 v3.0.1

    // CLI framework
    github.com/spf13/cobra v1.8.0
)
```

**Why these choices:**

- **yaml.v3**: Lightweight, no Viper overhead (saves 500KB+ in binary)
- **aws-sdk-go-v2**: Modern, modular, better performance than v1
- **pkg/sftp**: Standard SFTP library in Go ecosystem
- **robfig/cron**: Simple, reliable, widely used
- **cobra**: Excellent UX, active maintenance, good docs

### Standard Library Usage

Maximize stdlib where possible:

- `archive/tar`: Tar operations
- `compress/gzip`: Gzip compression
- `context`: Cancellation and timeouts
- `io`: Streaming interfaces
- `os/exec`: Execute database tools (mysqldump, pg_dump, etc.)

---

## Core Architecture

### Key Interfaces

**Database Interface:**

```go
// apps/api/internal/database/database.go
type Dumper interface {
    Dump(ctx context.Context, w io.Writer) (*DumpMetadata, error)
    Name() string
    Validate(ctx context.Context) error
}

type Restorer interface {
    Restore(ctx context.Context, r io.Reader) error
    Name() string
}
```

**Storage Interface:**

```go
// apps/api/internal/storage/storage.go
type Storage interface {
    Store(ctx context.Context, path string, r io.Reader, size int64) error
    Retrieve(ctx context.Context, path string, w io.Writer) error
    List(ctx context.Context) ([]*BackupInfo, error)
    Delete(ctx context.Context, path string) error
    Name() string
    Validate(ctx context.Context) error
}
```

**Compressor Interface:**

```go
// apps/api/internal/compress/compress.go
type Compressor interface {
    Compress(ctx context.Context, r io.Reader, w io.Writer) error
    Decompress(ctx context.Context, r io.Reader, w io.Writer) error
    Extension() string
}
```

**Notifier Interface:**

```go
// apps/api/internal/notify/notifier.go
type Notifier interface {
    NotifySuccess(ctx context.Context, msg *Message) error
    NotifyFailure(ctx context.Context, msg *Message) error
    Name() string
}
```

### Pipeline Architecture

**Backup Pipeline (Streaming):**

```
[Database Dump] → [Compress] → [Storage Upload] → [Cleanup] → [Notify]
                      ↓
           [Connected via io.Pipe - no temp files]
```

**Restore Pipeline (Streaming):**

```
[Storage Retrieve] → [Decompress] → [Database Restore] → [Cleanup] → [Notify]
                         ↓
              [Connected via io.Pipe]
```

**Key Design Decisions:**

1. **Streaming**: Use `io.Pipe()` to connect stages - no disk I/O between steps
2. **Context Propagation**: Pass `context.Context` for cancellation
3. **Parallel Uploads**: Multiple storages upload concurrently
4. **Partial Failure Handling**: Track which operations succeed/fail
5. **Retry Logic**: Exponential backoff for network operations

---

## Configuration Format

```yaml
# Default storage for all targets
default_storage: s3

# Storage backend definitions
storages:
  local_disk:
    type: local
    path: /data/backups
    keep: 20

  s3:
    type: s3
    bucket: my-bucket
    region: us-east-1
    access_key_id: ${AWS_ACCESS_KEY_ID}      # Environment variable expansion
    secret_access_key: ${AWS_SECRET_ACCESS_KEY}
    endpoint_url: https://s3.example.com     # For S3-compatible (MinIO, etc.)
    path: backups/                           # Prefix within bucket
    keep: 30

  offsite:
    type: sftp
    host: backup.example.com
    port: 22
    username: backup
    password: ${SFTP_PASSWORD}
    path: /backups/
    keep: 90

# Notification definitions
notifiers:
  slack:
    type: slack
    url: https://hooks.slack.com/services/...
    on_success: false                        # Only notify on failure

# Backup targets (explicit array structure)
targets:
  - name: athena
    conn:
      type: mysql
      user: user
      password: password
      database: db_name
      host: host
      port: 3306
    exclude_tables:
      - temp_logs
      - session_data
    additional_args:
      - --single-transaction
    compress:
      enabled: true
      type: tgz
    storage:
      enabled: true
      name: main_s3
    schedule: "0 2 * * *"                    # Cron expression: daily at 2 AM

  - name: postgres_db
    conn:
      type: postgres
      user: postgres
      password: secret
      database: mydb
      host: localhost
      port: 5432
    compress:
      enabled: true
      type: tgz
    storage:
      enabled: true
      name: local_disk
    schedule: "0 3 * * *"                    # Daily at 3 AM

  - name: redis_cache
    conn:
      type: redis
      host: localhost
      port: 6379
      password: ${REDIS_PASSWORD}
    compress:
      enabled: true
      type: tgz
    storage:
      enabled: true
      name: s3
    schedule: "0 4 * * *"                    # Daily at 4 AM
```

---

## CLI Commands

```bash
# Backup a target
brd backup --config config.yml --target athena

# Restore latest backup
brd restore --config config.yml --target athena --backup latest

# Restore specific backup
brd restore --config config.yml --target athena --backup athena/mysql/2025-12-02T15-04-05Z/mydb.sql.tar.gz

# List backups for a target
brd list --config config.yml --target athena

# Validate configuration
brd validate-config --config config.yml

# Run as daemon (with scheduler)
brd daemon --config config.yml

# Future: Verify backup integrity
brd verify --config config.yml --target athena --backup latest
```

---

## Implementation Phases

### Phase 1: Foundation (3 days, ~300 LOC)

**Goal**: Working CLI with config parsing

**Tasks:**

1. Initialize Go module: `go mod init bared`
2. Implement config package (parser, validator, structs with `targets` array)
3. Set up Cobra CLI with command stubs
4. Implement `brd validate-config` command
5. Create example configuration file
6. Basic logging setup

**Critical Files:**

- `apps/api/cmd/brd/main.go` - CLI entry point with Cobra setup
- `apps/api/internal/config/config.go` - Configuration structs (with `targets` as array)
- `apps/api/internal/config/parser.go` - YAML parsing with env var expansion
- `apps/api/internal/config/validator.go` - Validation logic
- `examples/config.example.yml` - Example configuration

**Deliverable**: Can parse and validate YAML config, CLI responds to commands

---

### Phase 2: Database Dumpers (4 days, ~500 LOC)

**Goal**: Can dump MySQL, PostgreSQL, Redis to stdout

**Tasks:**

1. Define database interfaces (Dumper, Restorer)
2. Implement MySQL dumper (execute `mysqldump`)
3. Implement PostgreSQL dumper (execute `pg_dump`)
4. Implement Redis dumper (RDB file copy or `redis-cli --rdb`)
5. Create database factory (type → implementation)
6. Add connection validation
7. Unit tests with mocked commands

**Critical Files:**

- `apps/api/internal/database/database.go` - Interfaces and shared types
- `apps/api/internal/database/mysql.go` - MySQL implementation
- `apps/api/internal/database/postgres.go` - PostgreSQL implementation
- `apps/api/internal/database/redis.go` - Redis implementation (RDB snapshot)
- `apps/api/internal/database/factory.go` - Factory function
- `apps/api/internal/util/shell.go` - Command execution helpers

**Deliverable**: Can execute dumps for all three databases, stream to io.Writer

**Note on Redis**: Redis backup can be done via:

- RDB file copy (if local and file accessible)
- `redis-cli --rdb dump.rdb` (remote backup)
- `BGSAVE` command then wait and copy RDB file

---

### Phase 3: Compression (2 days, ~200 LOC)

**Goal**: Can compress dumps to tar.gz format

**Tasks:**

1. Define compressor interface
2. Implement tar.gz compressor using stdlib
3. Create compressor factory
4. Add streaming compression support
5. Unit tests for compression/decompression

**Critical Files:**

- `apps/api/internal/compress/compress.go` - Interface definition
- `apps/api/internal/compress/tgz.go` - tar.gz implementation
- `apps/api/internal/compress/factory.go` - Factory function

**Deliverable**: Streaming pipeline works (dump → compress)

---

### Phase 4: Storage Backends (5 days, ~600 LOC)

**Goal**: Can store backups in local, S3, and SFTP

**Tasks:**

1. Define storage interface
2. Implement local storage with path management
3. Implement S3 storage with aws-sdk-go-v2
4. Implement SFTP storage
5. Create storage factory
6. Add validation for storage configs
7. Implement retry logic with exponential backoff
8. Unit tests with mocked backends

**Critical Files:**

- `apps/api/internal/storage/storage.go` - Interface and shared types
- `apps/api/internal/storage/local.go` - Local filesystem implementation
- `apps/api/internal/storage/s3.go` - S3 implementation (handles S3-compatible too)
- `apps/api/internal/storage/sftp.go` - SFTP implementation
- `apps/api/internal/storage/factory.go` - Factory function
- `apps/api/internal/util/retry.go` - Retry logic

**Deliverable**: Can upload to all storage types with retry logic

---

### Phase 5: Backup Pipeline (4 days, ~400 LOC)

**Goal**: End-to-end backup workflow

**Tasks:**

1. Implement pipeline orchestration
2. Create unique backup path generator
3. Wire dump → compress → upload pipeline
4. Implement `brd backup` command
5. Add progress logging
6. Handle partial failures (some storages fail)
7. Integration tests with Docker databases

**Critical Files:**

- `apps/api/internal/app/pipeline.go` - Pipeline coordination
- `apps/api/internal/app/backup.go` - Backup workflow
- `apps/api/internal/util/paths.go` - Path generation
- `apps/api/cmd/brd/backup.go` - Backup command

**Path Format:**

```
{target}/{dbtype}/{timestamp}/{database}.{extension}
Example: athena/mysql/2025-12-02T15-04-05Z/mydb.sql.tar.gz
```

**Deliverable**: `brd backup --config config.yml --target athena` works end-to-end

---

### Phase 6: Restore Workflow (4 days, ~400 LOC)

**Goal**: Can restore from backups

**Tasks:**

1. Implement restore methods in database implementations
2. Implement reverse pipeline (retrieve → decompress → restore)
3. Implement `brd restore` command
4. Add "latest" backup detection
5. Implement `brd list` command
6. Integration tests for restore

**Critical Files:**

- `apps/api/internal/app/restore.go` - Restore workflow
- `apps/api/internal/app/list.go` - List backups logic
- `apps/api/cmd/brd/restore.go` - Restore command
- `apps/api/cmd/brd/list.go` - List command

**Deliverable**: Can restore latest or specific backup, list shows available backups

---

### Phase 7: Daemon & Scheduler (3 days, ~300 LOC)

**Goal**: Can run as daemon with cron scheduling

**Tasks:**

1. Implement daemon mode with signal handling
2. Integrate robfig/cron for scheduling
3. Add schedule field parsing from config
4. Implement `brd daemon` command
5. Add graceful shutdown (SIGTERM, SIGINT)
6. Create systemd service file example

**Critical Files:**

- `apps/api/internal/daemon/daemon.go` - Daemon mode logic
- `apps/api/internal/daemon/scheduler.go` - Cron integration
- `apps/api/cmd/brd/daemon.go` - Daemon command
- `examples/bared.service` - systemd service file

**Deliverable**: `brd daemon` runs scheduled backups, handles signals gracefully

---

### Phase 8: Retention & Notifications (3 days, ~300 LOC)

**Goal**: Automatic cleanup and notifications

**Tasks:**

1. Implement retention tracker (JSON metadata)
2. Implement cleanup logic based on `keep` setting
3. Define notifier interface
4. Implement Slack notifier
5. Create notifier factory
6. Integrate notifications into pipeline (async)

**Critical Files:**

- `apps/api/internal/retention/tracker.go` - Backup tracking with JSON
- `apps/api/internal/retention/cleaner.go` - Cleanup logic
- `apps/api/internal/notify/notifier.go` - Interface definition
- `apps/api/internal/notify/slack.go` - Slack implementation
- `apps/api/internal/notify/factory.go` - Factory function

**Tracker Format (JSON):**

```json
{
  "storage": "s3_main",
  "target": "athena",
  "backups": [
    {
      "path": "athena/mysql/2025-12-02T15:04:05Z/mydb.sql.tar.gz",
      "size": 1048576,
      "created": "2025-12-02T15:04:05Z"
    }
  ]
}
```

**Deliverable**: Old backups cleaned up automatically, Slack notifications work

---

### Phase 9: Polish & Documentation (3 days, ~100 LOC)

**Goal**: Production-ready release

**Tasks:**

1. Comprehensive README with examples
2. Add `--dry-run` flag
3. Add `--verbose` flag for debugging
4. Implement proper exit codes
5. Shell completion generation
6. Security audit (credentials in logs?)
7. Create Makefile (build, test, install)
8. Docker image (optional)

**Deliverable**: Ready for v1.0.0 release with complete documentation

---

## Key Design Decisions

### 1. Streaming Architecture

**Why**: Database dumps can be GBs - never load fully into memory

**Implementation**: Use `io.Pipe()` to connect stages

```go
pr, pw := io.Pipe()

// Dump in goroutine
go func() {
    defer pw.Close()
    database.Dump(ctx, pw)
}()

// Compress reads from pipe
compressor.Compress(ctx, pr, compressedWriter)
```

### 2. Parallel Operations

**Multiple Targets**: Use worker pool with `runtime.NumCPU()` limit

```go
sem := make(chan struct{}, runtime.NumCPU())
for _, target := range targets {
    go func(t *config.Target) {
        sem <- struct{}{}        // Acquire
        defer func() { <-sem }() // Release
        BackupTarget(ctx, t)
    }(target)
}
```

**Multiple Storages**: Upload to all storages concurrently

- Track partial failures
- Continue if at least one storage succeeds
- Report all failures in notification

### 3. Retry Logic

**Exponential Backoff**:

- Initial delay: 1s
- Max delay: 30s
- Multiplier: 2x
- Max attempts: 3

**Retry-able errors**:

- Network errors
- 5xx HTTP responses
- Temporary S3 errors
- SFTP connection timeouts

**Don't retry**:

- Authentication errors (401, 403)
- Invalid config
- Missing database binaries
- 4xx errors (except 429 rate limit)

### 4. Backup Path Generation

**Format**: `{target}/{dbtype}/{timestamp}/{database}.{extension}`

**Example**: `athena/mysql/2025-12-02T15-04-05Z/mydb.sql.tar.gz`

**Benefits**:

- Clear hierarchy
- Easy to find latest (sort by timestamp)
- No collisions
- Database type visible for troubleshooting

### 5. Retention Policy

**Tracking**: JSON file per storage+target combination

- Local: `{backup_root}/.bared/tracker-{target}.json`
- S3: `.bared/tracker-{target}.json` in bucket
- SFTP: Same as S3

**Cleanup**: After successful backup

1. Add new backup to tracker
2. Sort by timestamp (newest first)
3. Keep first N backups (based on `keep` setting)
4. Delete older backups
5. Update tracker

### 6. Error Handling

**Principles**:

1. Wrapped errors with context: `fmt.Errorf("upload to S3: %w", err)`
2. Partial success tracking: Continue if some operations succeed
3. Graceful degradation: Notification failures don't fail backup
4. Proper cleanup: Defer blocks ensure cleanup on error

**Partial Failure Example**:

```
Backup athena:
  ✓ Dump successful (2.3 GB, 45s)
  ✓ Compress successful (890 MB)
  ✓ Upload to s3_main successful
  ✗ Upload to offsite_sftp failed: connection timeout
  ⚠ Backup partially successful (1/2 storages)
```

---

## Simplifications vs GoBackup

### What We're Simplifying

1. **Configuration**: yaml.v3 instead of Viper (lighter, explicit)
2. **Web UI**: CLI-only initially (document API for future)
3. **Encryption**: Rely on S3 SSE or external encryption
4. **Compression Formats**: tar.gz only (stdlib, fast, common)
5. **Notifiers**: Slack initially (can add webhook pattern later)
6. **Archive Splitting**: Not needed (modern storage handles large files)

### What We're Keeping

1. **Hooks Support**: Pre/post scripts (valuable for app coordination)
2. **Retention Policies**: Essential feature, simple JSON tracking
3. **Multi-storage**: Critical for redundancy
4. **Interface Design**: Makes extension easy
5. **Scheduler**: Daemon mode with cron

### What We're Improving

1. **Streaming by Default**: No temp files between stages
2. **Parallel by Default**: Multiple targets and storages
3. **Retry by Default**: All network operations
4. **Better Error Messages**: Include context and suggestions

---

## Future Extensibility (Document, Don't Implement)

### Hooks System

```yaml
athena:
  hooks:
    before_backup: /scripts/pre-backup.sh
    after_backup: /scripts/post-backup.sh
    on_failure: /scripts/alert.sh
```

### Verify Command

```bash
brd verify --config config.yml --target athena --backup latest
# Checks: file exists, archive integrity, decompression, SQL syntax
```

### Additional Databases

- MongoDB (mongodump/mongorestore)
- Elasticsearch (snapshot API)
- Cassandra (nodetool)
- InfluxDB

### Additional Storage

- Google Cloud Storage
- Azure Blob Storage
- Backblaze B2 native API

### Additional Notifiers

- Discord
- Email (SMTP)
- PagerDuty
- Generic webhooks

### Web UI

- REST API in BareD (optional)
- Separate frontend (React/Vue)
- View history, trigger backups, monitor status

---

## Testing Strategy

### Unit Tests

- Mock external commands using interfaces
- Test each component in isolation
- Coverage target: >80%

### Integration Tests

- Use Docker containers for databases
- Test real backup/restore cycles
- Use MinIO for S3 testing

### End-to-End Tests

- Bash script that runs full workflow
- Docker Compose for complete environment
- Verify data integrity after restore

---

## Production Readiness Checklist

### Security

- [ ] Never log credentials
- [ ] Support environment variable expansion
- [ ] Validate config file permissions (warn if world-readable)
- [ ] Use secure defaults (TLS for SFTP, S3)
- [ ] Support IAM roles for S3 (no hardcoded credentials)

### Observability

- [ ] Structured logging with levels
- [ ] Log backup size, duration, success/failure
- [ ] Include operation context in all errors
- [ ] Metrics for monitoring (backup size, duration, failures)

### Reliability

- [ ] Graceful shutdown (SIGTERM/SIGINT)
- [ ] Context cancellation propagated
- [ ] Cleanup temp files on error
- [ ] Atomic operations where possible
- [ ] Idempotent backup operations

### Performance

- [ ] Streaming (no temp files for data)
- [ ] Parallel operations
- [ ] Connection pooling for S3/SFTP
- [ ] Efficient tar creation

### Usability

- [ ] Helpful error messages with suggestions
- [ ] `--dry-run` mode
- [ ] `--verbose` flag
- [ ] Shell completion
- [ ] Example configs for common scenarios

---

## Timeline & LOC Estimates

| Phase | Duration | LOC | Key Deliverable |
|-------|----------|-----|-----------------|
| 1. Foundation | 3 days | 300 | Config parsing + CLI |
| 2. Database Dumpers | 4 days | 500 | All databases dump |
| 3. Compression | 2 days | 200 | Streaming compression |
| 4. Storage Backends | 5 days | 600 | S3, local, SFTP |
| 5. Backup Pipeline | 4 days | 400 | End-to-end backup |
| 6. Restore Workflow | 4 days | 400 | Restore + list |
| 7. Daemon & Scheduler | 3 days | 300 | Cron scheduling |
| 8. Retention & Notify | 3 days | 300 | Cleanup + Slack |
| 9. Polish & Docs | 3 days | 100 | v1.0 ready |

**Total**: 31 days (6 weeks), ~3,100 LOC

---

## Success Criteria (v1.0)

1. **Functionality**: All core commands work (backup, restore, list, daemon)
2. **Reliability**: No data loss in 100 test backup/restore cycles
3. **Performance**: Can backup 1GB database in <2 minutes
4. **Code Quality**:
   - Total LOC < 3,500
   - Test coverage > 75%
   - Passes go vet, golangci-lint
   - No critical security issues
5. **Documentation**: README covers 80% of use cases
6. **Usability**: New user can backup MySQL in <5 minutes

---

## Next Steps

After plan approval, start with Phase 1:

1. Initialize Go module: `go mod init bared`
2. Create project structure (directories)
3. Implement config structs (with `targets` array)
4. Set up Cobra CLI
5. Create example config file
6. Implement config validation

**Note**: Module name is simply `bared` (not github.com/yourusername/bared). This keeps it simple and allows for flexible repository hosting later.

Once Phase 1 is complete, each subsequent phase builds on the previous one. The architecture is designed for incremental development - each phase produces working, testable code.
