#!/usr/bin/env bash
#
# Stop hook — fast, ADVISORY lint/vet pass over files changed in the working tree.
# Surfaces problems to the user but never blocks the turn (always exits 0), so it
# can't trap the session in a loop. Runs nothing when no Go/web files changed.
#
#   Go  : gofmt -l (changed) + go vet ./... + golangci-lint run (changed packages)
#   web : eslint (changed files)
set -uo pipefail

root="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$root" 2>/dev/null || exit 0

changed="$(git status --porcelain 2>/dev/null | sed 's/^...//')"
[ -z "$changed" ] && exit 0

go_changed="$(printf '%s\n' "$changed"  | grep -E '\.go$' || true)"
web_changed="$(printf '%s\n' "$changed" | grep -E '^web/.*\.(ts|tsx)$' || true)"

out=""
issues=0

if [ -n "$go_changed" ]; then
  unformatted="$(gofmt -l $go_changed 2>/dev/null || true)"
  [ -n "$unformatted" ] && { out+="• gofmt — needs formatting:\n$unformatted\n"; issues=1; }

  vet="$(go vet ./... 2>&1 || true)"
  [ -n "$vet" ] && { out+="• go vet:\n$vet\n"; issues=1; }

  if command -v golangci-lint >/dev/null 2>&1; then
    pkgs="$(printf '%s\n' "$go_changed" | xargs -n1 dirname 2>/dev/null | sort -u | sed 's#^#./#')"
    lint="$(golangci-lint run $pkgs 2>&1 || true)"
    [ -n "$lint" ] && { out+="• golangci-lint:\n$lint\n"; issues=1; }
  fi
fi

if [ -n "$web_changed" ] && [ -x web/node_modules/.bin/eslint ]; then
  files_rel="$(printf '%s\n' "$web_changed" | sed 's#^web/##')"
  weblint="$( (cd web && ./node_modules/.bin/eslint $files_rel) 2>&1 || true )"
  [ -n "$weblint" ] && { out+="• eslint (web):\n$weblint\n"; issues=1; }
fi

if [ "$issues" -eq 1 ]; then
  printf "\n── BareD lint-on-stop (advisory) ──────────────────────\n"
  printf "%b" "$out"
  printf "Full output: 'make validate' (Go) · 'make web-validate' (web)\n"
  printf "───────────────────────────────────────────────────────\n"
fi

exit 0
