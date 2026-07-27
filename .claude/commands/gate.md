---
description: Run BareD's real verify gate for whatever changed, and fix what it reports
argument-hint: [go|web|all]  (default: infer from the diff)
allowed-tools: Bash(make:*), Bash(go:*), Bash(gofmt:*), Bash(goimports:*), Bash(golangci-lint:*), Bash(bun run --cwd apps/web:*), Bash(bun install --cwd apps/web), Bash(git status:*), Bash(git diff:*), Read, Edit, Grep, Glob
---

Run the verify gate before this change ships. Scope: **${ARGUMENTS:-infer from the diff}**

## Which gate

`git status --porcelain` / `git diff --stat` first, then run only what's relevant:

| Changed | Command | What it covers |
|---|---|---|
| `**/*.go` | `make pre-commit` | `fmt` → `vet` → `lint` → `test-unit` → `coverage-check` (the coverage ratchet) |
| `apps/web/**` | `make web-validate` | `type-check` → `lint` → `format:check` → `test:run` |
| both | both, backend first | |
| embedding / `go:embed` / release paths | `make build-with-web` as well | binary actually builds with a fresh UI |

> `make validate` is **not** the gate — it only builds and validates `examples/config.example.yml`. Use `make pre-commit`.

> To re-run a single web script instead of the whole `make web-validate`, the form is
> `bun run --cwd apps/web <script>` (Bun's version is pinned in `apps/web/.bun-version`). The flag must come
> **after** `run`: `bun --cwd apps/web run <script>` prints Bun's help and exits **0** without running
> anything — a silent false pass.

> **Coverage is a ratchet.** `coverage-check` compares against `COVERAGE_THRESHOLD` in the `Makefile`, which is set just under the real number (test helpers under `internal/testutil/` are excluded) and is enforced in CI too. It passes on a clean checkout, so if it fails, your diff lowered coverage: add tests for what you changed. Do **not** lower `COVERAGE_THRESHOLD`. If your change lifts coverage well past it, raise it in the same PR.

Integration tests (`make test-integration`) need `make setup-test-env` and Docker; run them only if the change touches a real database/storage path and the user has Docker up.

## Then

1. **Fix what it reports.** Don't just relay failures — diagnose and fix them, then re-run that gate until it's clean. If a failure is pre-existing on `main` and unrelated to the diff, say so explicitly instead of fixing it silently.
2. **Coverage.** If `coverage-check` fails, add the missing tests rather than lowering `COVERAGE_THRESHOLD`.
3. **Report honestly.** State each gate you ran and its result. If you skipped one, say which and why. Paste the actual failing output — never claim a gate passed that you didn't run.

## Finally

If everything is green and the change touches Go under `apps/api/internal/`/`apps/api/cmd/`, delegate to **`go-backend-reviewer`**; if it touches `apps/web/src/`, delegate to **`web-frontend-reviewer`**. Run them in parallel when both apply, then summarize their findings.
