# Architecture Documentation

Deep dives into BareD's system design, architectural decisions, and implementation details.

## Contents

### [Design Decisions](design-decisions.md)

Key architectural choices, trade-offs considered, and rationale behind major design decisions.

### [Streaming Pipeline](streaming-pipeline.md)

Detailed explanation of the streaming architecture using `io.Pipe`, how data flows through the system without temporary files.

### [Notification System](notification-system.md)

Architecture of the notification system, message structure, delivery guarantees, and extensibility.

### [Persistence Layer](persistence-layer.md)

Database persistence design for job history, log storage, and distributed locking.

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

**See**: [Streaming Pipeline](streaming-pipeline.md)

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

**See**: [Design Decisions](design-decisions.md#interface-driven-design)

### 3. Graceful Error Handling

Operations handle partial failures gracefully:

- Retry logic with exponential backoff
- Cleanup on failure
- Detailed error messages
- Non-blocking notifications

**See**: [Design Decisions](design-decisions.md#error-handling)

### 4. Minimal Dependencies

Prefer standard library over external packages:

- stdlib for compression (tar, gzip)
- stdlib for HTTP server
- Few external dependencies

**See**: [Design Decisions](design-decisions.md#dependency-philosophy)

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

**See**: [Streaming Pipeline](streaming-pipeline.md#performance)

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

**See**: [Design Decisions](design-decisions.md#security)

## Further Reading

- [Design Decisions](design-decisions.md) - Why we built it this way
- [Streaming Pipeline](streaming-pipeline.md) - Deep dive into streaming
- [Notification System](notification-system.md) - Notification architecture
- [Persistence Layer](persistence-layer.md) - Database design

---

[← Back to Documentation](../README.md)
