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

- Streaming backup pipeline (dump -> compress -> upload)
- Path generation utilities
- Backup command implementation
- Progress logging

**Phase 6: Restore Workflow** ✓

- Restore pipeline (retrieve -> decompress -> restore)
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
# Build the binary
make build

# Or manually with Go
go build -o bin/brd ./cmd/brd
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
./bin/brd validate-config --config bared.yml

# Backup a target
./bin/brd backup --config bared.yml --target my_mysql_db

# List backups
./bin/brd list --config bared.yml --target my_mysql_db

# Restore latest backup
./bin/brd restore --config bared.yml --target my_mysql_db --backup latest

# Restore specific backup
./bin/brd restore --config bared.yml --target my_mysql_db --backup path/to/backup.tar.gz

# Run as daemon with scheduler
./bin/brd daemon --config bared.yml
```

### Daemon Modes

BareD daemon can run in three distinct modes to fit different operational needs:

#### 1. Cron-only Mode (Scheduled Backups)
Run automated backups on a schedule without HTTP API access:

```bash
./bin/brd daemon --config bared.yml
```

**Requirements:**
- At least one target must have a `schedule` field configured in YAML
- No HTTP server flags needed

**Use case:** Traditional scheduled backups in environments where API access is not needed.

#### 2. API-only Mode (Manual Backups)
Run the HTTP server for manual backups via web UI or API without any scheduled jobs:

```bash
./bin/brd daemon --config bared.yml --http :8080 --http-user admin --http-pass secret
```

**Requirements:**
- `--http` flag must be specified
- Targets do NOT need `schedule` fields (they can be omitted)

**Use case:** On-demand backups triggered manually through the web interface or API calls, ideal for development environments or when you need full control over backup timing.

#### 3. Hybrid Mode (Scheduled + Manual)
Run both scheduled backups AND provide HTTP API access:

```bash
./bin/brd daemon --config bared.yml --http :8080 --http-user admin --http-pass secret
```

**Requirements:**
- `--http` flag specified
- Some targets have `schedule` fields (scheduled), others may omit schedules (API-only)

**Use case:** Production environments where you want automated scheduled backups plus the ability to trigger ad-hoc backups when needed.

**Note:** At least one mode (cron or API) must be active. The daemon will fail with a helpful error if neither schedules nor HTTP server are configured.

## Documentation

📚 **[Complete Documentation Hub](docs/README.md)** - Navigate all documentation by audience and topic

### Quick Links

**For Users**:

- [Getting Started](docs/user-guide/getting-started.md) - New to BareD? Start here
- [Configuration Guide](docs/user-guide/configuration.md) - Configure BareD for your needs
- [Configuration Examples](examples/) - Ready-to-use YAML configs
- [Notification Setup](examples/NOTIFICATIONS.md) - Set up Slack, Email, or Webhooks
- [Web Interface](docs/user-guide/web-interface.md) - Use the web dashboard

**For Operators**:

- [Deployment Guide](docs/operations/deployment.md) - Deploy in production
- [Docker Deployment](docs/operations/docker.md) - Docker and Docker Compose
- [Systemd Service](docs/operations/systemd.md) - Run as a system service
- [Troubleshooting](docs/operations/troubleshooting.md) - Fix common issues

**For Developers**:

- [Development Setup](docs/development/setup.md) - Set up dev environment
- [Contributing Guide](CONTRIBUTING.md) - How to contribute
- [System Architecture](docs/development/architecture.md) - How BareD works

**For Integrators**:

- [REST API](docs/api/endpoints.md) - HTTP API reference
- [WebSocket API](docs/api/websocket.md) - Real-time log streaming

## Architecture

### Pipeline Flow

**Backup:**

```
[Database Dump] -> [Compress] -> [Storage Upload] -> [Cleanup]
         ↓              ↓              ↓
    (streaming via io.Pipe - no temp files)
```

**Restore:**

```
[Storage Retrieve] -> [Decompress] -> [Database Restore] -> [Cleanup]
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
└── docs/                 # Complete documentation
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

See `docs/` for complete documentation including user guides, operations, and architecture details.

### Building

```bash
# Using Makefile (outputs to bin/brd)
make build

# Or manually with Go
go build -o bin/brd ./cmd/brd

# Cross-platform builds (outputs to dist/)
make build-all
```

### Testing (Coming in Phase 9)

```bash
go test ./...
```

## License

To be determined

## Contributing

Contributions welcome! Please see `CONTRIBUTING.md` and `docs/development/` for implementation details and architecture.
