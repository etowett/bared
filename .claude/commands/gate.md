---
description: Run BareD's real verify gate for whatever changed, and fix what it reports
argument-hint: [go|web|all]  (default: infer from the diff)
allowed-tools: Bash(make:*), Bash(go:*), Bash(gofmt:*), Bash(goimports:*), Bash(golangci-lint:*), Bash(npm --prefix web run:*), Bash(git status:*), Bash(git diff:*), Read, Edit, Grep, Glob
---

Run the verify gate before this change ships. Scope: **${ARGUMENTS:-infer from the diff}**

## Which gate

`git status --porcelain` / `git diff --stat` first, then run only what's relevant:

| Changed | Command | What it covers |
|---|---|---|
| `**/*.go` | `make pre-commit` | `fmt` → `vet` → `lint` → `test-unit` → `coverage-check` (75% threshold) |
| `web/**` | `make web-validate` | `type-check` → `lint` → `format:check` → `test:run` |
| both | both, backend first | |
| embedding / `go:embed` / release paths | `make build-with-web` as well | binary actually builds with a fresh UI |

> `make validate` is **not** the gate — it only builds and validates `examples/config.example.yml`. Use `make pre-commit`.

> **Known pre-existing failure.** `make pre-commit` ends in `coverage-check`, which demands 75% while the repo is at ~27% (`cmd/brd`, `internal/client`, `internal/configservice` have no tests). Everything before it — `fmt` → `vet` → `lint` → `test-unit` — must pass, and that's what you're responsible for. If `coverage-check` is the *only* failure and your diff didn't lower coverage, say so and move on. Do **not** start writing tests for untouched packages, and do **not** lower the threshold.

Integration tests (`make test-integration`) need `make setup-test-env` and Docker; run them only if the change touches a real database/storage path and the user has Docker up.

## Then

1. **Fix what it reports.** Don't just relay failures — diagnose and fix them, then re-run that gate until it's clean. If a failure is pre-existing on `main` and unrelated to the diff, say so explicitly instead of fixing it silently.
2. **Coverage.** If `coverage-check` fails, add the missing tests rather than lowering the threshold.
3. **Report honestly.** State each gate you ran and its result. If you skipped one, say which and why. Paste the actual failing output — never claim a gate passed that you didn't run.

## Finally

If everything is green and the change touches Go under `internal/`/`cmd/`, delegate to **`go-backend-reviewer`**; if it touches `web/src/`, delegate to **`web-frontend-reviewer`**. Run them in parallel when both apply, then summarize their findings.
