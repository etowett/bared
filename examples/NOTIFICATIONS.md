# BareD Notification Guide

This guide covers all notification options available in BareD, including setup, configuration, and troubleshooting.

## Table of Contents

- [Overview](#overview)
- [Notification Types](#notification-types)
  - [Slack](#slack-notifications)
  - [Email (SMTP)](#email-notifications-smtp)
  - [Webhook](#webhook-notifications)
- [Configuration Examples](#configuration-examples)
- [Notification Content](#notification-content)
- [Testing Notifications](#testing-notifications)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

---

## Overview

BareD supports three types of notifications:

1. **Slack** - Rich formatted messages to Slack channels
2. **Email** - HTML formatted emails via SMTP
3. **Webhook** - JSON payloads to custom HTTP endpoints

All notifiers support:
- ✅ Success and failure notifications (configurable)
- ✅ Rich metrics (compression ratios, storage details, database info)
- ✅ Stage timelines showing operation progress
- ✅ Manual vs scheduled backup indicators
- ✅ Both backup and restore notifications
- ✅ Automatic retry logic (3 attempts with exponential backoff)

---

## Notification Types

### Slack Notifications

#### Setup

1. **Create a Slack Webhook**:
   - Go to https://api.slack.com/apps
   - Create a new app or select existing
   - Navigate to "Incoming Webhooks"
   - Click "Add New Webhook to Workspace"
   - Select the channel and authorize
   - Copy the webhook URL

2. **Configure BareD**:
   ```yaml
   notifiers:
     slack-alerts:
       type: slack
       url: https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX
       on_success: false  # true = notify on success, false = only failures
   ```

#### Features

Slack messages include:
- Operation type (Backup/Restore) with success/failure indicator
- Target name and duration
- Trigger information (Manual or Scheduled with cron expression)
- **Size metrics**: Uncompressed → Compressed with reduction percentage
- **Storage details**: Name, type, and path
- **Database details**: Name and type
- **Stage timeline**: Duration for each operation stage (VALIDATING, DUMPING, etc.)

**Example Success Message**:
```
✓ Backup Successful
Target: `mysql-prod`
Duration: 2m 15s
Trigger: Scheduled (0 2 * * *)

Size: 1.2 GB (uncompressed) → 320 MB (compressed)
Compression: 73.3% reduction

Storage: s3-backups (s3)
Path: `backups/mysql-prod/backup-2025-12-15.tar.gz`

Database: mysql-prod (mysql)

Time: 2025-12-15 02:00:15

Stages:
  • VALIDATING: 5s
  • DUMPING: 1m 30s
  • COMPRESSING: 35s
  • UPLOADING: 5s
```

**Example Failure Message**:
```
✗ Backup Failed
Target: `postgres-analytics`
Error: Failed to connect to database: connection refused
Trigger: Scheduled (0 3 * * *)

Database: postgres-analytics (postgres)
Storage: local-disk (local)
Duration: 15s
Time: 2025-12-15 03:00:10

Stages:
  ✗ VALIDATING: 15s
```

---

### Email Notifications (SMTP)

#### Setup

1. **Obtain SMTP Credentials**:
   - **Gmail**: Use App Password (not your regular password)
     - Enable 2FA on your Google account
     - Go to https://myaccount.google.com/apppasswords
     - Generate app password for "Mail"
   - **Office 365**: Use your email and password
   - **Custom Server**: Contact your IT administrator

2. **Configure BareD**:
   ```yaml
   notifiers:
     email-admin:
       type: email
       on_success: true
       smtp_host: smtp.gmail.com
       smtp_port: 587
       smtp_username: backups@company.com
       smtp_password: ${SMTP_PASSWORD}
       smtp_from: backups@company.com
       smtp_to:
         - admin@company.com
         - ops-team@company.com
       smtp_use_tls: true
   ```

3. **Set Environment Variable**:
   ```bash
   export SMTP_PASSWORD="your-app-password-here"
   ```

#### SMTP Server Settings

| Provider | Host | Port | TLS | Notes |
|----------|------|------|-----|-------|
| Gmail | smtp.gmail.com | 587 | Yes | Requires App Password |
| Office 365 | smtp.office365.com | 587 | Yes | Use email + password |
| Outlook.com | smtp-mail.outlook.com | 587 | Yes | Use email + password |
| Yahoo | smtp.mail.yahoo.com | 587 | Yes | Requires App Password |
| SendGrid | smtp.sendgrid.net | 587 | Yes | API key as password |
| Mailgun | smtp.mailgun.org | 587 | Yes | Use SMTP credentials |
| Custom | mail.example.com | 25/587/465 | Varies | Contact admin |

#### Features

Email notifications include:
- **HTML formatting** with beautiful responsive design
- **Color-coded headers**: Green for success, red for failures
- **Organized sections**:
  - Summary (target, duration, trigger, timestamp)
  - Size metrics (with compression details)
  - Storage information
  - Database information
  - Validation results (for restores)
  - Stage timeline
- **Mobile-friendly** layout
- **Automatic retries** (3 attempts with exponential backoff)

---

### Webhook Notifications

#### Setup

1. **Prepare Webhook Endpoint**:
   - Your endpoint should accept POST requests
   - Should return 2xx status code on success
   - Should handle JSON payload

2. **Configure BareD**:
   ```yaml
   notifiers:
     webhook-monitoring:
       type: webhook
       url: https://api.example.com/v1/events
       on_success: true
       webhook_method: POST
       webhook_headers:
         Content-Type: application/json
       webhook_auth:
         type: bearer
         token: ${WEBHOOK_TOKEN}
   ```

#### Authentication Types

**Bearer Token** (JWT, API tokens):
```yaml
webhook_auth:
  type: bearer
  token: ${WEBHOOK_TOKEN}
```

**Basic Authentication**:
```yaml
webhook_auth:
  type: basic
  username: webhook-user
  password: ${WEBHOOK_PASSWORD}
```

**Custom Header** (API keys):
```yaml
webhook_auth:
  type: header
  header_name: X-API-Key
  header_value: ${API_KEY}
```

#### JSON Payload Format

```json
{
  "event": "backup.success",
  "timestamp": "2025-12-15T02:00:15Z",
  "target": "mysql-prod",
  "operation": "backup",
  "status": "success",
  "duration_seconds": 135.5,
  "manual": false,
  "scheduled_by": "0 2 * * *",
  "dry_run": false,
  "backup": {
    "path": "backups/mysql-prod/backup.tar.gz",
    "size": 335544320,
    "uncompressed_size": 1288490188,
    "compression_ratio": 73.3
  },
  "database": {
    "name": "mysql-prod",
    "type": "mysql"
  },
  "storage": {
    "name": "s3-backups",
    "type": "s3",
    "path": "s3://bucket/backups/mysql-prod/backup.tar.gz"
  },
  "stages": [
    {
      "name": "VALIDATING",
      "status": "completed",
      "duration_seconds": 5
    },
    {
      "name": "DUMPING",
      "status": "completed",
      "duration_seconds": 90
    },
    {
      "name": "COMPRESSING",
      "status": "completed",
      "duration_seconds": 35
    },
    {
      "name": "UPLOADING",
      "status": "completed",
      "duration_seconds": 5
    }
  ]
}
```

**Event Types**:
- `backup.success` - Successful backup
- `backup.failure` - Failed backup
- `restore.success` - Successful restore
- `restore.failure` - Failed restore

---

## Configuration Examples

### Multiple Notifiers

You can configure multiple notifiers to receive notifications simultaneously:

```yaml
notifiers:
  # Slack for quick team alerts (failures only)
  slack-team:
    type: slack
    url: ${SLACK_WEBHOOK_URL}
    on_success: false

  # Email for detailed reports (all events)
  email-admins:
    type: email
    on_success: true
    smtp_host: smtp.gmail.com
    smtp_port: 587
    smtp_username: ${SMTP_USER}
    smtp_password: ${SMTP_PASSWORD}
    smtp_from: backups@company.com
    smtp_to:
      - admin@company.com
    smtp_use_tls: true

  # Webhook to monitoring system (all events)
  webhook-datadog:
    type: webhook
    url: https://api.datadoghq.com/api/v1/events
    on_success: true
    webhook_auth:
      type: header
      header_name: DD-API-KEY
      header_value: ${DATADOG_API_KEY}
```

### Gmail with App Password

```yaml
notifiers:
  gmail-alerts:
    type: email
    on_success: false  # Only failures
    smtp_host: smtp.gmail.com
    smtp_port: 587
    smtp_username: your-email@gmail.com
    smtp_password: ${GMAIL_APP_PASSWORD}  # 16-character app password
    smtp_from: your-email@gmail.com
    smtp_to:
      - recipient@example.com
    smtp_use_tls: true
```

### Webhook for Zapier/n8n/Make

```yaml
notifiers:
  zapier-workflow:
    type: webhook
    url: https://hooks.zapier.com/hooks/catch/123456/abcdef/
    on_success: true
    webhook_method: POST
```

### Webhook with Custom Headers

```yaml
notifiers:
  custom-api:
    type: webhook
    url: https://api.internal.com/backup-events
    on_success: true
    webhook_headers:
      Content-Type: application/json
      X-Service-Name: bared
      X-Environment: production
      X-Team: platform
    webhook_auth:
      type: bearer
      token: ${INTERNAL_API_TOKEN}
```

---

## Notification Content

### What's Included

All notifications (Slack, Email, Webhook) include:

#### Basic Information
- Target name
- Operation type (backup or restore)
- Status (success or failure)
- Duration
- Timestamp
- Error message (on failure)

#### Size Metrics (Backups)
- Compressed size
- Uncompressed size (if compression enabled)
- Compression ratio percentage

#### Storage Details
- Storage backend name
- Storage type (s3, local, sftp)
- Full path to backup file

#### Database Details
- Database name
- Database type (mysql, postgres, redis)

#### Operation Context
- Manual vs Scheduled
- Cron schedule (for scheduled jobs)
- Dry-run indicator (for restore operations)

#### Stage Timeline
Each operation stage with:
- Stage name (VALIDATING, DUMPING, COMPRESSING, UPLOADING, etc.)
- Duration
- Status (completed or failed)

#### Restore-Specific
- Validation checks performed
- Number of validations passed
- Backup file size

---

## Testing Notifications

### Test via API

```bash
# Trigger a manual backup (will send notifications)
curl -X POST http://localhost:8080/api/jobs/backup \
  -H "Content-Type: application/json" \
  -d '{"target": "your-target-name"}'
```

### Test Email Configuration

```bash
# Use a test SMTP service
notifiers:
  test-email:
    type: email
    smtp_host: smtp.mailtrap.io  # Test SMTP service
    smtp_port: 587
    smtp_username: ${MAILTRAP_USER}
    smtp_password: ${MAILTRAP_PASS}
    smtp_from: test@example.com
    smtp_to:
      - your-email@example.com
    smtp_use_tls: true
```

### Validate Configuration

```bash
# Validate config file
./bin/brd validate-config --config config.yml

# Check logs for notification errors
tail -f /var/log/bared.log | grep -i notif
```

---

## Troubleshooting

### Slack Notifications Not Received

**Problem**: Messages not appearing in Slack

**Solutions**:
1. Verify webhook URL is correct and active
2. Check Slack app permissions
3. Test webhook manually:
   ```bash
   curl -X POST https://hooks.slack.com/services/YOUR/WEBHOOK/URL \
     -H 'Content-Type: application/json' \
     -d '{"text": "Test message"}'
   ```
4. Check BareD logs for errors:
   ```bash
   grep "slack" /var/log/bared.log
   ```

### Email Notifications Not Received

**Problem**: Emails not being delivered

**Solutions**:

1. **Authentication Failed**:
   - Gmail: Use App Password, not regular password
   - Enable "Less secure app access" (not recommended) or use App Password
   - Office 365: Ensure account is not blocked

2. **Connection Timeout**:
   - Check firewall rules (allow outbound port 587)
   - Verify SMTP host and port are correct
   - Try different port (587 for TLS, 465 for SSL, 25 for plain)

3. **TLS Errors**:
   - Set `smtp_use_tls: false` for local/internal SMTP servers
   - Verify server supports TLS on the specified port

4. **Emails in Spam**:
   - Check spam folder
   - Configure SPF/DKIM records for your domain
   - Use a verified from address

5. **Test Credentials**:
   ```bash
   # Test SMTP connection manually
   openssl s_client -starttls smtp -connect smtp.gmail.com:587
   # Then type: EHLO localhost
   ```

### Webhook Notifications Failing

**Problem**: Webhook endpoint not receiving data

**Solutions**:

1. **Check Endpoint**:
   - Verify URL is accessible from BareD server
   - Ensure endpoint returns 2xx status code
   - Check endpoint logs for incoming requests

2. **Authentication Issues**:
   - Verify token/credentials are correct
   - Check if token has expired
   - Ensure auth headers are properly formatted

3. **Network Issues**:
   - Check firewall rules
   - Verify DNS resolution
   - Test endpoint manually:
     ```bash
     curl -X POST https://your-webhook-url \
       -H "Content-Type: application/json" \
       -H "Authorization: Bearer YOUR_TOKEN" \
       -d '{"test": "message"}'
     ```

4. **Timeout Issues**:
   - Default timeout is 10 seconds
   - Ensure endpoint responds within timeout
   - Check for network latency

### Check Logs

```bash
# View all notification-related logs
grep -i "notif" /var/log/bared.log

# View only errors
grep -i "notification.*error" /var/log/bared.log

# Follow logs in real-time
tail -f /var/log/bared.log | grep -i notif
```

---

## Best Practices

### Security

1. **Use Environment Variables** for sensitive data:
   ```yaml
   smtp_password: ${SMTP_PASSWORD}
   webhook_auth:
     token: ${WEBHOOK_TOKEN}
   ```

2. **Enable TLS** for email:
   ```yaml
   smtp_use_tls: true
   ```

3. **Restrict on_success** to reduce notification volume:
   ```yaml
   # For high-frequency backups, only notify on failures
   on_success: false
   ```

4. **Use dedicated service accounts** for SMTP and webhooks

5. **Rotate credentials regularly**

### Performance

1. **Limit recipients** for email notifications
2. **Use webhook batching** if sending to analytics systems
3. **Set appropriate on_success flags** to avoid notification fatigue

### Monitoring

1. **Monitor notification delivery**:
   - Check BareD logs regularly
   - Set up alerts for notification failures
   - Use webhook for monitoring system integration

2. **Test notifications** after configuration changes

3. **Keep backup of notification configs**

### Organization

1. **Use descriptive notifier names**:
   ```yaml
   notifiers:
     slack-ops-team:      # Clear purpose
     email-oncall:        # Clear audience
     webhook-datadog:     # Clear destination
   ```

2. **Group notifiers by purpose**:
   - Development notifications
   - Production alerts
   - Monitoring integrations

3. **Document custom webhook formats** for team reference

---

## Integration Examples

### Datadog

```yaml
webhook-datadog:
  type: webhook
  url: https://api.datadoghq.com/api/v1/events
  on_success: true
  webhook_headers:
    Content-Type: application/json
  webhook_auth:
    type: header
    header_name: DD-API-KEY
    header_value: ${DATADOG_API_KEY}
```

### PagerDuty (via webhook)

```yaml
webhook-pagerduty:
  type: webhook
  url: https://events.pagerduty.com/v2/enqueue
  on_success: false  # Only alert on failures
  webhook_headers:
    Content-Type: application/json
  webhook_auth:
    type: header
    header_name: Authorization
    header_value: Token token=${PAGERDUTY_TOKEN}
```

### Microsoft Teams (via webhook)

```yaml
webhook-teams:
  type: webhook
  url: https://outlook.office.com/webhook/YOUR/WEBHOOK/URL
  on_success: true
  webhook_method: POST
```

### Slack Alternative (Direct API)

```yaml
webhook-slack-api:
  type: webhook
  url: https://slack.com/api/chat.postMessage
  on_success: true
  webhook_headers:
    Content-Type: application/json
  webhook_auth:
    type: bearer
    token: ${SLACK_BOT_TOKEN}
```

---

## Support

For issues or questions:
1. Check logs: `grep -i notif /var/log/bared.log`
2. Validate config: `./bin/brd validate-config --config config.yml`
3. Test manually using curl examples above
4. Open an issue on GitHub with logs and configuration (redact sensitive data)
