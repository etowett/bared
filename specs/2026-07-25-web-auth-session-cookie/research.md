# Research — web-auth-session-cookie (issue #46)

> Phase 1. Understand the current state before proposing a change. Cite real code (`file:line`).

## Goal / problem

Replace the web dashboard's "Base64 credentials in `sessionStorage`" auth model with a server-issued
session cookie, make WebSocket log streaming authenticate through the same mechanism, and stop the
WebSocket upgrader from accepting every origin.

Tracking issue: [#46](https://github.com/<owner>/bared/issues/46) — *Web auth: replace Base64 creds in
sessionStorage, fix WebSocket auth, restrict CheckOrigin*. Labelled `release:minor`.

This matters more than the other open web issues (#47 code-splitting, #48 SPA navigation) because BareD
holds database credentials and can trigger destructive restores — a recovered dashboard password is a
path to customer data.

## Area & governing guide

- Primary areas: `apps/api/internal/api/` (middleware, server, websocket), `apps/api/cmd/brd/`,
  `apps/api/internal/daemon/`, `apps/web/src/`
- Nested guides: [`apps/api/internal/AGENTS.md`](../../apps/api/internal/AGENTS.md) (backend
  conventions + "Safety & Security"), [`apps/web/AGENTS.md`](../../apps/web/AGENTS.md) (state layering,
  API client pattern, Vitest). There is no `apps/api/internal/api/AGENTS.md`; the `internal/` guide governs.

## What exists today

### Backend

- **Credential source.** `--http-user` / `--http-pass` on `brd daemon`
  (`apps/api/cmd/brd/main.go:492-507`), threaded through `daemon.WithHTTP`
  (`apps/api/internal/daemon/daemon.go:53-59`) into `api.NewServer`
  (`apps/api/internal/daemon/daemon.go:177`). The server keeps them as plain fields
  `authUser` / `authPass` (`apps/api/internal/api/server.go:24-25`). Both flags are **required**
  when `--http` is set, so there is always exactly one credential pair — no user store.
- **Auth middleware.** `basicAuthMiddleware` (`apps/api/internal/api/middleware.go:12-22`) compares
  `r.BasicAuth()` against those fields with `!=` (not constant-time) and sets
  `WWW-Authenticate: Basic realm="BareD API"` on failure — which makes browsers pop a native auth
  dialog on a failed XHR, even though the app has its own login form.
- **Routes.** `/api/health` is public; everything else sits in one `r.Group` behind
  `basicAuthMiddleware` (`apps/api/internal/api/server.go:88-146`), including the WebSocket route
  `GET /api/jobs/{id}/logs/stream` (`server.go:106`). There is **no** `/api/login`, `/api/logout`,
  or `/api/me` today.
- **CORS.** `corsMiddleware` (`apps/api/internal/api/middleware.go:57-71`) is applied globally and
  returns `Access-Control-Allow-Origin: *` for every request. It never echoes an origin and never
  sets `Allow-Credentials`, so browsers currently refuse to attach cookies cross-origin — but `*`
  is also incompatible with any future credentialed CORS, and it advertises the API to any page.
- **WebSocket.** `upgrader.CheckOrigin` returns `true` unconditionally
  (`apps/api/internal/api/websocket.go:17-20`). `handleStreamJobLogs`
  (`apps/api/internal/api/websocket.go:24-128`) reads client messages only to detect disconnect
  (`websocket.go:83-90`) — it **never parses** an auth frame.
- **Tests that will move.** `apps/api/internal/api/middleware_test.go`,
  `websocket_test.go`, `server_test.go` exist and exercise the current Basic-only behaviour.

### Frontend

- **Credential storage.** `setAuth` does `btoa(\`${username}:${password}\`)` into
  `sessionStorage['bared_auth']` (`apps/web/src/api/client.ts:29-32`); `getAuthHeader` reads it back
  as `Basic …` (`client.ts:23-26`); `isAuthenticated()` is a truthiness check on that key
  (`client.ts:45-47`). `atob()` on that value returns the plaintext password.
- **Login.** `Login.tsx:26-38` probes `GET /api/dashboard` with a hand-built `Authorization` header,
  then calls `setAuth`. There is no login endpoint — the dashboard *is* the credential check.
- **Request path.** `apiFetch` injects the header and, on 401, calls `clearAuth()` then
  `window.location.reload()` (`client.ts:50-74`) — the full-page reload that issue #48 is about.
- **Route guard.** `__root.tsx:7-19` calls the synchronous `isAuthenticated()` in `beforeLoad`;
  logout does `window.location.href = '/login'` (`__root.tsx:23-26`).
- **WebSocket.** `useWebSocket.ts` builds the URL with no credential, then sends
  `{ type: 'auth', token }` as the first frame (`useWebSocket.ts:33-56`) — dead code, since the
  server never reads it. The handshake therefore carries no `Authorization` header and the
  middleware 401s it.

### Verified consequences

1. WS log streaming is effectively broken in the browser today (401 at handshake) unless the browser
   has cached Basic credentials from a native dialog — which this app's custom form never triggers.
2. Any XSS, or anyone with devtools access on a shared machine, recovers the plaintext password.
3. Any web page can *attempt* a cross-site WS handshake against a reachable BareD instance — the
   upgrader admits every origin (`websocket.go:17-20`). It cannot currently establish a usable socket,
   because the route is behind Basic auth (`server.go:90-106`) and an attacker-origin handshake carries
   no credentials, so it 401s before upgrade. That changes the moment a cookie exists: the browser will
   attach it automatically, so `CheckOrigin` must be fixed **in the same change**, not after.

## Constraints & considerations

- **Plain HTTP is the normal deployment.** `--http :8080` serves over LAN with no TLS in the box
  (`http.Server` at `apps/api/internal/api/server.go:50-56`, no TLS config anywhere). A hard-coded
  `Secure` cookie attribute would silently break login on every non-TLS install — `Secure` must be
  conditional on the request actually being TLS (or `X-Forwarded-Proto: https`).
- **Stdlib-first** (root `AGENTS.md`, "Engineering principles"). Sessions can be built from
  `crypto/rand` + `crypto/subtle` + `net/http` cookies; no new dependency is justified.
- **Basic auth must keep working** for CLI/`curl` — it's the documented API auth and
  `apps/api/internal/client/` uses it.
- **Single binary, single process, no clustering.** An in-memory session table is viable; there is no
  horizontal-scaling requirement. Restarting the daemon logging everyone out is acceptable (and is
  arguably correct given credentials can change on restart).
- **Dev proxy quirk.** `apps/web/vite.config.ts` proxies `/api` to `localhost:8080` with
  `changeOrigin: true`, which rewrites `Host` but forwards the browser's `Origin`
  (`http://localhost:5173`). A strict `Origin == Host` check will therefore reject the dev server —
  an allowlist knob is required, not optional. Separately the proxy lacks `ws: true`, so WS in
  `make web-dev` doesn't upgrade at all today.
- **No secrets in logs** — session tokens are bearer credentials and must never be logged, same rule
  as DB passwords.
- **Cookie ⇒ CSRF surface.** Cookies ride along automatically, so the CSRF risk that Basic-in-a-header
  didn't have becomes real. `SameSite=Strict` plus an `Origin`/`Sec-Fetch-Site` check on state-changing
  methods covers it without a token-ceremony.

## Prior art in the repo

- **Handler shape / error responses** — `handleUpdateTargetSchedule`
  (`apps/api/internal/api/config_handlers.go:359-388`) is the model for request decoding and
  `respondError`/`respondJSON` usage. *Note:* it calls `s.triggerReload()` because it mutates **daemon
  configuration** — a login handler mutates only the session table and must **not** signal a reload.
- **Basic-auth API consumer** — `apps/api/internal/client/import_client.go:324-325` (`req.SetBasicAuth`)
  is the concrete client that fixes Basic auth as a permanent, first-class path.
- **Middleware + table-driven tests** — `apps/api/internal/api/middleware_test.go` is the model for
  the new cookie/Basic middleware tests; `websocket_test.go` for the `CheckOrigin` tests.
  `server_test.go` constructs `NewServer` in **11** places and `websocket_test.go` in 6 — any
  constructor change lands on all of them.
- **Frontend API-client tests** — `apps/web/src/api/client.test.ts:355-367` already asserts the 401
  behaviour and will need to change with it.
- **Issue overlap** — #48 (stop `window.location` reloads) touches `client.ts:64` and
  `__root.tsx:25`, exactly the lines this change rewrites. #48's own notes say to fold the 401
  redirect into the auth work if that lands first. This spec does that; #47 (code-splitting) is
  independent.
- **Related but out of scope** — `internal/api` sits at 22.6% coverage (#53). New auth code should
  land well-tested, but closing the repo-wide coverage gap is not this change's job.
