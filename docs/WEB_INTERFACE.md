# BareD Web Interface Guide

Complete guide to using the BareD web interface for backup management.

## Overview

The BareD web interface provides a modern, real-time dashboard for monitoring and managing database backups. It includes:

- **Dashboard** - Overview of targets, jobs, and storage
- **Manual Triggers** - Start backups on demand
- **Progress Tracking** - Real-time progress with ETA
- **Log Streaming** - Live logs via WebSocket
- **Job Management** - View history and cancel jobs

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
2. **Authenticate** - Sends Basic Auth token
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

## Authentication

### Login Flow

1. Access `http://localhost:8080`
2. Enter username and password
3. Credentials stored in `sessionStorage`
4. Automatic login on subsequent page loads

### Session Management

- Session persists across page reloads
- Session expires on:
  - Browser close (sessionStorage cleared)
  - 401 Unauthorized response
  - Manual logout

### Logout

1. Click **"Logout"** button in header
2. Credentials cleared from sessionStorage
3. Redirected to login page

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

## Troubleshooting

### Web UI Not Loading

1. Check daemon is running: `docker ps` or `ps aux | grep brd`
2. Verify HTTP flags: `--http :8080 --http-user admin --http-pass changeme`
3. Check logs: `docker logs bared` or check daemon output
4. Test health endpoint: `curl http://localhost:8080/api/health`

### Authentication Fails

1. Verify credentials match command-line flags
2. Check for typos in username/password
3. Clear browser cache and sessionStorage
4. Try incognito/private browsing mode

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
- sessionStorage support
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
- [ ] Email notifications
- [ ] Backup retention policy management
- [ ] Restore from backup UI
- [ ] Multi-target batch operations
- [ ] Export job history to CSV
- [ ] Grafana dashboard integration

## Support

For issues, questions, or feature requests:
- GitHub Issues: https://github.com/yourusername/bared/issues
- Documentation: https://github.com/yourusername/bared/docs

## License

Same as BareD project.
