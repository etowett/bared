---
name: add-config-field
description: Add a new configuration field to BareD full-stack — Go struct + yaml tag + validation + usage on the backend, plus the TypeScript type, form input, and API wiring on the frontend. Use when the user wants a new setting on a target/storage/notifier/global config (e.g. "add a timeout field to targets", "add a retention setting", "new config option exposed in the UI", "add a yaml field to config").
---

# Add a Config Field (Full-Stack)

Add a new configuration field end-to-end: the Go struct field + `yaml` tag + validation + usage in `internal/config/` (and wherever it's consumed), then the matching TypeScript type, form input, and API client/handler wiring in `web/src/`. This is FULL-STACK. The worked recipe is Task 1 of the backend full-stack recipes.

## When to use
- "Add a `timeout` / `retention` / `<setting>` field to targets/storage/notifiers/global config"
- "Expose a new config option in the web UI"
- "Add a yaml field to the config" / a new key in `config.example.yml`

## Reference
- `internal/AGENTS.md` → "Full-Stack Recipes" → "Task 1: Add a New Configuration Field" (struct, validation, usage, examples). Read it first.
- `web/AGENTS.md` → "API Client Pattern" and "Component Patterns" for how config flows to the React forms.
- Config structs live in `internal/config/config.go`; validation in `internal/config/validator.go`; YAML parse/env-expansion in `internal/config/parser.go`. Config CRUD over HTTP is in `internal/api/config_handlers.go` / `config_types.go`.

## Steps
Backend:
1. Add the field with a `yaml:"...,omitempty"` tag to the relevant struct in `internal/config/config.go` (`Target`, `Storage`, `Notifier`, `Connection`, or global config).
2. Add validation in `internal/config/validator.go`.
3. Consume the field where it takes effect (e.g. `internal/app/backup.go`, the relevant database/storage/notify code, etc.).
4. If the field is editable via the API, surface it in `internal/api/config_types.go` (request/response structs) and the relevant handler in `internal/api/config_handlers.go`.

Frontend (`web/src/`):
5. Add the field to the TypeScript types in `web/src/types/` (and the API client payloads in `web/src/api/client.ts` if it's editable).
6. Add the input/control to the relevant config form component under `web/src/components/` and wire it through the config hook (`web/src/hooks/useConfig.ts`).

Docs/examples:
7. Update `examples/config.example.yml`, `README.md`, and `docs/user-guide/configuration.md`.

## Verify
- `make test` — backend (config/validator) tests pass
- `make build` — backend compiles
- `make web-validate` — frontend type-check + lint + tests
- `make build-with-web` — full build with embedded assets
- `make validate` — run before finishing
- If the config schema/contract changed, keep `internal/AGENTS.md`, `web/AGENTS.md`, and `docs/` in sync.
