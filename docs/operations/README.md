# Operations Guide

This section covers deploying, managing, and maintaining BareD in production.

> **Note on coverage.** Dedicated deployment, Docker, monitoring and troubleshooting
> pages do not exist yet. This page carries the checklist and guidance; the links
> below point at the real deployment artifacts in the repository.

## Contents

### [Version Management](versioning.md)

BareD versioning, checking versions, building with version info, and managing releases.

### [Security & Hardening](../../SECURITY.md)

Known security limitations and how to harden a deployment. Read this before going to
production.

### Deployment artifacts

- **[compose.yml](../../compose.yml)** - Docker Compose stack
- **[Dockerfile](../../Dockerfile)** - Container image, including the database client tools
- **[examples/bared.service](../../examples/bared.service)** - systemd unit
- **[examples/config.persistence.yml](../../examples/config.persistence.yml)** - Job history, log persistence and distributed locking

## Production Deployment Checklist

### Prerequisites

- [ ] Database client tools installed (mysqldump, pg_dump, redis-cli)
- [ ] Storage backend accessible (S3, SFTP, or local filesystem)
- [ ] Configuration file prepared and validated
- [ ] Environment variables set for secrets
- [ ] Backup storage location configured

### Deployment Steps

1. Choose a deployment method (Docker, systemd, bare metal)
2. Deploy with [compose.yml](../../compose.yml) or the [systemd unit](../../examples/bared.service)
3. Review [SECURITY.md](../../SECURITY.md) and restrict access to the HTTP interface
4. Test backup **and restore** — see [Usage](../../README.md#usage)
5. [Verify notifications](../../examples/NOTIFICATIONS.md)

### Post-Deployment

- [ ] Monitor first few backup runs
- [ ] Verify backups are being created
- [ ] Test restore procedure
- [ ] Verify retention policy works
- [ ] Confirm notifications are received
- [ ] Document custom configuration
- [ ] Set up monitoring alerts

## Common Scenarios

### Docker Deployment

For most users, Docker provides the simplest deployment:

- Pre-built images with all database clients
- Easy updates and rollbacks
- Isolated from host system
- Works on any platform

**See**: [compose.yml](../../compose.yml) and the [Dockerfile](../../Dockerfile)

### Bare Metal / VM

For maximum performance or specific requirements:

- Direct system access
- Lower resource overhead
- Full control over environment
- Systemd integration

**See**: the [systemd unit](../../examples/bared.service) and [Installation](../../README.md#installation)

### Cloud Platforms

Deploy on AWS, GCP, Azure, DigitalOcean:

- Use cloud-native storage (S3, Cloud Storage)
- Leverage managed databases
- Integrate with cloud monitoring
- Automated backups to cloud storage

**See**: the S3 storage examples in [examples/README.md](../../examples/README.md)

## Monitoring & Maintenance

### What to Monitor

- Backup success/failure rates
- Backup duration trends
- Storage space usage
- Database connection health
- Notification delivery

**See**: the [WebSocket API](../api/websocket.md) for live job and log streaming, the
[REST API](../api/endpoints.md) for job history, and
[examples/NOTIFICATIONS.md](../../examples/NOTIFICATIONS.md) for failure alerts.
There is no Prometheus/metrics endpoint yet.

### Regular Maintenance

- Review and update retention policies
- Test restore procedures monthly
- Update BareD to latest version
- Rotate access credentials
- Audit backup inventory

## Security Best Practices

- ✅ Store secrets in environment variables, not config files
- ✅ Use dedicated backup user accounts with minimal permissions
- ✅ Enable TLS for SMTP and webhook notifications
- ✅ Restrict filesystem permissions on config and backup files
- ✅ Use private networks or VPNs for database connections
- ✅ Regularly rotate access credentials
- ✅ Enable audit logging
- ✅ Review notification recipients periodically

**See [SECURITY.md](../../SECURITY.md)** for the known limitations these practices are
working around — in particular SFTP host key verification, login rate limiting, and
where the encryption key is stored.

## High Availability

For mission-critical deployments:

- Enable persistence for job history
- Run multiple instances with shared storage
- Use distributed locking (built-in with persistence)
- Monitor with external health checks
- Set up failover procedures

**See**: [Persistence Configuration](../../examples/config.persistence.yml)

## Getting Help

- **Deployment issues**: check the daemon logs and `brd validate-config`, then search the [issue tracker](https://github.com/etowett/bared/issues)
- **Configuration questions**: See the [User Guide](../user-guide/) and [examples/](../../examples/)
- **Security issues**: Report privately — see [SECURITY.md](../../SECURITY.md)

---

[← Back to Documentation](../README.md)
