# `.codex/` — Codex CLI configuration for BareD

This directory makes the Codex CLI a first-class client in this repo: same MCP servers, same
command permissions, same hooks, same subagents as Claude Code.

**Claude Code is the source of truth.** `.claude/` and `.mcp.json` are canonical; everything here is
derived from them. Read [`.claude/AGENTS.md`](../.claude/AGENTS.md) for the full sync map, and the
[root `AGENTS.md`](../AGENTS.md) for how to actually work in this codebase.

## Getting started

Project-scoped config only loads once you trust the project, so the first run needs a one-time
acknowledgement:

```bash
cd /path/to/bared
codex           # accept the trust prompt
```

Then `codex doctor` will show the resolved config. If Codex isn't picking anything up, the trust
prompt is almost always the reason.

## What's here

| File | What it does |
|---|---|
| `config.toml` | sandbox + approval policy, the MCP server set (mirrors `.mcp.json`), and the `[hooks]` table (mirrors `.claude/settings.json`) |
| `rules/allowlist.rules` | command permissions, mirroring `.claude/settings.json` `permissions` |
| `agents/*.toml` | subagents — **generated** from `.claude/agents/*.md`, do not hand-edit |

Everything else Codex writes into `.codex/` is local session state and is gitignored.

## What mirrors what

| Canonical | Mirror here | Sync |
|---|---|---|
| `.mcp.json` (`mcpServers`) | `config.toml` (`[mcp_servers.*]`) | by hand — checked by `make agents-doctor` |
| `.claude/settings.json` (`permissions`) | `rules/allowlist.rules` | by hand — checked by `make agents-doctor` |
| `.claude/settings.json` (`hooks`) | `config.toml` (`[hooks]`) | by hand — checked by `make agents-doctor` |
| `.claude/agents/*.md` | `agents/*.toml` | **generated** — `make agents-sync` |
| `.claude/skills/` | `.agents/skills` symlink | automatic — nothing to do |

Run `make agents-doctor` after changing anything on the left. CI runs it too, so a stale mirror
fails the PR.

## Codex-specific notes

**Hook scripts are not duplicated.** `[hooks]` points at `.claude/hooks/*.sh` through a
`bash -c 'exec "$(git rev-parse --show-toplevel)/…"'` wrapper. Codex hook commands are plain shell
strings with no project-directory variable, and an absolute path here would break for every other
contributor and in every git worktree — hence the wrapper. The scripts themselves read their payload
from stdin and resolve the repo root from either client, so they run unchanged under both.

**Permission decisions don't use Claude's vocabulary.** Codex rules take `allow`, `prompt`, and
`forbidden` — which map onto Claude's `allow`, `ask`, and `deny`. When several rules match, the most
restrictive wins (`forbidden` > `prompt` > `allow`), so `rules/allowlist.rules` allows a binary
broadly and then narrows it with `prompt` rules for the destructive subcommands.

**Compound commands need no special handling.** Codex splits on `&&`, `||`, `;`, and `|` itself and
evaluates each segment against the rules.

**`pattern` is an exact prefix over the argument list**, and any element may be a union of literals —
`pattern=["gh", "pr", ["view", "list"]]` matches `gh pr view` and `gh pr list`.

**Subagent files are generated, so edit the Claude Code side.** Change `.claude/agents/<name>.md`,
then `make agents-sync`. Editing a `.toml` here directly will be reverted by the next sync, and
`make agents-doctor` will flag it as stale in the meantime.
