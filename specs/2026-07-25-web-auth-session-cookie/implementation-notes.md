# Implementation notes — web-auth-session-cookie (issue #46)

> Phase 3. Written during/after implementation. The audit trail for why the code looks as it does.

## Status

**Implemented** 2026-07-25 on `feat/web-auth-session-cookie`. Spec drafted, reviewed by Codex, revised,
then built against the revised plan. All open questions resolved with the recommended answers.

## Pre-implementation review (Codex, 2026-07-25)

Verdict: *do not implement as written* — direction right (opaque server-side sessions + retained Basic
auth), two security-critical gaps. All findings below are folded into `research.md` / `plan.md` /
`open-questions.md`; recorded here so the reasoning survives.

**P0 — session expiry/logout did not revoke already-upgraded WebSockets.** `handleStreamJobLogs`
(`apps/api/internal/api/websocket.go:24`) validates only at handshake, then streams until disconnect.
The draft promised absolute TTL and server-side logout revocation with no session→connection
association, so a logged-out browser would keep receiving job logs. → connection registry + close on
revoke/expiry (plan step 5, Q10).

**P0 — CSRF control was specified as CORS.** The draft said `corsMiddleware` would "reject state-changing
cross-site requests", but CORS only emits headers and short-circuits `OPTIONS`
(`apps/api/internal/api/middleware.go:56-70`) — it rejects nothing, and CORS is not CSRF protection.
→ separate `csrfMiddleware`, tested against real state-changing endpoints (plan step 4, Q9).

**P1 — `X-Forwarded-Proto` trust contradicted the spec's own warning.** The plan trusted it for the
`Secure` attribute while the open questions noted it is spoofable. → explicit `--http-secure-cookies`,
no header inference (Q3).

**P1 — `CheckOrigin` needed canonical parsing**, not host string comparison; one parsed allowlist shared
with CORS, but CORS-allowed ≠ safe for cookie-authenticated mutations. → lookalike-host test cases.

**P1 — ceremonial `subtle.ConstantTimeCompare` on session lookup.** A random token used as a map key
can't be looked up in constant time; the ceremony implies a guarantee that doesn't exist. Constant-time
compare belongs on the **Basic credentials**, which use plain `!=` today (`middleware.go:15`).

**P1 — plain HTTP remains bearer-token theft.** The session cookie is not password-*recoverable* but is
password-*equivalent* until expiry. Corrected the plan's wording; added the startup warning and the risk
entry.

**Factual corrections to `research.md`:** "any web page can open a WS" → can only *attempt* a handshake
(Basic auth 401s it pre-upgrade today); dropped the misleading `triggerReload` analogy (a login handler
must not signal a config reload); cited the real Basic-auth consumer at
`apps/api/internal/client/import_client.go:324-325`.

**Missing work the review found:** `server_test.go` (11 `NewServer` sites), `daemon_test.go:47`,
`apps/web/src/components/Login.test.tsx:7`, `apps/api/cmd/AGENTS.md:20`, `docs/api/README.md:39`,
`docs/api/endpoints.md:1673`, `docs/api/websocket.md:26`, `docs/user-guide/web-interface.md:708`
(tells users to clear `sessionStorage`), `README.md:151`, `apps/web/README.md:46`,
`apps/web/TESTING.md:221`; plus cookie-attribute tests (exact name, `Expires`, host-only, matching-`Path`
deletion) and oversized-body handling.

**New open questions raised:** Q9 (CSRF algorithm), Q10 (WS expiry behaviour), Q11 (cookie name/scope),
Q12 (`WWW-Authenticate` suppression rule), Q13 (stale cookie on `/api/me` 401). Q4's proposal changed
from functional options to a `ServerOptions` struct.

## What changed

**Backend (`apps/api/internal/api`)**

- `session.go` (new) — `sessionStore`: opaque 32-byte `crypto/rand` tokens, in-memory map, absolute TTL,
  background sweeper, and cookie helpers. Every method is nil-safe so `&Server{}` literals in existing
  tests keep working. Each session owns a `done` channel closed on revoke/expiry.
- `auth_handlers.go` (new) — `POST /api/login`, `POST /api/logout`, `GET /api/me`.
- `csrf.go` (new) — `csrfMiddleware`, origin enforcement for cookie-authenticated unsafe methods.
- `origin.go` (new) — `canonicalOrigin` / `originAllowed`, shared by CORS, CSRF, and the WS upgrader.
- `middleware.go` — `basicAuthMiddleware` → `authMiddleware` (cookie → Basic → 401), identity in request
  context, constant-time credential compare, conditional `WWW-Authenticate`, stale-cookie clearing;
  `corsMiddleware` is now a method with an allowlist instead of a wildcard.
- `websocket.go` — `CheckOrigin` via the shared origin logic; the stream selects on the session's `Done()`
  channel and sends a 1008 close frame when the session ends.
- `server.go` — `ServerOptions` struct constructor, new fields, route registration, sweeper lifecycle,
  plain-HTTP startup warning.
- `daemon.go` / `main.go` — `WithSessionTTL`, `WithAllowedOrigins`, `WithSecureCookies`; three new
  `brd daemon` flags.

**Frontend (`apps/web/src`)**

- `api/client.ts` — `btoa`/`sessionStorage` gone. `login`/`logout`/`fetchCurrentUser`, `credentials:
  'same-origin'`, and a typed `AuthError` plus an `onAuthFailure` hook instead of `window.location.reload()`.
- `stores/auth.ts` (new) — Zustand auth status, caching `GET /api/me` and de-duplicating concurrent checks.
- `App.tsx` — wires `onAuthFailure` to router navigation + `queryClient.clear()`.
- `routes/__root.tsx` — async guard; logout navigates via the router instead of `window.location.href`.
- `components/Login.tsx` — calls the login endpoint instead of probing `/api/dashboard` with a hand-built
  Basic header.
- `hooks/useWebSocket.ts` — the dead `{type:'auth'}` frame is gone.
- `vite.config.ts` — `ws: true` on the `/api` proxy.

**Docs** — `docs/api/{README,endpoints,websocket}.md`, `docs/user-guide/web-interface.md`, `README.md`,
`apps/web/{README,TESTING,AGENTS}.md`, `apps/api/cmd/AGENTS.md`, `apps/api/internal/AGENTS.md`.

## Decisions & trade-offs

- **All eight original open questions took the recommended answer**, with Q3, Q4, Q9–Q13 as revised by
  the review. Notably: `ServerOptions` struct (not functional options), explicit `--http-secure-cookies`
  (no `X-Forwarded-Proto` inference), and a dedicated `csrfMiddleware` separate from CORS.
- **CSRF requires `Origin` on cookie-authenticated unsafe methods** — a missing `Origin` is rejected, not
  waved through. Browsers always send it on `POST`/`PUT`/`PATCH`/`DELETE`, including same-origin, so this
  only affects non-browser clients presenting a cookie, which should be using Basic auth anyway.
- **Same-origin is compared on host:port, ignoring scheme** (matching gorilla's own default). Comparing
  schemes would break every reverse-proxy deployment, where the browser sends `https://` and the daemon
  sees plain HTTP. The configured allowlist *is* compared with its scheme, since the operator wrote it.
- **The session object doubles as the WS connection registry.** A separate registry keyed by token was
  planned; putting the `done` channel on the session and the session in the request context achieves the
  same thing with far less machinery.

## Surprises / gotchas

- **The old middleware had an auth bypass.** `user != s.authUser || pass != s.authPass` meant a server
  with no credentials configured accepted `Basic ":"` (empty == empty). Unreachable via `brd daemon`,
  which requires both flags, but `api.NewServer` allowed it and a test asserted the 200. Now rejected;
  `TestAuthMiddleware_EmptyCredentialsRejected` documents it.
- **WS in `make web-dev` never worked.** The Vite `/api` proxy lacked `ws: true`, so the handshake was
  never upgraded in dev — independent of the auth problem, and fixed here.
- **gosec G124 flags conditional cookie attributes.** It wants a literal `Secure: true`; suppressed with
  `#nosec G124` and the reason, since hardcoding it would break plain-HTTP installs.
- **Draining matters when testing stream closure by hand.** A live stream has buffered backlog, so bytes
  arriving right after logout are not proof it is still authenticated — drain to idle first.

## Follow-ups

- [ ] Per-IP login rate limiting (deferred from Q7) — open as its own issue.
- [ ] TLS termination guidance / `--http-tls-cert` in the daemon, so plain HTTP stops being the default
      posture.
- [ ] Issues #33, #35, #41 are fixed on `main` (PRs #34, #36, #42) but still open — close them.

## Verification done

**Automated**

- `make fmt` / `make vet` / `make lint` — clean (0 issues).
- `make test-unit` — all packages pass. `internal/api` coverage 22.6% → **35.4%**.
- `go test -race ./internal/api/... ./internal/daemon/...` — pass.
- `make web-validate` — type-check, eslint, prettier, **252 Vitest tests** pass.
- `make build-with-web` — builds with the embedded UI.
- `coverage-check` still fails at ~27% vs the 75% threshold — the pre-existing repo-wide gap tracked in
  #53, not a regression from this change.

**Regression tests confirmed to fail without the fix:** stubbing out the `sessionDone` case makes
`TestHandleStreamJobLogs_LogoutClosesLiveStream` and `..._ExpiryClosesLiveStream` fail; they pass with it.

**Manual, against a running `brd daemon --http :18080 --http-allowed-origin http://localhost:5173`**

| Check | Result |
|---|---|
| Startup warning on plain HTTP | logged as designed |
| Login over plain HTTP | 200, cookie `HttpOnly; SameSite=Strict; Path=/; Max-Age=43200`, **no `Secure`** |
| Wrong password | 401, no cookie |
| Cookie authenticates `/api/me`, `/api/dashboard` | 200 |
| Basic auth still works | 200 |
| No credentials | 401 |
| Cookie `POST /api/jobs/backup` from `https://evil.example` | **403** |
| …same origin / allowlisted dev origin | passed CSRF (202 / 409-conflict) |
| …Basic auth, no `Origin` | 202 (exempt) |
| CORS for a foreign origin | no `Access-Control-Allow-Origin`, `Vary: Origin` present |
| WS handshake: no origin / same origin / allowlisted | 101 |
| WS handshake: `evil.local` / `evil-localhost:18080` | **403** |
| WS handshake unauthenticated | 401 |
| Logout while streaming | server sent close frame **1008 "session ended"** |
