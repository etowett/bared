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

Each deployable app lives under `apps/`; the repo root holds project-level concerns only.
Before editing, load the guide(s) for the area you're working in:

```
AGENTS.md                                  ← you are here: overview, workflow, conventions
├─ apps/
│  ├─ api/                                 the Go backend — module root (go.mod lives here)
│  │  ├─ internal/AGENTS.md                backend deep-dive (architecture, API, testing, full-stack recipes)
│  │  │  ├─ internal/database/AGENTS.md    ▸ add / change a database engine (Dumper/Restorer)
│  │  │  ├─ internal/storage/AGENTS.md     ▸ add / change a storage backend (Local/S3/SFTP)
│  │  │  └─ internal/notify/AGENTS.md      ▸ add / change a notification channel
│  │  └─ cmd/AGENTS.md                     CLI (`brd`): Cobra commands, config import, build & run
│  └─ web/                                 the React 19 + TS dashboard
│     ├─ AGENTS.md                         state, websocket, components, testing
│     ├─ README.md                         frontend overview
│     └─ TESTING.md                        frontend testing guide
├─ docs/                                   long-form docs: api/, architecture/, development/, operations/, user-guide/
├─ CONTRIBUTING.md                         contribution flow, code style, release process
└─ specs/                                  spec-driven feature work (research → plan → implementation notes)
```

**Rule of thumb:** touching `apps/api/internal/storage/` → read `apps/api/internal/storage/AGENTS.md` **and** `apps/api/internal/AGENTS.md`.
Touching `apps/web/` → read `apps/web/AGENTS.md`. A full-stack change → read both backend and frontend guides.

**The module is still `bared`.** `go.mod` sits at `apps/api/`, so every import path is unchanged
(`bared/internal/storage`, `bared/cmd/brd`). Run Go commands from `apps/api/` — or just use the
root `Makefile`, which already does.

Changing the **agent tooling itself** (skills, commands, subagents, hooks, permissions) is a separate
tree — `.claude/` is canonical and `.codex/` mirrors it:

```
.claude/AGENTS.md                  ← source of truth: sync map + authoring gates
│  └─ .claude/README.md            what's available: skills, commands, subagents, hooks
├─ .codex/AGENTS.md                Codex CLI mirror (config.toml, allowlist rules, generated subagents)
└─ .agents/AGENTS.md               the shared bits (the .claude/skills symlink)
```

Both clients read this file: `CLAUDE.md` is a symlink to `AGENTS.md`, and Codex reads `AGENTS.md`
directly.

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
apps/api/go.mod                                # module root (module name: bared)
apps/api/cmd/brd/main.go                       # CLI entry point (Cobra commands)
apps/api/internal/app/                         # high-level backup/restore orchestration
apps/api/internal/api/server.go                # HTTP API + WebSocket server
apps/api/internal/daemon/daemon.go             # cron scheduler & signal handling
apps/api/internal/jobs/manager.go              # job queue & worker pool
apps/api/internal/config/config.go             # configuration structures
apps/api/internal/{database,storage,notify}/   # the three pluggable backend families
apps/api/internal/web/embed.go                 # go:embed of the built dashboard
apps/web/src/App.tsx                           # frontend entry point
apps/web/src/api/client.ts                     # API client
apps/web/src/hooks/useWebSocket.ts             # WebSocket integration
```

---

## Architecture in one screen

Five mental models carry most of the codebase. Full detail in [`apps/api/internal/AGENTS.md`](apps/api/internal/AGENTS.md).

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
   - **Backend:** `make pre-commit` (fmt → vet → lint → unit tests → coverage), then run `./bin/brd …`
   - **Frontend:** `make web-validate` (type-check + lint + format + tests)
   - **Full-stack:** `make build-with-web`, then exercise the running daemon.
   - **Agent tooling:** `make agents-doctor`
   Record anything surprising in `specs/<date>-<slug>/implementation-notes.md`, then open a PR.

`/spec` scaffolds step 1–2 and `/gate` runs the right verification for step 3.

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

### Slash commands

| Command | Does |
|---------|------|
| `/spec <slug>` | scaffold `specs/<date>-<slug>/` and run the research phase |
| `/gate` | run the real verify gate for whatever changed, and fix what it reports |
| `/agents-doctor` | check the Claude Code ⇄ Codex config mirrors and repair drift |

### Subagents

Delegate with the Agent/Task tool. The first three are for research, the reviewers for pre-PR checks.

| Subagent | Answers |
|----------|---------|
| `codebase-locator` | where does X live? |
| `codebase-analyzer` | how does X actually work? (traced, with `file:line`) |
| `codebase-pattern-finder` | what's the closest existing implementation to copy? |
| `specs-locator` | what's already written down about this? |
| `go-backend-reviewer` | is this Go change up to BareD's conventions? |
| `web-frontend-reviewer` | is this React/TS change up to BareD's conventions? |

Skills, commands, and subagents are shared with the Codex CLI — see [`.claude/AGENTS.md`](.claude/AGENTS.md).

---

## Build, test & run — quick reference

Run from the repo root. `make help` lists everything; the high-value targets:

| Task | Command |
|------|---------|
| Build binary (embeds web UI) | `make build` |
| Build backend only (no web) | `go -C apps/api build ./cmd/brd` |
| Build + embed fresh web UI | `make build-with-web` |
| Run daemon (dev, CGO on for sqlite) | `make run-daemon` |
| Format Go | `make fmt` |
| Lint Go (golangci-lint) | `make lint` |
| Vet | `make vet` |
| Unit tests | `make test` / `make test-unit` |
| Integration tests | `make test-integration` (needs `make setup-test-env`) |
| Coverage | `make coverage` |
| **Backend verify gate** (fmt+vet+lint+test+coverage) | **`make pre-commit`** |
| Frontend dev server | `make web-dev` (or `npm --prefix apps/web run dev`) |
| Frontend lint | `make web-lint` |
| **Frontend verify gate** (types+lint+fmt+tests) | **`make web-validate`** |
| Frontend build | `make web-build` |
| Check the agent config mirrors | `make agents-doctor` |

> **`make validate` is not the verify gate** — it builds and validates
> `examples/config.example.yml`, nothing more. The gate is `make pre-commit`.

> **Known gap:** `make pre-commit`'s last step, `coverage-check`, currently fails — the repo sits at
> ~27% against a 75% threshold, and `apps/api/cmd/brd`, `apps/api/internal/client`, and `apps/api/internal/configservice` have
> no tests at all. Everything before it (`fmt` → `vet` → `lint` → `test-unit`) must pass. Don't try
> to close a 48-point coverage gap as a side quest; raising it is tracked separately.

> Hooks in `.claude/hooks/` run under both Claude Code and Codex: files are auto-formatted on save,
> secrets and direct-to-`main` commits are blocked, and a Stop hook surfaces lint/type errors.
> `/gate` runs the right gate for whatever you changed.

---

## Git, commits & PRs

Follow [`CONTRIBUTING.md`](CONTRIBUTING.md) — it is the authority on branch flow, code style, the
PR checklist, and the GoReleaser release process. In short:

- **Branch** off `main` with a descriptive name; never commit straight to `main`.
- **Commit** in small, coherent chunks. Match the existing history style and reference the issue/PR.
- **Before pushing:** `make pre-commit` (backend) and/or `make web-validate` (frontend) must pass —
  or just run `/gate`, which picks the right one from the diff.
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
(full detail under "Safety & Security" in [`apps/api/internal/AGENTS.md`](apps/api/internal/AGENTS.md)):

- **Never log secrets** (DB passwords, S3 keys, SFTP creds, encryption keys). Redact in errors/logs.
- **Validate and sanitize** all external input — config values, API request bodies, file paths
  (guard against path traversal in storage keys and restore targets).
- **Encryption** is available for backups; never weaken it or hardcode keys.
- **Least privilege** for storage and database credentials; surface errors without leaking internals.
- **Destructive operations** (restore overwrites data) must be explicit and confirmable; respect
  dry-run paths where they exist.

---

_This is the entry point. From here, open the nested `AGENTS.md` for the area you're changing._
