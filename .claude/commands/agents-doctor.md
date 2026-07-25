---
description: Check the Claude Code ⇄ Codex agent config mirrors and repair any drift
argument-hint: [--fix]
allowed-tools: Bash(make agents-doctor), Bash(make agents-sync), Bash(./scripts/agents-doctor.py:*), Bash(ls:*), Bash(cat:*), Read, Edit, Write, Grep, Glob
---

Audit this repo's agent configuration. `.claude/` + `.mcp.json` are canonical; `.codex/` mirrors them. See `.claude/AGENTS.md` for the sync map.

## 1. Run the doctor

```bash
./scripts/agents-doctor.py
```

With `--fix` in $ARGUMENTS, run `./scripts/agents-doctor.py --fix` — it regenerates `.codex/agents/*.toml` from `.claude/agents/*.md` and re-marks hook scripts executable, but it will **not** invent mirror content that needs judgement.

## 2. Repair what it can't

The doctor reports drift; you resolve it, always by editing the **mirror**, never by weakening the canonical side:

- **MCP server missing from `.codex/config.toml`** — add the matching `[mcp_servers.<name>]` block from `.mcp.json`. Remote servers use `url`; stdio servers use `command` + `args`.
- **Allowlist binary not covered by `.codex/rules/allowlist.rules`** — add `prefix_rule(pattern=["<binary>"], decision="allow")`, or `"prompt"` if it's in Claude's `ask` list. `pattern` is an exact prefix over the argument list and any element may be a union (`["gh", "pr", ["view", "list"]]`); the most restrictive matching decision wins.
- **Hook mirrored under one client only** — add or remove the matching entry in the `[hooks]` table of `.codex/config.toml`. Keep pointing at the shared `.claude/hooks/*.sh` script; never copy the script.
- **Stale or missing `.codex/agents/*.toml`** — these are generated; run `make agents-sync`. If the content is wrong, fix `.claude/agents/<name>.md` and sync again.
- **Absolute path in a config file** — replace it with a repo-relative path.
- **Hook registered in `settings.json` but missing/not executable** — create it or `chmod +x` it.

## 3. Verify

Re-run `./scripts/agents-doctor.py` until it exits 0, then report what drifted, what you changed, and anything you deliberately left alone.
