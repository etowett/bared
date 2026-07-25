# Open questions — web-auth-session-cookie (issue #46)

> Things a human needs to decide. Resolve before implementing; record the answers inline.
> Proposals below reflect the Codex spec review (2026-07-25) — see `implementation-notes.md`
> for the review's P0 findings, which are already folded into `plan.md`.

## Decisions needed before coding

- [x] **Q3 — `Secure` cookie / trusted-proxy policy.** *(raised to blocking by the review)*
  Proposed: set `Secure` when `r.TLS != nil`, **or** when the operator passes an explicit
  `--http-secure-cookies`. Do **not** infer trust from `X-Forwarded-Proto` — it is client-controlled and
  spoofable when the daemon is directly reachable. Plain HTTP stays supported but logs a startup warning
  that session cookies are unencrypted. Accept, or require a fuller trusted-proxy model
  (`--trusted-proxy <cidr>`) instead?
  → **Answer:** Resolved — TLS detection + explicit `--http-secure-cookies`; `X-Forwarded-Proto` is not trusted. Plain HTTP logs a startup warning.

- [x] **Q9 — CSRF enforcement algorithm.** *(new, from the review)*
  Proposed: a dedicated `csrfMiddleware` — unsafe methods (`POST`/`PUT`/`PATCH`/`DELETE`) authenticated
  **by cookie** require a canonically-matching same-origin or allowlisted `Origin`; Basic-authenticated
  requests with no `Origin` pass (CLI). `SameSite=Strict` and `Sec-Fetch-Site` are defence-in-depth, not
  the primary control. Alternative: a double-submit CSRF token (more ceremony, more moving parts).
  → **Answer:** Resolved — dedicated `csrfMiddleware` (`csrf.go`), origin-based, separate from CORS.

- [x] **Q10 — WebSocket behaviour on logout / session expiry.** *(new, from the review — P0)*
  Proposed: close live streams. Session identity goes into the request context, the server keeps a
  registry of open connections keyed by token, and logout/expiry closes them. The alternative — TTL and
  revocation apply to *new requests only* — means a logged-out browser keeps receiving job logs, which
  is not acceptable for an authenticated stream. Confirm the registry approach, or accept
  handshake-only validation with the limitation documented?
  → **Answer:** Resolved — live streams are closed. The session carries a `done` channel; the WS handler selects on it and sends a 1008 close frame.

- [x] **Q11 — Cookie name and scope.** *(new, from the review)*
  Proposed: name `bared_session`, `Path=/`, **host-only** (no `Domain` attribute). Prefixed names
  (`__Host-`) imply `Secure`, so they can't be used on plain HTTP — should the name switch to
  `__Host-bared_session` when secure cookies are enabled, or stay stable for simplicity?
  → **Answer:** Resolved — `bared_session`, `Path=/`, host-only. Name stays stable rather than switching to `__Host-` under TLS.

- [x] **Q4 — `api.NewServer` signature.** *(proposal revised by the review)*
  It already takes 8 positional parameters (`apps/api/internal/api/server.go:32-33`) and this change adds
  3 more. Previously proposed: functional options mirroring `daemon.Option`. **Revised proposal:** a
  typed `ServerOptions` struct — simpler than mixing positional and functional styles, and touches the
  11 `NewServer(` sites in `server_test.go` + 6 in `websocket_test.go` once. Confirm the struct, or keep
  positional and defer?
  → **Answer:** Resolved — `ServerOptions` struct. All 11 `server_test.go` call sites migrated.

- [x] **Q6 — Scope: absorb #48 here?** The plan rewrites both `window.location` call sites
  (`client.ts:62-65`, `__root.tsx:22-26`), which is the entirety of #48's *code* scope. Proposed: do it
  here and close #48 with this PR (#48's own notes recommend exactly this) — after re-reading #48 to
  confirm nothing else in it is outstanding.
  → **Answer:** Resolved — yes. Both `window.location` call sites are rewritten here; close #48 with this PR.

## Decisions with a recommended default (confirm or override)

- [x] **Q1 — Session storage: in-memory or persisted to SQLite?** Proposed: **in-memory** (single
  process, stdlib-only). Restart logs everyone out — acceptable, and arguably right since credentials
  come from CLI flags that may have changed. Requires a bounded cleanup sweep so the map can't grow
  without limit.
  → **Answer:** Resolved — in-memory, with a bounded sweeper. Restart logs everyone out; documented.

- [x] **Q2 — Session TTL and idle behaviour.** Proposed: absolute **12h**, no sliding refresh, exposed as
  `--http-session-ttl`. Depends on Q10 to actually *be* absolute for WebSockets.
  → **Answer:** Resolved — absolute 12h via `--http-session-ttl`, no sliding refresh. Genuinely absolute now that Q10 closes live streams.

- [x] **Q5 — Does `corsMiddleware` still need to exist?** It returns `Access-Control-Allow-Origin: *`
  globally (`middleware.go:57-71`), but the Vite dev server proxies `/api`, so dev is same-origin.
  Proposed: reduce to an explicit allowlist (empty by default, populated by `--http-allowed-origin`),
  drop the wildcard, add `Vary: Origin`, and keep it **headers-only** — CSRF rejection lives in Q9's
  middleware. Is any external consumer relying on the wildcard?
  → **Answer:** Resolved — explicit allowlist, wildcard dropped, `Vary: Origin` added, headers-only.

- [x] **Q7 — Login rate-limiting.** Proposed: a per-IP limiter is **out of scope** and tracked as a
  follow-up, but ship with a bounded request body, a generic failure message (no user-vs-password
  distinction), and a small constant delay on failure. Acceptable?
  → **Answer:** Resolved — per-IP limiter deferred to a follow-up. Shipped with a bounded body, generic failure message, and a 250ms constant failure delay.

- [x] **Q12 — `WWW-Authenticate` suppression rule.** *(new, from the review)* Today a failed XHR triggers
  a native browser auth dialog because the header is always set (`middleware.go:16`). Proposed: suppress
  it when the request looks like the SPA (session cookie present, or `Sec-Fetch-Mode: cors`), keep it for
  plain API clients. The heuristic is imperfect — is a simpler rule (never send it on `/api/me` and
  `/api/login`, always otherwise) preferable?
  → **Answer:** Resolved — suppressed when the request carries fetch-metadata headers or a session cookie; kept for plain API clients.

- [x] **Q13 — Stale cookie on `/api/me` 401.** *(new, from the review)* Proposed: `handleMe` clears the
  cookie when the presented session is unknown/expired, so the browser stops re-sending a dead token.
  → **Answer:** Resolved — handled in `authMiddleware` (which sees every 401), not just `/api/me`.

- [x] **Q8 — Release labelling.** Issue #46 is `release:minor`. Basic auth keeps working, so API clients
  are unaffected; the *browser* auth model changes. Proposed: keep `minor`, and document both the browser
  migration and the plain-HTTP posture in the changelog.
  → **Answer:** Resolved — `minor` stands. Basic auth is unchanged for API clients; the browser migration is documented.
