# Operations Guide

This section contains essential guides for deploying, managing, and maintaining BareD in production environments.

## Contents

### [Deployment Guide](deployment.md)
General deployment strategies for various environments: bare metal, VMs, containers, and cloud platforms.

### [Docker Deployment](docker.md)
Detailed guide for deploying BareD with Docker and Docker Compose, including multi-service setups and best practices.

### [Systemd Service](systemd.md)
Configure BareD as a systemd service for automatic startup, logging, and management on Linux systems.

### [Version Management](versioning.md)
Understanding BareD versioning, checking versions, building with version info, and managing releases.

### [Monitoring](monitoring.md)
Monitor BareD operations, set up alerts, integrate with monitoring systems, and track backup health.

### [Troubleshooting](troubleshooting.md)
Common issues and their solutions, debugging techniques, and how to get help.

## Production Deployment Checklist

### Prerequisites
- [ ] Database client tools installed (mysqldump, pg_dump, redis-cli)
- [ ] Storage backend accessible (S3, SFTP, or local filesystem)
- [ ] Configuration file prepared and validated
- [ ] Environment variables set for secrets
- [ ] Backup storage location configured

### Deployment Steps
1. [Choose deployment method](deployment.md) (Docker, systemd, bare metal)
2. [Install and configure](docker.md) or [set up systemd service](systemd.md)
3. [Configure monitoring](monitoring.md)
4. [Test backup and restore](../user-guide/backup-operations.md)
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

**See**: [Docker Deployment Guide](docker.md)

### Bare Metal / VM
For maximum performance or specific requirements:
- Direct system access
- Lower resource overhead
- Full control over environment
- Systemd integration

**See**: [Deployment Guide](deployment.md) + [Systemd Service](systemd.md)

### Cloud Platforms
Deploy on AWS, GCP, Azure, DigitalOcean:
- Use cloud-native storage (S3, Cloud Storage)
- Leverage managed databases
- Integrate with cloud monitoring
- Automated backups to cloud storage

**See**: [Deployment Guide](deployment.md#cloud-platforms)

## Monitoring & Maintenance

### What to Monitor
- Backup success/failure rates
- Backup duration trends
- Storage space usage
- Database connection health
- Notification delivery

**See**: [Monitoring Guide](monitoring.md)

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

## High Availability

For mission-critical deployments:
- Enable persistence for job history
- Run multiple instances with shared storage
- Use distributed locking (built-in with persistence)
- Monitor with external health checks
- Set up failover procedures

**See**: [Persistence Configuration](../../examples/config.persistence.yml)

## Getting Help

- **Deployment issues**: Check [Troubleshooting](troubleshooting.md)
- **Performance problems**: See [Monitoring](monitoring.md)
- **Configuration questions**: See [User Guide](../user-guide/)

---

[← Back to Documentation](../README.md)
