#!/usr/bin/env bash
#
# PreToolUse hook — CONTRIBUTING.md: "Branch off main with a descriptive name;
# never commit straight to main." This enforces it for commits and pushes.
#
# Escape hatch for the rare legitimate case:  BARED_ALLOW_MAIN_COMMIT=1
set -uo pipefail

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

[ "${BARED_ALLOW_MAIN_COMMIT:-0}" = "1" ] && exit 0

input="$(cat)"
cmd="$(hook_json_field "$input" '.tool_input.command')"
[ -n "$cmd" ] || exit 0

cd "$(hook_repo_root)" 2>/dev/null || exit 0
branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
case "$branch" in
  main | master) ;;
  *) exit 0 ;;
esac

if printf '%s' "$cmd" | grep -qE '(^|[;&|]|\bsudo )[[:space:]]*git[[:space:]]+commit\b'; then
  hook_deny "you are on '$branch' — commit to a branch instead." \
    "Run: git checkout -b <descriptive-name>   then commit." \
    "Override for a genuine exception with BARED_ALLOW_MAIN_COMMIT=1."
fi

if printf '%s' "$cmd" | grep -qE '(^|[;&|]|\bsudo )[[:space:]]*git[[:space:]]+push\b'; then
  hook_deny "refusing to push '$branch' directly — changes land on main via PR." \
    "Push a branch and open a PR (gh pr create)." \
    "Override for a genuine exception with BARED_ALLOW_MAIN_COMMIT=1."
fi

exit 0
