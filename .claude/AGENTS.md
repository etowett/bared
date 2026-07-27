# `.claude/` — the source of truth for BareD's agent configuration

Two clients are supported in this repo: **Claude Code** (`.claude/`) and the **Codex CLI**
(`.codex/`). This directory plus `.mcp.json` at the repo root is **canonical**; `.codex/` is a
mirror derived from it. Change things here first, then propagate.

> Looking for *how to work in this codebase*? That's the [root `AGENTS.md`](../AGENTS.md) and its
> nested guides. This file is about the agent tooling itself.
> Looking for *what's available*? That's [`README.md`](README.md) in this directory.

---

## Sync map

| Canonical source | Mirror | How it's kept in sync |
|---|---|---|
| `.mcp.json` (`mcpServers`) | `.codex/config.toml` (`[mcp_servers.*]`) | hand-maintained — checked by `make agents-doctor` |
| `.claude/settings.json` (`permissions.allow`) | `.codex/rules/allowlist.rules` | hand-maintained — checked by `make agents-doctor` |
| `.claude/agents/*.md` | `.codex/agents/*.toml` | **generated** — `make agents-sync` |
| `.claude/settings.json` (`hooks`) | `.codex/config.toml` (`[hooks]`) | hand-maintained — checked by `make agents-doctor` |
| `.claude/skills/` | `.agents/skills` symlink | **automatic** — nothing to do |

Run `make agents-doctor` after touching any of the left-hand column. It exits non-zero on drift and
CI runs it, so a stale mirror fails the PR. `make agents-sync` regenerates the generated bits.

**Hook scripts are never duplicated.** `.codex/hooks.json` points straight at `.claude/hooks/*.sh`;
the scripts read their payload from stdin and work under both clients.

---

## What lives here

| Path | What it is |
|---|---|
| `settings.json` | Committed, shared: `permissions.ask` / `permissions.deny`, hook registrations, MCP opt-in. **No `allow` list** — see `settings.local.json.example` |
| `settings.local.json` | Personal, **gitignored** — the command allow-list plus your own overrides. Copy from `settings.local.json.example` |
| `settings.local.json.example` | Committed template for the above; opting in is each maintainer's choice |
| `agents/*.md` | Subagents, delegated via the Agent/Task tool |
| `skills/<name>/SKILL.md` | Skills — auto-activate on matching requests or invoke with `/<name>` |
| `commands/*.md` | Slash commands — explicit, deterministic prompts (`/spec`, `/gate`, `/agents-doctor`) |
| `hooks/*.sh` | Hook scripts, shared with Codex |
| `local/` | Scratch, **gitignored** |

### Skills vs. commands vs. subagents

Reach for the right one — this is the most common mistake when extending the setup.

- **Skill** — a *recipe the model should find on its own*. It carries a `description` that the model
  matches against the request. Use for "when the user wants X, here's the procedure": adding a
  storage backend, cutting a release.
- **Command** — a *deterministic action the human triggers by name*. No routing, no ambiguity. Use
  when the user knows exactly what they want to run: scaffold a spec, run the gate.
- **Subagent** — a *separate context window with its own tools and system prompt*. Use to keep bulky
  work (searching hundreds of files, a full review pass) out of the main conversation, and to get an
  independent read on the code.

If it needs its own context budget → subagent. If the human names it → command. Otherwise → skill.

---

## Skill authoring gate

Every skill's frontmatter `description` is loaded by **every** client sharing `.claude/skills/` —
Claude Code, and through the `.agents/skills` symlink, Codex. A committed skill is therefore a
standing context cost in every session, for everyone. Before adding one:

1. **Keep `description` ≤ ~300 characters.** It's a routing hint — *when* to reach for the skill —
   not documentation. The how-to goes in the skill body.
2. **Justify repo scope.** A committed skill needs a shared-consumer reason: teammates or CI invoke
   it. A personal, single-author skill belongs in your **user-scope** directory
   (`~/.claude/skills/` for Claude Code; Codex scans its own), not in this repo.
3. **Don't duplicate a nested `AGENTS.md`.** BareD's subsystem skills are deliberately thin
   checklists that defer to `apps/api/internal/{database,storage,notify}/AGENTS.md` for the actual recipe.
   Keep it that way — one source of truth per fact.
4. **If it must live in-repo but shouldn't auto-load,** suppress implicit invocation per client —
   Claude Code: `disable-model-invocation: true` in the frontmatter (this also drops the description
   from the session budget); Codex: `allow_implicit_invocation: false`. It stays `/`-invocable
   either way.

## Subagent authoring gate

- **Use repo-relative paths.** A subagent prompt containing `/Users/...` breaks for every other
  contributor and in every git worktree. `make agents-doctor` fails on absolute paths.
- **Give it the narrowest `tools` list that works.** Reviewers and locators are read-only
  (`Read, Grep, Glob, Bash`) — they must not edit.
- **Mirror it to `.codex/agents/<name>.toml`** in the same change (`name`, `description`,
  `developer_instructions`).

## Hook authoring gate

- **Read the payload from stdin, exit 0 by default.** A hook that blocks on failure can trap a
  session in a loop. Only the two `guard-*.sh` hooks exit non-zero, and only on a deliberate policy
  violation.
- **Be fast and be silent when there's nothing to say.** Stop hooks run on every turn.
- **Don't hardcode the repo root** — use the `lib.sh` helpers so the script works under Codex too,
  and pick the right one: `hook_repo_root` for *installed tooling* (`apps/web/node_modules`), which
  lives in the project the client was opened on; `hook_worktree_root` for *git state* — branch,
  status, diff — which must come from the tree the command actually runs in. They differ inside a
  `git worktree`, where `$CLAUDE_PROJECT_DIR` keeps pointing at the main checkout.
- **Register it in `settings.json`** and run `make agents-sync` so Codex picks it up.
