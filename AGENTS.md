# AGENTS.md — BareD

> **BareD** (Backup and Restore Daemon) — a streaming backup/restore daemon for databases
> (MySQL/MariaDB, PostgreSQL, Redis) written in Go, with a React/TypeScript web dashboard.
>
> This is the **root agent guide**. It gives you the map, the workflow, and the conventions.
> Deep, area-specific detail lives in nested `AGENTS.md` files — **read the one for the area you're
> touching before you edit.** When guidance conflicts, **the innermost (most specific) guide wins.**

This file is the single source of truth for agents; `CLAUDE.md` is a symlink to it.

---

## How to use this guide (documentation tree)

Before editing, load the guide(s) for the area you're working in:

```
AGENTS.md                          ← you are here: overview, workflow, conventions
├─ internal/AGENTS.md              Go backend deep-dive (architecture, API, testing, full-stack recipes)
│  ├─ internal/database/AGENTS.md  ▸ add / change a database engine (Dumper/Restorer)
│  ├─ internal/storage/AGENTS.md   ▸ add / change a storage backend (Local/S3/SFTP)
│  └─ internal/notify/AGENTS.md    ▸ add / change a notification channel
├─ cmd/AGENTS.md                   CLI (`brd`): Cobra commands, config import, build & run
├─ web/AGENTS.md                   React 19 + TS dashboard (state, websocket, components, testing)
│  ├─ web/README.md                frontend overview
│  └─ web/TESTING.md               frontend testing guide
├─ docs/                           long-form docs: api/, architecture/, development/, operations/, user-guide/
├─ CONTRIBUTING.md                 contribution flow, code style, release process
└─ specs/                          spec-driven feature work (research → plan → implementation notes)
```

**Rule of thumb:** touching `internal/storage/` → read `internal/storage/AGENTS.md` **and** `internal/AGENTS.md`.
Touching `web/` → read `web/AGENTS.md`. A full-stack change → read both backend and frontend guides.

---

## Project overview

BareD streams database dumps through a pipeline and never buffers whole datasets in memory.

- **Backup**: dump → compress → (encrypt) → upload to storage → track & cleanup (retention)
- **Restore**: download from storage → (decrypt) → decompress → restore to database
- **Schedule**: run backups on cron schedules in daemon mode
- **Monitor**: web UI + REST API + WebSocket for real-time job monitoring and configuration

### Technology stack

**Backend (Go 1.26)** — stdlib-first, minimal dependencies. Cobra CLI, `net/http` API with
WebSocket, SQLite for job/config persistence. The web UI is embedded into the binary via `go:embed`.

**Frontend (React 19 + TypeScript)** — Vite, TanStack Router + Query (server state), Zustand
(client state), Radix UI + Tailwind CSS, Vitest. Talks to the backend over REST + WebSocket.

### Key files to know

```
cmd/brd/main.go                 # CLI entry point (Cobra commands)
internal/app/                   # high-level backup/restore orchestration
internal/api/server.go          # HTTP API + WebSocket server
internal/daemon/daemon.go       # cron scheduler & signal handling
internal/jobs/manager.go        # job queue & worker pool
internal/config/config.go       # configuration structures
internal/{database,storage,notify}/  # the three pluggable backend families
web/src/App.tsx                 # frontend entry point
web/src/api/client.ts           # API client
web/src/hooks/useWebSocket.ts   # WebSocket integration
```

---

## Architecture in one screen

Five mental models carry most of the codebase. Full detail in [`internal/AGENTS.md`](internal/AGENTS.md).

1. **Streaming is sacred.** Never buffer an entire dataset into memory. Connect pipeline stages with
   `io.Reader`/`io.Writer` and `io.Pipe`. New stages must preserve streaming.
2. **Interfaces enable extension.** Add a database engine → implement `Dumper`/`Restorer`. Add a
   storage backend → implement `Storage`. Add a notification → implement `Notifier`. Each family has
   a factory that dispatches on a type string and its own nested `AGENTS.md`.
3. **Context propagation.** `context.Context` is always the first parameter; check for cancellation in
   long-running operations.
4. **Frontend state layers.** Server state → TanStack Query; live updates → WebSocket; UI/client
   preferences → Zustand. Don't mix the layers.
5. **Type safety everywhere.** Go and TypeScript both enforce strong typing — lean on it; let the
   compiler catch bugs.

---

## Agent workflow

Work in three phases. For anything non-trivial (touches >3 files, adds a backend, or changes an
interface), capture the first two phases as a spec under [`specs/`](specs/) — see
[`specs/README.md`](specs/README.md).

1. **Research** — locate the relevant code and understand it before changing anything. Use the
   `codebase-locator` agent for "where is X", read the nested `AGENTS.md` for the area, and follow
   existing patterns. Capture findings in `specs/<date>-<slug>/research.md`.
2. **Plan** — write the change down before writing code: files to touch, interfaces affected, tests
   to add. Capture in `specs/<date>-<slug>/plan.md`. Resolve open questions before implementing.
3. **Implement → verify → ship** — make the change, then:
   - **Backend:** `make fmt` → `make test` → `make lint` → `make build` → run `./bin/brd …`
   - **Frontend:** `npm --prefix web run validate` (type-check + lint + format + tests)
   - **Full-stack:** `make build-with-web` then exercise the running daemon.
   Record anything surprising in `specs/<date>-<slug>/implementation-notes.md`, then open a PR.

### Project skills

These repo-specific skills scaffold the common changes following existing patterns. Invoke with `/<name>`.

| Skill | Use it to… |
|-------|-----------|
| `/add-database-type` | add a new database engine (Dumper/Restorer + factory + config + UI) |
| `/add-storage-backend` | add a new storage backend (Storage + retry + factory + config + UI) |
| `/add-notifier` | add a new notification channel (Notifier + factory + config + UI) |
| `/add-api-endpoint` | add a REST endpoint end-to-end (handler + route + web client + hook) |
| `/add-config-field` | thread a new config field through YAML → struct → API → DB → web form |
| `/run-daemon` | build and run the daemon locally and smoke-test a change |
| `/release` | drive the tag → GoReleaser → GitHub release flow |

Built-in skills also apply here: `/code-review`, `/simplify`, `/verify`, `/pr`, `/security-review`.
Custom review agents `go-backend-reviewer` and `web-frontend-reviewer` encode BareD's conventions.

---

## Build, test & run — quick reference

Run from the repo root. `make help` lists everything; the high-value targets:

| Task | Command |
|------|---------|
| Build binary (embeds web UI) | `make build` |
| Build backend only (no web) | `go build ./cmd/brd` |
| Build + embed fresh web UI | `make build-with-web` |
| Run daemon (dev, CGO on for sqlite) | `make run-daemon` |
| Format Go | `make fmt` |
| Lint Go (golangci-lint) | `make lint` |
| Vet | `make vet` |
| Unit tests | `make test` / `make test-unit` |
| Integration tests | `make test-integration` (needs `make setup-test-env`) |
| Coverage | `make coverage` |
| Validate backend (fmt+vet+lint+test) | `make validate` |
| Frontend dev server | `make web-dev` (or `npm --prefix web run dev`) |
| Frontend lint | `make web-lint` |
| Frontend validate (types+lint+fmt+tests) | `make web-validate` |
| Frontend build | `make web-build` |

> A PostToolUse hook auto-formats `.go` files (gofmt + goimports) and `web/` files (prettier) on save.
> A Stop hook runs a fast vet/lint pass and surfaces problems — see `.claude/hooks/`.

---

## Git, commits & PRs

Follow [`CONTRIBUTING.md`](CONTRIBUTING.md) — it is the authority on branch flow, code style, the
PR checklist, and the GoReleaser release process. In short:

- **Branch** off `main` with a descriptive name; never commit straight to `main`.
- **Commit** in small, coherent chunks. Match the existing history style and reference the issue/PR.
- **Before pushing:** `make validate` (backend) and/or `make web-validate` (frontend) must pass.
- **PR:** fill in `.github/PULL_REQUEST_TEMPLATE.md`; CI (`.github/workflows/`) runs Go + web checks,
  Docker build, and release tooling.
- **Secrets:** never commit `config.yml`, `bared.yml`, `*.local.yml`, `.env*`, or `*.db` (all gitignored).

---

## Engineering principles

- **Follow existing patterns.** Before adding anything, find the closest existing implementation
  (e.g. a new storage backend mirrors `local`/`s3`/`sftp`) and match its shape. The
  `codebase-pattern-finder`/`Explore` agents are good for this.
- **Stdlib-first.** Reach for the standard library before adding a dependency. Justify new deps.
- **Stream, don't buffer.** Memory usage must stay flat regardless of dataset size.
- **Context everywhere.** Thread `context.Context`; respect cancellation; clean up resources.
- **Test real behavior.** Prefer tests that exercise the real pipeline over mock theater. Add a
  regression test that fails before your fix and passes after.
- **Type-driven.** Let Go and TypeScript types enforce contracts across the API boundary.
- **Keep the docs honest.** If you change an interface or workflow, update the relevant nested
  `AGENTS.md` and `docs/` in the same change.

---

## Safety & security

Backups touch credentials and customer data — treat this code as security-sensitive. Highlights
(full detail under "Safety & Security" in [`internal/AGENTS.md`](internal/AGENTS.md)):

- **Never log secrets** (DB passwords, S3 keys, SFTP creds, encryption keys). Redact in errors/logs.
- **Validate and sanitize** all external input — config values, API request bodies, file paths
  (guard against path traversal in storage keys and restore targets).
- **Encryption** is available for backups; never weaken it or hardcode keys.
- **Least privilege** for storage and database credentials; surface errors without leaking internals.
- **Destructive operations** (restore overwrites data) must be explicit and confirmable; respect
  dry-run paths where they exist.

---

_This is the entry point. From here, open the nested `AGENTS.md` for the area you're changing._
