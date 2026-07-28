# `.claude/` — agent tooling for BareD

What's available when working on this repo (`brd` Go daemon + React/TS web UI), for humans and
agents alike.

- **How to work in this codebase** → the [root `AGENTS.md`](../AGENTS.md) and its nested guides.
- **How this tooling is wired and how to extend it** → [`AGENTS.md`](AGENTS.md) in this directory.
- **The Codex CLI mirror** → [`.codex/AGENTS.md`](../.codex/AGENTS.md).

Everything below is shared with the Codex CLI. `.claude/` is the source of truth; `.codex/` mirrors
it, and `make agents-doctor` fails when they drift.

## AGENTS.md tree (read before editing)

Area-specific guidance lives in nested `AGENTS.md` files. **The innermost guide wins** on conflict.

```
AGENTS.md                                 ← root: map, workflow, conventions
├─ apps/api/                              Go backend — module root (go.mod lives here)
│  ├─ internal/AGENTS.md                  backend deep-dive (architecture, API, testing, recipes)
│  │  ├─ internal/database/AGENTS.md      add/change a database engine (Dumper/Restorer)
│  │  ├─ internal/storage/AGENTS.md       add/change a storage backend (Local/S3/SFTP)
│  │  └─ internal/notify/AGENTS.md        add/change a notification channel
│  └─ cmd/AGENTS.md                       CLI (`brd`): Cobra commands, config import, build & run
└─ apps/web/AGENTS.md                     React 19 + TS dashboard (state, websocket, components, testing)
```

`CLAUDE.md` is a **symlink to the root `AGENTS.md`** — one source of truth for every client.
Touching `apps/api/internal/storage/` → read `apps/api/internal/storage/AGENTS.md` **and** `apps/api/internal/AGENTS.md`; a
full-stack change → read both backend and frontend guides.

## Skills (`skills/<name>/SKILL.md`)

Auto-activate on a matching request, or invoke explicitly with `/<name>`. The pluggable-subsystem
skills are deliberately thin checklists that defer to the nested `AGENTS.md` for the actual recipe.

| Skill | Use it to… |
|---|---|
| `add-database-type` | scaffold a new DB engine (Dumper/Restorer) like MySQL/Postgres/Redis |
| `add-storage-backend` | scaffold a new Storage backend like Local/S3/SFTP |
| `add-notifier` | scaffold a new Notifier channel like Slack/Email/Webhook |
| `add-api-endpoint` | add a full-stack REST endpoint (Go handler + route ⇄ TS client + hook + component) |
| `add-config-field` | add a config field full-stack (Go struct/yaml/validation ⇄ TS type + form) |
| `run-daemon` | build and run `brd` locally and smoke-test a change in the real app |
| `release` | cut a release (tag → GoReleaser → GitHub release + Docker) |

Built-in skills also apply: `/code-review`, `/simplify`, `/verify`, `/pr`, `/security-review`. The
built-in `/run` defers to `run-daemon`.

## Commands (`commands/<name>.md`)

Explicit, human-triggered actions — no routing, no ambiguity.

| Command | Does |
|---|---|
| `/spec <slug>` | scaffold `specs/<date>-<slug>/` from the template and run the research phase |
| `/gate` | run the real verify gate for whatever changed, then fix what it reports |
| `/agents-doctor` | check the Claude Code ⇄ Codex mirrors and repair drift |

## Subagents (`agents/<name>.md`)

Delegate via the Agent/Task tool ("use the go-backend-reviewer to review my changes"). Each runs in
its own context, so bulk searching and review passes stay out of the main conversation. All are
read-only. Mirrored to `.codex/agents/*.toml` by `make agents-sync`.

| Subagent | Model | Answers |
|---|---|---|
| `codebase-locator` | sonnet | where does X live? |
| `codebase-analyzer` | opus | how does X actually work? — traced with `file:line` |
| `codebase-pattern-finder` | sonnet | what's the closest existing implementation to copy, and every touch-point? |
| `specs-locator` | sonnet | what's already written down about this — specs, docs, prior PRs? |
| `go-backend-reviewer` | opus | is this Go change up to BareD's conventions? (run before a PR) |
| `web-frontend-reviewer` | sonnet | is this React/TS change up to BareD's conventions? (run before a PR) |

## Hooks (`hooks/*.sh`)

Registered in `settings.json` and mirrored into `.codex/config.toml`; the scripts themselves are
shared, not duplicated. Tested by `scripts/test-agent-hooks.sh`, which CI runs.

| Hook | Event | What it does |
|---|---|---|
| `session-start.sh` | SessionStart | branch, dirty count, and a warning if the toolchain the gate needs is missing |
| `guard-secrets.sh` | PreToolUse (Bash, Edit/Write) | **blocks** writing, reading, staging, or uploading `config.yml`, `bared.yml`, `*.local.yml`, `.env*`, `*.db` |
| `guard-main-branch.sh` | PreToolUse (Bash) | **blocks** `git commit` on `main`/`master` and any `git push` whose refspec targets them. Judges the tree the command will actually run in — a `cd` into a worktree and `git -C <dir>` are both followed — and reads pushes by refspec, so feature-branch and tag pushes go through from anywhere. Only text a shell would execute is walked: heredoc bodies are skipped and quoted punctuation cannot manufacture a command, so an issue or PR body that discusses committing is not a violation (escape hatch: `BARED_ALLOW_MAIN_COMMIT=1`, exported or written as a prefix on the command) |
| `format-on-save.sh` | PostToolUse (Edit/Write) | gofmt + goimports on Go, prettier on `apps/web/` |
| `ensure-newline.sh` | PostToolUse (Edit/Write) | appends a missing trailing newline |
| `lint-on-stop.sh` | Stop | advisory `gofmt -l` + `go vet` + golangci-lint + eslint over the diff |
| `typecheck-on-stop.sh` | Stop | advisory `tsc --noEmit` when web TS changed |
| `lib.sh` | — | shared payload parsing, root detection (`hook_repo_root` for installed tooling, `hook_worktree_root` for git state), and `hook_web_run` (runs `apps/web`'s lockfile-installed tools) |

Only the two `guard-*` hooks can block. Everything else always exits 0, so a hook can never trap a
session in a loop.

## Settings & config

- **`settings.json`** — committed and shared: `ask` and `deny` lists, hook registrations,
  `enableAllProjectMcpServers`. Secrets are denied for read (`config.yml`, `bared.yml`,
  `*.local.yml`, `.env*`). It deliberately carries **no `permissions.allow`** — see below.
- **`settings.local.json`** — personal, **gitignored**. This is where the command allow-list
  lives. Copy `settings.local.json.example` to `settings.local.json` to opt in.

> **Why the allow-list is not committed.** BareD is a public repository. An entry like
> `Bash(make:*)` or `Bash(go run:*)` auto-approves, with no prompt, a command whose behaviour
> is defined by files *in the working tree* — `Makefile`, `scripts/`, `package.json` scripts,
> `go:generate` directives. Checking out a fork's pull request to review it and then opening an
> agent session would hand that fork arbitrary code execution on your machine, silently. Keeping
> the allow-list local makes each maintainer opt in deliberately.
>
> **When reviewing an untrusted pull request, work without the local allow-list**, or review the
> diff to `Makefile`, `scripts/`, `apps/web/package.json` and any `go:generate` line before you
> run anything.
- **`../.mcp.json`** — the MCP server set, currently **Context7** for up-to-date library docs.
  `enableAllProjectMcpServers` is on, so it loads automatically. Mirrored to `.codex/config.toml`.
  Its `npx` launcher is unrelated to the web toolchain below.

### The web toolchain is Bun

`apps/web` is a **Bun** project — version pinned in `apps/web/.bun-version`, lockfile `apps/web/bun.lock`,
no `package-lock.json` and no `.nvmrc`. The allowlisted forms, and the ones the subagents and `/gate`
are told to use:

| Do | Not |
|---|---|
| `bun install --cwd apps/web` | `npm --prefix apps/web install` |
| `bun install --frozen-lockfile --cwd apps/web` | `npm --prefix apps/web ci` |
| `bun run --cwd apps/web <script>` | `bun --cwd apps/web run <script>` |

**`--cwd` goes after the subcommand.** Put it before and Bun treats the rest as a filename:
`bun --cwd apps/web run lint` prints the help menu and exits **0** having run nothing (a silent false
pass), and `bun --cwd apps/web install` fails with `Script not found "install"`. Confirmed still
present in Bun 1.3.14 — treat it as current behaviour, not a bug that got fixed.

The hooks keep calling `apps/web/node_modules/.bin/*` — `bun install` writes those shims exactly like
npm did. But they carry a `#!/usr/bin/env node` shebang, and Node is no longer a declared dependency
of this repo, so `hook_web_run` in `hooks/lib.sh` runs them under Bun when `node` is absent instead of
reporting `env: node: No such file or directory` as a lint error. It never shells out to `bunx`, which
could fetch a version other than the one in the lockfile.

## Recommended workflow

**research → plan → implement → verify → PR.** For non-trivial work capture the first phases under
`specs/<date>-<slug>/` — `/spec` scaffolds it. Verify by running the app (`run-daemon`) and the real
gate (`/gate`, i.e. `make pre-commit` and/or `make web-validate`) — note that `make validate` only
checks the example config and is *not* the gate. Run the relevant reviewer subagent, then open a PR
with the right `release:*` label, which drives the automated release.

> The last step of `make pre-commit`, `coverage-check`, is a **ratchet**: `COVERAGE_THRESHOLD` in the
> `Makefile` sits just below the real number (test helpers under `internal/testutil/` are excluded),
> and CI enforces the same value. If it goes red, add tests for what you changed — never lower the
> threshold. Raise it when a change lifts coverage.
