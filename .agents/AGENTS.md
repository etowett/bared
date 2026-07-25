# `.agents/` — shared across agent clients

A small directory for things more than one agent client needs to reach. It holds no configuration of
its own.

## What's here

- **`skills`** — a tracked symlink to `../.claude/skills`. Codex (and any other client that scans a
  skills directory) reads BareD's skills through it, so a skill added under `.claude/skills/` is
  immediately available everywhere with no sync step.

That's the whole directory. Client-specific configuration lives with its client:

| Client | Configuration |
|---|---|
| Claude Code | [`.claude/`](../.claude/AGENTS.md) — **canonical** |
| Codex CLI | [`.codex/`](../.codex/AGENTS.md) — mirrors `.claude/` |
| any client | `.mcp.json` at the repo root — the MCP server set |

## Adding another client

1. Give it its own top-level directory (`.cursor/`, `.antigravity/`, …).
2. Point its skills at `.agents/skills` if it supports a skills directory.
3. Mirror `.mcp.json` and `.claude/settings.json` into whatever format it wants — never the other way
   round; `.claude/` stays canonical.
4. Teach `scripts/agents-doctor.py` to check the new mirror, so it can't silently rot. See
   [`.claude/AGENTS.md`](../.claude/AGENTS.md) for the sync map and `.codex/` for a worked example.
