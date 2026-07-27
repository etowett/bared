# User Guide

Everything you need to run BareD day to day.

> **Note on coverage.** This section is thin. The web interface guide below is the
> only full-length page here; installation, configuration and backup/restore
> procedures currently live in the root README and in the annotated example configs
> rather than in dedicated pages. The links below point at what actually exists.

## Contents

### [Web Interface](web-interface.md)

Complete guide to using the BareD web dashboard. Monitor backups, trigger manual
runs, view logs in real time, and manage configuration through the browser.

### [Quick Start](../../README.md#quick-start)

Install BareD, write your first `bared.yml`, and run a backup, a list and a restore
from the CLI.

### [Configuration Examples](../../examples/README.md)

The de facto configuration guide: annotated YAML covering every database engine,
storage backend, notifier, scheduling, retention and encryption.

## Quick Links

- **[Configuration Examples](../../examples/)** - Ready-to-use configuration files
- **[Notification Setup](../../examples/NOTIFICATIONS.md)** - Set up Slack, Email, or Webhook notifications
- **[Quick Reference](../../examples/QUICK-REFERENCE.md)** - Quick lookup for configuration syntax
- **[Restore config example](../../examples/restore-config.example.yml)** - Restore target configuration

## Common User Tasks

**First-Time Setup**:

1. [Install BareD](../../README.md#installation)
2. [Create a configuration file](../../examples/config.example.yml)
3. [Run your first backup](../../README.md#usage)

**Production Setup**:

1. [Configure targets and storages](../../examples/README.md)
2. [Set up scheduling](../../README.md#daemon-modes) with a `schedule` field per target
3. [Configure notifications](../../examples/NOTIFICATIONS.md)
4. [Deploy as a service](../../examples/bared.service)
5. [Read the security guidance](../../SECURITY.md)

**Daily Operations**:

- [Trigger a manual backup](../../README.md#usage) (`brd backup`) or use the [web UI](web-interface.md)
- [Check backup status](web-interface.md#dashboard)
- [List available backups](../../README.md#usage) (`brd list`)
- [Restore from a backup](../../README.md#usage) (`brd restore`)

## Need Help?

- **Configuration issues**: run `brd validate-config --config bared.yml`, then check the daemon logs
- **Deployment questions**: check the [Operations Guide](../operations/)
- **Security questions**: see [SECURITY.md](../../SECURITY.md)
- **Anything else**: see the main [Documentation Hub](../README.md) or the [issue tracker](https://github.com/etowett/bared/issues)

---

[← Back to Documentation](../README.md)
