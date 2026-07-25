---
name: add-storage-backend
description: Scaffold a new storage backend (Storage) in BareD, following the Local/S3/SFTP pattern. Use for "add Azure Blob storage", "support Google Cloud Storage / GCS", "add a Backblaze B2 backend", "store backups on WebDAV/MinIO", "register a new storage.type".
---

# Add a Storage Backend

Scaffold a new storage destination that implements the `Storage` interface, mirroring the existing Local / S3 / SFTP backends. The authoritative, worked recipe lives in the nested guide — follow it; this skill is the checklist + entry point.

## When to use
- "Add Azure Blob / Google Cloud Storage / Backblaze B2 / MinIO / WebDAV storage"
- "Store backups somewhere new" / a new `storage.type` value is needed
- Implementing the `Storage` interface for a new backend

## Reference
- `apps/api/internal/storage/AGENTS.md` — the deep recipe (constructor, retry wrapping, full method set). Read it first.
- Interface lives in `apps/api/internal/storage/storage.go`: `Storage` requires `Store`, `Retrieve`, `List`, `Delete`, `Name`, `Validate`, `Exists`, `GetInfo` (returning `*BackupInfo` where applicable).
- Wrap network operations in `util.Retry(ctx, util.DefaultRetryConfig(), fn)` like S3/SFTP do; local needs no retries. Stream via `io.Reader`/`io.Writer` — never buffer whole backups.

## Steps
1. Create `apps/api/internal/storage/<backend>.go` with a struct implementing all `Storage` methods, plus a `New<Backend>(cfg)` constructor (existing constructors are `NewLocal` / `NewS3` / `NewSFTP`).
2. Add backend-specific fields to the `Storage` struct in `apps/api/internal/config/config.go` (e.g. account URL, container, credentials) with `yaml` tags.
3. Register the backend in the `New(cfg *config.Storage)` switch in `apps/api/internal/storage/factory.go` (the live factory function is `New`, dispatching on `cfg.Type`).
4. Allow the new `storage.type` in `apps/api/internal/config/validator.go` and validate the new required fields.
5. Add unit tests `apps/api/internal/storage/<backend>_test.go` following the existing pattern.
6. Wire the web UI: add the storage type to the React storage forms and the `storage.type` choices/types in `apps/web/src/` (see `apps/web/AGENTS.md`).
7. Update `examples/config.example.yml`, `README.md`, and `docs/user-guide/configuration.md`.

## Verify
- `make test` — unit tests pass
- `make build` — backend compiles
- `make web-validate` and `make build-with-web` — if you touched the React UI
- `make pre-commit` — the full backend gate; run before finishing
- If you changed the `Storage` interface, update `apps/api/internal/storage/AGENTS.md` and `docs/` accordingly.
