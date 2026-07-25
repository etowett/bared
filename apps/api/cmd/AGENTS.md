# CLI (brd) — Agent Guide

> Scope: the `brd` command-line binary built from `apps/api/cmd/brd/` (Cobra commands, including `config import`). Part of the BareD AGENTS.md tree — see the root [`AGENTS.md`](../../../AGENTS.md) and the backend guide [`apps/api/internal/AGENTS.md`](../internal/AGENTS.md). **The innermost guide wins** when instructions conflict.

`brd` is the single entrypoint for BareD: it runs one-off backup/restore/list operations, validates config, runs the long-lived daemon (scheduler + HTTP API), and imports YAML config into a running daemon's database. The Cobra command tree lives in two files:

- `apps/api/cmd/brd/main.go` — root command, `backup`, `restore`, `list`, `validate-config`, `daemon`.
- `apps/api/cmd/brd/config_import.go` — the `config` parent command and its `config import` subcommand.

## Command surface

Persistent flag on every command: `-c, --config` (default `bared.yml`). Cobra auto-adds `-v/--version` (wired to `version.GetFullVersion()`).

| Command | Purpose | Key flags |
| --- | --- | --- |
| `validate-config` | Load + validate the config file | (uses `--config` only) |
| `backup` | Back up one target | `--target` (required) |
| `restore` | Restore a target or restore-target from a backup | `--target` (required), `--backup` (default `latest`), `--dry-run`, `--skip-validation`, `--skip-verify`, `-y/--yes` |
| `list` | List backups for a target | `--target` (required) |
| `daemon` | Run scheduler and/or HTTP API | `--http` (e.g. `:8080`), `--http-user`, `--http-pass`, `--http-allowed-origin` (repeatable), `--http-session-ttl` (default `12h`), `--http-secure-cookies` |
| `config import <file>` | Push YAML config into a running daemon's DB via API | `--api-url` (default `http://localhost:8080`), `--user` (req), `--pass` (req), `--mode`, `--dry-run`, `--timeout` (default `30s`), `-y/--yes` |

Usage examples (real, from the source `Long` help and README):

```sh
# Validate before doing anything
brd validate-config --config bared.yml

# One-off backup / list / restore
brd backup  --config bared.yml --target athena_local_db
brd list    --config bared.yml --target athena_local_db
brd restore --config bared.yml --target athena_local_db --backup latest
brd restore --target staging_restore --backup et-backups/prod/prod-2025-12-03.tar.gz --dry-run

# Daemon modes
brd daemon                                                  # cron-only (needs >=1 schedule)
brd daemon --http :8080 --http-user admin --http-pass secret # API-only / hybrid

# Import YAML into a running daemon
brd config import bared.yml --user admin --pass secret --mode override
brd config import bared.yml --user admin --pass secret --dry-run
```

Notes on behavior worth knowing before editing:

- `restore` prompts for interactive confirmation ("yes/no") unless `--dry-run` or `-y/--yes` is set, because it overwrites a live database. It resolves the name as either a regular target or a `restore_target` via `cfg.ResolveRestoreTarget`. `--backup latest` triggers `app.FindLatestBackup`.
- `daemon` uses `config.LoadOrEmpty`: if the config file is absent it does **not** error — it loads config from the database inside `daemon.Start()` (requires persistence enabled). When `--http` is set, `--http-user` and `--http-pass` are both required.
- At least one daemon mode (a cron schedule on a target, or `--http`) must be active or the daemon refuses to start.

## How main wires the app

`main()` just calls `rootCmd.Execute()`. Each command's `RunE` does the same wiring:

1. **Config** — `config.Load(cfgFile)` (or `config.LoadOrEmpty` for `daemon`), then `cfg.Validate()`.
2. **Logging** — `initializeLogger(cfg)` translates `cfg.LogLevel` / `cfg.LogFormat` / `cfg.LogOptions` into `util.InitLoggerWithOptions`.
3. **App layer** — one-off commands call `apps/api/internal/app` directly: `app.BackupTarget`, `app.RestoreTargetWithOptions` (with `app.RestoreOptions`), `app.ListBackups`, `app.FindLatestBackup`. The CLI passes `nil` for progress tracking (that's used by the apps/web/API path).
4. **Daemon** — `daemon.New(cfg, opts...)` then `d.Start()`. HTTP is opt-in via the functional option `daemon.WithHTTP(addr, user, pass)`.

Imported packages: `apps/api/internal/app`, `apps/api/internal/config`, `apps/api/internal/daemon`, `apps/api/internal/util`, `apps/api/internal/version` (and for `config import`: `apps/api/internal/client`, `apps/api/internal/configservice`). The CLI is thin — real logic lives in `apps/api/internal/`.

## The `config import` flow

`config import` is the only command that talks to a **remote/running daemon** rather than doing work locally. Flow (`config_import.go`):

1. Validates required `--user`/`--pass` and parses `--mode` into a `client.ConflictMode`. `-y/--yes` forces `ModeOverride`.
2. `config.Load(file)` + `cfg.Validate()` locally first ("✓ Configuration is valid").
3. Builds `client.NewImportClient(apiURL, user, pass, timeout)` and `Ping`s the daemon.
4. `importConfig` pushes resources in dependency order: **storages → notifiers → targets → restore targets → global config** (`default_storage`, `log_level`, `log_format`). Storages and notifiers are validated client-side via `configservice.ValidateStorage` / `ValidateNotifier`.
5. For each resource it checks existence (`StorageExists`, `TargetExists`, …) and resolves conflicts:
   - **interactive** (default): prompts per resource — `[u]pdate / [s]kip / [a]bort`.
   - **override**: update everything (also retries create→update on a `409`).
   - **skip**: create new only, leave existing untouched.
6. `--dry-run` reports would-create / would-update / would-skip without calling the mutating endpoints.
7. Collects everything into a `client.ImportSummary`, prints a per-category summary, and returns a non-nil error if any resource failed (`summary.HasErrors()`). Choosing `[a]bort` interactively calls `os.Exit(0)`.

## Building & running

These targets live in the root `Makefile` (the canonical Build-System list is in [`../AGENTS.md`](../../../AGENTS.md); the CLI-relevant ones are mirrored here).

```sh
make build            # builds bin/brd WITH the embedded web UI
make build-with-web   # same effect — explicit web-build + sync, then build
make install          # build, then copy bin/brd to /usr/local/bin (sudo)
make run-daemon       # CGO=1 fresh-web build, then run daemon on :8080 (admin/admin)
make run-daemon-fast  # CGO_ENABLED=1 `go run` — no build, fastest dev loop
```

Key facts:

- `make build` is **not** backend-only: it depends on `web-build` + `web-sync-dist`, so it compiles the frontend and copies it into `apps/api/internal/web/dist` for embedding. There is no separate "backend-only" default target.
- Local daemon runs (`run-daemon`, `run-daemon-fast`) set **`CGO_ENABLED=1`** because the sqlite3 persistence driver needs cgo. A pure `go build` without CGO will not have working sqlite persistence.
- Version metadata (`Version`, `Commit`, `BuildDate`) is injected via `LDFLAGS` at link time — see `apps/api/internal/version/version.go`.

## Adding a new command

Follow the existing pattern in `main.go` (or `config_import.go` for a subcommand):

1. Declare a `var fooCmd = &cobra.Command{ Use, Short, Long, RunE: ... }`. Prefer `RunE` (return errors) over `Run`.
2. Add flags in an `init()`: `fooCmd.Flags().String("target", "", "...")`, read them back in `RunE` with `cmd.Flags().GetString(...)`.
3. Register it: top-level commands go in the main `init()` via `rootCmd.AddCommand(fooCmd)`; subcommands attach to their parent (e.g. `configCmd.AddCommand(configImportCmd)`).
4. In `RunE`, reuse the shared wiring: `config.Load(cfgFile)` → `initializeLogger(cfg)` → `cfg.Validate()` → call into `apps/api/internal/app` (or `apps/api/internal/daemon`/`apps/api/internal/client`). Keep the command thin; put logic in `apps/api/internal/`.
5. Log structured output through `util.GetLogger().InfoS(...)` with a `"component"` / `"command"` pair, matching the existing commands.

## See also

- [`../AGENTS.md`](../../../AGENTS.md) — root guide (architecture, full build system).
- [`../internal/AGENTS.md`](../internal/AGENTS.md) — backend packages (`app`, `daemon`, `config`, `client`, …).
- [`../web/AGENTS.md`](../../web/AGENTS.md) — the embedded web UI built into `brd`.
