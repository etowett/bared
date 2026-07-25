# Open questions — agentic setup for Claude Code + Codex

> Things a human needs to decide. Resolve before implementing; record the answers inline.

- [x] **`make pre-commit` cannot pass today — what do we do about it in this PR?**
      `coverage-check` demands 75%; the repo is at 27.0%. This predates the change (no `.go` file is
      touched here) and it means the gate the guides point agents at always fails, so an agent that
      follows the docs will thrash on it.
      → **Answer:** Leave the threshold alone — changing coverage policy is out of scope for an
      agent-tooling PR. Document the known gap wherever the gate is named so agents don't chase it,
      and track raising coverage in a follow-up issue. The `lint` half of the failure *was* in scope
      and is fixed here.

- [x] **Duplicate the hook scripts for Codex, or share them?**
      Codex hook commands are plain shell strings with no project-directory variable, so the obvious
      port hardcodes an absolute path (which is what the reference setup does, in a gitignored file).
      → **Answer:** Share them. `.codex/config.toml` invokes the `.claude/hooks/*.sh` scripts through
      a `bash -c 'exec "$(git rev-parse --show-toplevel)/…"'` wrapper. One copy of every hook, and the
      config works in any clone or worktree.

- [x] **Map Claude's `model: opus`/`sonnet` onto Codex models when generating `.codex/agents/*.toml`?**
      → **Answer:** No. There is no honest correspondence between the two model sets, so the
      generator omits `model` and Codex uses its own default subagent model. Revisit if the tiers
      start mattering.

- [ ] **Should `web-ci.yml` also run `make agents-doctor`?**
      Today only `ci.yml` does, so a PR that only touches `apps/web/` skips the mirror check. Low risk
      (agent config and `apps/web/` rarely change together) but cheap to add.
      → **Answer:** <deferred — see follow-ups in implementation-notes.md>
