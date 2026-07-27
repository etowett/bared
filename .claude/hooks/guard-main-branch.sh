#!/usr/bin/env bash
#
# PreToolUse hook — CONTRIBUTING.md: "Branch off main with a descriptive name;
# never commit straight to main." This enforces it for commits and pushes.
#
# Three things it is careful about (issue #94) — a guard with a high
# false-positive rate gets routinely overridden, and then it protects nothing:
#
#   * It judges the working tree the command will actually run in, resolved from
#     the hook's cwd. $CLAUDE_PROJECT_DIR always points at the main checkout, so
#     reading the branch through it denies every commit made from a `git worktree`
#     while the main checkout happens to sit on main.
#   * A push is judged by its REFSPEC, not by the checked-out branch. Pushing a
#     feature branch or a tag is fine from anywhere; pushing main/master — named
#     explicitly, implied by HEAD, or swept up by --all/--mirror — is not.
#   * The command is walked segment by segment (`;`, `&&`, `||`, `|`), tracking
#     branch changes as it goes, so `git checkout -b feat && git commit …` is
#     judged against `feat` rather than against the branch we started on.
#
# Limitation: only branch switches spelled out in the command itself are seen. A
# branch created inside a script, a subshell, or through a variable is invisible
# here, and the commit that follows is judged against the current branch.
#
# Escape hatch for the rare legitimate case: BARED_ALLOW_MAIN_COMMIT=1, either
# exported into the session or written as a prefix on the command itself —
# hooks run in their own process, so a prefix never reaches this script's
# environment and has to be read out of the command line.
set -uo pipefail

. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

[ "${BARED_ALLOW_MAIN_COMMIT:-0}" = "1" ] && exit 0

input="$(cat)"
cmd="$(hook_json_field "$input" '.tool_input.command')"
[ -n "$cmd" ] || exit 0

cd "$(hook_worktree_root)" 2>/dev/null || exit 0
branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"

# --- helpers -----------------------------------------------------------------

# Refs that may only be updated through a pull request.
guard_is_protected() {
  case "${1#refs/heads/}" in
    main | master) return 0 ;;
  esac
  return 1
}

# Shell quoting and grouping punctuation an unquoted token may still carry.
guard_unwrap() {
  local t="$1"
  t="${t#(}"
  t="${t%)}"
  t="${t#\"}"
  t="${t%\"}"
  t="${t#\'}"
  t="${t%\'}"
  printf '%s' "$t"
}

guard_deny_commit() {
  hook_deny "you are on '$1' — commit to a branch instead." \
    "Run: git checkout -b <descriptive-name>   as its own command, then commit." \
    "Branch read from the working tree this runs in: $(pwd)." \
    "Compound commands are read left to right, so 'git checkout -b x && git commit' is fine;" \
    "a branch created in a script or subshell is invisible here — split it out." \
    "Override for a genuine exception with BARED_ALLOW_MAIN_COMMIT=1."
}

guard_deny_push() {
  hook_deny "refusing to push $1 — changes land on main via PR." \
    "Push a feature branch or a tag instead, then open a PR (gh pr create)." \
    "Pushing any other ref from this tree is allowed, whatever branch is checked out." \
    "Override for a genuine exception with BARED_ALLOW_MAIN_COMMIT=1."
}

# guard_push <args-after-push...> — deny when the refspec would update main/master.
guard_push() {
  local tok want_value=0 remote_seen=0 bulk=0 refs="" ref target
  while [ "$#" -gt 0 ]; do
    tok="$(guard_unwrap "$1")"
    shift
    if [ "$want_value" -eq 1 ]; then
      want_value=0
      continue
    fi
    case "$tok" in
      --all | --mirror) bulk=1 ;;
      -o | --push-option | --repo | --receive-pack | --exec) want_value=1 ;;
      -*) ;;
      "") ;;
      *)
        # first bare word is the remote, everything after it is a refspec
        if [ "$remote_seen" -eq 0 ]; then
          remote_seen=1
        else
          refs="$refs $tok"
        fi
        ;;
    esac
  done

  [ "$bulk" -eq 1 ] && guard_deny_push "with --all/--mirror, which includes main"

  # No refspec: git pushes the current branch.
  if [ -z "${refs// /}" ]; then
    guard_is_protected "$effective" && guard_deny_push "'$effective' (the checked-out branch)"
    return 0
  fi

  set -f
  for ref in $refs; do
    ref="${ref#+}" # +src:dst — a forced update is still just an update
    case "$ref" in
      *:*)
        target="${ref#*:}" # `:branch` deletes the remote branch; still an update to it
        [ -n "$target" ] || target="${ref%%:*}"
        ;;
      *) target="$ref" ;;
    esac
    [ "$target" = "HEAD" ] && target="$effective"
    case "$target" in
      refs/tags/* | refs/remotes/*) continue ;;
    esac
    guard_is_protected "$target" && { set +f; guard_deny_push "'${target#refs/heads/}'"; }
  done
  set +f
}

# guard_checkout <args-after-checkout/switch...> — update the branch we believe
# the rest of the command runs on.
guard_checkout() {
  local tok create=0
  while [ "$#" -gt 0 ]; do
    tok="$(guard_unwrap "$1")"
    shift
    case "$tok" in
      -b | -B | -c | -C | --create | --force-create)
        create=1
        continue
        ;;
      --) return 0 ;;
      -*) continue ;;
      "") continue ;;
    esac
    if [ "$create" -eq 1 ]; then
      effective="$tok"
    elif git rev-parse --verify --quiet "refs/heads/$tok" >/dev/null 2>&1; then
      # a real local branch — anything else is a pathspec, which changes nothing
      effective="$tok"
    fi
    return 0
  done
}

# guard_segment <segment> — classify one command in the chain and act on it.
guard_segment() {
  local seg="$1" saw_git=0 sub
  set -f
  # Deliberate word split: this is command text, not a value.
  # shellcheck disable=SC2086
  set -- $seg
  set +f

  while [ "$#" -gt 0 ]; do
    case "$(guard_unwrap "$1")" in
      sudo | command | env | time | nohup) shift ;;
      # The documented escape hatch, spelled the way it is actually used. A hook
      # runs in its own process, so an env prefix on the command never reaches
      # the check at the top of this file — read it off the command line too.
      BARED_ALLOW_MAIN_COMMIT=1) return 0 ;;
      *=*) shift ;; # any other leading VAR=value assignment
      git) shift; saw_git=1; break ;;
      *) return 0 ;;
    esac
  done
  [ "$saw_git" -eq 1 ] || return 0

  # git's own options sit before the subcommand
  while [ "$#" -gt 0 ]; do
    case "$1" in
      -C | -c | --git-dir | --work-tree | --namespace | --exec-path)
        [ "$#" -ge 2 ] || return 0
        shift 2
        ;;
      -*) shift ;;
      *) break ;;
    esac
  done
  [ "$#" -gt 0 ] || return 0

  sub="$(guard_unwrap "$1")"
  shift
  case "$sub" in
    checkout | switch) guard_checkout "$@" ;;
    commit) guard_is_protected "$effective" && guard_deny_commit "$effective" ;;
    push) guard_push "$@" ;;
  esac
  return 0
}

# --- walk the command --------------------------------------------------------

effective="$branch"
while IFS= read -r segment; do
  case "$segment" in
    *git*) guard_segment "$segment" ;;
  esac
done < <(printf '%s\n' "$cmd" | awk '{ gsub(/[[:space:]]*(\|\||&&|;|\||&)[[:space:]]*/, "\n"); print }')

exit 0
