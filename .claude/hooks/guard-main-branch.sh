#!/usr/bin/env bash
#
# PreToolUse hook — CONTRIBUTING.md: "Branch off main with a descriptive name;
# never commit straight to main." This enforces it for commits and pushes.
#
# What it is careful about (issues #94, #147) — a guard with a high
# false-positive rate gets routinely overridden, and then it protects nothing:
#
#   * It judges the working tree the command will actually run in, which is not
#     necessarily the one the hook runs in. A PreToolUse hook runs BEFORE the
#     command and in the session's directory, so `cd <worktree> && git commit`
#     has to be followed: a `cd` moves the directory believed current and the
#     branch is re-read there. `git -C <dir>` does the same for one command —
#     and because it names *another* tree, a `checkout` under it must not change
#     what we believe about this one. $CLAUDE_PROJECT_DIR is never used for git
#     state; it always points at the main checkout.
#   * A push is judged by its REFSPEC, not by the checked-out branch. Pushing a
#     feature branch or a tag is fine from anywhere; pushing main/master — named
#     explicitly, implied by HEAD, or swept up by --all/--mirror — is not.
#   * The command is walked segment by segment (`;`, `&&`, `||`, `|`, newline),
#     tracking branch and directory changes as it goes, so
#     `git checkout -b feat && git commit …` is judged against `feat`.
#   * Only text a shell would EXECUTE is walked. The splitter tracks quoting, so
#     a heredoc body is skipped entirely and punctuation inside a quoted string
#     cannot manufacture a segment — a commit message, a PR body or an issue
#     body that merely discusses committing is data, not a policy violation
#     (issue #147). Quoting is not a way out, though: quotes are removed rather
#     than their contents skipped, so `"git" commit` and `git push origin
#     "main"` are still read as the commands they are, and `$(…)`, backticks and
#     `(…)` are walked as the executable contexts they are — with directory
#     changes inside them scoped to the subshell, as the shell would scope them.
#
# Limitations — all of them the same shape: only what is spelled out in the
# command text is seen.
#
#   * A branch or directory change made inside a script, through `eval`, or
#     through a variable is invisible, and what follows is judged against the
#     branch believed at that point.
#   * A heredoc fed to an interpreter (`bash <<EOF … EOF`) is skipped with every
#     other heredoc body, so a commit made from one is not seen. That is the
#     same blind spot as `eval` and `sh script.sh`, and it is not something an
#     agent does by accident.
#   * When a `cd` or `-C` target cannot be resolved (it does not exist yet, or
#     it is spelled with a variable) the previous belief is kept, which errs
#     towards blocking rather than towards letting a commit onto main.
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

# --- helpers -----------------------------------------------------------------

# Refs that may only be updated through a pull request.
guard_is_protected() {
  case "${1#refs/heads/}" in
    main | master) return 0 ;;
  esac
  return 1
}

# Grouping punctuation an unquoted token may still carry. The splitter already
# removes quotes; this catches the rest.
guard_unwrap() {
  local t="$1"
  t="${t#(}"
  t="${t%)}"
  printf '%s' "$t"
}

# guard_branch_at <dir> — the branch checked out in <dir>, or nothing when it is
# not a repository (an unborn branch still counts, a detached HEAD reports HEAD).
guard_branch_at() {
  git -C "$1" symbolic-ref --quiet --short HEAD 2>/dev/null ||
    git -C "$1" rev-parse --abbrev-ref HEAD 2>/dev/null ||
    true
}

# guard_resolve <path> <base> — absolute <path>, resolved against <base> the way
# the shell would. Fails (and prints nothing) when it does not name a directory.
guard_resolve() {
  local target="$1" base="$2"
  # shellcheck disable=SC2088 # these are patterns; $HOME is expanded by hand
  case "$target" in
    "~") target="$HOME" ;;
    "~/"*) target="$HOME/${target#\~/}" ;;
  esac
  case "$target" in
    /*) ;;
    *) target="$base/$target" ;;
  esac
  (cd "$target" >/dev/null 2>&1 && pwd -P) || return 1
}

guard_deny_commit() {
  hook_deny "you are on '$1' — commit to a branch instead." \
    "Run: git checkout -b <descriptive-name>   as its own command, then commit." \
    "Branch read from the working tree this command runs in: $2." \
    "Compound commands are read left to right, so 'git checkout -b x && git commit' is fine," \
    "and a 'cd' into another worktree is followed; a branch created in a script," \
    "a subshell or a variable is invisible here — split it out." \
    "Override for a genuine exception with BARED_ALLOW_MAIN_COMMIT=1."
}

guard_deny_push() {
  hook_deny "refusing to push $1 — changes land on main via PR." \
    "Push a feature branch or a tag instead, then open a PR (gh pr create)." \
    "Pushing any other ref from this tree is allowed, whatever branch is checked out." \
    "Override for a genuine exception with BARED_ALLOW_MAIN_COMMIT=1."
}

# guard_push <args-after-push...> — deny when the refspec would update main/master.
# $judged is the branch of the tree this push runs in.
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
    guard_is_protected "$judged" && guard_deny_push "'$judged' (the checked-out branch)"
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
    [ "$target" = "HEAD" ] && target="$judged"
    case "$target" in
      refs/tags/* | refs/remotes/*) continue ;;
    esac
    guard_is_protected "$target" && { set +f; guard_deny_push "'${target#refs/heads/}'"; }
  done
  set +f
}

# guard_checkout <args-after-checkout/switch...> — update the branch we believe
# the rest of the command runs on. Only called for the tree we are tracking.
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
    elif git -C "$cwd" rev-parse --verify --quiet "refs/heads/$tok" >/dev/null 2>&1; then
      # a real local branch — anything else is a pathspec, which changes nothing
      effective="$tok"
    fi
    return 0
  done
}

# guard_cd <args-after-cd...> — follow the directory change and re-read the
# branch there. This hook runs before the command, in the session's directory,
# so `cd <worktree> && git commit` is otherwise judged in the wrong tree (#147).
guard_cd() {
  local tok target="" resolved
  while [ "$#" -gt 0 ]; do
    tok="$(guard_unwrap "$1")"
    shift
    case "$tok" in
      --) continue ;;
      -) return 0 ;; # the previous directory, which we have no way to know
      -*) continue ;;
      "") continue ;;
    esac
    target="$tok"
    break
  done
  [ -n "$target" ] || target="$HOME" # a bare `cd` goes home
  # Unresolvable (does not exist yet, or spelled with a variable): keep believing
  # what we believed, which is the conservative direction.
  resolved="$(guard_resolve "$target" "$cwd")" || return 0
  cwd="$resolved"
  # Not a repository: there is no branch here, so there is nothing to protect.
  effective="$(guard_branch_at "$cwd")"
}

# guard_segment <segment> — classify one command in the chain and act on it.
guard_segment() {
  local seg="$1" saw_git=0 sub optdir="" own_tree=1 judged where
  set -f
  # Deliberate word split: the splitter has already reduced this to command text.
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
      cd)
        shift
        guard_cd "$@"
        return 0
        ;;
      git)
        shift
        saw_git=1
        break
        ;;
      *) return 0 ;;
    esac
  done
  [ "$saw_git" -eq 1 ] || return 0

  # git's own options sit before the subcommand. -C is the only one that says
  # which working tree the command operates on; -c and friends are noise here.
  while [ "$#" -gt 0 ]; do
    case "$1" in
      -C)
        [ "$#" -ge 2 ] || return 0
        optdir="$(guard_resolve "$(guard_unwrap "$2")" "${optdir:-$cwd}")" || optdir="?"
        shift 2
        ;;
      -C?*)
        optdir="$(guard_resolve "$(guard_unwrap "${1#-C}")" "${optdir:-$cwd}")" || optdir="?"
        shift
        ;;
      -c | --git-dir | --work-tree | --namespace | --exec-path)
        [ "$#" -ge 2 ] || return 0
        shift 2
        ;;
      -*) shift ;;
      *) break ;;
    esac
  done
  [ "$#" -gt 0 ] || return 0

  judged="$effective"
  where="$cwd"
  if [ -n "$optdir" ] && [ "$optdir" != "$cwd" ]; then
    # Another tree (or one we could not identify) — judge the command there, and
    # never let it change what we believe about the tree we are tracking.
    own_tree=0
    if [ "$optdir" != "?" ]; then
      judged="$(guard_branch_at "$optdir")"
      where="$optdir"
    fi
  fi

  sub="$(guard_unwrap "$1")"
  shift
  case "$sub" in
    checkout | switch) [ "$own_tree" -eq 1 ] && guard_checkout "$@" ;;
    commit) guard_is_protected "$judged" && guard_deny_commit "$judged" "$where" ;;
    push) guard_push "$@" ;;
  esac
  return 0
}

# guard_split — read the command on stdin, print the pieces a shell would run,
# one per line, plus \001( and \001) around each nested execution context.
#
# This is the difference between script and data (issue #147). Splitting the raw
# string on punctuation treats every quoted mention of a git subcommand as a
# command, which is why filing #147 was itself blocked. So: track quoting.
# Quotes are removed rather than their contents dropped — a quoted string is
# still part of the word it sits in, so `"git" commit` and `git push origin
# "main"` stay visible — but separators and newlines inside one lose their
# power, heredoc bodies are dropped whole, and comments are ignored.
guard_split() {
  awk '
    function flush(   t) {
      t = seg
      sub(/^[ \t]+/, "", t)
      sub(/[ \t]+$/, "", t)
      if (t != "") print t
      seg = ""
    }
    function mark(m) { flush(); print "\001" m }
    # Drop the queued heredoc bodies that start at position p; return the
    # position just past them.
    function skiphd(p,   k, e, line) {
      for (k = 1; k <= hdn; k++) {
        while (p <= n) {
          e = index(substr(buf, p), "\n")
          if (e == 0) { line = substr(buf, p); p = n + 1 } else { line = substr(buf, p, e - 1); p = p + e }
          if (hdtab[k]) sub(/^\t+/, "", line)
          if (line == hd[k]) break
        }
      }
      hdn = 0
      return p
    }
    { buf = buf $0 "\n" }
    END {
      n = length(buf); i = 1; depth = 0; st[0] = "U"; hdn = 0
      while (i <= n) {
        c = substr(buf, i, 1)
        s = st[depth]

        # Single quotes: every character is literal, including the closing rule.
        if (s == "S") {
          if (c == "\047") { depth--; i++; continue }
          if (c == "\n") seg = seg " "; else seg = seg c
          i++; continue
        }

        # Double quotes: literal too, except for escapes and substitutions.
        if (s == "D") {
          if (c == "\\") {
            d = substr(buf, i + 1, 1)
            if (d == "\n") { i += 2; continue }
            if (d == "$" || d == "`" || d == "\"" || d == "\\") { seg = seg d; i += 2; continue }
            seg = seg c; i++; continue
          }
          if (c == "\"") { depth--; i++; continue }
          if (c == "$" && substr(buf, i + 1, 1) == "(") { mark("("); depth++; st[depth] = "U"; i += 2; continue }
          if (c == "`") { mark("("); depth++; st[depth] = "B"; i++; continue }
          if (c == "\n") seg = seg " "; else seg = seg c
          i++; continue
        }

        # U (unquoted) and B (backticks): the shell runs this text.
        if (c == "\\") {
          d = substr(buf, i + 1, 1)
          if (d == "\n") { i += 2; continue }
          seg = seg d; i += 2; continue
        }
        # $'…' and $"…" quote a word just like '…' and "…" do.
        if (c == "$" && (substr(buf, i + 1, 1) == "\047" || substr(buf, i + 1, 1) == "\"")) { i++; continue }
        if (c == "\047") { depth++; st[depth] = "S"; i++; continue }
        if (c == "\"") { depth++; st[depth] = "D"; i++; continue }
        if (c == "$" && substr(buf, i + 1, 1) == "(") { mark("("); depth++; st[depth] = "U"; i += 2; continue }
        if (c == "`") {
          if (s == "B") { mark(")"); depth--; i++; continue }
          mark("("); depth++; st[depth] = "B"; i++; continue
        }
        if (c == "(") { mark("("); depth++; st[depth] = "U"; i++; continue }
        if (c == ")") {
          if (depth > 0) { mark(")"); depth--; i++; continue }
          flush(); i++; continue
        }
        if (c == "#" && (i == 1 || substr(buf, i - 1, 1) ~ /[ \t\n;&|(]/)) {
          while (i <= n && substr(buf, i, 1) != "\n") i++
          continue
        }
        # <<WORD / <<-WORD queue a body to drop; <<<WORD is a here-string.
        if (c == "<" && substr(buf, i + 1, 1) == "<" && substr(buf, i + 2, 1) != "<") {
          i += 2
          tab = 0
          if (substr(buf, i, 1) == "-") { tab = 1; i++ }
          while (i <= n && substr(buf, i, 1) ~ /[ \t]/) i++
          delim = ""
          while (i <= n) {
            c2 = substr(buf, i, 1)
            if (c2 ~ /[ \t\n;&|)]/) break
            if (c2 == "\"" || c2 == "\047" || c2 == "\\") { i++; continue }
            delim = delim c2; i++
          }
          if (delim != "") { hdn++; hd[hdn] = delim; hdtab[hdn] = tab }
          seg = seg " "
          continue
        }
        if (c == "\n" || c == ";" || c == "&" || c == "|") {
          flush()
          i++
          if (c == "\n" && hdn > 0) i = skiphd(i)
          continue
        }
        seg = seg c
        i++
      }
      flush()
    }
  '
}

# A nested execution context gets its own directory: `(cd elsewhere && …)` and
# `$(cd elsewhere && …)` do not move the shell that follows them.
guard_scope_push() {
  scope_dir[scope_n]="$cwd"
  scope_branch[scope_n]="$effective"
  scope_n=$((scope_n + 1))
}

guard_scope_pop() {
  [ "$scope_n" -gt 0 ] || return 0
  scope_n=$((scope_n - 1))
  cwd="${scope_dir[scope_n]}"
  effective="${scope_branch[scope_n]}"
}

# --- walk the command --------------------------------------------------------

# Where the command starts out. The hook's own cwd is the session's directory,
# which is what a relative `cd` in the command resolves against; only when that
# is not inside a repository at all do we fall back to the client's project dir.
cwd="$(pwd -P 2>/dev/null || pwd)"
git rev-parse --show-toplevel >/dev/null 2>&1 || cwd="$(hook_worktree_root)"
effective="$(guard_branch_at "$cwd")"
scope_n=0

while IFS= read -r segment; do
  case "$segment" in
    $'\001(') guard_scope_push ;;
    $'\001)') guard_scope_pop ;;
    # Only `git …` and `cd …` change the verdict, and a segment that runs either
    # contains the word. Cheap filter: guard_segment forks a subshell per token,
    # and a long script would otherwise pay for it on every line.
    *git* | *cd*) guard_segment "$segment" ;;
  esac
done < <(printf '%s\n' "$cmd" | guard_split)

exit 0
