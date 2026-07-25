#!/usr/bin/env python3
"""Check (and repair) BareD's agent configuration.

`.claude/` plus `.mcp.json` are canonical; `.codex/` mirrors them. This script is
what stops the mirror from rotting: it is wired into `make agents-doctor` and CI,
so drift fails the PR instead of silently degrading the Codex setup.

    ./scripts/agents-doctor.py          check only, exit 1 on any problem
    ./scripts/agents-doctor.py --fix    regenerate what can be generated, then check

Only what is genuinely derivable is regenerated (`.codex/agents/*.toml`, the
executable bit on hooks). Mirrors that need judgement — MCP servers, the command
allowlist — are reported for a human or agent to resolve.

Standard library only, and no tomllib: this has to run on whatever python3 a
contributor happens to have.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

CLAUDE = ROOT / ".claude"
CODEX = ROOT / ".codex"
MCP_JSON = ROOT / ".mcp.json"
SETTINGS = CLAUDE / "settings.json"
CODEX_CONFIG = CODEX / "config.toml"
CODEX_RULES = CODEX / "rules" / "allowlist.rules"

# Sourced from .claude/AGENTS.md — a skill description is a standing context cost
# in every session of every client sharing .claude/skills.
MAX_SKILL_DESCRIPTION = 300

problems: list[str] = []
fixes: list[str] = []
notes: list[str] = []


def problem(check: str, msg: str, remedy: str = "") -> None:
    problems.append(f"{check}: {msg}" + (f"\n      → {remedy}" if remedy else ""))


def rel(path: Path) -> str:
    return str(path.relative_to(ROOT))


# --------------------------------------------------------------------------
# Parsing helpers
# --------------------------------------------------------------------------


def read_json(path: Path):
    try:
        return json.loads(path.read_text())
    except FileNotFoundError:
        problem("config", f"{rel(path)} is missing")
    except json.JSONDecodeError as exc:
        problem("config", f"{rel(path)} is not valid JSON: {exc}")
    return None


def parse_frontmatter(path: Path) -> tuple[dict, str]:
    """Split a `---` YAML frontmatter block from the body.

    Deliberately a flat `key: value` parser — the agent, skill, and command files
    in this repo use scalar frontmatter only, and depending on PyYAML here would
    make the doctor unrunnable on a clean machine.
    """
    text = path.read_text()
    if not text.startswith("---"):
        return {}, text
    end = text.find("\n---", 3)
    if end == -1:
        return {}, text
    block, body = text[3:end], text[end + 4 :]
    meta: dict[str, str] = {}
    for line in block.splitlines():
        line = line.rstrip()
        if not line.strip() or line.lstrip().startswith("#") or ":" not in line:
            continue
        if line[0].isspace():  # nested value; not used by this repo
            continue
        key, _, value = line.partition(":")
        meta[key.strip()] = value.strip().strip("'\"")
    return meta, body.lstrip("\n")


# --------------------------------------------------------------------------
# Codex subagent generation
# --------------------------------------------------------------------------


def toml_basic_string(value: str) -> str:
    escaped = value.replace("\\", "\\\\").replace('"', '\\"')
    return f'"{escaped}"'


def toml_multiline(value: str) -> str:
    # A multi-line basic string: escape backslashes, then any `"""` that would
    # close it early. Trailing newline is trimmed so the value round-trips.
    escaped = value.replace("\\", "\\\\").replace('"""', '\\"\\"\\"')
    return '"""\n' + escaped.rstrip("\n") + '\n"""'


def render_codex_agent(md_path: Path) -> str:
    meta, body = parse_frontmatter(md_path)
    name = meta.get("name", md_path.stem)
    description = meta.get("description", "")
    header = (
        "# GENERATED from .claude/agents/{src} by scripts/agents-doctor.py — do not edit.\n"
        "# Change the Claude Code subagent, then run `make agents-sync`.\n"
        "# Codex subagent reference: https://learn.chatgpt.com/docs/agent-configuration/subagents\n"
    ).format(src=md_path.name)
    return (
        header
        + f"\nname = {toml_basic_string(name)}\n"
        + f"description = {toml_basic_string(description)}\n"
        + f"developer_instructions = {toml_multiline(body)}\n"
    )


def check_subagent_mirrors(fix: bool) -> None:
    agents_dir = CLAUDE / "agents"
    codex_agents = CODEX / "agents"
    if not agents_dir.is_dir():
        problem("subagents", f"{rel(agents_dir)} is missing")
        return
    codex_agents.mkdir(parents=True, exist_ok=True)

    expected = {}
    for md in sorted(agents_dir.glob("*.md")):
        meta, body = parse_frontmatter(md)
        if not meta.get("name"):
            problem("subagents", f"{rel(md)} has no `name` in its frontmatter")
        if not meta.get("description"):
            problem("subagents", f"{rel(md)} has no `description` in its frontmatter")
        if not body.strip():
            problem("subagents", f"{rel(md)} has an empty body")
        if meta.get("name") and meta["name"] != md.stem:
            problem(
                "subagents",
                f"{rel(md)} declares name '{meta['name']}' but is filed as '{md.stem}.md'",
                "Rename the file or the `name` field so they agree.",
            )
        expected[md.stem] = render_codex_agent(md)

    for stem, content in expected.items():
        target = codex_agents / f"{stem}.toml"
        current = target.read_text() if target.exists() else None
        if current == content:
            continue
        if fix:
            target.write_text(content)
            fixes.append(f"regenerated {rel(target)}")
        else:
            what = "is missing" if current is None else "is stale"
            problem(
                "subagents",
                f"{rel(target)} {what} relative to {rel(agents_dir / (stem + '.md'))}",
                "Run `make agents-sync`.",
            )

    for orphan in sorted(codex_agents.glob("*.toml")):
        if orphan.stem not in expected:
            if fix:
                orphan.unlink()
                fixes.append(f"removed orphaned {rel(orphan)}")
            else:
                problem(
                    "subagents",
                    f"{rel(orphan)} has no counterpart in {rel(agents_dir)}",
                    "Run `make agents-sync` to drop it, or restore the Claude Code subagent.",
                )


# --------------------------------------------------------------------------
# Checks
# --------------------------------------------------------------------------


def check_skills_symlink() -> None:
    link = ROOT / ".agents" / "skills"
    if not link.is_symlink():
        problem(
            "skills",
            f"{rel(link)} is not a symlink",
            "Run: ln -s ../.claude/skills .agents/skills",
        )
        return
    if link.resolve() != (CLAUDE / "skills").resolve():
        problem(
            "skills",
            f"{rel(link)} points at {os.readlink(link)}, expected ../.claude/skills",
        )


def check_skills() -> None:
    skills_dir = CLAUDE / "skills"
    if not skills_dir.is_dir():
        problem("skills", f"{rel(skills_dir)} is missing")
        return
    for skill in sorted(p for p in skills_dir.iterdir() if p.is_dir()):
        md = skill / "SKILL.md"
        if not md.exists():
            problem("skills", f"{rel(skill)} has no SKILL.md")
            continue
        meta, body = parse_frontmatter(md)
        if not meta.get("name"):
            problem("skills", f"{rel(md)} has no `name` in its frontmatter")
        elif meta["name"] != skill.name:
            problem(
                "skills",
                f"{rel(md)} declares name '{meta['name']}' but lives in '{skill.name}/'",
            )
        description = meta.get("description", "")
        if not description:
            problem("skills", f"{rel(md)} has no `description` — the model cannot route to it")
        elif len(description) > MAX_SKILL_DESCRIPTION:
            problem(
                "skills",
                f"{rel(md)} description is {len(description)} chars (max {MAX_SKILL_DESCRIPTION})",
                "It is a routing hint, not documentation — move the detail into the body. See .claude/AGENTS.md.",
            )
        if not body.strip():
            problem("skills", f"{rel(md)} has an empty body")


def check_commands() -> None:
    commands_dir = CLAUDE / "commands"
    if not commands_dir.is_dir():
        notes.append("no .claude/commands/ directory — slash commands are optional")
        return
    for cmd in sorted(commands_dir.glob("*.md")):
        meta, body = parse_frontmatter(cmd)
        if not meta.get("description"):
            problem("commands", f"{rel(cmd)} has no `description` — it will be unlabelled in /help")
        if not body.strip():
            problem("commands", f"{rel(cmd)} has an empty body")


def hook_scripts_referenced_by_settings(settings: dict) -> set[str]:
    found: set[str] = set()
    for groups in (settings.get("hooks") or {}).values():
        for group in groups:
            for hook in group.get("hooks", []):
                for name in re.findall(r"\.claude/hooks/([A-Za-z0-9_.-]+\.sh)", hook.get("command", "")):
                    found.add(name)
    return found


def check_hooks(settings: dict, codex_config: str, fix: bool) -> None:
    hooks_dir = CLAUDE / "hooks"
    if not hooks_dir.is_dir():
        problem("hooks", f"{rel(hooks_dir)} is missing")
        return

    on_disk = {p.name for p in hooks_dir.glob("*.sh")}
    library = {"lib.sh"}
    registered_claude = hook_scripts_referenced_by_settings(settings)
    registered_codex = set(re.findall(r"\.claude/hooks/([A-Za-z0-9_.-]+\.sh)", codex_config))

    for name in sorted(registered_claude - on_disk):
        problem("hooks", f".claude/settings.json registers {name}, which does not exist")

    for name in sorted(on_disk - library - registered_claude):
        problem(
            "hooks",
            f"{name} exists but is not registered in .claude/settings.json",
            "Register it, or delete it — an unregistered hook never runs.",
        )

    for name in sorted(registered_claude - registered_codex):
        problem(
            "hooks",
            f"{name} runs under Claude Code but not Codex",
            "Mirror it into the [hooks] table in .codex/config.toml.",
        )
    for name in sorted(registered_codex - registered_claude):
        problem("hooks", f".codex/config.toml registers {name}, which .claude/settings.json does not")

    for name in sorted(on_disk):
        script = hooks_dir / name
        if os.access(script, os.X_OK):
            continue
        if fix:
            script.chmod(script.stat().st_mode | 0o111)
            fixes.append(f"chmod +x {rel(script)}")
        else:
            problem("hooks", f"{rel(script)} is not executable", f"Run: chmod +x {rel(script)}")


def check_mcp_mirror(codex_config: str) -> None:
    mcp = read_json(MCP_JSON)
    if mcp is None:
        return
    claude_servers = set((mcp.get("mcpServers") or {}).keys())
    codex_servers = set(re.findall(r"^\[mcp_servers\.([A-Za-z0-9_-]+)\]", codex_config, re.M))

    for name in sorted(claude_servers - codex_servers):
        problem(
            "mcp",
            f"server '{name}' is in .mcp.json but not in .codex/config.toml",
            f"Add a [mcp_servers.{name}] block mirroring .mcp.json.",
        )
    for name in sorted(codex_servers - claude_servers):
        problem(
            "mcp",
            f"server '{name}' is in .codex/config.toml but not in .mcp.json",
            "Remove it, or add it to .mcp.json so both clients see it.",
        )


def allowlist_binaries(settings: dict) -> set[str]:
    """First token of every Bash() permission entry, allow + ask alike."""
    binaries: set[str] = set()
    perms = settings.get("permissions") or {}
    for bucket in ("allow", "ask"):
        for entry in perms.get(bucket, []):
            match = re.match(r"^Bash\((.+)\)$", entry)
            if not match:
                continue
            first = match.group(1).strip().split()[0]
            first = first.rstrip(":*")
            if first:
                binaries.add(first)
    return binaries


def check_allowlist_mirror(settings: dict) -> None:
    if not CODEX_RULES.exists():
        problem("rules", f"{rel(CODEX_RULES)} is missing")
        return
    rules = CODEX_RULES.read_text()
    covered = set()
    for pattern in re.findall(r"pattern\s*=\s*\[(.*?)\]\s*,?\s*\n?\s*decision", rules, re.S):
        # The first element of the pattern is the binary (or a union of them).
        head = pattern.strip()
        for literal in re.findall(r'"([^"]+)"', head):
            covered.add(literal)

    for binary in sorted(allowlist_binaries(settings)):
        if binary in covered:
            continue
        problem(
            "rules",
            f"'{binary}' is permitted in .claude/settings.json but has no rule in {rel(CODEX_RULES)}",
            f'Add: prefix_rule(pattern=["{binary}"], decision="allow")  (or "prompt" for the ask list)',
        )


def check_no_absolute_paths() -> None:
    """A machine-specific path in shared config breaks every other contributor."""
    targets: list[Path] = []
    for base, patterns in (
        (CLAUDE, ("agents/*.md", "commands/*.md", "skills/*/SKILL.md", "hooks/*.sh", "AGENTS.md", "README.md")),
        (CODEX, ("*.toml", "agents/*.toml", "rules/*.rules", "AGENTS.md")),
    ):
        for pattern in patterns:
            targets.extend(base.glob(pattern))
    targets.append(SETTINGS)
    targets.extend(ROOT.glob("*.md"))

    bad = re.compile(r"(?<!\.)/(?:Users|home)/[A-Za-z0-9._-]+/")
    for path in targets:
        if not path.is_file():
            continue
        for lineno, line in enumerate(path.read_text(errors="replace").splitlines(), 1):
            if bad.search(line) and "/Users/..." not in line:
                problem(
                    "portability",
                    f"{rel(path)}:{lineno} contains a machine-specific absolute path",
                    "Use a repo-relative path.",
                )


def check_codex_docs() -> None:
    for path in (CODEX / "AGENTS.md", CLAUDE / "AGENTS.md", ROOT / ".agents" / "AGENTS.md"):
        if not path.exists():
            problem("docs", f"{rel(path)} is missing")


# --------------------------------------------------------------------------


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--fix", action="store_true", help="regenerate derivable config before checking")
    args = parser.parse_args()

    settings = read_json(SETTINGS) or {}
    codex_config = CODEX_CONFIG.read_text() if CODEX_CONFIG.exists() else ""
    if not codex_config:
        problem("config", f"{rel(CODEX_CONFIG)} is missing — Codex gets no project configuration")

    check_skills_symlink()
    check_skills()
    check_commands()
    check_subagent_mirrors(args.fix)
    check_hooks(settings, codex_config, args.fix)
    check_mcp_mirror(codex_config)
    check_allowlist_mirror(settings)
    check_no_absolute_paths()
    check_codex_docs()

    for fix in fixes:
        print(f"  fixed  {fix}")
    for note in notes:
        print(f"  note   {note}")

    if problems:
        print(f"\nagents-doctor: {len(problems)} problem(s)\n", file=sys.stderr)
        for item in problems:
            print(f"  ✗ {item}", file=sys.stderr)
        print(
            "\nSee .claude/AGENTS.md for the sync map. `make agents-sync` regenerates what it can.",
            file=sys.stderr,
        )
        return 1

    print("agents-doctor: agent configuration is consistent ✓")
    return 0


if __name__ == "__main__":
    sys.exit(main())
