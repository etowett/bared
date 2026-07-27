# BareD Documentation

Welcome to the complete BareD documentation! This guide will help you find exactly what you need, whether you're a new user, an operator deploying in production, a contributor, or an architect exploring the system design.

## 📚 Documentation by Audience

### For Users

Start here if you're new to BareD or want to use it for database backups:

- **[Quick Start](../README.md#quick-start)** - Install, configure, and run your first backup
- **[Configuration Examples](../examples/README.md)** - Annotated YAML for every database, storage backend and notifier
- **[Quick Reference](../examples/QUICK-REFERENCE.md)** - Config syntax lookup
- **[Notification Setup](../examples/NOTIFICATIONS.md)** - Slack, Email and Webhook setup
- **[Web Interface](user-guide/web-interface.md)** - Using the web UI dashboard

### For Operators

Deploying and running BareD in production:

- **[Security & Hardening](../SECURITY.md)** - Known limitations and deployment guidance — read this first
- **[Docker Compose](../compose.yml)** / **[Dockerfile](../Dockerfile)** - Container deployment
- **[systemd unit](../examples/bared.service)** - Run BareD as a system service
- **[Persistence & HA](../examples/config.persistence.yml)** - Job history and distributed locking
- **[Version Management](operations/versioning.md)** - Working with versions and releases
- **[Operations Guide](operations/README.md)** - Deployment checklist and common scenarios

### For Developers

Resources for contributors and those extending BareD:

- **[Development Setup](development/setup.md)** - Set up your development environment
- **[Tooling Guide](development/tooling.md)** - Development tools and workflows
- **[Testing Strategy](development/testing.md)** - Testing philosophy and practices
- **[Contributing](../CONTRIBUTING.md)** - Contribution guidelines
- **[Agent Guide](../AGENTS.md)** - Architecture map and conventions, for humans and agents alike

### For Architects

Deep dives into system design:

- **[Architecture Overview](architecture/README.md)** - Components, data flow, and design principles
- **[Release Process](architecture/release-process.md)** - How releases are cut
- **[Release Uniformity](architecture/release-uniformity.md)** - Release standardisation
- **[Historical Plans](architecture/plans/)** - Original implementation plans, kept for context

### API Reference

For developers integrating with BareD programmatically:

- **[REST API](api/endpoints.md)** - HTTP API endpoints
- **[WebSocket API](api/websocket.md)** - Real-time log streaming

## 🎯 Common Tasks

### I want to

**...set up BareD for the first time**
→ Start with [Quick Start](../README.md#quick-start), then the [configuration examples](../examples/README.md)

**...deploy BareD in production**
→ Read [SECURITY.md](../SECURITY.md), then use [compose.yml](../compose.yml) or the [systemd unit](../examples/bared.service). The [Operations Guide](operations/README.md) has a deployment checklist.

**...configure notifications**
→ Check out [../examples/NOTIFICATIONS.md](../examples/NOTIFICATIONS.md) - comprehensive notification setup guide

**...add a new database or storage backend**
→ See [Contributing](../CONTRIBUTING.md) and [Development Setup](development/setup.md)

**...troubleshoot an issue**
→ There is no troubleshooting guide yet. Check the daemon logs, re-run with `brd validate-config`, and search the [issue tracker](https://github.com/etowett/bared/issues).

**...understand how BareD works internally**
→ Read the [Architecture Overview](architecture/README.md) and [AGENTS.md](../AGENTS.md)

**...use the web interface**
→ See [Web Interface Guide](user-guide/web-interface.md)

**...manage configuration dynamically via UI**
→ See [Configuration Management](user-guide/web-interface.md#configuration-management) in the Web Interface Guide

**...integrate with BareD via API**
→ Check [REST API](api/endpoints.md) and [WebSocket API](api/websocket.md)

**...migrate from YAML to database-backed config**
→ See [YAML to Database Migration](user-guide/web-interface.md#yaml-to-database-migration)

## 📖 Configuration Examples

Looking for configuration examples? See the **[examples/](../examples/)** directory for:

- Complete configuration examples with all features
- Notification setup guides (Slack, Email, Webhook)
- Quick configuration reference
- Multiple real-world scenarios

## 🚀 Quick Links

- **[Project README](../README.md)** - Main project overview
- **[Contributing Guide](../CONTRIBUTING.md)** - How to contribute
- **[Configuration Examples](../examples/)** - YAML config files and setup guides
- **[GitHub Repository](https://github.com/etowett/bared)** - Source code

## 💡 Documentation Tips

- All documentation uses relative links - works offline and in any viewer
- Code blocks include language hints for syntax highlighting
- Each guide is self-contained but cross-referenced where helpful
- This documentation set is incomplete. Where a topic has no page, the links above
  point at the closest thing that actually exists — usually an annotated example
  config or the code itself.

## 📝 Documentation Structure

```
docs/
├── README.md (you are here)     # Main navigation hub
├── user-guide/                  # For end users
├── operations/                  # For operators/SREs
├── development/                 # For contributors
├── architecture/                # For architects
└── api/                         # For integrators
```

## 🤝 Need Help?

- **Configuration issues**: Check the daemon logs and `brd validate-config`, then search the [issue tracker](https://github.com/etowett/bared/issues)
- **Security issues**: Report privately — see [SECURITY.md](../SECURITY.md)
- **Feature requests**: Open an issue on GitHub
- **Contributing**: Read [CONTRIBUTING.md](../CONTRIBUTING.md)
- **Questions**: Check existing docs or open a discussion

---

**Happy backing up!** 🎉
