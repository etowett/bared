# Plan — agentic setup for Claude Code + Codex

> Phase 2. The concrete change. Resolve `open-questions.md` before you start implementing.

## Approach

Keep `.claude/` + `.mcp.json` as the single source of truth and derive `.codex/` from it, mirroring
the `.agents/skills` symlink pattern that already works: **share what can be shared, generate what
can be generated, check the rest.**

Three tiers, in order of preference:

1. **Shared** — hook *scripts* live once in `.claude/hooks/` and both clients invoke them. Skills are
   already shared through the `.agents/skills` symlink.
2. **Generated** — `.codex/agents/*.toml` is rendered from `.claude/agents/*.md`, so subagents cannot
   drift by construction.
3. **Checked** — MCP servers, the command allowlist, and hook *registrations* need per-client
   formats and judgement, so they are hand-maintained and a doctor script fails CI on drift.

## Files to create / edit

| File | Change |
|------|--------|
| `.claude/AGENTS.md` | create — source-of-truth statement, sync map, authoring gates for skills/subagents/hooks |
| `.claude/README.md` | rewrite — what's available now (skills, commands, subagents, hooks) |
| `.claude/commands/{spec,gate,agents-doctor}.md` | create — the three slash commands |
| `.claude/agents/{codebase-analyzer,codebase-pattern-finder,specs-locator}.md` | create — research subagents |
| `.claude/agents/{go,web}-*-reviewer.md` | edit — strip the hardcoded absolute paths |
| `.claude/hooks/lib.sh` | create — shared payload parsing + repo-root detection |
| `.claude/hooks/{ensure-newline,session-start,typecheck-on-stop,guard-secrets,guard-main-branch}.sh` | create |
| `.claude/hooks/{format-on-save,lint-on-stop}.sh` | edit — use `lib.sh`; fix the `make validate` message |
| `.claude/settings.json` | edit — register the new hooks; permit the doctor script |
| `.claude/skills/*/SKILL.md` | edit — trim descriptions under the new 300-char gate; fix the gate command |
| `.codex/{AGENTS.md,config.toml,rules/allowlist.rules}` | create — the Codex mirror |
| `.codex/agents/*.toml` | create — generated from `.claude/agents/*.md` |
| `.agents/AGENTS.md` | create — what the shared directory is for |
| `scripts/agents-doctor.py` | create — the drift checker / generator |
| `scripts/test-agent-hooks.sh` | create — tests for the hooks, especially the blocking ones |
| `Makefile` | edit — `agents-doctor`, `agents-sync` targets + help |
| `.github/workflows/ci.yml` | edit — an `agent-config` job |
| `.gitignore` | edit — ignore `.codex/*`, re-include the shared files |
| `.golangci.yml` | edit — exclude `node_modules` so `make lint` passes |
| `AGENTS.md`, `CONTRIBUTING.md`, `internal/AGENTS.md`, `specs/TEMPLATE/plan.md` | edit — fix the verify-gate error, document the new tooling |

## Interfaces affected

None in the Go or TypeScript sense — no product code changes. The "interface" this touches is the
agent-configuration contract between `.claude/` and `.codex/`, which `scripts/agents-doctor.py`
codifies.

## Ordered steps

1. `.claude/AGENTS.md` — write the sync map and gates first; everything else follows from it.
2. Hooks: `lib.sh`, then the five new scripts, then refactor the two existing ones onto it.
3. Subagents and commands.
4. `.codex/`: `config.toml` (sandbox, MCP, `[hooks]`), `rules/allowlist.rules`, `AGENTS.md`.
5. `scripts/agents-doctor.py` + `make agents-sync` to generate `.codex/agents/*.toml`.
6. Fix everything the doctor reports — including the pre-existing skill-description overruns.
7. `scripts/test-agent-hooks.sh`; wire both into `Makefile` and CI.
8. Docs pass: the `make validate` → `make pre-commit` correction everywhere it's wrong.

## Tests

- `scripts/test-agent-hooks.sh` — the two `guard-*` hooks can block a tool call, so both directions
  matter: a false positive stops real work, a false negative leaks credentials. Cover blocked cases
  (`config.yml`, `.env.production`, `git add config.yml`, `cat *.yml` via glob), allowed cases
  (`examples/config.example.yml`, `.env.example`, `grep config.yml AGENTS.md`, `make build`), the
  `BARED_ALLOW_MAIN_COMMIT` escape hatch, and that every advisory hook always exits 0.
- `make agents-doctor` is itself the regression test for mirror drift; CI runs it.
- `bash -n .claude/hooks/*.sh` in CI so a syntax error can't ship a broken hook.

## Verification

- Backend: `make pre-commit` (fmt + vet + lint + unit tests + coverage), then `make build`
- Frontend: `make web-validate`
- Agent tooling: `make agents-doctor` and `./scripts/test-agent-hooks.sh`
- Codex: `codex doctor` against a scratch `CODEX_HOME` to confirm `.codex/config.toml` parses and
  the sandbox/approval/MCP settings are actually applied
