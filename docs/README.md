# BareD Documentation

Welcome to the complete BareD documentation! This guide will help you find exactly what you need, whether you're a new user, an operator deploying in production, a contributor, or an architect exploring the system design.

## 📚 Documentation by Audience

### For Users

Start here if you're new to BareD or want to use it for database backups:

- **[Getting Started](user-guide/getting-started.md)** - Quick start guide to get up and running
- **[Configuration Guide](user-guide/configuration.md)** - How to configure BareD
- **[Web Interface](user-guide/web-interface.md)** - Using the web UI dashboard
- **[Backup Operations](user-guide/backup-operations.md)** - Performing and scheduling backups
- **[Restore Operations](user-guide/restore-operations.md)** - Restoring from backups

### For Operators

Essential guides for deploying and managing BareD in production:

- **[Deployment Guide](operations/deployment.md)** - Deploy BareD in various environments
- **[Docker Deployment](operations/docker.md)** - Docker and Docker Compose setup
- **[Systemd Service](operations/systemd.md)** - Run BareD as a system service
- **[Version Management](operations/versioning.md)** - Working with versions and releases
- **[Monitoring](operations/monitoring.md)** - Monitor BareD operations
- **[Troubleshooting](operations/troubleshooting.md)** - Fix common issues

### For Developers

Resources for contributors and those extending BareD:

- **[Development Setup](development/setup.md)** - Set up your development environment
- **[Tooling Guide](development/tooling.md)** - Development tools and workflows
- **[Testing Strategy](development/testing.md)** - Testing philosophy and practices
- **[System Architecture](development/architecture.md)** - Overview of system design
- **[Contributing](../CONTRIBUTING.md)** - Contribution guidelines

### For Architects

Deep dives into system design and architectural decisions:

- **[Architecture Overview](architecture/README.md)** - System architecture and components
- **[Design Decisions](architecture/design-decisions.md)** - Key architectural choices
- **[Streaming Pipeline](architecture/streaming-pipeline.md)** - How streaming works
- **[Notification System](architecture/notification-system.md)** - Notification architecture
- **[Persistence Layer](architecture/persistence-layer.md)** - Database persistence design
- **[Implementation Plans](architecture/plans/)** - Historical development plans

### API Reference

For developers integrating with BareD programmatically:

- **[REST API](api/endpoints.md)** - HTTP API endpoints
- **[WebSocket API](api/websocket.md)** - Real-time log streaming

## 🎯 Common Tasks

### I want to

**...set up BareD for the first time**
→ Start with [Getting Started](user-guide/getting-started.md), then [Configuration Guide](user-guide/configuration.md)

**...deploy BareD in production**
→ See [Deployment Guide](operations/deployment.md) and [Docker Deployment](operations/docker.md)

**...configure notifications**
→ Check out [../examples/NOTIFICATIONS.md](../examples/NOTIFICATIONS.md) - comprehensive notification setup guide

**...add a new database or storage backend**
→ See [Contributing](../CONTRIBUTING.md) and [Development Setup](development/setup.md)

**...troubleshoot an issue**
→ Start with [Troubleshooting](operations/troubleshooting.md), check logs, verify configuration

**...understand how BareD works internally**
→ Read [System Architecture](development/architecture.md) and [Design Decisions](architecture/design-decisions.md)

**...use the web interface**
→ See [Web Interface Guide](user-guide/web-interface.md)

**...integrate with BareD via API**
→ Check [REST API](api/endpoints.md) and [WebSocket API](api/websocket.md)

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
- Examples are production-ready and tested
- Each guide is self-contained but cross-referenced where helpful

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

- **Configuration issues**: See [Troubleshooting](operations/troubleshooting.md)
- **Feature requests**: Open an issue on GitHub
- **Contributing**: Read [CONTRIBUTING.md](../CONTRIBUTING.md)
- **Questions**: Check existing docs or open a discussion

---

**Happy backing up!** 🎉
