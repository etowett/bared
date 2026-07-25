#!/usr/bin/env bash
#
# Stop hook — ADVISORY TypeScript type-check over the web UI.
#
# lint-on-stop.sh runs eslint, which does not type-check; a change that compiles
# under eslint can still break `npm run build`. This closes that gap. Only runs
# when web TS files actually changed, and always exits 0 so it can never trap
# the session in a loop.
set -uo pipefail

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

cd "$(hook_repo_root)" 2>/dev/null || exit 0

changed="$(git status --porcelain 2>/dev/null | sed 's/^...//')"
[ -n "$changed" ] || exit 0
printf '%s\n' "$changed" | grep -qE '^web/.*\.(ts|tsx)$' || exit 0

tsc="web/node_modules/.bin/tsc"
[ -x "$tsc" ] || exit 0

out="$( (cd web && ./node_modules/.bin/tsc --noEmit) 2>&1 || true )"
[ -n "$out" ] || exit 0

printf '\n── BareD typecheck-on-stop (advisory) ─────────────────\n'
printf '%s\n' "$out"
printf "Full output: 'make web-validate'\n"
printf '───────────────────────────────────────────────────────\n'

exit 0
