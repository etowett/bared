#!/usr/bin/env bash
#
# Stop hook — fast, ADVISORY lint/vet pass over files changed in the working tree.
# Surfaces problems to the user but never blocks the turn (always exits 0), so it
# can't trap the session in a loop. Runs nothing when no Go/web files changed.
#
#   Go  : gofmt -l (changed) + go vet ./... + golangci-lint run (changed packages)
#   web : eslint (changed files)
#
# Paths are repo-relative; the Go module lives in apps/api and the dashboard in
# apps/web, so the Go and web tools are invoked from inside those directories.
# eslint comes from apps/web's lockfile via hook_web_run — see lib.sh.
set -uo pipefail

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

# The tree the edits landed in: inside a `git worktree` the client's project dir
# still points at the main checkout, whose `git status` is somebody else's.
# (hook_web_run keeps using the project dir — that is where node_modules lives.)
cd "$(hook_worktree_root)" 2>/dev/null || exit 0

changed="$(git status --porcelain 2>/dev/null | sed 's/^...//')"
[ -z "$changed" ] && exit 0

go_changed="$(printf '%s\n' "$changed"  | grep -E '^apps/api/.*\.go$' || true)"
web_changed="$(printf '%s\n' "$changed" | grep -E '^apps/web/.*\.(ts|tsx)$' || true)"

out=""
issues=0

if [ -n "$go_changed" ]; then
  unformatted="$(gofmt -l $go_changed 2>/dev/null || true)"
  [ -n "$unformatted" ] && { out+="• gofmt — needs formatting:\n$unformatted\n"; issues=1; }

  vet="$(go -C apps/api vet ./... 2>&1 || true)"
  [ -n "$vet" ] && { out+="• go vet:\n$vet\n"; issues=1; }

  if command -v golangci-lint >/dev/null 2>&1; then
    pkgs="$(printf '%s\n' "$go_changed" | xargs -n1 dirname 2>/dev/null | sort -u | sed 's#^apps/api/##; s#^#./#')"
    lint="$( (cd apps/api && golangci-lint run $pkgs) 2>&1 || true )"
    [ -n "$lint" ] && { out+="• golangci-lint:\n$lint\n"; issues=1; }
  fi
fi

if [ -n "$web_changed" ]; then
  files_rel="$(printf '%s\n' "$web_changed" | sed 's#^apps/web/##')"
  # Silent no-op when deps aren't installed: hook_web_run returns 127 with no output.
  weblint="$( (cd apps/web && hook_web_run eslint $files_rel) 2>&1 || true )"
  [ -n "$weblint" ] && { out+="• eslint (web):\n$weblint\n"; issues=1; }
fi

if [ "$issues" -eq 1 ]; then
  printf "\n── BareD lint-on-stop (advisory) ──────────────────────\n"
  printf "%b" "$out"
  printf "Full gate: 'make pre-commit' (Go) · 'make web-validate' (web)\n"
  printf "───────────────────────────────────────────────────────\n"
fi

exit 0
