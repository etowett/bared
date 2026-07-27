# Architecture Documentation

Deep dives into BareD's system design, architectural decisions, and implementation details.

## Contents

> **Note on coverage.** Separate design-decisions, streaming-pipeline,
> notification-system and persistence-layer pages were planned but never written.
> This page carries that material; the deeper detail lives in the nested `AGENTS.md`
> files next to the code, which are kept current because changes to those areas are
> required to update them.

On this page:

- **[Key Design Principles](#key-design-principles)** — streaming, interface-driven
  extension, error handling and the minimal-dependency policy. See also
  [apps/api/internal/AGENTS.md](../../apps/api/internal/AGENTS.md).
- **[Data Flow](#data-flow)** — how a backup and a restore move through the pipeline
  with `io.Pipe`, and why no temporary files are involved.
- **[Core Components](#core-components)** — including the persistence layer (job
  history, log storage, distributed locking); configure it with
  [examples/config.persistence.yml](../../examples/config.persistence.yml).
- **[Security Architecture](#security-architecture)** — and
  [SECURITY.md](../../SECURITY.md) for known limitations.

Notification design lives in
[apps/api/internal/notify/AGENTS.md](../../apps/api/internal/notify/AGENTS.md), with
configuration in [examples/NOTIFICATIONS.md](../../examples/NOTIFICATIONS.md).

### Historical Documents

#### [Original Implementation Plan](original-plan.md)

The original comprehensive plan that guided BareD's development. Historical reference.

#### [Implementation Complete](implementation-complete.md)

Summary of completed implementation phases and verification.

#### [Release Uniformity](release-uniformity.md)

Documentation of release process standardization.

#### [Implementation Plans](plans/)

Historical implementation plans for major features:

- [Logging & Alerting Enhancement](plans/logging-alerting.md)

## System Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    BareD Daemon                         │
├─────────────────────────────────────────────────────────┤
│  Scheduler    │  Job Manager  │  HTTP API  │  CLI       │
└────┬──────────┴───────┬───────┴─────┬──────┴────┬───────┘
     │                  │              │           │
     ▼                  ▼              ▼           ▼
┌─────────────────────────────────────────────────────────┐
│              Application Layer (apps/api/internal/app/)          │
│  - Backup Orchestration      - Restore Orchestration    │
│  - Progress Tracking         - Result Aggregation       │
└────┬────────────────────────────────────────────┬───────┘
     │                                             │
     ▼                                             ▼
┌───────────────────────┐              ┌──────────────────┐
│   Database Layer      │              │  Storage Layer   │
│  (apps/api/internal/database/) │              │ (apps/api/internal/       │
│  - MySQL              │              │  storage/)       │
│  - PostgreSQL         │              │  - Local FS      │
│  - Redis              │              │  - S3            │
│  - Dumper/Restorer    │              │  - SFTP          │
└───────────────────────┘              └──────────────────┘
            │                                    │
            ▼                                    ▼
┌───────────────────────┐              ┌──────────────────┐
│ Compression Layer     │              │ Notification     │
│ (apps/api/internal/compress/)  │              │ (apps/api/internal/       │
│  - tar.gz             │              │  notify/)        │
│  - Streaming          │              │  - Slack         │
└───────────────────────┘              │  - Email         │
                                       │  - Webhook       │
                                       └──────────────────┘
```

### Core Components

**Application Layer** (`apps/api/internal/app/`):

- High-level orchestration of backup/restore operations
- Pipeline coordination
- Result aggregation and reporting

**Database Layer** (`apps/api/internal/database/`):

- Database-specific dump and restore implementations
- Unified interface for all database types
- Streaming output/input

**Storage Layer** (`apps/api/internal/storage/`):

- Storage backend abstractions
- Retry logic for network operations
- Metadata management

**Compression Layer** (`apps/api/internal/compress/`):

- Streaming compression using tar + gzip
- No temporary files

**Notification Layer** (`apps/api/internal/notify/`):

- Multi-channel notifications
- Rich message formatting
- Delivery tracking

**Job Management** (`apps/api/internal/jobs/`):

- Job lifecycle management
- Progress tracking
- Log aggregation

**Persistence** (`apps/api/internal/persistence/`):

- Job history storage
- Log persistence
- Distributed locking

## Key Design Principles

### 1. Streaming Architecture

No temporary files are created during backup or restore operations. Data streams through the pipeline using `io.Pipe`.

**Benefits**:

- Reduced disk I/O
- Lower storage requirements
- Better performance
- Simpler cleanup

**See**: [Data Flow](#data-flow) below.

### 2. Interface-Driven Design

Core functionality is abstracted behind interfaces:

- `Dumper` / `Restorer` for databases
- `Storage` for storage backends
- `Compressor` / `Decompressor` for compression
- `Notifier` for notifications

**Benefits**:

- Easy extensibility
- Clear contracts
- Testability
- Pluggable components

**See**: [apps/api/internal/AGENTS.md](../../apps/api/internal/AGENTS.md), and the nested guides for [storage](../../apps/api/internal/storage/AGENTS.md) and [notify](../../apps/api/internal/notify/AGENTS.md).

### 3. Graceful Error Handling

Operations handle partial failures gracefully:

- Retry logic with exponential backoff
- Cleanup on failure
- Detailed error messages
- Non-blocking notifications

**See**: the error-handling conventions in [CONTRIBUTING.md](../../CONTRIBUTING.md#error-handling).

### 4. Minimal Dependencies

Prefer standard library over external packages:

- stdlib for compression (tar, gzip)
- stdlib for HTTP server
- Few external dependencies

**See**: the stdlib-first principle in [AGENTS.md](../../AGENTS.md#engineering-principles).

## Data Flow

### Backup Operation

```
1. Validation
   ├─ Validate database connection
   ├─ Validate storage backend
   └─ Check permissions

2. Pipeline Setup
   ├─ Create dump reader
   ├─ Create compression writer
   └─ Create storage writer

3. Streaming Execution
   Database → io.Pipe → Compressor → io.Pipe → Storage

4. Cleanup & Notification
   ├─ Close all pipes
   ├─ Update metadata
   ├─ Apply retention policy
   └─ Send notifications
```

### Restore Operation

```
1. Validation
   ├─ Validate backup exists
   ├─ Validate database connection
   └─ Optionally: dry-run mode

2. Pipeline Setup
   ├─ Create storage reader
   ├─ Create decompression reader
   └─ Create database writer

3. Streaming Execution
   Storage → io.Pipe → Decompressor → io.Pipe → Database

4. Cleanup & Notification
   ├─ Close all pipes
   ├─ Verify restore success
   └─ Send notifications
```

## Extending BareD

The architecture is designed for extensibility:

**Add a Database**: Implement `Dumper` and `Restorer` interfaces
**Add Storage**: Implement `Storage` interface
**Add Compression**: Implement `Compressor` and `Decompressor` interfaces
**Add Notifications**: Implement `Notifier` interface

**See**: [Development Guide](../development/) for implementation details

## Performance Considerations

### Streaming Benefits

- **Memory**: Fixed memory usage regardless of backup size
- **Disk**: No temporary file creation
- **Performance**: Parallel stages (dump while compressing while uploading)

### Optimization Opportunities

- Concurrent backups (configurable limit)
- Compression level tuning
- Buffer size configuration
- Network retry strategies

**See**: [Streaming Architecture](#1-streaming-architecture) and [Data Flow](#data-flow).

## Security Architecture

### Secrets Management

- Environment variable expansion in config
- No secrets in config files
- Secure credential storage

### Access Control

- Minimal database permissions required
- Storage backend isolation
- Network security

### Audit Trail

- Job history persistence
- Log retention
- Notification tracking

**See**: [SECURITY.md](../../SECURITY.md) for the security model, its known limitations, and deployment hardening.

## Further Reading

- [apps/api/internal/AGENTS.md](../../apps/api/internal/AGENTS.md) - Backend deep dive, kept current with the code
- [AGENTS.md](../../AGENTS.md) - Repository map and engineering principles
- [SECURITY.md](../../SECURITY.md) - Security model and known limitations
- [Original Implementation Plan](original-plan.md) - Historical design rationale
- [REST API](../api/endpoints.md) / [WebSocket API](../api/websocket.md) - Interface reference

---

[← Back to Documentation](../README.md)
