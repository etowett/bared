# BareD Documentation Consolidation Plan

## Overview

Consolidate all documentation from multiple locations into a single, logical structure under `docs/` directory. This will create a single source of truth for all documentation and make it easier for users and contributors to find what they need.

## Current Documentation Inventory

### Root Level (8 files)

| File | Size | Purpose | Target Location |
|------|------|---------|-----------------|
| `README.md` | 5.8 KB | Main project overview | **Keep in root** (standard) |
| `CONTRIBUTING.md` | 6.6 KB | Contribution guidelines | **Keep in root** (standard) |
| `TOOLING.md` | 11.6 KB | Development tooling guide | Move to `docs/development/` |
| `DEVELOPMENT.md` | 11.4 KB | Development guide | Move to `docs/development/` |
| `plan.md` | 23.9 KB | Original implementation plan | Move to `docs/architecture/` |
| `IMPLEMENTATION_COMPLETE.md` | 7.5 KB | Completion summary | Move to `docs/architecture/` |
| `RELEASE_UNIFORMITY.md` | 11.2 KB | Release uniformity | Move to `docs/architecture/` |
| `TESTING_PLAN.md` | 23.4 KB | Testing strategy | Move to `docs/development/` |

### docs/ Directory (3 files)

| File | Size | Purpose | Target Location |
|------|------|---------|-----------------|
| `compose.md` | 17 lines | Docker compose quick ref | Merge into deployment guide |
| `VERSIONING.md` | 332 lines | Version management | **Keep**, move to `docs/operations/` |
| `WEB_INTERFACE.md` | 471 lines | Web UI guide | **Keep**, move to `docs/user-guide/` |

### examples/ Directory (3 MD + 5 YAML)

| File | Size | Purpose | Target Location |
|------|------|---------|-----------------|
| `README.md` | 13 KB | Examples navigation | **Keep** (examples should stay) |
| `NOTIFICATIONS.md` | 15.8 KB | Notification setup | **Keep** (configuration guide) |
| `QUICK-REFERENCE.md` | 8.4 KB | Quick config reference | **Keep** (configuration guide) |
| `config.*.yml` | Various | Config examples | **Keep** (examples) |

### Plan File (.claude/plans/)

| File | Size | Purpose | Target Location |
|------|------|---------|-----------------|
| `quiet-munching-snowflake.md` | - | Logging/alerting plan | Archive to `docs/architecture/plans/` |

## Previous Implementation (COMPLETED ✅)

The logging and alerting enhancement has been fully implemented:

- ✅ **Logging**: Stage-based with clear markers and detailed status
- ✅ **Log Persistence**: Store logs in database for historical access
- ✅ **Notification Channels**: Email (SMTP) and Generic Webhook (+ enhanced Slack)
- ✅ **Success Notifications**: Detailed summary with all metrics
- ✅ **Error Notifications**: Detailed info on failures

---

## Proposed Documentation Structure

```
docs/
├── README.md                          # NEW: Navigation hub for all docs
├── user-guide/
│   ├── README.md                      # NEW: User guide overview
│   ├── getting-started.md             # NEW: Quick start guide
│   ├── configuration.md               # NEW: Configuration guide (reference examples/)
│   ├── web-interface.md               # MOVED from docs/WEB_INTERFACE.md
│   ├── backup-operations.md           # NEW: How to perform backups
│   └── restore-operations.md          # NEW: How to perform restores
├── operations/
│   ├── README.md                      # NEW: Operations overview
│   ├── deployment.md                  # NEW: Deployment guide (includes compose.md content)
│   ├── docker.md                      # NEW: Docker-specific deployment
│   ├── systemd.md                     # NEW: Systemd service setup
│   ├── versioning.md                  # MOVED from docs/VERSIONING.md
│   ├── monitoring.md                  # NEW: Monitoring and observability
│   └── troubleshooting.md             # NEW: Common issues and solutions
├── development/
│   ├── README.md                      # NEW: Development overview
│   ├── setup.md                       # MOVED from DEVELOPMENT.md
│   ├── tooling.md                     # MOVED from TOOLING.md
│   ├── testing.md                     # MOVED from TESTING_PLAN.md
│   ├── contributing.md                # SYMLINK to root CONTRIBUTING.md
│   └── architecture.md                # NEW: System architecture overview
├── architecture/
│   ├── README.md                      # NEW: Architecture overview
│   ├── design-decisions.md            # NEW: Key architectural decisions
│   ├── streaming-pipeline.md          # NEW: Streaming architecture details
│   ├── notification-system.md         # NEW: Notification system design
│   ├── persistence-layer.md           # NEW: Persistence implementation
│   ├── original-plan.md               # MOVED from plan.md
│   ├── implementation-complete.md     # MOVED from IMPLEMENTATION_COMPLETE.md
│   ├── release-uniformity.md          # MOVED from RELEASE_UNIFORMITY.md
│   └── plans/
│       └── logging-alerting.md        # ARCHIVED from .claude/plans/
└── api/
    ├── README.md                      # NEW: API overview
    ├── endpoints.md                   # NEW: API endpoints reference
    └── websocket.md                   # NEW: WebSocket streaming docs

examples/                              # STAYS AS IS
├── README.md                          # Navigation for examples
├── NOTIFICATIONS.md                   # Notification setup guide
├── QUICK-REFERENCE.md                 # Quick config reference
└── *.yml                              # Configuration examples

Root Directory (keep minimal)
├── README.md                          # Main entry point
├── CONTRIBUTING.md                    # Contribution guidelines (standard location)
├── LICENSE                            # License file
├── CHANGELOG.md                       # If exists
└── ...source code...
```

## Documentation Categories

### 1. User-Facing Documentation (`docs/user-guide/`)

**Audience**: End users who want to use BareD for backups

**Content**:

- Getting started guide
- Configuration reference
- Web interface usage
- Backup/restore procedures
- Notification setup

**Priority**: High - Most users start here

### 2. Operations Documentation (`docs/operations/`)

**Audience**: DevOps/SRE teams deploying BareD in production

**Content**:

- Deployment strategies (Docker, bare metal, Kubernetes)
- Systemd service configuration
- Version management
- Monitoring and metrics
- Troubleshooting

**Priority**: High - Critical for production usage

### 3. Development Documentation (`docs/development/`)

**Audience**: Contributors and developers extending BareD

**Content**:

- Development environment setup
- Tooling guide
- Testing strategy
- Contributing guidelines (linked)
- Architecture overview

**Priority**: Medium - For contributors

### 4. Architecture Documentation (`docs/architecture/`)

**Audience**: Advanced developers, maintainers, system designers

**Content**:

- Design decisions
- Streaming pipeline architecture
- System components
- Historical implementation plans
- Completed work summaries

**Priority**: Low - Reference material

### 5. API Documentation (`docs/api/`)

**Audience**: Developers integrating with BareD programmatically

**Content**:

- REST API endpoints
- WebSocket streaming
- Authentication
- Examples

**Priority**: Medium - For integration developers

---

## Implementation Phases

### Phase 1: Create Directory Structure (Low Risk)

**Objective**: Create new directory structure without moving files yet

**Actions**:

```bash
mkdir -p docs/user-guide
mkdir -p docs/operations
mkdir -p docs/development
mkdir -p docs/architecture/plans
mkdir -p docs/api
```

**Files to Create**:

- `docs/README.md` - Main navigation hub
- Each subdirectory gets a `README.md` overview

**Risk**: None - Just creating directories

---

### Phase 2: Move Existing Files (Medium Risk)

**Objective**: Relocate existing documentation to new structure

**File Moves**:

```bash
# From root to docs/
git mv TOOLING.md docs/development/tooling.md
git mv DEVELOPMENT.md docs/development/setup.md
git mv TESTING_PLAN.md docs/development/testing.md
git mv plan.md docs/architecture/original-plan.md
git mv IMPLEMENTATION_COMPLETE.md docs/architecture/implementation-complete.md
git mv RELEASE_UNIFORMITY.md docs/architecture/release-uniformity.md

# Within docs/
git mv docs/WEB_INTERFACE.md docs/user-guide/web-interface.md
git mv docs/VERSIONING.md docs/operations/versioning.md

# Archive plan file
cp ~/.claude/plans/quiet-munching-snowflake.md docs/architecture/plans/logging-alerting.md

# Remove old compose.md (will merge content into deployment.md)
git rm docs/compose.md
```

**Update Internal Links**:

- `CONTRIBUTING.md` references to moved files
- `README.md` references to moved files  - Any cross-references between docs

**Risk**: Medium - Links may break if not updated

---

### Phase 3: Create New Documentation Files (Low Risk)

**Objective**: Fill gaps in documentation with new guides

**New Files to Create**:

**User Guide**:

- `docs/user-guide/README.md` - Overview of user documentation
- `docs/user-guide/getting-started.md` - Quick start guide
- `docs/user-guide/configuration.md` - Configuration guide (references examples/)
- `docs/user-guide/backup-operations.md` - Backup procedures
- `docs/user-guide/restore-operations.md` - Restore procedures

**Operations**:

- `docs/operations/README.md` - Operations overview
- `docs/operations/deployment.md` - General deployment (includes compose.md content)
- `docs/operations/docker.md` - Docker-specific deployment
- `docs/operations/systemd.md` - Systemd service configuration
- `docs/operations/monitoring.md` - Monitoring and observability
- `docs/operations/troubleshooting.md` - Common issues

**Development**:

- `docs/development/README.md` - Development overview
- `docs/development/architecture.md` - System architecture

**Architecture**:

- `docs/architecture/README.md` - Architecture overview
- `docs/architecture/design-decisions.md` - Key design decisions
- `docs/architecture/streaming-pipeline.md` - Streaming architecture
- `docs/architecture/notification-system.md` - Notification design
- `docs/architecture/persistence-layer.md` - Persistence implementation

**API**:

- `docs/api/README.md` - API overview
- `docs/api/endpoints.md` - REST API reference
- `docs/api/websocket.md` - WebSocket streaming

**Risk**: Low - New files don't break existing links

---

### Phase 4: Update Root README (High Impact)

**Objective**: Update main README with new documentation structure

**Changes to `README.md`**:

- Add "Documentation" section near the top
- Link to `docs/README.md` as the main docs hub
- Link to `examples/README.md` for configuration
- Keep README focused on project overview and quick start

**Example Documentation Section**:

```markdown
## Documentation

📚 **[Complete Documentation](docs/README.md)** - Full documentation hub

Quick Links:
- [Getting Started](docs/user-guide/getting-started.md) - New to BareD?
- [Configuration Guide](examples/README.md) - Set up your backups
- [Web Interface](docs/user-guide/web-interface.md) - Use the web UI
- [API Reference](docs/api/endpoints.md) - Integrate programmatically
- [Contributing](CONTRIBUTING.md) - Join the project
```

**Risk**: High impact but safe - README is entry point for users

---

### Phase 5: Create Navigation Hubs (High Value)

**Objective**: Create comprehensive README files for each category

**`docs/README.md`** (Main Documentation Hub):

```markdown
# BareD Documentation

## For Users
- [Getting Started](user-guide/getting-started.md) - Quick start guide
- [Configuration](user-guide/configuration.md) - How to configure BareD
- [Web Interface](user-guide/web-interface.md) - Using the web UI
- [Backup Operations](user-guide/backup-operations.md) - Performing backups
- [Restore Operations](user-guide/restore-operations.md) - Restoring from backups

## For Operators
- [Deployment](operations/deployment.md) - Deploy BareD in production
- [Docker](operations/docker.md) - Docker deployment
- [Systemd](operations/systemd.md) - Run as system service
- [Versioning](operations/versioning.md) - Version management
- [Monitoring](operations/monitoring.md) - Monitor BareD
- [Troubleshooting](operations/troubleshooting.md) - Fix common issues

## For Developers
- [Development Setup](development/setup.md) - Set up dev environment
- [Tooling](development/tooling.md) - Development tools
- [Testing](development/testing.md) - Testing strategy
- [Architecture](development/architecture.md) - System design
- [Contributing](../CONTRIBUTING.md) - Contribution guidelines

## For Architects
- [Design Decisions](architecture/design-decisions.md) - Why we built it this way
- [Streaming Pipeline](architecture/streaming-pipeline.md) - How streaming works
- [Notification System](architecture/notification-system.md) - Notification design
- [Persistence Layer](architecture/persistence-layer.md) - Database persistence
- [Implementation Plans](architecture/plans/) - Historical plans

## API Reference
- [REST API](api/endpoints.md) - HTTP endpoints
- [WebSocket](api/websocket.md) - Real-time streaming

## Configuration Examples
See [examples/](../examples/) directory for:
- Complete configuration examples
- Notification setup guides
- Quick reference
```

**Individual Category READMEs**:

- Each subdirectory gets overview with links to docs within
- Consistent structure across all categories
- Clear navigation paths

**Risk**: High value - Makes documentation discoverable

---

## Key Decisions

### What Stays in Root

- `README.md` - Main entry point (industry standard)
- `CONTRIBUTING.md` - Contribution guidelines (GitHub standard)
- `LICENSE` - License file (standard)
- Source code directories (`cmd/`, `internal/`, etc.)

### What Moves to docs/

- All technical documentation
- Development guides
- Operations guides
- Architecture documentation
- Historical implementation plans

### What Stays in examples/

- Configuration examples (YAML files)
- Notification setup guides (NOTIFICATIONS.md)
- Quick reference (QUICK-REFERENCE.md)
- Examples navigation (README.md)

**Rationale**: Configuration examples are closely tied to actual config files and should stay together. Users looking for "how to configure" naturally look in examples/.

### examples/ vs docs/

- `examples/` = Configuration files + setup guides
- `docs/` = Conceptual guides + system documentation
- Cross-reference between them extensively

---

## File Move Summary

### Moves (8 files)

```bash
TOOLING.md                    → docs/development/tooling.md
DEVELOPMENT.md                → docs/development/setup.md
TESTING_PLAN.md               → docs/development/testing.md
plan.md                       → docs/architecture/original-plan.md
IMPLEMENTATION_COMPLETE.md    → docs/architecture/implementation-complete.md
RELEASE_UNIFORMITY.md         → docs/architecture/release-uniformity.md
docs/WEB_INTERFACE.md         → docs/user-guide/web-interface.md
docs/VERSIONING.md            → docs/operations/versioning.md
```

### Removes (1 file)

```bash
docs/compose.md               → Content merged into docs/operations/deployment.md
```

### Archives (1 file)

```bash
.claude/plans/quiet-munching-snowflake.md → docs/architecture/plans/logging-alerting.md
```

### Creates (23+ new files)

- 5 category READMEs (docs/README.md + 4 subdirectories)
- 6 user-guide docs
- 6 operations docs
- 2 development docs
- 5 architecture docs
- 3 API docs

---

## Link Updates Required

### CONTRIBUTING.md

- Update references to DEVELOPMENT.md → `docs/development/setup.md`
- Update references to plan.md → `docs/architecture/original-plan.md`

### README.md (root)

- Add "Documentation" section with links to docs/
- Update any references to moved files

### docs/ Internal Links

- Cross-reference between user-guide, operations, development
- Link to examples/ for configuration
- Link to architecture/ for deep dives

### examples/README.md

- Link to docs/user-guide/ for conceptual guides
- Reference docs/operations/ for deployment

---

## Benefits of This Structure

### For New Users

- Clear entry point: `README.md` → `docs/README.md` → category
- Natural progression: overview → getting started → configuration → operations
- Examples directory dedicated to "how do I configure this?"

### For Operators

- `docs/operations/` has everything for production deployment
- Versioning, Docker, systemd, monitoring all in one place
- Clear troubleshooting guide

### For Contributors

- `docs/development/` has complete dev setup
- Architecture docs for understanding system design
- Historical plans for context on why decisions were made

### For Maintainers

- Single source of truth in `docs/`
- Consistent structure across categories
- Easy to find and update documentation
- Historical context preserved in `architecture/`

### For Documentation

- Logical categorization by audience
- Easy to maintain and extend
- Clear navigation paths
- Avoids duplication

---

## Implementation Checklist

### Phase 1: Directory Structure

- [ ] Create `docs/user-guide/`
- [ ] Create `docs/operations/`
- [ ] Create `docs/development/`
- [ ] Create `docs/architecture/plans/`
- [ ] Create `docs/api/`

### Phase 2: File Moves

- [ ] Move root MD files to appropriate docs/ subdirectories
- [ ] Move existing docs/ files to new structure
- [ ] Archive plan file from .claude/plans/
- [ ] Remove compose.md (merge content first)

### Phase 3: New Documentation

- [ ] Create docs/README.md (main hub)
- [ ] Create category README files (5 files)
- [ ] Create user-guide documentation (6 files)
- [ ] Create operations documentation (6 files)
- [ ] Create development documentation (2 files)
- [ ] Create architecture documentation (5 files)
- [ ] Create API documentation (3 files)

### Phase 4: Updates

- [ ] Update root README.md with docs section
- [ ] Update CONTRIBUTING.md links
- [ ] Update internal doc cross-references
- [ ] Update examples/README.md links

### Phase 5: Validation

- [ ] Verify all links work
- [ ] Check navigation flows
- [ ] Ensure no broken references
- [ ] Test from user perspective

---

## Estimated Effort

- **Phase 1**: 5 minutes (just mkdir)
- **Phase 2**: 30 minutes (git mv + initial link updates)
- **Phase 3**: 4-6 hours (writing new documentation)
- **Phase 4**: 1 hour (updating existing docs)
- **Phase 5**: 30 minutes (validation)

**Total**: ~6-8 hours for complete reorganization

---

## Success Criteria

After implementation:

- ✅ Single source of truth for all documentation in `docs/`
- ✅ Clear categorization by audience (users, operators, developers, architects)
- ✅ Easy navigation with README hubs at each level
- ✅ Root directory clean with only essential files
- ✅ examples/ focused on configuration
- ✅ All internal links working
- ✅ Historical context preserved in architecture/
- ✅ Easy for new contributors to find what they need

---

## Future Enhancements

After consolidation:

- Add docs/ to GitHub Pages for nice web view
- Add search functionality
- Create video tutorials linked from docs
- Add diagrams for architecture docs
- Automated link checking in CI
- Documentation versioning (by release)

---

## Notes

- This is a NON-BREAKING change - just reorganization
- All existing content preserved (just moved)
- Can be done incrementally (phase by phase)
- Easy to roll back if needed (just git revert)
- Will significantly improve documentation discoverability
- Sets foundation for future documentation growth
