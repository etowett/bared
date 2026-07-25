#!/usr/bin/env bash
#
# PostToolUse hook — make sure every file an agent writes ends with a newline.
# gofmt and prettier already do this for Go/TS, so this mostly catches Markdown,
# YAML, and shell. Always exits 0; never blocks.
set -uo pipefail

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

input="$(cat)"
file="$(hook_json_field "$input" '.tool_input.file_path')"

[ -n "$file" ] || exit 0
[ -f "$file" ] || exit 0
[ -s "$file" ] || exit 0

# tail -c1 on a file ending in "\n" yields one line; anything else yields zero.
if [ "$(tail -c1 -- "$file" | wc -l | tr -d ' ')" -eq 0 ]; then
  printf '\n' >>"$file"
fi

exit 0
