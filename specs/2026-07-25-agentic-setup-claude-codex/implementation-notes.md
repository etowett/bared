# Implementation notes — agentic setup for Claude Code + Codex

> Phase 3. Written during/after implementation. The audit trail for why the code looks as it does.

## What changed

- **`.claude/`** — added `AGENTS.md` (source of truth + sync map + authoring gates), three commands
  (`/spec`, `/gate`, `/agents-doctor`), three research subagents (`codebase-analyzer`,
  `codebase-pattern-finder`, `specs-locator`), five hooks plus a shared `lib.sh`, and rewrote
  `README.md`. Stripped the hardcoded absolute paths from the two reviewer subagents.
- **`.codex/`** — new: `config.toml` (sandbox/approval, the Context7 MCP server, and a `[hooks]`
  table pointing at the shared scripts), `rules/allowlist.rules`, generated `agents/*.toml`, and an
  `AGENTS.md` explaining the mirror.
- **`.agents/AGENTS.md`** — documents what the shared directory is and how to add a third client.
- **`scripts/`** — `agents-doctor.py` (drift checker + generator) and `test-agent-hooks.sh`.
- **Docs** — corrected the verify gate everywhere it was wrong, and fixed `.golangci.yml`.

## Decisions & trade-offs

- **Codex hooks reuse the Claude scripts rather than copying them.** The reference setup this was
  modelled on duplicates the scripts and wires them up with absolute paths in a gitignored
  `hooks.json`. Codex hook commands have no project-directory variable, so the workaround here is a
  `bash -c 'exec "$(git rev-parse --show-toplevel)/.claude/hooks/x.sh"'` wrapper. Slightly ugly in
  the TOML; buys one copy of every hook and a config that works in any clone or worktree.
- **`.codex/agents/*.toml` is generated, not hand-mirrored.** Six subagent prompts of a few hundred
  words each would drift within a release. `model:` is deliberately *not* mapped — Claude's
  `opus`/`sonnet` don't correspond to Codex model IDs, and inventing a mapping would be a guess, so
  Codex uses its own default subagent model.
- **The allowlist mirror is broad-allow + narrow-prompt, not a line-for-line copy.** Codex applies
  the most restrictive matching rule, so `git` is allowed broadly and then narrowed by a `prompt`
  rule on `push`/`commit`/`reset`/`rebase`/`clean`/`filter-branch`. This tracks Claude's allow/ask
  split with far fewer rules.
- **The doctor is Python, and avoids `tomllib`.** `tomllib` is 3.11+, and macOS still ships 3.9. It
  regex-scans `config.toml` for the handful of facts it needs instead.
- **Only `guard-secrets.sh` and `guard-main-branch.sh` can block.** Everything else exits 0
  unconditionally, so no hook can trap a session in a loop.
- **The 300-char skill-description gate was applied to the existing skills, not waived.** All seven
  were over (313–399 chars). Writing a rule and grandfathering every current violation makes the rule
  decorative. The trimmed detail already existed in each skill's body.

## Surprises / gotchas

- **`make validate` was never the verify gate.** Root `AGENTS.md` advertised it as
  "fmt+vet+lint+test"; it actually runs `brd validate-config` against the example config. The real
  gate is `make pre-commit`. Five files repeated the error, including the Stop hook's own output —
  so an agent following the guide ran the wrong command and got a green result.
- **`make lint` fails on any machine that has run `npm ci`.** `apps/web/node_modules/flatted/golang/`
  ships a Go source file, and `.golangci.yml` excluded only `vendor` and `examples`. CI never caught
  it because the lint job doesn't install npm dependencies. Fixed by excluding `node_modules`.
- **Codex needs a one-time trust acknowledgement** before it reads `.codex/config.toml`. Without it
  the whole directory is silently inert — worth knowing before debugging anything else.
- **TOML literal strings can't contain single quotes.** The first cut of the `[hooks]` commands used
  `'...''...'''` doubling, which is Starlark, not TOML. Basic strings with escaped inner quotes work.
- **`guard-secrets.sh` blocked its own author.** The first version scanned the Bash command with an
  unquoted `for token in $cmd`, deliberately leaving globbing on so `cat *.yml` would be caught. But
  that makes the verdict depend on the working directory: a bare `*` *anywhere* in the command — a
  `**bold**` in a heredoc body, a pathspec, quoted prose — expands to every file in the repo root,
  which includes `bared.db`. Writing an unrelated Markdown file got refused. Fixed by turning
  globbing off (`set -f`), truncating the command at the first `<<` so heredoc bodies are treated as
  document text rather than arguments, and judging glob *patterns* by their own suffix instead of by
  what they expand to. Both directions now have regression tests. The lesson generalises: a guard
  hook whose decision depends on ambient filesystem state will eventually fire on the wrong thing,
  and a false positive is not a harmless failure mode — it stops real work with a scary message.

## Follow-ups

- [ ] Codex hook *firing* was not exercised end to end — that needs an authenticated interactive
      session in a trusted project. Verified instead that `.codex/config.toml` parses and that
      sandbox/approval/MCP settings are applied (`codex doctor` against a scratch `CODEX_HOME`), and
      that the `[hooks]` table deserializes to the documented `hooks.json` event schema.
- [ ] `codex doctor` does not validate hook event names — a typo'd event parses fine and is ignored.
      Consider teaching `agents-doctor.py` to check them against the documented list.
- [ ] The `web-ci.yml` workflow could run `make agents-doctor` too, so web-only PRs also gate on it.

## Verification done

- `make agents-doctor` → consistent ✓
- `./scripts/test-agent-hooks.sh` → 30 passed, 0 failed
- `make web-validate` → type-check, lint, format:check clean; 237 tests in 15 files pass
- `make fmt`, `make vet`, `make lint`, `make test-unit` → all pass
- `make pre-commit` → **fails at `coverage-check`**: 27.0% against a 75% threshold. Pre-existing and
  unrelated to this change — no `.go` file is touched here, and `apps/api/cmd/brd`, `apps/api/internal/client`, and
  `apps/api/internal/configservice` have no tests at all. See `open-questions.md` and the follow-up issue.
  The `lint` step of `pre-commit` *did* fail before this change and now passes.
- `bash -n .claude/hooks/*.sh` → clean
- `codex doctor` with `CODEX_HOME` pointed at a copy of `.codex/config.toml` → `config.toml parse
  ok`, 1 stdio MCP server, `restricted fs + enabled network · approval OnRequest`
