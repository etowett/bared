# Plan — web-auth-session-cookie (issue #46)

> Phase 2. The concrete change. Resolve `open-questions.md` before you start implementing.

## Approach

Issue a **server-side opaque session token** at login and hand it to the browser in an
`HttpOnly; SameSite=Strict; Path=/` cookie. The token is 32 bytes from `crypto/rand`, base64url-encoded,
and stored in an in-memory table on the `Server` (token → expiry). Nothing is derived from the password,
so the cookie is not password-**recoverable** the way `btoa(user:pass)` is — but it *is* a bearer
credential with equivalent authority until it expires or is revoked, and must be treated as a secret
(never logged, never in a URL). Logout deletes the entry server-side, which a signed/stateless token
could not do without key rotation. State lives in one process and the daemon is a single binary, so an
in-memory map is the stdlib-first fit (root `AGENTS.md`: "Reach for the standard library before adding
a dependency").

`Secure` is **explicitly configured**, not inferred: set automatically when the request is genuinely TLS
(`r.TLS != nil`), and otherwise only when the operator passes `--http-secure-cookies` (for a
TLS-terminating proxy). `X-Forwarded-Proto` is **not** trusted by default — it is client-controlled and
spoofable whenever the daemon is directly reachable. A hard-coded `Secure` would silently break login on
every plain-HTTP LAN install, so the default stays off with a **startup warning** that sessions are
unencrypted over plain HTTP.

Auth middleware becomes: **valid session cookie → allow; else Basic auth → allow; else 401**. Basic
stays first-class for CLI/`curl`/`internal/client` (`import_client.go:324`), satisfying the issue's
backward-compat criterion. Because the same middleware now guards the WS route, the browser
authenticates the handshake with the cookie it sends automatically — no header hack, and the dead
`{type:'auth'}` frame goes away.

Two consequences of introducing a cookie must be handled **in this change**, not deferred:

- **CSRF.** Cookies ride along automatically; Basic-in-a-header did not. CORS headers are *not* CSRF
  protection — the current `corsMiddleware` only emits headers and short-circuits `OPTIONS`
  (`middleware.go:56-70`), it rejects nothing. A **separate** `csrfMiddleware` enforces: for unsafe
  methods (`POST`/`PUT`/`PATCH`/`DELETE`) authenticated **by cookie**, require a canonically-matching
  same-origin or allowlisted `Origin`; requests with no `Origin` authenticated by **Basic** pass
  through (CLI clients). `Sec-Fetch-Site` is defence-in-depth only, never the sole check.
- **Live WebSockets outlive their session.** `handleStreamJobLogs` validates only at handshake and then
  streams until disconnect (`websocket.go:24-128`), so logout or TTL expiry would leave an authenticated
  log stream running. The session identity goes into the request context, the server keeps a registry of
  live connections keyed by token, and logout/expiry closes them. Without this, "server-side revocation"
  and "absolute TTL" are both false for the one endpoint that streams job output.

`CheckOrigin` flips to same-origin-by-default with an explicit `--http-allowed-origin` allowlist (needed
for the Vite dev proxy, which forwards `Origin: http://localhost:5173` while rewriting `Host`). The
comparison **parses** the `Origin` URL and compares canonical scheme+host+port — never a string prefix.
Absent `Origin` (non-browser clients) is accepted, matching gorilla's own default.

Frontend: `POST /api/login` / `POST /api/logout`, `credentials: 'same-origin'` on every fetch, no
`Authorization` header, no `sessionStorage`. Since JS can't read an `HttpOnly` cookie, `isAuthenticated()`
becomes an async `GET /api/me` used by the router's `beforeLoad`, cached in a small Zustand auth store so
the guard doesn't round-trip on every navigation.

Per issue #48's note, the 401 and logout flows are rewritten to router navigation **here** rather than
twice — that closes the `window.location` half of #48 as a side effect (the remaining #48 work is nil;
see "Scope" below).

## Scope

**In:** backend session store + login/logout/me endpoints + combined auth middleware + `CheckOrigin` +
CORS tightening; frontend client/login/root/useWebSocket rewrite; tests both sides; docs.

**Out:** TLS termination in the daemon, multi-user accounts, password hashing/rotation, rate-limiting
login attempts (noted as a follow-up), route code-splitting (#47), the coverage gate (#53).

**Absorbs:** #48 — both `window.location` call sites are rewritten here. Close #48 with this PR.

## Files to create / edit

| File | Change |
|------|--------|
| `apps/api/internal/api/session.go` | **create** — `sessionStore`: `New(ttl)`, `Issue() (token, error)`, `Validate(token) (session, bool)`, `Revoke(token)`, background sweep of expired entries (bounded, stops on shutdown). `sync.RWMutex`-guarded map keyed by token. 32 bytes from `crypto/rand`; **no** `subtle.ConstantTimeCompare` here — a map lookup on a high-entropy random key can't be constant-time and the ceremony would imply a guarantee we don't have. (Constant-time compare belongs on the **Basic credentials**, which today use plain `!=` at `middleware.go:15`.) |
| `apps/api/internal/api/session_test.go` | **create** — issue/validate/revoke/expiry boundary, sweep removes expired entries, concurrent access under `-race`. |
| `apps/api/internal/api/auth_handlers.go` | **create** — `handleLogin` (bounded body via `http.MaxBytesReader`, constant-time credential compare, generic failure message — no user-vs-password distinction, issue token, set cookie), `handleLogout` (revoke + expire cookie + close that token's live WebSockets), `handleMe` (returns `{username}`; on 401 also clears a stale cookie). |
| `apps/api/internal/api/auth_handlers_test.go` | **create** — login happy/wrong-password/malformed body/oversized body; logout revokes server-side; `/api/me` 401 without cookie. |
| `apps/api/internal/api/middleware.go` | **edit** — `authMiddleware`: cookie → Basic → 401, putting the authenticated session (or "basic") into the request context. Constant-time credential compare. Suppress `WWW-Authenticate` when the request came from the SPA (cookie present, or `X-Requested-With`/`Sec-Fetch-Mode: cors`) so failed XHRs stop triggering the native browser dialog; keep it for plain API clients. Reduce `corsMiddleware` to an explicit allowlist with `Vary: Origin` (no wildcard) — headers only, **no** rejection logic. |
| `apps/api/internal/api/csrf.go` | **create** — `csrfMiddleware`: for unsafe methods on **cookie-authenticated** requests, require canonical same-origin or an allowlisted `Origin`; pass Basic-authenticated requests with no `Origin`. Independent of `corsMiddleware`. |
| `apps/api/internal/api/csrf_test.go` | **create** — exercised against **real state-changing endpoints** (`POST /api/jobs/backup`, `POST /api/jobs/restore`, one `PUT /api/config/...`), not just the middleware in isolation. |
| `apps/api/internal/api/middleware_test.go` | **edit** — cookie-auth, Basic fallback, both absent, expired session, garbage cookie, `WWW-Authenticate` suppression rule, CORS header behaviour. |
| `apps/api/internal/api/websocket.go` | **edit** — `CheckOrigin` parses `Origin` and compares canonical scheme/host/port against `r.Host` plus the allowlist; absent `Origin` accepted. Upgrader becomes a `Server` method (not a package-level var) so it sees config. Register each live connection against its session token; select on a per-session close channel so logout/expiry terminates the stream. |
| `apps/api/internal/api/websocket_test.go` | **edit** — same-origin ✓, foreign ✗, allowlisted ✓, absent `Origin` ✓; **plus** an integration test that logout and TTL expiry each close a live cookie-authenticated stream. |
| `apps/api/internal/api/server.go` | **edit** — add `sessions`, `allowedOrigins`, `secureCookies`, `wsRegistry` fields; register `POST /api/login`, `POST /api/logout` (public) and `GET /api/me` (authenticated); swap `basicAuthMiddleware` → `authMiddleware` + `csrfMiddleware`. Constructor takes a `ServerOptions` struct (see Q4). |
| `apps/api/internal/api/server_test.go` | **edit** — **11** `NewServer(` call sites plus the existing route/CORS expectations. |
| `apps/api/internal/daemon/daemon.go` | **edit** — thread allowed origins, session TTL, secure-cookie flag from `WithHTTP` into `api.NewServer`. |
| `apps/api/internal/daemon/daemon_test.go` | **edit** — `WithHTTP` option assertions (`daemon_test.go:47` onward) for the new fields. |
| `apps/api/cmd/brd/main.go` | **edit** — add `--http-allowed-origin` (repeatable), `--http-session-ttl` (default 12h), `--http-secure-cookies` to `daemon`; log the plain-HTTP session warning at startup. |
| `apps/web/src/api/client.ts` | **edit** — delete `btoa`/`sessionStorage`; `login()`/`logout()` hit the new endpoints; `fetchMe()`; `apiFetch` uses `credentials: 'same-origin'` and throws a typed `AuthError` on 401 instead of reloading. |
| `apps/web/src/api/client.test.ts` | **edit** — replace the `window.location.reload` assertion (`:355-367`) with `AuthError`; add login/logout/me tests. |
| `apps/web/src/stores/auth.ts` | **create** (or extend an existing store) — Zustand: `status: 'unknown'\|'authed'\|'anon'`, `check()`, `signOut()`. Client-state layer per `apps/web/AGENTS.md`. |
| `apps/web/src/routes/__root.tsx` | **edit** — `beforeLoad` awaits the auth store's `check()`; logout calls `signOut()` → `queryClient.clear()` → `navigate({to:'/login'})`. |
| `apps/web/src/components/Login.tsx` | **edit** — call `login(username, password)`; drop the hand-built header and the `/api/dashboard` probe. |
| `apps/web/src/components/Login.test.tsx` | **edit** — currently mocks `setAuth` and asserts the old Basic dashboard probe (`Login.test.tsx:7`); must assert `POST /api/login`. |
| `apps/web/src/routes/login.tsx` | **edit** — async auth check instead of the sync `isAuthenticated()`. |
| `apps/web/src/hooks/useWebSocket.ts` | **edit** — remove `getAuthHeader` import and the `{type:'auth'}` frame and its stale comment. |
| `apps/web/vite.config.ts` | **edit** — add `ws: true` to the `/api` proxy so dev-mode WS upgrades (broken today). |
| `docs/api/README.md:39`, `docs/api/endpoints.md:1673`, `docs/api/websocket.md:26` | **edit** — all document Basic-only auth and a WS Basic header; add the login/logout/me endpoints and the cookie handshake. |
| `docs/user-guide/web-interface.md:708` | **edit** — currently tells users to clear `sessionStorage`; replace with the logout/session model. |
| `README.md:151`, `apps/web/README.md:46`, `apps/web/TESTING.md:221` | **edit** — HTTP-auth launch instructions and web-UI auth behaviour. CLI Basic examples stay; browser session behaviour is documented separately. |
| `apps/api/cmd/AGENTS.md:20`, `apps/api/internal/AGENTS.md`, `apps/web/AGENTS.md` | **edit** — the three new daemon flags, the cookie session model, and that Basic remains first-class for API clients. |

## Interfaces affected

- **HTTP API (public contract):** three new endpoints — `POST /api/login`, `POST /api/logout`,
  `GET /api/me`. No existing endpoint changes shape. Existing Basic-auth clients are unaffected.
- **`api.NewServer` signature** (internal) — grows session TTL + allowed origins; see Q4.
- No `Dumper`/`Restorer`/`Storage`/`Notifier` interface is touched.

## Ordered steps

1. `session.go` + `session_test.go` — the store, standalone and fully tested first.
2. `auth_handlers.go` + tests — login/logout/me against the store, cookie attributes asserted
   (name, `HttpOnly`, `SameSite=Strict`, conditional `Secure`, `Path=/`, `Max-Age` **and** `Expires`,
   host-only i.e. no `Domain`).
3. `middleware.go` — `authMiddleware` (cookie → Basic → 401), session identity into request context,
   constant-time Basic compare; update tests.
   **Regression test:** a WS-style request carrying only the session cookie is authorized — fails
   before, passes after.
4. `csrf.go` + `csrf_test.go` — unsafe-method origin enforcement against real state-changing endpoints.
   **Regression test:** a cookie-authenticated `POST /api/jobs/backup` with a foreign `Origin` is
   rejected.
5. `websocket.go` — canonical-origin `CheckOrigin` + allowlist; connection registry keyed by session.
   **Regression tests:** `Origin: https://evil.example` rejected (fails before, passes after); a live
   cookie-authenticated stream is closed by logout and by TTL expiry.
6. `server.go` wiring (`ServerOptions`, route registration, `corsMiddleware` allowlist) → the 11
   `server_test.go` call sites → `daemon.go` + `daemon_test.go` → `main.go` flags and startup warning.
7. `make pre-commit` (expect `coverage-check` to still fail per the known gap in root `AGENTS.md`;
   everything before it must be green).
8. Frontend: `client.ts` → auth store → `Login.tsx`/`Login.test.tsx` → `__root.tsx` / `login.tsx` →
   `useWebSocket.ts` → `vite.config.ts`.
9. `make web-validate`.
10. `make build-with-web`, then the manual matrix below.
11. Docs (`docs/api/*`, `docs/user-guide/web-interface.md`, the three READMEs, the three `AGENTS.md`)
    + `implementation-notes.md`; PR referencing #46 and closing #48.

## Tests

**Backend (Go, table-driven, `-race`):**
- Session store: issue → validate → revoke; expiry boundary; unknown token; sweep; parallel issue/validate.
- Login: correct creds → 200 + `Set-Cookie` asserting **exact name**, `HttpOnly`, `SameSite=Strict`,
  `Path=/`, no `Domain` (host-only), `Max-Age` **and** `Expires`, no `Secure` on plain HTTP, `Secure`
  when `r.TLS != nil` or `--http-secure-cookies`; wrong password → 401, **no** cookie, and a generic
  message that doesn't distinguish bad user from bad password; malformed JSON → 400; oversized body →
  413 (not an OOM).
- Logout: revokes server-side (token rejected afterwards), expires the cookie with a matching `Path`,
  **and** closes that session's live WebSockets.
- Middleware: cookie-only → 200; Basic-only → 200; expired cookie + no Basic → 401; garbage cookie → 401;
  neither → 401; `WWW-Authenticate` present for API clients, suppressed for SPA requests.
- CSRF: cookie-authenticated unsafe request with foreign `Origin` → 403 on `POST /api/jobs/backup`,
  `POST /api/jobs/restore`, and a config `PUT`; same-origin → allowed; Basic + no `Origin` → allowed.
- `CheckOrigin`: same-origin ✓, foreign ✗, allowlisted ✓, absent `Origin` ✓ (non-browser clients);
  a lookalike host (`evil-localhost:8080`, `localhost:8081`) is rejected — proves canonical parsing,
  not prefix matching.
- WS lifecycle: live stream closed on logout; live stream closed on TTL expiry.
- Assert no session token appears in log output (secret-safety, per `internal/AGENTS.md`).

**Frontend (Vitest):**
- `login()` posts credentials and does not touch `sessionStorage` (assert the key is never written).
- 401 from any endpoint throws `AuthError` and does **not** call `window.location.reload`.
- `logout()` posts `/api/logout`, clears the Query cache, navigates to `/login`.
- Auth store: `check()` maps 200 → `authed`, 401 → `anon`.
- Deep-link guard: unauthenticated navigation to a protected route redirects to `/login`.

## Verification

- Backend: `make pre-commit` — `fmt` → `vet` → `lint` → `test-unit` must pass; `coverage-check` is the
  known repo-wide 27%-vs-75% gap (#53), not a regression from this change.
- Frontend: `make web-validate`.
- Full-stack: `make build-with-web`, then `/run-daemon` and manually confirm:
  1. Login over plain HTTP works (cookie is **not** `Secure`) and no `bared_auth` key exists in
     `sessionStorage` (devtools → Application).
  2. Job log streaming connects and streams (the thing that is broken today).
  3. Logout returns to `/login` with no full document reload (Network tab shows no document request);
     hitting a protected URL afterwards redirects to `/login`.
  4. `curl -u user:pass http://localhost:8080/api/dashboard` still returns 200 (Basic preserved).
  5. A WS handshake with `Origin: http://evil.local` is rejected (`wscat`/`curl` with an `Origin` header).
  6. `make web-dev` against the running daemon still logs in and streams (validates the allowlist +
     `ws: true` proxy change).

## Risks

- **Breaking login on plain-HTTP installs** if `Secure` is unconditional — mitigated by the explicit
  flag + TLS detection and by manual check (1).
- **Plain HTTP is still the weak link.** An opaque session cookie over unencrypted LAN HTTP is
  observable and replayable by anyone on the wire. This change removes *password recovery* via XSS or
  devtools; it does **not** make plain HTTP safe. Startup warning + docs, and TLS/reverse-proxy guidance
  stays the real answer.
- **`SameSite=Strict` and reverse proxies** — a proxy on a different host/port than the SPA would make
  requests cross-site. Documented; the allowlist flag is the escape hatch.
- **Sessions vanish on restart** — accepted (single process, credentials can change on restart); users
  re-log in. Called out in the docs.
- **New brute-force surface.** `/api/login` is unauthenticated and checks a password. Mitigated here by
  a generic failure message, bounded body, and a small constant failure delay; a real per-IP limiter is
  a tracked follow-up (Q7).
