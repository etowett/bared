---
name: add-api-endpoint
description: Add a full-stack REST API endpoint to BareD — Go handler + chi route on the backend, plus the TypeScript client method, React Query hook, and component on the frontend. Use when the user wants a new HTTP endpoint and/or to surface new data in the web UI (e.g. "add a /api/stats endpoint", "expose backup statistics in the UI", "add an endpoint to trigger X", "new API route + React hook").
---

# Add an API Endpoint (Full-Stack)

Add a new REST endpoint end-to-end: a Go handler and chi route in `internal/api/`, then the matching client method, React Query hook, and UI component in `web/src/`. This is FULL-STACK — do both sides. The worked recipe is Task 3 of the backend full-stack recipes.

## When to use
- "Add a new `/api/...` endpoint" / "expose <data> via the API"
- "Surface <something> in the web dashboard" (needs a new backend route + hook)
- "Add an endpoint to trigger/return X"

## Reference
- `internal/AGENTS.md` → "Full-Stack Recipes" → "Task 3: Add a New API Endpoint" (handler + hook + component). Read it first.
- `web/AGENTS.md` → "API Client Pattern" and "Component Patterns" for the frontend conventions.
- Live routing uses the chi router in `internal/api/server.go` (`setupRoutes` returns a `chi.Router`). Authenticated routes go inside the `r.Group(func(r chi.Router){ r.Use(s.basicAuthMiddleware); ... })` block — NOT the `s.mux.HandleFunc`/`withAuth` shape shown in the recipe's older snippet. Config CRUD routes live under the `/api/config` sub-router.

## Steps
Backend (`internal/api/`):
1. Add a `func (s *Server) handle<Name>(w http.ResponseWriter, r *http.Request)` handler (in `handlers.go`, or `config_handlers.go` for `/api/config/*`). Use `s.respondJSON` / `s.respondError`; read path params with `chi.URLParam`.
2. Define request/response structs (with `json` tags) in `internal/api/types.go` or `config_types.go`.
3. Register the route in `setupRoutes` in `internal/api/server.go` using the chi verb method (`r.Get`/`r.Post`/...) inside the authenticated group (or under the `/api/config` route for config endpoints).

Frontend (`web/src/`):
4. Add the request method to the API client in `web/src/api/client.ts`.
5. Add a React Query hook in `web/src/hooks/use<Name>.ts` (`useQuery`/`useMutation` calling the client).
6. Add/extend the TypeScript types in `web/src/types/`.
7. Consume the hook in a component under `web/src/components/` (or a route in `web/src/routes/`).

## Verify
- `make test` — backend handler tests pass
- `make build` — backend compiles
- `make web-validate` — frontend type-check + lint + tests
- `make build-with-web` — full build with embedded assets
- `make validate` — run before finishing
- If you changed the API contract (request/response shapes), keep `internal/AGENTS.md`, `web/AGENTS.md`, and `docs/` in sync.
