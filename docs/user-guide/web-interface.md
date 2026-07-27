# BareD Web Interface Guide

Complete guide to using the BareD web interface for backup management.

## Overview

The BareD web interface provides a modern, real-time dashboard for monitoring and managing database backups. It includes:

- **Dashboard** - Overview of targets, jobs, and storage
- **Manual Triggers** - Start backups on demand
- **Progress Tracking** - Real-time progress with ETA
- **Log Streaming** - Live logs via WebSocket
- **Job Management** - View history and cancel jobs
- **Configuration Management** - Dynamic config management for storages, notifiers, targets, and restore targets

## Quick Start

### Enable HTTP Server

Add HTTP configuration to your `bared.yml`:

```yaml
http:
  enabled: true
  address: ":8080"
  auth:
    username: "admin"
    password: "your-secure-password-here"
```

Or use CLI flags:

```bash
brd daemon --config bared.yml \
  --http :8080 \
  --http-user admin \
  --http-pass secure-password
```

### Access Web UI

1. Start the daemon: `brd daemon --config bared.yml --http :8080 --http-user admin --http-pass changeme`
2. Open browser: `http://localhost:8080`
3. Login with credentials: `admin` / `changeme`

## Dashboard Features

### Statistics Cards

The dashboard displays four key metrics:

- **Targets** - Number of configured backup targets
- **Active Jobs** - Currently running jobs
- **Total Jobs** - All jobs (completed + failed + running)
- **Storage Used** - Total storage consumed by backups

These stats auto-refresh every 5 seconds.

### Backup Targets

Each target card shows:

- **Name** - Target identifier
- **Type** - Database type (MySQL, PostgreSQL, Redis)
- **Database** - Database name
- **Last Backup** - Timestamp of last successful backup
- **Schedule** - Cron expression for scheduled backups
- **Next Run** - When the next scheduled backup will run
- **Status** - Current state (Idle / Running)

**Actions**:

- Click **"Backup Now"** to trigger manual backup
- Button is disabled if backup is already running

### Job List

The job list shows all backup/restore operations:

**Columns**:

- **ID** - First 8 characters of job UUID
- **Type** - backup or restore
- **Target** - Target name
- **Status** - Current job state with badge:
  - 🔵 Queued - Waiting to start
  - 🔵 Running - Currently executing
  - 🟢 Completed - Successfully finished
  - 🔴 Failed - Encountered error
  - ⚫ Cancelled - User cancelled
- **Progress** - Visual progress bar with percentage
- **Created** - When job was created
- **Duration** - How long job took (or is taking)
- **Actions** - Cancel button for running jobs

**Filters**:

- Filter by status using dropdown menu
- List auto-refreshes every 3 seconds

**Interaction**:

- Click any row to open detailed job view
- Selected row is highlighted in blue

## Job Details Modal

Click any job to see full details:

### Job Information

- **Job ID** - Full UUID
- **Type** - backup or restore
- **Target** - Target name
- **Status** - Current state with badge
- **Manual** - Badge if manually triggered
- **Created** - Creation timestamp
- **Started** - When job started executing
- **Completed** - When job finished
- **Duration** - Total time in seconds
- **Backup Path** - Location of backup file (if completed)

### Progress Section (Running Jobs Only)

Shows detailed progress information:

- **Stage** - Current stage (validating, dumping, compressing, uploading)
- **Percentage** - Overall completion (0-100%)
- **Progress Bar** - Visual indicator
- **Bytes Processed** - Data processed so far
- **Bytes Total** - Total data size
- **ETA** - Estimated time to completion

### Error Section (Failed Jobs Only)

If job failed, displays:

- Full error message
- Stack trace (if available)

### Logs Section

Real-time streaming logs with:

- **Timestamp** - When log entry was created
- **Level** - Severity (ERROR, WARN, INFO, DEBUG)
- **Message** - Log content
- **Color Coding**:
  - 🔴 Red - ERROR
  - 🟡 Yellow - WARN
  - 🔵 Blue - INFO
  - 🟣 Purple - DEBUG

**Features**:

- Auto-scroll to bottom as new logs arrive
- WebSocket connection status indicator:
  - 🟢 Live - Connected and streaming
  - 🟡 Connecting... - Reconnecting
- Connection auto-reconnects if dropped
- Logs are buffered (last 1000 entries)

## Progress Tracking

### How Progress is Calculated

Backup progress is divided into weighted stages:

1. **Validating** (0%) - Quick validation checks
2. **Dumping** (40%) - Database dump to file
3. **Compressing** (30%) - Compress dump file
4. **Uploading** (30%) - Upload to storage backend
5. **Cleanup** (100%) - Final cleanup

### ETA Calculation

- Uses exponential moving average (EMA) for smoothing
- Only displayed after 10% progress (more accurate)
- Updates in real-time as job progresses
- Shows time remaining in human-readable format (e.g., "2h 15m")

### Progress Bar States

- **Normal** - Blue progress bar filling left to right
- **Indeterminate** - Pulsing animation (when size unknown)
- **Completed** - 100% filled, green
- **Failed** - Red with error message

## WebSocket Log Streaming

### Connection Lifecycle

1. **Connect** - Establishes WebSocket on job detail open
2. **Authenticate** - The session cookie rides along on the handshake
3. **Stream** - Receives real-time log entries
4. **Ping** - Keepalive every 30 seconds
5. **Reconnect** - Auto-reconnect with exponential backoff

### Reconnection Strategy

- Initial delay: 1 second
- Max delay: 30 seconds
- Exponential backoff: delay × 2 on each attempt
- Resets to 1 second on successful connection

### Connection Status

- 🟢 **Live** - Actively streaming logs
- 🟡 **Connecting...** - Attempting to reconnect
- Automatically connects/disconnects based on job status

## Job Management

### Triggering Manual Backups

1. Navigate to dashboard
2. Find target card
3. Click **"Backup Now"** button
4. Confirmation appears in job list
5. Job starts immediately (or queues if at max concurrent)

### Cancelling Jobs

1. Click job row to open details
2. Click **"Cancel"** button in Actions column
3. Confirm cancellation
4. Job status changes to "Cancelling"
5. Job gracefully stops and status becomes "Cancelled"

**Note**: Jobs may take time to cancel if in middle of operation (e.g., uploading large file).

### Job History

- All jobs are retained in history
- Filter by status to find specific jobs
- Job list is paginated (if many jobs)
- Completed jobs show final backup path

## Configuration Management

BareD provides comprehensive configuration management through the web interface, enabling dynamic updates without editing YAML files or restarting the daemon.

### Overview

Access configuration management at `/config` in the web interface. The configuration dashboard provides:

- **Storages** - Manage backup storage backends (Local, S3, SFTP)
- **Notifiers** - Configure notification channels (Slack, Email, Webhook)
- **Targets** - Manage backup targets with schedules
- **Restore Targets** - Configure restore destinations
- **Config Source Badge** - Shows whether configs are loaded from Database or YAML

### Configuration Sources

The web interface displays badges indicating the configuration source:

- **DB Badge** - Configuration loaded from database (can be edited via UI)
- **YAML Badge** - Configuration loaded from YAML file (read-only in UI)

When using YAML configuration, edit/delete buttons are disabled with helpful tooltips.

### Storage Management

Navigate to **Configuration → Storages** to manage storage backends.

**Features**:

- List all configured storages with type, path/bucket info, and retention settings
- Create new storage backends
- Edit existing storage configurations
- Delete unused storages
- Enable/disable storages

**Storage Types**:

1. **Local Storage**:
   - Path - Local directory path
   - Keep - Number of backups to retain

2. **S3 Storage**:
   - Bucket - S3 bucket name
   - Region - AWS region
   - Endpoint - Custom endpoint for S3-compatible services (MinIO, DigitalOcean Spaces)
   - Access Key ID - AWS access key (encrypted at rest)
   - Secret Access Key - AWS secret key (encrypted at rest)
   - Keep - Number of backups to retain

3. **SFTP Storage**:
   - Host - SFTP server hostname
   - Port - SFTP port (usually 22)
   - User - Username
   - Password - Password (encrypted at rest) or Private Key
   - Path - Remote directory path
   - Keep - Number of backups to retain

**Secret Fields**:

- Passwords and keys are masked with `***REDACTED***` in API responses
- When editing, leave secret fields blank to keep existing values
- Enter new values only when changing secrets
- All secrets are encrypted using AES-256-GCM

### Notifier Management

Navigate to **Configuration → Notifiers** to manage notification channels.

**Features**:

- List all notifiers with type badges and configuration details
- Create new notification channels
- Edit existing notifiers
- Delete unused notifiers
- Toggle notifications on success vs. failures only

**Notifier Types**:

1. **Slack**:
   - Webhook URL - Slack incoming webhook URL (encrypted at rest)
   - Channel - Override default channel (optional)
   - Notify On Success - Send notifications for successful backups

2. **Email**:
   - SMTP Host - Mail server hostname
   - SMTP Port - Mail server port (usually 587)
   - From Email - Sender email address
   - To Email - Recipient email address
   - Username - SMTP authentication username
   - Password - SMTP password (encrypted at rest)
   - Notify On Success - Send emails for successful backups

3. **Webhook**:
   - URL - Webhook endpoint URL
   - Method - HTTP method (GET, POST, PUT)
   - Headers - Custom HTTP headers (optional)
   - Notify On Success - Trigger webhook for successful backups

### Target Management

Navigate to **Configuration → Targets** to manage backup targets.

**Features**:

- List all backup targets with connection info, storage, and schedule
- Create new backup targets
- Edit existing target configurations
- Delete targets
- View last backup timestamp
- View next scheduled backup time
- Cron schedule builder (simple + advanced modes)

**Target Configuration**:

**Connection Settings** (by database type):

1. **MySQL/MariaDB**:
   - Host - Database server hostname
   - Port - Database port (usually 3306)
   - User - Database user
   - Password - Database password (encrypted at rest)
   - Database - Database name to backup

2. **PostgreSQL**:
   - Host - Database server hostname
   - Port - Database port (usually 5432)
   - User - Database user
   - Password - Database password (encrypted at rest)
   - Database - Database name to backup

3. **Redis**:
   - Host - Redis server hostname
   - Port - Redis port (usually 6379)
   - Password - Redis password if AUTH enabled (encrypted at rest)
   - DB - Redis database number (optional)

**Backup Settings**:

- **Schedule** - Cron expression (e.g., `0 2 * * *` for daily at 2 AM)
  - Use visual cron builder or enter expression manually
  - Leave blank for manual-only backups
- **Storage** - Select storage backend to use
- **Notifiers** - Select notification channels (multiple allowed)
- **Compression** - Enable/disable tar.gz compression
- **Exclude Tables** - List of table names to exclude (optional)
- **Additional Args** - Extra database-specific flags (e.g., `--single-transaction`)
- **Enabled** - Enable/disable target

**Cron Schedule Builder**:

- Simple mode with dropdowns for common schedules
- Advanced mode for custom cron expressions
- Validation with human-readable description
- Common presets: Daily, Weekly, Monthly

### Restore Target Management

Navigate to **Configuration → Restore Targets** to manage restore destinations.

**Features**:

- List all restore targets with source target linkage
- Create new restore destinations
- Edit existing restore targets
- Delete restore targets
- Link to specific source backup target

**Restore Target Configuration**:

- **Connection Settings** - Same as backup targets (host, port, user, password, database)
- **Source Target** - Link to backup target (optional, for filtering backups)
- **Storage** - Storage backend to retrieve backups from
- **Description** - Optional description (e.g., "Staging environment")
- **Enabled** - Enable/disable restore target

**Use Case**:
Define restore destinations (e.g., staging databases) that can receive backups from production targets for testing or development.

### YAML to Database Migration

If you're currently using YAML configuration and want to switch to database-backed configuration:

1. Navigate to **Configuration → Dashboard**
2. Click **"Migrate to Database"** button
3. Confirm migration
4. All configs are imported to database
5. Secrets are encrypted automatically
6. UI shows "DB" badges instead of "YAML"
7. Configuration becomes editable via UI

**Migration Details**:

- Imports: storages, notifiers, targets, restore targets, global settings
- Encrypts: all passwords, keys, tokens, SMTP credentials
- Validates: all configs before importing
- Atomic: migration succeeds or fails completely (no partial state)
- Non-destructive: YAML file remains unchanged

**Post-Migration**:

- Daemon automatically uses database config on next reload
- YAML file is no longer read (database takes precedence)
- Use "Reload Configuration" to apply changes without restart

### Hot Reload

After making configuration changes via the web UI, click **"Reload Configuration"** to apply changes immediately without restarting the daemon.

**What Gets Reloaded**:

- Storage backends
- Notification channels
- Backup targets (new/updated/deleted)
- Restore targets
- Cron schedules (jobs are automatically rescheduled)

**Reload Process**:

1. Click "Reload" button in header or config dashboard
2. Daemon validates new configuration
3. If valid, applies changes and reschedules jobs
4. If invalid, shows error and keeps current config
5. Success/error notification appears

**Use Cases**:

- Change backup schedule without downtime
- Add new storage backend and start using immediately
- Update database credentials without restart
- Enable/disable notification channels on the fly

### Encryption & Security

**Encryption**:

- All secrets are encrypted using AES-256-GCM
- Encryption key from `BARED_ENCRYPTION_KEY` environment variable (recommended)
- Or auto-generated and stored in database (development only)

**Secret Handling**:

- Secrets never appear in API responses (shown as `***REDACTED***`)
- Secrets are decrypted only when needed for backup/restore operations
- Leave secret fields blank when editing to preserve existing values

**Best Practices**:

### Encryption & Security

**Encryption**:

- All secrets are encrypted using AES-256-GCM
- Encryption key from `BARED_ENCRYPTION_KEY` environment variable (recommended)
- Or auto-generated and stored in database (development only)

**Secret Handling**:

- Secrets never appear in API responses (shown as `***REDACTED***`)
- Secrets are decrypted only when needed for backup/restore operations
- Leave secret fields blank when editing to preserve existing values

**Best Practices**:

- Set `BARED_ENCRYPTION_KEY` environment variable in production
- Use 32-byte base64-encoded key: `openssl rand -base64 32`
- Rotate encryption keys periodically (requires re-encrypting secrets)
- Restrict database file permissions: `chmod 600 bared.db`
- Use HTTPS for web interface in production

## Authentication

### Login Flow

1. Access `http://localhost:8080`
2. Enter username and password
3. The server validates them and sets an `httpOnly`, `SameSite=Strict` session
   cookie holding an opaque token
4. Automatic login on subsequent page loads, for as long as the session lives

Your password is never stored in the browser and the cookie is not readable by
JavaScript, so page scripts have no credential to leak.

### Session Management

- Session persists across page reloads
- Session ends on:
  - Manual logout (revoked server-side, closing any live log streams)
  - Reaching `--http-session-ttl` (default 12 hours)
  - A daemon restart — sessions are held in memory
  - Any 401 response, which returns you to the login page

### Logout

1. Click **"Logout"** button in header
2. The session is revoked on the server and the cookie is cleared
3. Redirected to login page — without reloading the whole app

### Security Notes

- Always use HTTPS in production
- Change default password immediately
- Use strong, random passwords (32+ characters)
- Consider putting behind reverse proxy (nginx, Caddy)
- Restrict access with firewall rules

## Docker Deployment

### Basic Deployment

```bash
# docker-compose.yml includes web interface by default
docker-compose up -d

# Access at http://localhost:8080
# Default credentials: admin / changeme
```

### Custom Configuration

```yaml
services:
  bared:
    command: >
      daemon
      --config /etc/bared/bared.yml
      --http :8080
      --http-user your-username
      --http-pass your-secure-password
    ports:
      - "8080:8080"
```

### Environment Variables

```bash
export BARED_HTTP_ADDR=":8080"
export BARED_HTTP_USER="admin"
export BARED_HTTP_PASS="secure-password"
```

## Production Deployment

### Reverse Proxy (nginx)

```nginx
server {
    listen 443 ssl http2;
    server_name backups.example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

### Reverse Proxy (Caddy)

```caddyfile
backups.example.com {
    reverse_proxy localhost:8080
}
```

Caddy automatically handles HTTPS and WebSocket upgrades.

### Security Checklist

- ✅ Use HTTPS (TLS certificate)
- ✅ Change default password
- ✅ Use strong password (32+ chars)
- ✅ Restrict access by IP (firewall)
- ✅ Enable firewall on server
- ✅ Keep BareD updated
- ✅ Monitor logs for suspicious activity
- ✅ Use read-only config file permissions
- ✅ Run as non-root user (default in Docker)
- ✅ Set `stop_grace_period` for graceful shutdown

## API Reference

For programmatic access, see API endpoints:

### Health Check

```bash
curl http://localhost:8080/api/health
```

### Dashboard

```bash
curl -u admin:changeme http://localhost:8080/api/dashboard
```

### List Jobs

```bash
curl -u admin:changeme http://localhost:8080/api/jobs
curl -u admin:changeme http://localhost:8080/api/jobs?status=running
```

### Trigger Backup

```bash
curl -u admin:changeme -X POST \
  http://localhost:8080/api/jobs/backup \
  -H "Content-Type: application/json" \
  -d '{"target": "mydb"}'
```

### Cancel Job

```bash
curl -u admin:changeme -X DELETE \
  http://localhost:8080/api/jobs/{job-id}
```

### Stream Logs (WebSocket)

```bash
# Using websocat
websocat -H "Authorization: Basic $(echo -n admin:changeme | base64)" \
  ws://localhost:8080/api/jobs/{job-id}/logs/stream
```

### Configuration Management

```bash
# List storages
curl -u admin:changeme http://localhost:8080/api/config/storages

# Create storage
curl -u admin:changeme -X POST \
  http://localhost:8080/api/config/storages \
  -H "Content-Type: application/json" \
  -d '{"name": "s3-backups", "type": "s3", "enabled": true, "config": {...}, "keep": 30}'

# Update target
curl -u admin:changeme -X PUT \
  http://localhost:8080/api/config/targets/mysql-prod \
  -H "Content-Type: application/json" \
  -d '{"name": "mysql-prod", "enabled": true, "connection": {...}, "schedule": "0 3 * * *"}'

# Hot reload configuration
curl -u admin:changeme -X POST \
  http://localhost:8080/api/config/reload

# Check config source
curl -u admin:changeme http://localhost:8080/api/config/source
```

For complete API reference, see [API Endpoints Documentation](../api/endpoints.md#configuration-management).

## Troubleshooting

### Web UI Not Loading

1. Check daemon is running: `docker ps` or `ps aux | grep brd`
2. Verify HTTP flags: `--http :8080 --http-user admin --http-pass changeme`
3. Check logs: `docker logs bared` or check daemon output
4. Test health endpoint: `curl http://localhost:8080/api/health`

### Authentication Fails

1. Verify credentials match command-line flags
2. Check for typos in username/password
3. Sign out (which clears the session cookie), or try incognito/private browsing
4. If the daemon was restarted, sessions are gone — sign in again
5. Behind a reverse proxy on a different host or port, pass
   `--http-allowed-origin https://your-dashboard.example` so the origin check
   accepts the dashboard

### WebSocket Disconnects

1. Check network connectivity
2. Verify reverse proxy WebSocket config
3. Check browser console for errors
4. Ensure job is still running (completed jobs close WebSocket)

### Progress Not Updating

1. Verify job is actually running (not queued)
2. Check browser network tab for API calls
3. Verify auto-refresh is working (should see requests every 2-3 seconds)
4. Check daemon logs for errors

### Slow Performance

1. Reduce auto-refresh intervals in code
2. Check database size (large jobs take time)
3. Verify network speed (for uploads)
4. Check CPU/memory usage on server
5. Consider increasing concurrent job limit

## Browser Compatibility

Tested and supported:

- ✅ Chrome/Edge 90+
- ✅ Firefox 88+
- ✅ Safari 14+

Requires:

- JavaScript enabled
- WebSocket support
- Cookies enabled for the dashboard's origin
- Fetch API support

## Mobile Support

The web interface is responsive and works on mobile devices:

- Optimized layouts for small screens
- Touch-friendly buttons
- Readable on phones and tablets

## Performance Notes

- Dashboard queries run every 5 seconds
- Job list refreshes every 3 seconds
- Individual jobs refresh every 2 seconds
- WebSocket keeps one connection per job detail view
- Logs are capped at 1000 entries per job (circular buffer)

## Future Enhancements

Planned features:

- [ ] Dark mode toggle
- [ ] Restore from backup UI (direct restore trigger from job history)
- [ ] Multi-target batch operations (bulk backup triggers)
- [ ] Export job history to CSV
- [ ] Grafana dashboard integration
- [ ] Config change audit logs
- [ ] Backup retention policy visualization

## Support

For issues, questions, or feature requests:

- GitHub Issues: <https://github.com/etowett/bared/issues>
- Documentation: [docs/README.md](../README.md)
- Security issues: report privately — see [SECURITY.md](../../SECURITY.md)

## License

MIT, same as the rest of BareD — see [LICENSE](../../LICENSE).
