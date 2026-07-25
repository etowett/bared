---
name: add-database-type
description: Scaffold a new database engine (Dumper/Restorer) in BareD, following the MySQL/PostgreSQL/Redis pattern. Use for "add MongoDB support", "support SQL Server backups", "back up MariaDB/ClickHouse", "implement a new Dumper/Restorer", "register a new conn.type".
---

# Add a Database Type

Scaffold a new database engine that streams a backup (`Dumper`) and consumes one for restore (`Restorer`), mirroring the existing MySQL/PostgreSQL/Redis implementations. The authoritative, worked recipe lives in the nested guide — follow it; this skill is the checklist + entry point.

## When to use
- "Add MongoDB / SQL Server / MariaDB / Cassandra / ClickHouse support"
- "Support backing up / restoring a new database type"
- A new value needs to be accepted for `conn.type` in the config
- Implementing the `Dumper` or `Restorer` interface for a new engine

## Reference
- `internal/database/AGENTS.md` — the deep recipe (full code for `Dump`/`Restore`, factory registration, validator, tests). Read it first.
- Interfaces live in `internal/database/database.go`: `Dumper` (`Dump(ctx, w) (*DumpMetadata, error)`, `Name()`, `Validate(ctx)`) and `Restorer` (`Restore(ctx, r)`, `Name()`, `ValidateConnection(ctx)`).
- Stream to/from `io.Writer`/`io.Reader` via `internal/util` shell executors — never buffer whole dumps.

## Steps
1. Create `internal/database/<engine>.go` with a struct implementing both `Dumper` and `Restorer`, plus a `New<Engine>(...)` constructor (match existing constructor signatures — MySQL/Postgres take `conn, ExcludeTables, AdditionalArgs`).
2. Register the engine in BOTH switches in `internal/database/factory.go`: `NewDumper(target)` and `NewRestorer(target)` (the live factory functions are `NewDumper`/`NewRestorer`, not `New`).
3. Allow the new `conn.type` in `internal/config/validator.go` (add it to the valid-types list).
4. Add unit tests `internal/database/<engine>_test.go` (skip when the CLI tool is absent, as the existing tests do).
5. Wire the web UI option: add the type to the connection-type choices in the React config forms and the `conn.type` union in `web/src/types/` (see `web/AGENTS.md`).
6. Update `examples/config.example.yml`, `README.md`, and `docs/user-guide/configuration.md`.

## Verify
- `make test` — unit tests pass
- `make build` — backend compiles
- `make web-validate` and `make build-with-web` — if you touched the React UI
- `make pre-commit` — the full backend gate; run before finishing
- If you changed the `Dumper`/`Restorer` interfaces, update `internal/database/AGENTS.md` and `docs/` accordingly.
