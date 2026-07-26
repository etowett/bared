package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	// sessionCookieName is the cookie the dashboard authenticates with. The
	// value is an opaque server-issued token — nothing is derived from the
	// operator's password, so the cookie cannot be reversed into credentials.
	sessionCookieName = "bared_session"

	// defaultSessionTTL is the absolute lifetime of a dashboard session.
	defaultSessionTTL = 12 * time.Hour

	// sessionSweepInterval is how often expired sessions are reaped. It also
	// bounds how long a live WebSocket can outlive its session's expiry.
	sessionSweepInterval = time.Minute

	// sessionTokenBytes is the entropy behind a session token.
	sessionTokenBytes = 32
)

// session is a single authenticated dashboard session.
//
// done is closed when the session is revoked (logout) or reaped (expiry), which
// is how already-upgraded WebSockets learn to stop streaming — they only get
// authenticated at the handshake, so without this a logged-out browser would
// keep receiving job logs until it disconnected on its own.
type session struct {
	username  string
	expiresAt time.Time

	done      chan struct{}
	closeOnce sync.Once
}

// close terminates the session's live connections. Safe to call repeatedly.
func (s *session) close() {
	s.closeOnce.Do(func() { close(s.done) })
}

// Done returns a channel closed when the session is revoked or expires.
func (s *session) Done() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.done
}

// sessionStore holds active dashboard sessions in memory.
//
// In-memory is the deliberate choice: the daemon is a single process serving a
// single credential pair, so there is nothing to share. Sessions do not survive
// a restart — which is arguably correct, since the credentials come from CLI
// flags that may have changed.
//
// All methods are safe on a nil receiver, so a Server built without a store
// (tests, Basic-auth-only setups) simply has no sessions.
type sessionStore struct {
	mu       sync.RWMutex
	ttl      time.Duration
	sessions map[string]*session

	// now is swappable so tests can cross the expiry boundary without sleeping.
	now func() time.Time

	stopOnce sync.Once
	stop     chan struct{}
}

func newSessionStore(ttl time.Duration) *sessionStore {
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}
	return &sessionStore{
		ttl:      ttl,
		sessions: make(map[string]*session),
		now:      time.Now,
		stop:     make(chan struct{}),
	}
}

// Issue mints a new session token for username.
func (st *sessionStore) Issue(username string) (string, error) {
	if st == nil {
		return "", fmt.Errorf("session store not configured")
	}

	buf := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(buf)

	st.mu.Lock()
	defer st.mu.Unlock()
	st.sessions[token] = &session{
		username:  username,
		expiresAt: st.now().Add(st.ttl),
		done:      make(chan struct{}),
	}

	return token, nil
}

// Validate returns the session for token if it exists and has not expired. An
// expired session is reaped (and its connections closed) on the way out.
func (st *sessionStore) Validate(token string) (*session, bool) {
	if st == nil || token == "" {
		return nil, false
	}

	st.mu.RLock()
	sess, ok := st.sessions[token]
	st.mu.RUnlock()
	if !ok {
		return nil, false
	}

	if !st.now().Before(sess.expiresAt) {
		st.Revoke(token)
		return nil, false
	}

	return sess, true
}

// Revoke drops a session and terminates its live connections.
func (st *sessionStore) Revoke(token string) {
	if st == nil || token == "" {
		return
	}

	st.mu.Lock()
	sess, ok := st.sessions[token]
	delete(st.sessions, token)
	st.mu.Unlock()

	if ok {
		sess.close()
	}
}

// sweep reaps every expired session, closing their connections.
func (st *sessionStore) sweep() {
	if st == nil {
		return
	}

	now := st.now()

	st.mu.Lock()
	expired := make([]*session, 0)
	for token, sess := range st.sessions {
		if !now.Before(sess.expiresAt) {
			expired = append(expired, sess)
			delete(st.sessions, token)
		}
	}
	st.mu.Unlock()

	for _, sess := range expired {
		sess.close()
	}
}

// count reports the number of live sessions (used by tests).
func (st *sessionStore) count() int {
	if st == nil {
		return 0
	}
	st.mu.RLock()
	defer st.mu.RUnlock()
	return len(st.sessions)
}

// startSweeper runs the expiry reaper until stopSweeper is called. Without it,
// a session's absolute TTL would only be enforced on the next request — live
// WebSockets would stream past their expiry.
func (st *sessionStore) startSweeper(interval time.Duration) {
	if st == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				st.sweep()
			case <-st.stop:
				return
			}
		}
	}()
}

// stopSweeper halts the reaper. Safe to call more than once.
func (st *sessionStore) stopSweeper() {
	if st == nil {
		return
	}
	st.stopOnce.Do(func() { close(st.stop) })
}

// isSecureRequest reports whether the session cookie may carry the Secure
// attribute. It is deliberately NOT inferred from X-Forwarded-Proto: that
// header is client-controlled and spoofable whenever the daemon is reachable
// directly, so operators behind a TLS-terminating proxy opt in explicitly with
// --http-secure-cookies.
func isSecureRequest(r *http.Request, forceSecure bool) bool {
	return forceSecure || r.TLS != nil
}

// setSessionCookie writes the session cookie for a successful login.
//
// Secure is deliberately conditional rather than constant — see isSecureRequest.
// #nosec G124 -- Secure is set by isSecureRequest; hardcoding it would drop the
// cookie on the plain-HTTP LAN deployments this daemon is normally run in.
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration, forceSecure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecureRequest(r, forceSecure),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(ttl.Seconds()),
		Expires:  time.Now().Add(ttl),
	})
}

// clearSessionCookie expires the session cookie. The attributes must match the
// ones it was set with or the browser will keep the original.
//
// #nosec G124 -- mirrors setSessionCookie; see the note there.
func clearSessionCookie(w http.ResponseWriter, r *http.Request, forceSecure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecureRequest(r, forceSecure),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// sessionTokenFromRequest returns the raw session token presented by a request.
func sessionTokenFromRequest(r *http.Request) string {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}
