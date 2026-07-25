#!/usr/bin/env bash
#
# PostToolUse hook — auto-format files right after Claude edits them.
#   • Go files          : gofmt -w  +  goimports -w
#   • apps/web TS/TSX/CSS : prettier --write  (web-local binary)
#
# Reads the hook payload (JSON) on stdin, formats the edited file, and always
# exits 0 so it never blocks the session. Missing tools are silently skipped.
set -uo pipefail

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

input="$(cat)"
file="$(hook_json_field "$input" '.tool_input.file_path')"

[ -z "$file" ] && exit 0
[ -f "$file" ] || exit 0

case "$file" in
  *.go)
    command -v gofmt >/dev/null 2>&1 && gofmt -w "$file" 2>/dev/null || true
    if command -v goimports >/dev/null 2>&1; then
      goimports -w "$file" 2>/dev/null || true
    elif [ -x "$HOME/go/bin/goimports" ]; then
      "$HOME/go/bin/goimports" -w "$file" 2>/dev/null || true
    fi
    ;;
  *apps/web/*.ts | *apps/web/*.tsx | *apps/web/*.css)
    prettier="$(hook_repo_root)/apps/web/node_modules/.bin/prettier"
    [ -x "$prettier" ] && "$prettier" --write "$file" >/dev/null 2>&1 || true
    ;;
esac

exit 0
