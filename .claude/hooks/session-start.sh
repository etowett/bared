#!/usr/bin/env bash
#
# SessionStart hook — a few lines of orientation, printed into the agent's
# context at the top of a session: where we are in git, and whether the
# toolchain the verify gate needs is actually installed.
#
# Deliberately terse: this is a standing context cost on every session.
set -uo pipefail

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

cd "$(hook_repo_root)" 2>/dev/null || exit 0

branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
dirty="$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')"

missing=""
for tool in go golangci-lint node; do
  command -v "$tool" >/dev/null 2>&1 || missing="$missing $tool"
done
[ -x web/node_modules/.bin/eslint ] || missing="$missing web/node_modules(npm --prefix web ci)"

printf 'BareD · branch %s · %s uncommitted file(s)\n' "$branch" "$dirty"
case "$branch" in
  main | master) printf 'On %s — branch before committing (CONTRIBUTING.md); the guard hook enforces it.\n' "$branch" ;;
esac
[ -n "$missing" ] && printf 'Missing from PATH:%s — the verify gate will fail until installed.\n' "$missing"
printf 'Verify gate: make pre-commit (Go) · make web-validate (web). Read the nested AGENTS.md for the area you touch.\n'

exit 0
