#!/usr/bin/env bash
#
# PostToolUse hook — auto-format files right after Claude edits them.
#   • Go files          : gofmt -w  +  goimports -w
#   • web/ TS/TSX/CSS    : prettier --write  (web-local binary)
#
# Reads the hook payload (JSON) on stdin, formats the edited file, and always
# exits 0 so it never blocks the session. Missing tools are silently skipped.
set -uo pipefail

input="$(cat)"

# Pull tool_input.file_path out of the hook payload.
file="$(printf '%s' "$input" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
    print(d.get("tool_input", {}).get("file_path", "") or "")
except Exception:
    print("")
' 2>/dev/null || true)"

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
  *web/*.ts | *web/*.tsx | *web/*.css)
    root="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
    prettier="$root/web/node_modules/.bin/prettier"
    [ -x "$prettier" ] && "$prettier" --write "$file" >/dev/null 2>&1 || true
    ;;
esac

exit 0
