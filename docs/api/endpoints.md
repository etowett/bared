# REST API Endpoints

Complete reference for BareD's HTTP REST API endpoints.

## Table of Contents

- [Health Check](#health-check)
- [Session](#session)
- [Dashboard](#dashboard)
- [Targets](#targets)
- [Restore Targets](#restore-targets)
- [Configuration Management](#configuration-management)
  - [Storages](#storages)
  - [Notifiers](#notifiers)
  - [Targets Config](#targets-config)
  - [Restore Targets Config](#restore-targets-config)
  - [Global Config](#global-config)
  - [Config Utilities](#config-utilities)
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

## Session

### Log In

Validates credentials and issues a session cookie.

**Endpoint**: `POST /api/login`

**Authentication**: None (public endpoint)

**Request**:

```json
{
  "username": "admin",
  "password": "your-password"
}
```

**Response** (`200 OK`), plus a `Set-Cookie: bared_session=...` header:

```json
{
  "username": "admin"
}
```

**Errors**: `400` malformed body · `401` invalid credentials (the message never
distinguishes a bad username from a bad password) · `413` body too large ·
`503` no credentials configured on the daemon.

**Example**:

```bash
curl -c cookies.txt -X POST http://localhost:8080/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-password"}'
```

---

### Log Out

Revokes the session server-side and clears the cookie. Any WebSocket log streams
belonging to the session are closed.

**Endpoint**: `POST /api/logout`

**Authentication**: None (safe to call with an already-expired session)

**Response** (`200 OK`):

```json
{
  "message": "Logged out"
}
```

---

### Current User

Reports the authenticated identity. The dashboard uses this as its auth check,
since an `httpOnly` cookie is invisible to JavaScript.

**Endpoint**: `GET /api/me`

**Authentication**: Required (Basic Auth)

**Response** (`200 OK`):

```json
{
  "username": "admin"
}
```

Returns `401` when unauthenticated, and clears a stale session cookie if one was
presented.


---

## Dashboard

Get dashboard statistics: job counts, per-target health, and success-rate rollups.

**Endpoint**: `GET /api/dashboard`

**Authentication**: Required (Basic Auth)

**Response**:

```json
{
  "targets": [
    {
      "name": "mysql-prod",
      "type": "mysql",
      "database": "production",
      "schedule": "0 2 * * *",
      "is_running": false,
      "last_backup": "2025-12-15T02:04:11Z",
      "next_scheduled": "2025-12-16T02:00:00Z",
      "last_backup_status": "success",
      "consecutive_failures": 0,
      "last_backup_bytes": 41943040,
      "last_backup_duration_seconds": 251.4,
      "overdue": false
    }
  ],
  "active_jobs": 2,
  "total_jobs": 150,
  "success_rate_24h": 92.3,
  "success_rate_7d": 97.6,
  "failed_jobs_24h": 1
}
```

**Fields**:

| Field | Meaning |
|---|---|
| `targets[]` | One `TargetSummary` per configured target — the same shape `/api/targets` returns. |
| `active_jobs` | Jobs currently queued or running. |
| `total_jobs` | Jobs known to the daemon (memory plus persisted history). |
| `total_storage_bytes` | Bytes currently held across storage backends. **Always absent** — see below. |
| `success_rate_24h` | Percentage (0-100) of backup jobs that finished in the last 24 hours and succeeded. Absent when history cannot cover the window — see below. |
| `success_rate_7d` | The same over 7 days. |
| `failed_jobs_24h` | Backup jobs that failed in the last 24 hours. Absent under the same conditions as `success_rate_24h`. |

**TargetSummary fields**:

| Field | Meaning |
|---|---|
| `last_backup` | Completion time of the most recent **successful** backup. |
| `next_scheduled` | Next cron fire time, computed from `schedule`. |
| `last_backup_status` | `success`, `failed`, `never`, or `unknown` — the outcome of the backup job that **finished** most recently (not the one created most recently; overlapping runs can finish out of order). Cancelled jobs count as neither. `unknown` means the daemon could not establish the target's history and is not a claim about backups — see below. |
| `consecutive_failures` | Failed backup jobs since the last successful one. `0` when `last_backup_status` is `unknown`. |
| `last_backup_bytes` | Artifact size recorded by the last successful backup. |
| `last_backup_duration_seconds` | How long that backup job ran. |
| `overdue` | A scheduled run has come due since the last successful backup, and the grace period on top of it — one schedule period, capped at an hour — has passed too. The grace exists because a backup is due from the moment its slot opens until it finishes, so flagging the instant it comes due would mark a healthy target late for the duration of its own run; the cap stops a daily or yearly target from inheriting a daily or yearly grace. Always `false` while `is_running` is true, for targets with no schedule, for targets with no job history at all — nothing records when a target was configured — and whenever the history behind it could not be read. |

**Absent fields are unknown, not zero.** Every optional field above is omitted
when the daemon cannot establish it. Clients must render an omitted value as
unknown or unavailable; showing `0` would report a healthy-looking number the
backend never claimed.

Several fields are deliberately conservative:

- **`total_storage_bytes` is never populated.** Job history records the size of
  every backup ever taken, including the ones retention has since deleted, so
  summing it would overstate usage; listing every storage backend on each
  dashboard request would be slow and, on S3, billable. The field stays absent
  until something tracks retained bytes directly.
- **A window the daemon has not observed is omitted, not estimated.** Rates and
  counts are computed from a bounded scan of backup history. Whenever that scan
  cannot cover the window — the job store was unreachable, the scan hit its row
  cap after the window started, or the daemon has no store and has been up for
  less than the window, or longer than the daemon keeps in-memory history — the
  figure is omitted. Without a job store, `success_rate_7d` is never reported:
  in-memory history is pruned at 72 hours, whatever the uptime.
- **`unknown` is not `never`.** When the daemon cannot read a target's history
  it says so. Reporting a target it cannot see as one that has never been backed
  up would turn a persistence outage into a dashboard full of healthy, brand-new
  targets — the one answer a backup dashboard must never give.

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
      "database": "production",
      "schedule": "0 2 * * *",
      "is_running": false,
      "last_backup": "2025-12-15T02:04:11Z",
      "next_scheduled": "2025-12-16T02:00:00Z",
      "last_backup_status": "success",
      "consecutive_failures": 0,
      "last_backup_bytes": 41943040,
      "last_backup_duration_seconds": 251.4,
      "overdue": false
    }
  ],
  "total": 1
}
```

Targets are rendered by the same code path as
[`/api/dashboard`](#dashboard); see that section for what each field means and
for the rule that absent fields are unknown rather than zero.

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

## Configuration Management

Manage BareD configuration dynamically through the API. These endpoints require database-backed configuration to be enabled.

**Note**: Configuration management is only available when running with database-backed config. When using YAML-only configuration, these endpoints will return appropriate error messages.

### Storages

#### List Storages

Get all configured storage backends.

**Endpoint**: `GET /api/config/storages`

**Authentication**: Required (Basic Auth)

**Response**:

```json
{
  "storages": [
    {
      "name": "s3-backups",
      "type": "s3",
      "enabled": true,
      "config": {
        "bucket": "my-backups",
        "region": "us-east-1",
        "endpoint": "",
        "access_key_id": "***REDACTED***",
        "secret_access_key": "***REDACTED***"
      },
      "keep": 30
    },
    {
      "name": "local-disk",
      "type": "local",
      "enabled": true,
      "config": {
        "path": "/var/backups"
      },
      "keep": 20
    }
  ],
  "source": "database",
  "total": 2
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/config/storages
```

---

#### Get Storage

Get details of a specific storage backend.

**Endpoint**: `GET /api/config/storages/{name}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Storage name |

**Response**:

```json
{
  "name": "s3-backups",
  "type": "s3",
  "enabled": true,
  "config": {
    "bucket": "my-backups",
    "region": "us-east-1",
    "endpoint": "",
    "access_key_id": "***REDACTED***",
    "secret_access_key": "***REDACTED***"
  },
  "keep": 30
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/config/storages/s3-backups
```

---

#### Create Storage

Create a new storage backend.

**Endpoint**: `POST /api/config/storages`

**Authentication**: Required (Basic Auth)

**Request Body**:

```json
{
  "name": "s3-backups",
  "type": "s3",
  "enabled": true,
  "config": {
    "bucket": "my-backups",
    "region": "us-east-1",
    "access_key_id": "AKIAIOSFODNN7EXAMPLE",
    "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  },
  "keep": 30
}
```

**Config Fields by Type**:

**Local Storage**:
- `path` (string, required) - Local directory path

**S3 Storage**:
- `bucket` (string, required) - S3 bucket name
- `region` (string, required) - AWS region
- `endpoint` (string, optional) - Custom endpoint for S3-compatible services
- `access_key_id` (string, required) - AWS access key
- `secret_access_key` (string, required) - AWS secret key

**SFTP Storage**:
- `host` (string, required) - SFTP server hostname
- `port` (integer, required) - SFTP port (usually 22)
- `user` (string, required) - Username
- `password` (string, optional) - Password (if not using key auth)
- `private_key` (string, optional) - Private key path
- `path` (string, required) - Remote directory path

**Response**:

```json
{
  "message": "Storage created successfully"
}
```

**Status Codes**:

- `200` - Storage created
- `400` - Invalid request (validation error)
- `409` - Storage name already exists
- `500` - Internal error

**Example**:

```bash
curl -u admin:password -X POST \
  http://localhost:8080/api/config/storages \
  -H "Content-Type: application/json" \
  -d '{
    "name": "s3-backups",
    "type": "s3",
    "enabled": true,
    "config": {
      "bucket": "my-backups",
      "region": "us-east-1",
      "access_key_id": "AKIAIOSFODNN7EXAMPLE",
      "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    },
    "keep": 30
  }'
```

---

#### Update Storage

Update an existing storage backend.

**Endpoint**: `PUT /api/config/storages/{name}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Storage name to update |

**Request Body**: Same as Create Storage

**Response**:

```json
{
  "message": "Storage updated successfully"
}
```

**Example**:

```bash
curl -u admin:password -X PUT \
  http://localhost:8080/api/config/storages/s3-backups \
  -H "Content-Type: application/json" \
  -d '{
    "name": "s3-backups",
    "type": "s3",
    "enabled": true,
    "config": {
      "bucket": "my-new-bucket",
      "region": "us-west-2",
      "access_key_id": "AKIAIOSFODNN7EXAMPLE",
      "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    },
    "keep": 60
  }'
```

---

#### Delete Storage

Delete a storage backend.

**Endpoint**: `DELETE /api/config/storages/{name}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Storage name to delete |

**Response**:

```json
{
  "message": "Storage deleted successfully"
}
```

**Example**:

```bash
curl -u admin:password -X DELETE \
  http://localhost:8080/api/config/storages/s3-backups
```

---

### Notifiers

#### List Notifiers

Get all configured notification channels.

**Endpoint**: `GET /api/config/notifiers`

**Authentication**: Required (Basic Auth)

**Response**:

```json
{
  "notifiers": [
    {
      "name": "slack-alerts",
      "type": "slack",
      "enabled": true,
      "on_success": false,
      "config": {
        "webhook_url": "***REDACTED***",
        "channel": "#backups"
      }
    },
    {
      "name": "email-alerts",
      "type": "email",
      "enabled": true,
      "on_success": true,
      "config": {
        "smtp_host": "smtp.example.com",
        "smtp_port": 587,
        "from_email": "backups@example.com",
        "to_email": "admin@example.com",
        "username": "backups@example.com",
        "password": "***REDACTED***"
      }
    }
  ],
  "source": "database",
  "total": 2
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/config/notifiers
```

---

#### Get Notifier

Get details of a specific notifier.

**Endpoint**: `GET /api/config/notifiers/{name}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Notifier name |

**Response**:

```json
{
  "name": "slack-alerts",
  "type": "slack",
  "enabled": true,
  "on_success": false,
  "config": {
    "webhook_url": "***REDACTED***",
    "channel": "#backups"
  }
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/config/notifiers/slack-alerts
```

---

#### Create Notifier

Create a new notification channel.

**Endpoint**: `POST /api/config/notifiers`

**Authentication**: Required (Basic Auth)

**Request Body**:

```json
{
  "name": "slack-alerts",
  "type": "slack",
  "enabled": true,
  "on_success": false,
  "config": {
    "webhook_url": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX",
    "channel": "#backups"
  }
}
```

**Config Fields by Type**:

**Slack**:
- `webhook_url` (string, required) - Slack webhook URL
- `channel` (string, optional) - Channel name (overrides webhook default)

**Email**:
- `smtp_host` (string, required) - SMTP server hostname
- `smtp_port` (integer, required) - SMTP port (usually 587)
- `from_email` (string, required) - From email address
- `to_email` (string, required) - Recipient email address
- `username` (string, required) - SMTP username
- `password` (string, required) - SMTP password

**Webhook**:
- `url` (string, required) - Webhook URL
- `method` (string, optional) - HTTP method (default: POST)
- `headers` (object, optional) - Custom HTTP headers

**Response**:

```json
{
  "message": "Notifier created successfully"
}
```

**Example**:

```bash
curl -u admin:password -X POST \
  http://localhost:8080/api/config/notifiers \
  -H "Content-Type: application/json" \
  -d '{
    "name": "slack-alerts",
    "type": "slack",
    "enabled": true,
    "on_success": false,
    "config": {
      "webhook_url": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX",
      "channel": "#backups"
    }
  }'
```

---

#### Update Notifier

Update an existing notifier.

**Endpoint**: `PUT /api/config/notifiers/{name}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Notifier name to update |

**Request Body**: Same as Create Notifier

**Response**:

```json
{
  "message": "Notifier updated successfully"
}
```

**Example**:

```bash
curl -u admin:password -X PUT \
  http://localhost:8080/api/config/notifiers/slack-alerts \
  -H "Content-Type: application/json" \
  -d '{
    "name": "slack-alerts",
    "type": "slack",
    "enabled": true,
    "on_success": true,
    "config": {
      "webhook_url": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX",
      "channel": "#backup-alerts"
    }
  }'
```

---

#### Delete Notifier

Delete a notifier.

**Endpoint**: `DELETE /api/config/notifiers/{name}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Notifier name to delete |

**Response**:

```json
{
  "message": "Notifier deleted successfully"
}
```

**Example**:

```bash
curl -u admin:password -X DELETE \
  http://localhost:8080/api/config/notifiers/slack-alerts
```

---

### Targets Config

#### List Targets

Get all configured backup targets.

**Endpoint**: `GET /api/config/targets`

**Authentication**: Required (Basic Auth)

**Response**:

```json
{
  "targets": [
    {
      "name": "mysql-prod",
      "enabled": true,
      "connection": {
        "type": "mysql",
        "host": "localhost",
        "port": 3306,
        "user": "backup_user",
        "password": "***REDACTED***",
        "database": "production"
      },
      "schedule": "0 2 * * *",
      "storage_name": "s3-backups",
      "notifier_names": ["slack-alerts"],
      "compress_type": "tgz",
      "exclude_tables": [],
      "additional_args": []
    }
  ],
  "source": "database",
  "total": 1
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/config/targets
```

---

#### Get Target

Get details of a specific backup target.

**Endpoint**: `GET /api/config/targets/{name}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Target name |

**Response**:

```json
{
  "name": "mysql-prod",
  "enabled": true,
  "connection": {
    "type": "mysql",
    "host": "localhost",
    "port": 3306,
    "user": "backup_user",
    "password": "***REDACTED***",
    "database": "production"
  },
  "schedule": "0 2 * * *",
  "storage_name": "s3-backups",
  "notifier_names": ["slack-alerts"],
  "compress_type": "tgz",
  "exclude_tables": [],
  "additional_args": []
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/config/targets/mysql-prod
```

---

#### Create Target

Create a new backup target.

**Endpoint**: `POST /api/config/targets`

**Authentication**: Required (Basic Auth)

**Request Body**:

```json
{
  "name": "mysql-prod",
  "enabled": true,
  "connection": {
    "type": "mysql",
    "host": "localhost",
    "port": 3306,
    "user": "backup_user",
    "password": "secure_password",
    "database": "production"
  },
  "schedule": "0 2 * * *",
  "storage_name": "s3-backups",
  "notifier_names": ["slack-alerts"],
  "compress_type": "tgz",
  "exclude_tables": ["temp_cache"],
  "additional_args": ["--single-transaction"]
}
```

**Connection Fields by Type**:

**MySQL/MariaDB**:
- `type`: "mysql"
- `host` (string, required)
- `port` (integer, required)
- `user` (string, required)
- `password` (string, required)
- `database` (string, required)

**PostgreSQL**:
- `type`: "postgres"
- `host` (string, required)
- `port` (integer, required)
- `user` (string, required)
- `password` (string, required)
- `database` (string, required)

**Redis**:
- `type`: "redis"
- `host` (string, required)
- `port` (integer, required)
- `password` (string, optional)
- `db` (integer, optional)

**Response**:

```json
{
  "message": "Target created successfully"
}
```

**Example**:

```bash
curl -u admin:password -X POST \
  http://localhost:8080/api/config/targets \
  -H "Content-Type: application/json" \
  -d '{
    "name": "mysql-prod",
    "enabled": true,
    "connection": {
      "type": "mysql",
      "host": "localhost",
      "port": 3306,
      "user": "backup_user",
      "password": "secure_password",
      "database": "production"
    },
    "schedule": "0 2 * * *",
    "storage_name": "s3-backups",
    "compress_type": "tgz"
  }'
```

---

#### Update Target

Update an existing backup target.

**Endpoint**: `PUT /api/config/targets/{name}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Target name to update |

**Request Body**: Same as Create Target

**Response**:

```json
{
  "message": "Target updated successfully"
}
```

**Example**:

```bash
curl -u admin:password -X PUT \
  http://localhost:8080/api/config/targets/mysql-prod \
  -H "Content-Type: application/json" \
  -d '{
    "name": "mysql-prod",
    "enabled": true,
    "connection": {
      "type": "mysql",
      "host": "localhost",
      "port": 3306,
      "user": "backup_user",
      "password": "secure_password",
      "database": "production"
    },
    "schedule": "0 3 * * *",
    "storage_name": "s3-backups",
    "compress_type": "tgz"
  }'
```

---

#### Delete Target

Delete a backup target.

**Endpoint**: `DELETE /api/config/targets/{name}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Target name to delete |

**Response**:

```json
{
  "message": "Target deleted successfully"
}
```

**Example**:

```bash
curl -u admin:password -X DELETE \
  http://localhost:8080/api/config/targets/mysql-prod
```

---

### Restore Targets Config

#### List Restore Targets

Get all configured restore targets.

**Endpoint**: `GET /api/config/restore-targets`

**Authentication**: Required (Basic Auth)

**Response**:

```json
{
  "restore_targets": [
    {
      "name": "mysql-staging",
      "enabled": true,
      "connection": {
        "type": "mysql",
        "host": "staging-db.local",
        "port": 3306,
        "user": "restore_user",
        "password": "***REDACTED***",
        "database": "staging"
      },
      "source_target": "mysql-prod",
      "storage_name": "s3-backups",
      "description": "Staging environment for testing"
    }
  ],
  "source": "database",
  "total": 1
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/config/restore-targets
```

---

#### Get Restore Target

Get details of a specific restore target.

**Endpoint**: `GET /api/config/restore-targets/{name}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Restore target name |

**Response**:

```json
{
  "name": "mysql-staging",
  "enabled": true,
  "connection": {
    "type": "mysql",
    "host": "staging-db.local",
    "port": 3306,
    "user": "restore_user",
    "password": "***REDACTED***",
    "database": "staging"
  },
  "source_target": "mysql-prod",
  "storage_name": "s3-backups",
  "description": "Staging environment for testing"
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/config/restore-targets/mysql-staging
```

---

#### Create Restore Target

Create a new restore target.

**Endpoint**: `POST /api/config/restore-targets`

**Authentication**: Required (Basic Auth)

**Request Body**:

```json
{
  "name": "mysql-staging",
  "enabled": true,
  "connection": {
    "type": "mysql",
    "host": "staging-db.local",
    "port": 3306,
    "user": "restore_user",
    "password": "secure_password",
    "database": "staging"
  },
  "source_target": "mysql-prod",
  "storage_name": "s3-backups",
  "description": "Staging environment for testing"
}
```

**Response**:

```json
{
  "message": "Restore target created successfully"
}
```

**Example**:

```bash
curl -u admin:password -X POST \
  http://localhost:8080/api/config/restore-targets \
  -H "Content-Type: application/json" \
  -d '{
    "name": "mysql-staging",
    "enabled": true,
    "connection": {
      "type": "mysql",
      "host": "staging-db.local",
      "port": 3306,
      "user": "restore_user",
      "password": "secure_password",
      "database": "staging"
    },
    "source_target": "mysql-prod",
    "storage_name": "s3-backups"
  }'
```

---

#### Update Restore Target

Update an existing restore target.

**Endpoint**: `PUT /api/config/restore-targets/{name}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Restore target name to update |

**Request Body**: Same as Create Restore Target

**Response**:

```json
{
  "message": "Restore target updated successfully"
}
```

**Example**:

```bash
curl -u admin:password -X PUT \
  http://localhost:8080/api/config/restore-targets/mysql-staging \
  -H "Content-Type: application/json" \
  -d '{
    "name": "mysql-staging",
    "enabled": true,
    "connection": {
      "type": "mysql",
      "host": "new-staging-db.local",
      "port": 3306,
      "user": "restore_user",
      "password": "secure_password",
      "database": "staging"
    },
    "source_target": "mysql-prod",
    "storage_name": "s3-backups"
  }'
```

---

#### Delete Restore Target

Delete a restore target.

**Endpoint**: `DELETE /api/config/restore-targets/{name}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Restore target name to delete |

**Response**:

```json
{
  "message": "Restore target deleted successfully"
}
```

**Example**:

```bash
curl -u admin:password -X DELETE \
  http://localhost:8080/api/config/restore-targets/mysql-staging
```

---

### Global Config

#### Get Global Configuration

Get all global configuration settings.

**Endpoint**: `GET /api/config/global`

**Authentication**: Required (Basic Auth)

**Response**:

```json
{
  "config": {
    "default_storage": "s3-backups",
    "log_level": "info",
    "log_format": "json"
  },
  "source": "database"
}
```

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/config/global
```

---

#### Update Global Configuration Setting

Update a specific global configuration key.

**Endpoint**: `PUT /api/config/global/{key}`

**Authentication**: Required (Basic Auth)

**Path Parameters**:

| Parameter | Type | Description |
|-----------|------|-------------|
| `key` | string | Config key to update |

**Request Body**:

```json
{
  "value": "s3-backups"
}
```

**Supported Keys**:

- `default_storage` (string) - Default storage backend name
- `log_level` (string) - Logging level: debug, info, warn, error
- `log_format` (string) - Log format: text, json

**Response**:

```json
{
  "message": "Global config updated successfully"
}
```

**Example**:

```bash
curl -u admin:password -X PUT \
  http://localhost:8080/api/config/global/default_storage \
  -H "Content-Type: application/json" \
  -d '{"value": "s3-backups"}'
```

---

### Config Utilities

#### Check Configuration Source

Check whether configuration is loaded from database or YAML.

**Endpoint**: `GET /api/config/source`

**Authentication**: Required (Basic Auth)

**Response**:

```json
{
  "source": "database",
  "description": "Configuration loaded from database"
}
```

**Possible Values**:

- `database` - Config loaded from SQLite database
- `yaml` - Config loaded from YAML file
- `unavailable` - Config management not available

**Example**:

```bash
curl -u admin:password http://localhost:8080/api/config/source
```

---

#### Migrate YAML to Database

Import YAML configuration into the database.

**Endpoint**: `POST /api/config/migrate`

**Authentication**: Required (Basic Auth)

**Request Body**:

```json
{
  "yaml_path": "/path/to/bared.yml"
}
```

**Response**:

```json
{
  "message": "Configuration migrated successfully",
  "imported": {
    "storages": 2,
    "notifiers": 1,
    "targets": 3,
    "restore_targets": 1,
    "global_settings": 3
  }
}
```

**Status Codes**:

- `200` - Migration successful
- `400` - Invalid YAML or validation error
- `409` - Database already has configuration
- `500` - Migration failed

**Example**:

```bash
curl -u admin:password -X POST \
  http://localhost:8080/api/config/migrate \
  -H "Content-Type: application/json" \
  -d '{"yaml_path": "/etc/bared/bared.yml"}'
```

**Note**: After successful migration, restart the daemon or use the reload endpoint to apply changes.

---

#### Reload Configuration

Hot reload configuration without restarting the daemon.

**Endpoint**: `POST /api/config/reload`

**Authentication**: Required (Basic Auth)

**Request Body**: None

**Response**:

```json
{
  "message": "Configuration reloaded successfully",
  "source": "database",
  "reloaded": {
    "storages": 2,
    "notifiers": 1,
    "targets": 3,
    "restore_targets": 1,
    "schedules_updated": true
  }
}
```

**What Gets Reloaded**:

- Storage backends
- Notification channels
- Backup targets
- Restore targets
- Global settings
- Cron schedules (jobs are rescheduled automatically)

**Status Codes**:

- `200` - Reload successful
- `400` - Validation error in new configuration
- `500` - Reload failed (daemon continues with old config)

**Example**:

```bash
curl -u admin:password -X POST \
  http://localhost:8080/api/config/reload
```

**Use Case**: After updating configuration through the UI or API, trigger a reload to apply changes immediately without restarting the daemon.

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
| `403` | Forbidden (cross-site request rejected — see Authentication) |
| `404` | Not found (job or target doesn't exist) |
| `405` | Method not allowed |
| `413` | Request body too large |
| `500` | Internal server error |

---

## Authentication

All endpoints except `/api/health`, `/api/login`, and `/api/logout` require
authentication. Two mechanisms are accepted, and the endpoint reference below
marks the requirement as "Basic Auth" for historical reasons — a session cookie
works everywhere Basic Auth does.

### HTTP Basic Auth (CLI and API clients)

```bash
# Using curl
curl -u username:password http://localhost:8080/api/endpoint

# Or with explicit header
curl -H "Authorization: Basic $(echo -n username:password | base64)" \
  http://localhost:8080/api/endpoint
```

### Session cookie (web dashboard)

`POST /api/login` returns an `httpOnly; SameSite=Strict; Path=/` cookie holding
an opaque, server-issued token. The browser attaches it to every subsequent
request, including the WebSocket handshake — where a browser cannot set an
`Authorization` header. See [Session](#session).

Because a cookie is attached ambiently, cookie-authenticated **state-changing**
requests (`POST`/`PUT`/`PATCH`/`DELETE`) must also carry an `Origin` header that
matches the server or one of `--http-allowed-origin`; otherwise they are rejected
with `403`. Basic-auth clients are exempt.

Session cookies carry `Secure` automatically over TLS. Behind a TLS-terminating
proxy pass `--http-secure-cookies`; `X-Forwarded-Proto` is not trusted, because
it is client-controlled whenever the daemon is directly reachable.

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
