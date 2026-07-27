#!/usr/bin/env bash
#
# Shared helpers for BareD's agent hooks. Sourced by the scripts in this
# directory, which run under both Claude Code and the Codex CLI, so nothing
# here may assume a client-specific environment.
#
#   . "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# Two roots, and they are not the same thing — pick deliberately.
#
#   hook_repo_root      the *project* the session was opened on. Where installed
#                       tooling lives (apps/web/node_modules). Client env var first.
#   hook_worktree_root  the working tree the command actually runs in. What git
#                       state — branch, status — must be read from.
#
# They diverge inside a `git worktree`: $CLAUDE_PROJECT_DIR keeps pointing at the
# main checkout, so anything deciding on a branch through hook_repo_root judges
# the wrong tree (issue #94). Use hook_worktree_root for git state.

# Project root, whichever client is driving.
hook_repo_root() {
  printf '%s' "${CLAUDE_PROJECT_DIR:-${CODEX_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
}

# Root of the working tree the hook is running in. Resolved from the hook's cwd
# so linked worktrees report themselves; only falls back to the client's project
# dir when cwd is not inside a repository at all.
hook_worktree_root() {
  local top
  top="$(git rev-parse --show-toplevel 2>/dev/null || true)"
  if [ -n "$top" ]; then
    printf '%s' "$top"
    return 0
  fi
  printf '%s' "${CLAUDE_PROJECT_DIR:-${CODEX_PROJECT_DIR:-$(pwd)}}"
}

# hook_json_field <payload> <dotted.path> — echo a string field, or nothing.
# Uses jq when available and falls back to python3; if neither exists the hook
# simply sees an empty value and no-ops rather than failing the turn.
hook_json_field() {
  local payload="$1" path="$2"
  if command -v jq >/dev/null 2>&1; then
    printf '%s' "$payload" | jq -r "${path} // empty" 2>/dev/null || true
  elif command -v python3 >/dev/null 2>&1; then
    printf '%s' "$payload" | python3 -c '
import json, sys
keys = [k for k in sys.argv[1].lstrip(".").split(".") if k]
try:
    cur = json.load(sys.stdin)
    for key in keys:
        cur = cur.get(key) if isinstance(cur, dict) else None
    print("" if cur is None else cur if isinstance(cur, str) else json.dumps(cur))
except Exception:
    print("")
' "$path" 2>/dev/null || true
  fi
}

# hook_web_run <tool> [args...] — run apps/web's locally installed <tool> (the one
# apps/web/bun.lock pinned), or return 127 without output when it cannot be run.
#
# `bun install` still writes real node_modules/.bin/* shims, so the tool is found
# exactly where it always was. But those shims are `#!/usr/bin/env node` scripts,
# and Bun is now the pinned toolchain (apps/web/.bun-version, no .nvmrc), so a
# contributor may legitimately have no `node` at all — running the shim directly
# then dies with `env: node: No such file or directory` (127), which an advisory
# hook would happily print as if it were a lint or type error. So: use node when
# it's there (unchanged behaviour), fall back to Bun's runtime, skip when neither.
#
# Never `bunx`/`npx` here — those may fetch a *different* version than the lockfile
# installed, which would make the hook lie about the state of the project.
hook_web_run() {
  local tool="$1" shim
  shift
  shim="$(hook_repo_root)/apps/web/node_modules/.bin/$tool"
  [ -x "$shim" ] || return 127
  if command -v node >/dev/null 2>&1; then
    "$shim" "$@"
  elif command -v bun >/dev/null 2>&1; then
    bun "$shim" "$@"
  else
    return 127
  fi
}

# hook_is_secret_path <path> — true for files that must never be read, written,
# staged, or echoed by an agent. Mirrors .gitignore and settings.json deny list.
# `*.example.*` templates are explicitly fine.
hook_is_secret_path() {
  local base
  # GitHub's issue-template chooser is named config.yml by GitHub's own rules and
  # contains no credentials. Match on the full path so a real config.yml elsewhere
  # is still caught.
  case "$1" in
    .github/ISSUE_TEMPLATE/config.yml | */.github/ISSUE_TEMPLATE/config.yml) return 1 ;;
  esac
  base="$(basename -- "$1")"
  case "$base" in
    *.example | *.example.* | *example.yml | *example.yaml) return 1 ;;
  esac
  case "$base" in
    config.yml | config.yaml | bared.yml | bared.yaml) return 0 ;;
    *.local.yml | *.local.yaml) return 0 ;;
    .env | .env.*) return 0 ;;
    *.db | *.sqlite | *.sqlite3) return 0 ;;
  esac
  return 1
}

# hook_deny <message...> — refuse the tool call and tell the agent why.
# Exit status 2 is the "block with feedback" contract in both clients.
hook_deny() {
  printf '\n⛔ BareD hook: %s\n' "$1" >&2
  shift
  for line in "$@"; do printf '   %s\n' "$line" >&2; done
  printf '\n' >&2
  exit 2
}
