# BareD Configuration Examples

This directory contains comprehensive configuration examples and documentation for BareD.

## 📁 Files Overview

### Configuration Examples

| File | Description | Use Case |
|------|-------------|----------|
| **config.example.yml** | Complete configuration example | Start here for full feature overview |
| **config.notifications-all.yml** | All notification types together | Multi-channel alerting setup |
| **config.notifications-email.yml** | Email-only notifications | Multiple SMTP provider examples |
| **config.notifications-webhook.yml** | Webhook-only notifications | Various integration examples |
| **config.persistence.yml** | Persistence configuration | Job history and distributed locking |
| **restore-config.example.yml** | Restore-specific configuration | Database restoration setup |

### Documentation

| File | Description | When to Read |
|------|-------------|--------------|
| **QUICK-REFERENCE.md** | Quick reference guide | Quick lookup of config options |
| **NOTIFICATIONS.md** | Comprehensive notification guide | Setting up alerts and integrations |

### Service Files

| File | Description |
|------|-------------|
| **bared.service** | systemd service file for Linux |

---

## 🚀 Quick Start

### 1. Choose Your Starting Point

**Just Starting?** → Use `config.example.yml`
```bash
cp config.example.yml config.yml
```

**Need Notifications?** → Check out:
- `config.notifications-all.yml` - All three types (Slack, Email, Webhook)
- `config.notifications-email.yml` - Email-only with multiple providers
- `config.notifications-webhook.yml` - Webhooks for various platforms

**Production Deployment?** → Use `config.persistence.yml`
```bash
# Combines persistence + notifications for production
cp config.persistence.yml config.yml
```

### 2. Configure Secrets

Use environment variables for sensitive data:

```bash
# Create .env file
cat > .env << 'EOF'
# AWS Credentials
export AWS_ACCESS_KEY_ID="your-key"
export AWS_SECRET_ACCESS_KEY="your-secret"

# Database Passwords
export MYSQL_PASSWORD="your-mysql-password"
export POSTGRES_PASSWORD="your-postgres-password"

# Notification Credentials
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK"
export SMTP_PASSWORD="your-smtp-password"
export WEBHOOK_TOKEN="your-webhook-token"
EOF

# Load environment
source .env
```

### 3. Validate Configuration

```bash
./bin/brd validate-config --config config.yml
```

### 4. Test Run

```bash
# Start daemon
./bin/brd daemon --config config.yml

# Trigger manual backup (in another terminal)
curl -X POST http://localhost:8080/api/backups \
  -H "Content-Type: application/json" \
  -d '{"target": "your-target-name"}'
```

---

## 📚 Documentation Guide

### For First-Time Setup

1. Read **QUICK-REFERENCE.md** - Get familiar with config structure
2. Copy **config.example.yml** - Start with complete example
3. Read **NOTIFICATIONS.md** (sections you need) - Set up alerts
4. Validate and test your configuration

### For Notification Setup

1. Read **NOTIFICATIONS.md** - Comprehensive guide with:
   - Setup instructions for Slack, Email, Webhook
   - SMTP server settings for popular providers
   - Authentication examples
   - Troubleshooting guide
   - Integration examples (Datadog, PagerDuty, etc.)

2. Choose example config:
   - **All types**: `config.notifications-all.yml`
   - **Email focus**: `config.notifications-email.yml`
   - **Webhook focus**: `config.notifications-webhook.yml`

3. Use **QUICK-REFERENCE.md** for quick syntax lookup

### For Production Deployment

1. Use **config.persistence.yml** as base
2. Review persistence section in **QUICK-REFERENCE.md**
3. Set up systemd service using **bared.service**
4. Configure monitoring with webhooks (see **NOTIFICATIONS.md**)

---

## 🔔 Notification Features

BareD supports three notification types:

### 1. **Slack** 💬
- Rich formatted messages
- Compression metrics
- Stage timelines
- Color-coded (success/failure)

**Quick Setup**:
```yaml
notifiers:
  slack-team:
    type: slack
    url: ${SLACK_WEBHOOK_URL}
    on_success: false  # Only failures
```

### 2. **Email** 📧
- HTML formatted emails
- Beautiful responsive design
- Multiple recipients
- Works with Gmail, Office 365, SendGrid, etc.

**Quick Setup**:
```yaml
notifiers:
  email-admin:
    type: email
    on_success: true
    smtp_host: smtp.gmail.com
    smtp_port: 587
    smtp_username: backups@gmail.com
    smtp_password: ${SMTP_PASSWORD}
    smtp_from: backups@gmail.com
    smtp_to: [admin@company.com]
    smtp_use_tls: true
```

### 3. **Webhook** 🔗
- JSON payloads with complete metrics
- Flexible authentication (Bearer, Basic, Custom)
- Works with Zapier, n8n, Datadog, etc.

**Quick Setup**:
```yaml
notifiers:
  webhook-api:
    type: webhook
    url: https://api.example.com/events
    on_success: true
    webhook_auth:
      type: bearer
      token: ${WEBHOOK_TOKEN}
```

**See NOTIFICATIONS.md for complete setup guides!**

---

## 💾 Persistence Features

Enable persistence for:
- ✅ Job history survives daemon restarts
- ✅ Historical log retrieval via API
- ✅ Distributed locking (multi-instance deployments)
- ✅ Audit trail of all operations

**Quick Setup**:
```yaml
persistence:
  enabled: true
  type: sqlite3
  dsn: /var/lib/bared/bared.db
```

**See config.persistence.yml for detailed guide!**

---

## 📝 What's Included in Notifications

All notifications now include:

### Basic Information
- Target name
- Operation type (backup/restore)
- Success/failure status
- Duration and timestamp
- Error messages (on failure)

### Size Metrics
- Compressed file size
- Uncompressed size
- Compression ratio percentage

### Storage Details
- Storage backend name
- Storage type (s3, local, sftp)
- Full backup path

### Database Details
- Database name
- Database type (mysql, postgres, etc.)

### Operation Context
- **Manual vs Scheduled** indicator
- Cron schedule (for scheduled jobs)
- Dry-run flag (for restore operations)

### Stage Timeline
Each operation stage with duration:
- VALIDATING
- DUMPING
- COMPRESSING
- UPLOADING
- (or RETRIEVING, DECOMPRESSING, RESTORING for restores)

**This is new!** All these details were added in the recent enhancement.

---

## 🔧 Configuration Tips

### Environment Variables

Always use environment variables for secrets:

```yaml
# Good ✅
smtp_password: ${SMTP_PASSWORD}
access_key_id: ${AWS_ACCESS_KEY_ID}

# Bad ❌
smtp_password: "my-actual-password"
access_key_id: "AKIAIOSFODNN7EXAMPLE"
```

### Notification Strategy

**High-frequency backups** (hourly):
```yaml
on_success: false  # Only notify on failures
```

**Critical databases** (daily):
```yaml
on_success: true   # Notify on all events
```

**Multiple channels**:
```yaml
notifiers:
  slack-team:
    on_success: false  # Failures → Slack for quick alerts

  email-admins:
    on_success: true   # All events → Email for audit trail

  webhook-monitoring:
    on_success: true   # All events → Monitoring system
```

### Production Best Practices

1. **Enable Persistence**:
   ```yaml
   persistence:
     enabled: true
     dsn: /var/lib/bared/bared.db
   ```

2. **Use Multiple Notifiers**:
   - Slack for quick team alerts
   - Email for detailed reports
   - Webhook for monitoring integration

3. **Set Up Systemd Service**:
   ```bash
   sudo cp bared.service /etc/systemd/system/
   sudo systemctl enable bared
   sudo systemctl start bared
   ```

4. **Monitor the Monitor**:
   - Use webhooks to send to monitoring system
   - Alert on notification failures
   - Regular config validation

5. **Backup the Backup Database**:
   ```bash
   # If using persistence
   sqlite3 /var/lib/bared/bared.db ".backup /backups/bared.db.backup"
   ```

---

## 🐛 Troubleshooting

### Configuration Issues

```bash
# Validate configuration
./bin/brd validate-config --config config.yml

# Check for syntax errors
yamllint config.yml
```

### Notification Issues

**Not receiving notifications?**

1. Check BareD logs:
   ```bash
   grep -i "notif" /var/log/bared.log
   ```

2. Test manually:
   ```bash
   # Slack
   curl -X POST $SLACK_WEBHOOK_URL \
     -H 'Content-Type: application/json' \
     -d '{"text": "Test"}'

   # Webhook
   curl -X POST $WEBHOOK_URL \
     -H "Authorization: Bearer $TOKEN" \
     -d '{"test": "data"}'
   ```

3. See **NOTIFICATIONS.md** → Troubleshooting section

### Email Issues

**Gmail**: Use App Password, not regular password
**Office 365**: Ensure account isn't blocked
**TLS Errors**: Try `smtp_use_tls: false` for local servers

**See NOTIFICATIONS.md for complete troubleshooting guide!**

---

## 📖 Additional Resources

### In This Directory
- `NOTIFICATIONS.md` - Complete notification setup guide (15+ pages)
- `QUICK-REFERENCE.md` - Quick config reference (concise)
- `config.*.yml` - Example configurations (copy and modify)

### Online Resources
- GitHub Repository: [Issues and Discussions]
- API Documentation: `http://localhost:8080/api/`

### Command Line Help
```bash
# General help
./bin/brd --help

# Command-specific help
./bin/brd daemon --help
./bin/brd backup --help
./bin/brd restore --help
```

---

## 🎯 Common Use Cases

### Case 1: Development Environment
- **File**: `config.example.yml`
- **Notifications**: Email only (all events)
- **Persistence**: Optional
- **Storage**: Local filesystem

### Case 2: Production Single Instance
- **File**: `config.persistence.yml`
- **Notifications**: Slack (failures) + Email (all)
- **Persistence**: Enabled
- **Storage**: S3

### Case 3: Production Multi-Instance
- **File**: `config.persistence.yml`
- **Notifications**: Webhook to monitoring + Email
- **Persistence**: Enabled (shared database)
- **Storage**: S3 or SFTP

### Case 4: Monitoring Integration
- **File**: `config.notifications-webhook.yml`
- **Notifications**: Webhooks to Datadog, PagerDuty, etc.
- **Persistence**: Enabled
- **Storage**: S3

---

## 🚦 Getting Help

1. **Quick Questions**: Check `QUICK-REFERENCE.md`
2. **Notification Setup**: Read `NOTIFICATIONS.md`
3. **Config Issues**: Validate with `brd validate-config`
4. **Logs**: `grep -i error /var/log/bared.log`
5. **Testing**: Try manual backup via API
6. **Still Stuck**: Open GitHub issue with:
   - Redacted config
   - Relevant log entries
   - What you've tried

---

## 📋 Configuration Checklist

- [ ] Copy example config
- [ ] Set environment variables
- [ ] Configure storage backend
- [ ] Set up at least one notifier
- [ ] Enable persistence (recommended)
- [ ] Configure backup targets
- [ ] Set schedules
- [ ] Validate configuration
- [ ] Test manual backup
- [ ] Verify notifications received
- [ ] Set up systemd service (production)
- [ ] Configure log rotation
- [ ] Document team-specific setup

---

**Ready to start?** Copy `config.example.yml` and follow the comments!

**Need help?** Read `QUICK-REFERENCE.md` for fast answers or `NOTIFICATIONS.md` for comprehensive guides!
