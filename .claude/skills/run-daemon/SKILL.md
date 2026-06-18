---
name: run-daemon
description: Build and run the BareD daemon (brd) locally and smoke-test a change in the real app. Use when the user wants to "run the daemon", "start brd", "run the app", "launch the server", check the web UI / dashboard, watch a backup job, or asks "does this actually work" / "verify this change runs". This is the project's run recipe — the built-in /run defers to it.
---

# Run the BareD daemon locally

Build `brd`, start the daemon with the HTTP API + embedded web UI, and confirm a change behaves correctly in the running app (not just in tests).

## When to use
- "Run the daemon" / "start brd" / "launch the app" / "spin up the server"
- "Open the web UI" / "check the dashboard" / "watch a job run"
- "Does this actually work?" — verify a code change in the real binary, end-to-end
- Smoke-testing before opening a PR

## Run it

```sh
make run-daemon        # CGO=1: builds fresh web UI + embeds it, then runs daemon on :8080
make run-daemon-fast   # CGO_ENABLED=1 `go run` — no rebuild, fastest backend-only dev loop
```

Notes:
- Both targets set **`CGO_ENABLED=1`** because the sqlite3 persistence driver (`github.com/mattn/go-sqlite3`) needs cgo. A plain CGO-off `go build` has no working sqlite persistence.
- `make build` produces `bin/brd` **with the web UI embedded** (it depends on `web-build` + `web-sync-dist`); there is no backend-only default build.
- Default dev credentials are `admin` / `changeme` on `:8080` (see the `run-daemon` target).

Point at a config and run the binary directly when you need custom flags:

```sh
make build
./bin/brd daemon --config bared.yml --http :8080 --http-user admin --http-pass changeme
```

`daemon` uses `config.LoadOrEmpty` — if the config file is absent it loads config from the persistence DB instead of erroring (requires persistence enabled). At least one mode must be active: a cron `schedule` on a target, or `--http`.

## Exercise it
- Open the web UI / dashboard: **http://localhost:8080** (log in with the `--http-user`/`--http-pass` above).
- Hit the API, e.g. `curl -u admin:changeme http://localhost:8080/api/dashboard` (jobs at `/api/jobs`, trigger backup via `POST /api/jobs/backup`).
- Trigger a backup and watch it live: the UI streams logs over WebSocket (`/api/ws`); or poll `GET /api/jobs/<id>`.
- One-off CLI ops (no daemon) for quick checks: `./bin/brd backup|list|restore --config bared.yml --target <name>`.

## Verify
- Daemon starts cleanly (no startup error; logs show the HTTP server listening on `:8080`).
- The web UI loads at http://localhost:8080 and you can authenticate.
- The behavior you changed is actually observed in the app (run the relevant job / open the relevant view), not assumed.
- Stop with Ctrl-C when done.
