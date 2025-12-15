# User Guide

Welcome to the BareD user guide! This section contains everything you need to get started with BareD and use it effectively for your database backup needs.

## Contents

### [Getting Started](getting-started.md)
Quick start guide to install BareD, create your first configuration, and run your first backup. Perfect for new users.

### [Configuration Guide](configuration.md)
Comprehensive guide to configuring BareD. Learn about all configuration options, environment variables, and best practices.

### [Web Interface](web-interface.md)
Complete guide to using the BareD web dashboard. Monitor backups, trigger manual runs, view logs in real-time, and manage jobs through the browser.

### [Backup Operations](backup-operations.md)
Everything about performing backups: manual vs scheduled, backup strategies, retention policies, and best practices.

### [Restore Operations](restore-operations.md)
How to restore from backups: finding the right backup, validation modes, restore procedures, and recovery strategies.

## Quick Links

- **[Configuration Examples](../../examples/)** - Ready-to-use configuration files
- **[Notification Setup](../../examples/NOTIFICATIONS.md)** - Set up Slack, Email, or Webhook notifications
- **[Quick Reference](../../examples/QUICK-REFERENCE.md)** - Quick lookup for configuration syntax

## Common User Tasks

**First-Time Setup**:
1. [Install BareD](getting-started.md#installation)
2. [Create configuration file](configuration.md#basic-configuration)
3. [Test your first backup](backup-operations.md#manual-backups)

**Production Setup**:
1. [Configure multiple targets](configuration.md#multiple-targets)
2. [Set up scheduling](backup-operations.md#scheduled-backups)
3. [Configure notifications](../../examples/NOTIFICATIONS.md)
4. [Deploy as service](../operations/systemd.md)

**Daily Operations**:
- [Trigger manual backup](backup-operations.md#manual-backups)
- [Check backup status](web-interface.md#dashboard)
- [List available backups](backup-operations.md#listing-backups)
- [Restore from backup](restore-operations.md)

## Need Help?

- **Configuration issues**: See [Troubleshooting](../operations/troubleshooting.md)
- **Deployment questions**: Check [Operations Guide](../operations/)
- **Feature questions**: See main [Documentation Hub](../README.md)

---

[← Back to Documentation](../README.md)
