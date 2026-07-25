#!/usr/bin/env bash
#
# PreToolUse hook — BareD handles database credentials, storage keys, and
# encryption keys, so live config and database files are off limits to agents.
#
# settings.json already denies Read() on those paths; this hook closes the other
# doors: writing them, staging them, or shelling out to cat/copy/upload them.
#
# Blocks (exit 2) only on a real violation. Everything else exits 0.
set -uo pipefail

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

input="$(cat)"

# --- File tools: never create or modify a live secret file --------------------
file="$(hook_json_field "$input" '.tool_input.file_path')"
if [ -n "$file" ] && hook_is_secret_path "$file"; then
  hook_deny "refusing to write $(basename -- "$file") — it holds live credentials." \
    "Edit examples/config.example.yml instead, or ask the user to make the change by hand." \
    "See 'Safety & security' in AGENTS.md."
fi

# --- Bash: block reading, staging, or shipping a secret file ------------------
cmd="$(hook_json_field "$input" '.tool_input.command')"
[ -n "$cmd" ] || exit 0

# A heredoc body is document text, not arguments — `cat > notes.md <<EOF` followed
# by prose must not be scanned. Drop everything from the first heredoc operator.
cmd="${cmd%%<<*}"

# Only commands that actually exfiltrate or stage content are interesting; a
# grep for the word "config.yml" in the docs is not a leak.
printf '%s' "$cmd" | grep -qE '(^|[;&|]|\bsudo )[[:space:]]*(cat|head|tail|less|more|bat|xxd|od|strings|base64|cp|scp|rsync|curl|git[[:space:]]+add)\b' || exit 0

# Globbing stays OFF for the token scan. Letting the shell expand these tokens
# would make the hook's verdict depend on the contents of the working directory:
# a bare `*` anywhere in the command — including inside quoted prose — expands to
# every file in the repo root and matches bared.db, blocking unrelated work.
# Glob *patterns* are still caught, by inspecting the pattern itself below.
set -f

for token in $cmd; do
  case "$token" in
    -*) continue ;;
  esac

  if hook_is_secret_path "$token"; then
    hook_deny "refusing to run a command that touches $(basename -- "$token")." \
      "That file holds live credentials or backup data and is gitignored for a reason." \
      "Use examples/config.example.yml, or ask the user to run it themselves."
  fi

  # `cat *.yml` / `cp conf?.db` never name the file, but would still expose one.
  # Judge the pattern, not what it happens to expand to right now.
  case "$token" in
    *[*?]*)
      case "$token" in
        *.yml | *.yaml | *.db | *.sqlite | *.sqlite3 | *.env | .env*)
          hook_deny "refusing to run a glob that could match a secret file ($token)." \
            "Name the specific file you need, or use examples/config.example.yml."
          ;;
      esac
      ;;
  esac
done

exit 0
