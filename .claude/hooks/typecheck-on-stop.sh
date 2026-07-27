#!/usr/bin/env bash
#
# Stop hook — ADVISORY TypeScript type-check over the web UI.
#
# lint-on-stop.sh runs eslint, which does not type-check; a change that compiles
# under eslint can still break `bun run build`. This closes that gap. Only runs
# when web TS files actually changed, and always exits 0 so it can never trap
# the session in a loop.
#
# tsc comes from apps/web's lockfile via hook_web_run — see lib.sh.
set -uo pipefail

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# The tree the edits landed in — see lint-on-stop.sh.
cd "$(hook_worktree_root)" 2>/dev/null || exit 0

changed="$(git status --porcelain 2>/dev/null | sed 's/^...//')"
[ -n "$changed" ] || exit 0
printf '%s\n' "$changed" | grep -qE '^apps/web/.*\.(ts|tsx)$' || exit 0

# Silent no-op when deps aren't installed: hook_web_run returns 127 with no output.
out="$( (cd apps/web && hook_web_run tsc --noEmit) 2>&1 || true )"
[ -n "$out" ] || exit 0

printf '\n── BareD typecheck-on-stop (advisory) ─────────────────\n'
printf '%s\n' "$out"
printf "Full output: 'make web-validate'\n"
printf '───────────────────────────────────────────────────────\n'

exit 0
