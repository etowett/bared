# Research — agentic setup for Claude Code + Codex

> Phase 1. Understand the current state before proposing a change. Cite real code (`file:line`).

## Goal / problem

Make BareD properly workable by two agent clients — Claude Code and the Codex CLI — from checked-in
configuration, and stop the two from drifting apart. Issue: #52.

## Area & governing guide

- Primary area(s): `.claude/`, `.codex/`, `.agents/`, `scripts/`, `Makefile`, `.github/workflows/`
- Nested guide(s) to follow: root `AGENTS.md` (workflow, principles). No nested guide covered the
  agent tooling itself before this change — `.claude/AGENTS.md` is created here to fill that gap.

## What exists today

- PR #45 added the first pass: the nested `AGENTS.md` tree, 7 skills under `.claude/skills/`,
  3 subagents under `.claude/agents/`, 2 hooks (`format-on-save.sh`, `lint-on-stop.sh`),
  `.claude/settings.json`, and `.mcp.json` (Context7 only).
- `.agents/skills` is already a symlink to `../.claude/skills`, so a skills-sharing mechanism exists
  and works — but nothing else is shared.
- **No `.codex/` at all.** The Codex CLI gets no MCP servers, no allowlist, no sandbox/approval
  policy, no hooks, and no subagents in this repo.
- **No slash commands.** `.claude/commands/` did not exist.
- Two subagent prompts hardcoded the author's home directory —
  `.claude/agents/go-backend-reviewer.md:11` and `.claude/agents/web-frontend-reviewer.md:11` both
  read `/Users/eutychus/Code/my/bared/...`, which resolves for exactly one machine.
- **The documented verify gate was wrong.** `AGENTS.md:142` labelled `make validate` as
  "Validate backend (fmt+vet+lint+test)" and `AGENTS.md:160` told contributors to run it before
  pushing. `Makefile:139-142` shows `validate: build` → `brd validate-config --config
  examples/config.example.yml`. The real gate is `pre-commit` (`Makefile:135`):
  `fmt vet lint test-unit coverage-check`. `.claude/README.md`, `internal/AGENTS.md:360`,
  `specs/TEMPLATE/plan.md`, `CONTRIBUTING.md:87`, and `.claude/hooks/lint-on-stop.sh` repeated the
  same error.
- **`make lint` fails on a clean checkout** once `npm ci` has run: `.golangci.yml` excluded only
  `vendor` and `examples`, so golangci-lint walked into
  `web/node_modules/flatted/golang/pkg/flatted/` — a Go file shipped inside an npm package — and
  reported 5 issues. CI's lint job never runs `npm ci`, which is why it stayed hidden.

## Constraints & considerations

- **Hook scripts must not be duplicated.** They read their payload from stdin and need the repo root;
  both clients can supply that if the scripts detect it rather than assuming a client env var.
- **No machine-specific absolute paths anywhere.** Codex hook commands are plain shell strings with
  no project-directory variable, so a naive port would hardcode a path.
- **Codex permission vocabulary differs.** Rules take `allow`/`prompt`/`forbidden`, `pattern` is an
  exact prefix over the argument list with union elements allowed, and the most restrictive matching
  decision wins. Codex also splits compound commands itself.
- **Project-scoped `.codex/config.toml` only loads for a trusted project** — a one-time interactive
  acknowledgement that has to be documented or the setup silently does nothing.
- **Secrets.** BareD's `config.yml`/`bared.yml`/`.env*`/`*.db` hold database passwords, storage keys,
  and encryption keys. `settings.json` denies `Read()` on them, but writes, `git add`, and shelling
  out to `cat` were all unguarded.

## Prior art in the repo

- `.agents/skills` symlink (PR #45) — the pattern for sharing without duplication; extended, not
  replaced.
- An external two-client setup by the same author was used as the reference for the sync-map idea.
  Its Codex hooks are *copies* of the Claude scripts, wired up with absolute paths in a gitignored
  `hooks.json`; this change deliberately diverges by sharing the scripts instead.
