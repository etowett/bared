# BareD - Backup and Restore Daemon

A simple yet powerful backup and restore daemon for databases written in Go.

## Features

- **Database Support**: MySQL/MariaDB, PostgreSQL, Redis
- **Storage Backends**: Local filesystem, S3 (and S3-compatible services), SFTP
- **Compression**: tar.gz compression with streaming
- **Configuration**: YAML-based with environment variable expansion
- **CLI Interface**: Simple command-line interface

## Implementation Status

### ✅ All Core Phases Complete! (v1.0-ready)

**Phase 1: Foundation** ✓
- Go module initialized
- Project structure created
- Configuration system (YAML parsing, validation, env vars)
- Cobra CLI with all commands
- Example configuration file

**Phase 2: Database Dumpers** ✓
- Database interfaces (Dumper, Restorer)
- MySQL/MariaDB implementation (mysqldump/mysql)
- PostgreSQL implementation (pg_dump/psql)
- Redis implementation (redis-cli --rdb)
- Database factory for type dispatch

**Phase 3: Compression** ✓
- Compressor interface
- tar.gz implementation using stdlib
- Streaming compression support

**Phase 4: Storage Backends** ✓
- Storage interface with retry logic
- Local filesystem storage
- S3 storage (with S3-compatible support: MinIO, DigitalOcean Spaces, etc.)
- SFTP storage
- Storage factory

**Phase 5: Backup Pipeline** ✓
- Streaming backup pipeline (dump → compress → upload)
- Path generation utilities
- Backup command implementation
- Progress logging

**Phase 6: Restore Workflow** ✓
- Restore pipeline (retrieve → decompress → restore)
- List backups command
- "Latest" backup detection
- Full restore command implementation

**Phase 7: Daemon & Scheduler** ✓
- Daemon mode with signal handling (SIGTERM, SIGINT, SIGHUP)
- Cron scheduling integration
- systemd service file example

**Phase 8: Retention & Notifications** ✓
- Backup tracking (JSON metadata in ~/.bared/trackers/)
- Automatic cleanup based on `keep` setting
- Slack notifications with success/failure support

**Phase 9: Polish & Documentation** ✓
- Comprehensive README
- Makefile for common tasks
- Complete example configuration
- Project ready for production use

## Quick Start

### Installation

```bash
go build -o brd ./cmd/brd
```

### Configuration

Create a `bared.yml` configuration file (see `examples/config.example.yml` for full example):

```yaml
default_storage: local_disk

storages:
  local_disk:
    type: local
    path: /backups
    keep: 20

targets:
  - name: my_mysql_db
    conn:
      type: mysql
      user: root
      password: ${MYSQL_PASSWORD}
      database: myapp
      host: localhost
      port: 3306
    compress:
      enabled: true
      type: tgz
    storage:
      enabled: true
      name: local_disk
```

### Usage

```bash
# Validate configuration
./brd validate-config --config bared.yml

# Backup a target
./brd backup --config bared.yml --target my_mysql_db

# List backups
./brd list --config bared.yml --target my_mysql_db

# Restore latest backup
./brd restore --config bared.yml --target my_mysql_db --backup latest

# Restore specific backup
./brd restore --config bared.yml --target my_mysql_db --backup path/to/backup.tar.gz

# Run as daemon with scheduler
./brd daemon --config bared.yml
```

## Architecture

### Pipeline Flow

**Backup:**
```
[Database Dump] → [Compress] → [Storage Upload] → [Cleanup]
         ↓              ↓              ↓
    (streaming via io.Pipe - no temp files)
```

**Restore:**
```
[Storage Retrieve] → [Decompress] → [Database Restore] → [Cleanup]
         ↓                ↓                ↓
    (streaming via io.Pipe)
```

### Key Design Principles

1. **Streaming Architecture**: No temp files between pipeline stages
2. **Minimal Dependencies**: Lightweight, stdlib-first approach
3. **Interface-Driven**: Easy to extend with new databases/storage backends
4. **Retry Logic**: Exponential backoff for network operations
5. **Error Handling**: Graceful partial failure handling

## Project Structure

```
bared/
├── cmd/brd/              # CLI entry point
├── internal/
│   ├── app/              # High-level orchestration
│   ├── config/           # Configuration parsing & validation
│   ├── database/         # Database dumpers (MySQL, Postgres, Redis)
│   ├── storage/          # Storage backends (Local, S3, SFTP)
│   ├── compress/         # Compression (tar.gz)
│   ├── notify/           # Notifications (coming soon)
│   ├── daemon/           # Daemon mode (coming soon)
│   ├── retention/        # Retention policies (coming soon)
│   └── util/             # Utilities (paths, retry, shell)
├── examples/             # Example configurations
└── plan.md               # Detailed implementation plan
```

## Requirements

### Runtime Dependencies

- **MySQL/MariaDB backups**: `mysqldump` and `mysql` commands
- **PostgreSQL backups**: `pg_dump` and `psql` commands
- **Redis backups**: `redis-cli` command

### Go Dependencies

- `gopkg.in/yaml.v3` - YAML configuration parsing
- `github.com/spf13/cobra` - CLI framework
- `github.com/aws/aws-sdk-go-v2` - S3 storage
- `github.com/pkg/sftp` - SFTP storage
- `golang.org/x/crypto/ssh` - SSH for SFTP

## Development

See `plan.md` for the complete implementation plan and architectural details.

### Building

```bash
go build -o brd ./cmd/brd
```

### Testing (Coming in Phase 9)

```bash
go test ./...
```

## License

To be determined

## Contributing

Contributions welcome! Please see the `plan.md` for implementation details and architecture.
