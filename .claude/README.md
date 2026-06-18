# `.claude/` — Agent tooling for BareD

Orientation for humans and agents working in this repo (`brd` Go daemon + React/TS web UI).
This directory holds the skills, subagents, hooks, and settings that shape how Claude Code
operates here. Start with the AGENTS.md tree below, then reach for the right skill/agent.

## AGENTS.md tree (read before editing)

Area-specific guidance lives in nested `AGENTS.md` files. **The innermost (most specific) guide wins** when guidance conflicts.

```
AGENTS.md                          ← root: map, workflow, conventions
├─ internal/AGENTS.md              Go backend deep-dive (architecture, API, testing, recipes)
│  ├─ internal/database/AGENTS.md  add/change a database engine (Dumper/Restorer)
│  ├─ internal/storage/AGENTS.md   add/change a storage backend (Local/S3/SFTP)
│  └─ internal/notify/AGENTS.md    add/change a notification channel
├─ cmd/AGENTS.md                   CLI (`brd`): Cobra commands, config import, build & run
└─ web/AGENTS.md                   React 19 + TS dashboard (state, websocket, components, testing)
```

`CLAUDE.md` is a **symlink to the root `AGENTS.md`** — one source of truth for both Claude Code and other agents.
Rule of thumb: touching `internal/storage/` → read `internal/storage/AGENTS.md` **and** `internal/AGENTS.md`; full-stack change → read both backend and frontend guides.

## Skills (`.claude/skills/<name>/SKILL.md`)

Auto-activate on matching requests, or invoke explicitly. The pluggable-subsystem skills are checklists that defer to the nested AGENTS.md recipes.

- **add-database-type** — scaffold a new DB engine (Dumper/Restorer) like MySQL/Postgres/Redis.
- **add-storage-backend** — scaffold a new Storage backend like Local/S3/SFTP.
- **add-notifier** — scaffold a new Notifier channel like Slack/Email/Webhook.
- **add-api-endpoint** — add a full-stack REST endpoint (Go handler + route ⇄ TS client + React Query hook + component).
- **add-config-field** — add a config field full-stack (Go struct/yaml/validation ⇄ TS type + form).
- **run-daemon** — build and run `brd` locally and smoke-test a change in the real app.
- **release** — cut a release (tag → GoReleaser → GitHub release + Docker).

Built-in skills also apply: **/code-review**, **/simplify**, **/verify**, **/pr** (and /security-review). The built-in **/run** defers to `run-daemon`.

## Subagents (`.claude/agents/<name>.md`)

Delegate via the Agent/Task tool (e.g. "use the go-backend-reviewer to review my changes"); the reviewers are meant to run proactively before a PR.

- **go-backend-reviewer** (opus) — reviews Go changes under `internal/`/`cmd/`: streaming/io.Pipe, interface+factory extension, context propagation, error wrapping, no-secrets-in-logs, path-traversal safety.
- **web-frontend-reviewer** (sonnet) — reviews React/TS changes under `web/src/`: state layering (Query vs WebSocket vs Zustand), API client pattern, components, Radix+Tailwind, Vitest, strict TS.
- **codebase-locator** (sonnet) — fast "where is X" map across `internal/`, `cmd/`, `web/src/`, and which nested AGENTS.md governs each area.

## Hooks (`.claude/hooks/`)

- **format-on-save.sh** — PostToolUse (Edit/Write/MultiEdit): runs gofmt/goimports on Go files and prettier on web files. **Runs automatically** after edits.
- **lint-on-stop.sh** — Stop: advisory `go vet` + golangci-lint + eslint. **Advisory only — it never blocks**; it surfaces issues, it doesn't fail the turn.

## Settings & config

- **settings.json** — shared, committed: permission allow/ask/deny lists, the two hooks, and `enableAllProjectMcpServers`. Secrets are denied for read (`config.yml`, `bared.yml`, `*.local.yml`, `.env*`).
- **settings.local.json** — personal, **gitignored**: your own overrides; don't put shared config here.
- **`.mcp.json`** (repo root) — provides the **Context7** MCP server for up-to-date library docs lookup. `enableAllProjectMcpServers` is on, so it loads automatically.

## Recommended workflow

**research → plan → implement → verify → PR.** For non-trivial work, capture the first phases as a
spec under `specs/<date>-<slug>/` (`research.md`, `plan.md`, `implementation-notes.md`). Verify by
running the app (`run-daemon`) and the gates (`make validate` / `make test` / `make build`;
`make web-validate` for UI), run the relevant reviewer subagent, then open a PR with the right
`release:*` label (drives the automated release).
