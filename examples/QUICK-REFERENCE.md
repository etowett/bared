# BareD Quick Reference Guide

## Configuration Files

| File | Description |
|------|-------------|
| `config.example.yml` | Complete example with all features |
| `config.notifications-all.yml` | All three notification types together |
| `config.notifications-email.yml` | Email-only with multiple SMTP providers |
| `config.notifications-webhook.yml` | Webhook-only with various integrations |
| `config.persistence.yml` | Persistence configuration and usage |
| `NOTIFICATIONS.md` | Comprehensive notification guide |
| `QUICK-REFERENCE.md` | This file - quick reference |

---

## Persistence Configuration

```yaml
persistence:
  enabled: true      # Enable/disable persistence
  type: sqlite3      # Database type (only sqlite3 currently)
  dsn: bared.db      # Database file path
```

**Features**:
- Job history survives restarts
- Historical log retrieval
- Distributed locking for multi-instance
- ~10MB RAM usage + disk storage

**Recommended Paths**:
- Linux: `/var/lib/bared/bared.db`
- macOS: `/usr/local/var/bared/bared.db`
- Docker: `/data/bared.db` (with volume mount)

---

## Notification Types

### 1. Slack

```yaml
notifiers:
  slack-alerts:
    type: slack
    url: https://hooks.slack.com/services/T00/B00/XXX
    on_success: false  # true = all events, false = failures only
```

**Setup**: Create webhook at https://api.slack.com/apps

### 2. Email (SMTP)

```yaml
notifiers:
  email-admin:
    type: email
    on_success: true
    smtp_host: smtp.gmail.com      # SMTP server
    smtp_port: 587                 # Port (587=TLS, 465=SSL, 25=plain)
    smtp_username: user@gmail.com  # SMTP username
    smtp_password: ${SMTP_PASSWORD}
    smtp_from: backups@company.com
    smtp_to:
      - admin@company.com
      - ops@company.com
    smtp_use_tls: true             # Enable TLS
```

**Common SMTP Servers**:
| Provider | Host | Port | TLS | Notes |
|----------|------|------|-----|-------|
| Gmail | smtp.gmail.com | 587 | Yes | Use App Password |
| Office 365 | smtp.office365.com | 587 | Yes | Email + password |
| SendGrid | smtp.sendgrid.net | 587 | Yes | API key as password |
| Mailgun | smtp.mailgun.org | 587 | Yes | SMTP credentials |

### 3. Webhook

```yaml
notifiers:
  webhook-api:
    type: webhook
    url: https://api.example.com/webhooks
    on_success: true
    webhook_method: POST           # Optional, default: POST
    webhook_headers:               # Optional
      Content-Type: application/json
    webhook_auth:                  # Optional
      type: bearer                 # bearer, basic, or header
      token: ${WEBHOOK_TOKEN}
```

**Authentication Types**:

**Bearer Token**:
```yaml
webhook_auth:
  type: bearer
  token: ${TOKEN}
```

**Basic Auth**:
```yaml
webhook_auth:
  type: basic
  username: user
  password: ${PASS}
```

**Custom Header**:
```yaml
webhook_auth:
  type: header
  header_name: X-API-Key
  header_value: ${KEY}
```

---

## Notification Content

All notifications include:

**Basic Info**:
- Target name
- Operation type (backup/restore)
- Status (success/failure)
- Duration
- Timestamp
- Error (if failed)

**Size Metrics** (backups):
- Compressed size
- Uncompressed size
- Compression ratio %

**Storage Details**:
- Storage name
- Storage type
- Full path

**Database Details**:
- Database name
- Database type

**Context**:
- Manual vs Scheduled
- Cron schedule
- Dry-run flag

**Stages**:
- Stage name
- Duration
- Status

---

## Environment Variables

Use environment variables for sensitive data:

```bash
# Export variables
export SMTP_PASSWORD="your-password"
export WEBHOOK_TOKEN="your-token"
export AWS_ACCESS_KEY_ID="your-key"
export AWS_SECRET_ACCESS_KEY="your-secret"

# In config file
smtp_password: ${SMTP_PASSWORD}
webhook_auth:
  token: ${WEBHOOK_TOKEN}
```

---

## CLI Commands

### Validate Configuration
```bash
./bin/brd validate-config --config config.yml
```

### Start Daemon
```bash
# Development
./bin/brd daemon --config config.yml

# Production (with systemd)
sudo systemctl start bared
sudo systemctl enable bared
```

### Manual Backup (via API)
```bash
curl -X POST http://localhost:8080/api/backups \
  -H "Content-Type: application/json" \
  -d '{"target": "mysql-prod"}'
```

### View Jobs
```bash
curl http://localhost:8080/api/jobs

# With filters
curl "http://localhost:8080/api/jobs?target=mysql-prod&status=completed"
```

### View Job Logs
```bash
curl http://localhost:8080/api/jobs/{job-id}/logs
```

### Stream Logs (Real-time)
```bash
curl -N http://localhost:8080/api/jobs/{job-id}/logs/stream
```

---

## Common Patterns

### Multi-Channel Notifications

```yaml
notifiers:
  # Quick alerts for team
  slack-team:
    type: slack
    url: ${SLACK_URL}
    on_success: false

  # Detailed reports for admins
  email-admin:
    type: email
    on_success: true
    smtp_host: smtp.gmail.com
    smtp_port: 587
    smtp_username: ${SMTP_USER}
    smtp_password: ${SMTP_PASS}
    smtp_from: backups@company.com
    smtp_to: [admin@company.com]
    smtp_use_tls: true

  # Monitoring system integration
  webhook-datadog:
    type: webhook
    url: https://api.datadoghq.com/api/v1/events
    on_success: true
    webhook_auth:
      type: header
      header_name: DD-API-KEY
      header_value: ${DATADOG_KEY}
```

### Development vs Production

**Development** (verbose notifications):
```yaml
notifiers:
  email-dev:
    type: email
    on_success: true  # All events
    smtp_host: smtp.mailtrap.io
    # ... test SMTP config
```

**Production** (failures only):
```yaml
notifiers:
  slack-ops:
    type: slack
    url: ${SLACK_URL}
    on_success: false  # Failures only

  email-oncall:
    type: email
    on_success: false  # Failures only
    # ... production SMTP
```

---

## Troubleshooting

### Check Logs
```bash
# View all logs
tail -f /var/log/bared.log

# Notification-related only
grep -i "notif" /var/log/bared.log

# Errors only
grep -i "error" /var/log/bared.log
```

### Test Notifications

**Slack**:
```bash
curl -X POST https://hooks.slack.com/services/YOUR/WEBHOOK \
  -H 'Content-Type: application/json' \
  -d '{"text": "Test"}'
```

**Email** (manual SMTP test):
```bash
openssl s_client -starttls smtp -connect smtp.gmail.com:587
# Then: EHLO localhost
```

**Webhook**:
```bash
curl -X POST https://your-webhook-url \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"test": "data"}'
```

### Common Issues

**Email Not Received**:
1. Check spam folder
2. Use App Password for Gmail (not regular password)
3. Verify SMTP host and port
4. Check firewall (allow outbound 587/465)
5. Try `smtp_use_tls: false` for local servers

**Webhook Failing**:
1. Verify URL is accessible
2. Check authentication credentials
3. Ensure endpoint returns 2xx
4. Test manually with curl
5. Check endpoint logs

**Slack Not Working**:
1. Verify webhook URL is active
2. Test with curl
3. Check app permissions
4. Review BareD logs

---

## Best Practices

### Security
- ✅ Use environment variables for secrets
- ✅ Enable TLS for SMTP (`smtp_use_tls: true`)
- ✅ Restrict `on_success` to reduce noise
- ✅ Use dedicated service accounts
- ✅ Rotate credentials regularly

### Performance
- ✅ Limit email recipients
- ✅ Use `on_success: false` for high-frequency backups
- ✅ Enable persistence for production
- ✅ Monitor notification delivery

### Organization
- ✅ Use descriptive notifier names
- ✅ Group notifiers by purpose
- ✅ Document custom webhook formats
- ✅ Keep example configs for team

---

## Quick Setup Checklist

- [ ] Configure persistence (optional but recommended)
- [ ] Set up storage backend
- [ ] Configure at least one notifier
- [ ] Set environment variables for secrets
- [ ] Validate config: `brd validate-config`
- [ ] Test notifications with manual backup
- [ ] Configure target schedules
- [ ] Start daemon
- [ ] Monitor logs for first few runs
- [ ] Verify notifications received
- [ ] Set up systemd service (production)
- [ ] Configure log rotation
- [ ] Document custom integrations

---

## Support Resources

- **Example Configs**: `/examples/*.yml`
- **Notification Guide**: `/examples/NOTIFICATIONS.md`
- **Logs**: `grep -i notif /var/log/bared.log`
- **Config Validation**: `./bin/brd validate-config`
- **API Docs**: `http://localhost:8080/api/`

For issues:
1. Check logs
2. Validate config
3. Test manually (curl)
4. Review NOTIFICATIONS.md
5. Open GitHub issue with redacted logs
