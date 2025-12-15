# REST API Endpoints

Complete reference for BareD's HTTP REST API endpoints.

## Table of Contents

- [Health Check](#health-check)
- [Dashboard](#dashboard)
- [Targets](#targets)
- [Restore Targets](#restore-targets)
- [Jobs](#jobs)
  - [List Jobs](#list-jobs)
  - [Get Job Details](#get-job-details)
  - [Trigger Backup](#trigger-backup)
  - [Trigger Restore](#trigger-restore)
  - [Get Job Logs](#get-job-logs)
  - [Cancel Job](#cancel-job)

---

## Health Check

Check if the API server is running.

**Endpoint**: `GET /api/health`

**Authentication**: None (public endpoint)

**Response**:

```json
{
  "status": "ok",
  "version": "1.0.0"
}
```

**Example**:

```bash
curl http://localhost:8080/api/health
```

---

## Dashboard

Get dashboard statistics including job counts, target information, and recent activity.

**Endpoint**: `GET /api/dashboard`

**Authentication**: Required (Basic Auth)

**Response**:

```json
{
  "targets_count": 5,
  "active_jobs": 2,
  "total_jobs": 150,
  "failed_jobs": 3,
  "last_backup": "2025-12-15T02:00:00Z",
  "jobs": [
    {
      "id": "job-123",
      "type": "backup",
      "target": "mysql-prod",
      "status": "running",
      "created_at": "2025-12-15T02:00:00Z",
      "started_at": "2025-12-15T02:00:01Z"
    }
  ]
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/dashboard
```

---

## Targets

List all configured backup targets.

**Endpoint**: `GET /api/targets`

**Authentication**: Required (Basic Auth)

**Response**:

```json
{
  "targets": [
    {
      "name": "mysql-prod",
      "type": "mysql",
      "schedule": "0 2 * * *",
      "storage": "s3-backups",
      "compression": "tgz",
      "last_backup": "2025-12-15T02:00:00Z",
      "next_backup": "2025-12-16T02:00:00Z"
    }
  ],
  "total": 1
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/targets
```

---

## Restore Targets

List all configured restore targets.

**Endpoint**: `GET /api/restore-targets`

**Authentication**: Required (Basic Auth)

**Response**:

```json
{
  "restore_targets": [
    {
      "name": "mysql-staging",
      "type": "mysql",
      "storage": "s3-backups"
    }
  ],
  "total": 1
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/restore-targets
```

---

## Jobs

### List Jobs

List all jobs with optional filtering.

**Endpoint**: `GET /api/jobs`

**Authentication**: Required (Basic Auth)

**Query Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by status: `pending`, `running`, `completed`, `failed` |
| `target` | string | Filter by target name |
| `type` | string | Filter by type: `backup`, `restore` |
| `limit` | integer | Limit number of results (default: 100) |

**Response**:

```json
{
  "jobs": [
    {
      "id": "job-123",
      "type": "backup",
      "target": "mysql-prod",
      "status": "completed",
      "created_at": "2025-12-15T02:00:00Z",
      "started_at": "2025-12-15T02:00:01Z",
      "completed_at": "2025-12-15T02:02:30Z",
      "duration": 149.5,
      "manual": false,
      "schedule": "0 2 * * *"
    }
  ],
  "total": 1
}
```

**Example**:

```bash
# List all jobs
curl -u admin:password http://localhost:8080/api/jobs

# List failed jobs only
curl -u admin:password "http://localhost:8080/api/jobs?status=failed"

# List jobs for specific target
curl -u admin:password "http://localhost:8080/api/jobs?target=mysql-prod"
```

---

### Get Job Details

Get detailed information about a specific job.

**Endpoint**: `GET /api/jobs/{id}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Job ID (UUID) |

**Response**:

```json
{
  "id": "job-123",
  "type": "backup",
  "target": "mysql-prod",
  "status": "completed",
  "created_at": "2025-12-15T02:00:00Z",
  "started_at": "2025-12-15T02:00:01Z",
  "completed_at": "2025-12-15T02:02:30Z",
  "duration": 149.5,
  "manual": false,
  "schedule": "0 2 * * *",
  "result": {
    "backup_path": "s3://bucket/backups/mysql-prod/backup-2025-12-15.tar.gz",
    "size": 335544320,
    "uncompressed_size": 1288490188,
    "compression_ratio": 73.3
  },
  "stages": [
    {
      "name": "VALIDATING",
      "status": "completed",
      "duration": 5.0
    },
    {
      "name": "DUMPING",
      "status": "completed",
      "duration": 90.0
    }
  ]
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/jobs/job-123
```

---

### Trigger Backup

Manually trigger a backup for a target.

**Endpoint**: `POST /api/jobs/backup`

**Authentication**: Required (Basic Auth)

**Request Body**:

```json
{
  "target": "mysql-prod"
}
```

**Response**:

```json
{
  "job_id": "job-456",
  "message": "Backup job started for target: mysql-prod"
}
```

**Status Codes**:

- `200` - Job created successfully
- `400` - Invalid request (target not found or invalid)
- `401` - Authentication required
- `500` - Internal server error

**Example**:

```bash
curl -u admin:password -X POST \
  http://localhost:8080/api/jobs/backup \
  -H "Content-Type: application/json" \
  -d '{"target": "mysql-prod"}'
```

**Important**: This is the **canonical endpoint** for triggering manual backups. There is no `/api/backups` endpoint.

---

### Trigger Restore

Manually trigger a restore operation.

**Endpoint**: `POST /api/jobs/restore`

**Authentication**: Required (Basic Auth)

**Request Body**:

```json
{
  "target": "mysql-staging",
  "backup_path": "s3://bucket/backups/mysql-prod/backup-2025-12-15.tar.gz",
  "dry_run": false
}
```

**Parameters**:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `target` | string | Yes | Restore target name |
| `backup_path` | string | Yes | Full path to backup file |
| `dry_run` | boolean | No | If true, validate only (default: false) |

**Response**:

```json
{
  "job_id": "job-789",
  "message": "Restore job started for target: mysql-staging"
}
```

**Example**:

```bash
curl -u admin:password -X POST \
  http://localhost:8080/api/jobs/restore \
  -H "Content-Type: application/json" \
  -d '{
    "target": "mysql-staging",
    "backup_path": "s3://bucket/backups/mysql-prod/backup-2025-12-15.tar.gz",
    "dry_run": false
  }'
```

---

### Get Job Logs

Get logs for a specific job (paginated, historical).

**Endpoint**: `GET /api/jobs/{id}/logs`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Job ID (UUID) |

**Query Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `since` | string | ISO 8601 timestamp - only return logs after this time |
| `limit` | integer | Maximum number of log entries (default: 1000) |

**Response**:

```json
{
  "job_id": "job-123",
  "logs": [
    {
      "timestamp": "2025-12-15T02:00:01Z",
      "level": "info",
      "message": "Starting backup for target: mysql-prod",
      "stage": "VALIDATING"
    },
    {
      "timestamp": "2025-12-15T02:00:05Z",
      "level": "info",
      "message": "Validation successful",
      "stage": "VALIDATING"
    }
  ],
  "total": 2
}
```

**Example**:

```bash
# Get all logs
curl -u admin:password http://localhost:8080/api/jobs/job-123/logs

# Get logs since specific time
curl -u admin:password "http://localhost:8080/api/jobs/job-123/logs?since=2025-12-15T02:00:00Z"
```

**Note**: For real-time log streaming, use the WebSocket endpoint instead. See [WebSocket API](websocket.md).

---

### Cancel Job

Cancel a running job.

**Endpoint**: `DELETE /api/jobs/{id}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string | Job ID (UUID) |

**Response**:

```json
{
  "message": "Job cancelled successfully"
}
```

**Example**:

```bash
curl -u admin:password -X DELETE \
  http://localhost:8080/api/jobs/job-123
```

**Note**: Only running jobs can be cancelled. Completed or failed jobs cannot be cancelled.

---

## Error Responses

All endpoints return errors in the following format:

```json
{
  "error": "target not found",
  "details": "no target named 'nonexistent' in configuration"
}
```

**Common Status Codes**:

| Code | Description |
|------|-------------|
| `200` | Success |
| `400` | Bad request (invalid parameters) |
| `401` | Unauthorized (missing or invalid credentials) |
| `404` | Not found (job or target doesn't exist) |
| `405` | Method not allowed |
| `500` | Internal server error |

---

## Authentication

All endpoints (except `/api/health`) require HTTP Basic Authentication:

```bash
# Using curl
curl -u username:password http://localhost:8080/api/endpoint

# Or with explicit header
curl -H "Authorization: Basic $(echo -n username:password | base64)" \
  http://localhost:8080/api/endpoint
```

Configure credentials in `bared.yml`:

```yaml
http:
  enabled: true
  address: ":8080"
  auth:
    username: "admin"
    password: "your-secure-password"
```

Or via CLI flags:

```bash
brd daemon --http :8080 --http-user admin --http-pass password
```

---

## Rate Limiting

The API does not implement rate limiting by default. For production deployments, implement rate limiting at the reverse proxy level (nginx, Caddy, etc.).

---

## CORS

CORS is enabled for all origins by default to support web UI access. Configure your reverse proxy to restrict origins in production.

---

## Further Reading

- **[WebSocket API](websocket.md)** - Real-time log streaming
- **[API Overview](README.md)** - Quick start and examples
- **[Web Interface Guide](../user-guide/web-interface.md)** - Using the web UI

---

[← Back to API Documentation](README.md)
