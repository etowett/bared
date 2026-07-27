package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"bared/internal/util"
)

const (
	// maxLoginBodyBytes bounds the unauthenticated login body so a malicious
	// client cannot make the daemon allocate.
	maxLoginBodyBytes = 4 << 10

	// failedLoginDelay is a small constant penalty on every failed login. It is
	// not rate limiting — that is loginLimiter's job — but it takes the edge
	// off trivial online guessing.
	failedLoginDelay = 250 * time.Millisecond

	// loginFailureLogThreshold is the consecutive-failure count at which an IP
	// starts being logged as a suspected brute force rather than a typo.
	loginFailureLogThreshold = 3
)

// loginRequest is the POST /api/login body.
type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// identityResponse is returned by POST /api/login and GET /api/me.
type identityResponse struct {
	Username string `json:"username"`
}

// credentialsValid compares presented credentials against the configured pair
// in constant time. Both halves are always compared — no short-circuit — so the
// comparison doesn't leak which one was wrong via timing.
func (s *Server) credentialsValid(user, pass string) bool {
	if s.authUser == "" || s.authPass == "" {
		return false
	}

	userOK := subtle.ConstantTimeCompare([]byte(user), []byte(s.authUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(s.authPass)) == 1

	return userOK && passOK
}

// handleLogin validates credentials and issues a session cookie.
//
// This is the only unauthenticated endpoint that checks a password, against a
// single static credential pair, so it is the whole attack surface for online
// guessing. A per-IP token bucket is spent before anything else happens —
// before the body is even read — so a rate-limited attacker costs the daemon a
// map lookup.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.authUser == "" || s.authPass == "" {
		respondError(w, http.StatusServiceUnavailable, "Authentication is not configured")
		return
	}

	ip := s.clientIP(r)
	if !s.loginLimiter.Allow(ip) {
		util.GetLogger().WarnS("Login rate limit exceeded",
			"component", "api",
			"ip", ip)
		w.Header().Set("Retry-After", retryAfterSeconds)
		respondError(w, http.StatusTooManyRequests, "Too many login attempts. Try again later.")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			respondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			return
		}
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if !s.credentialsValid(req.Username, req.Password) {
		// The counter is what makes a brute force visible in the log. Neither
		// half of the presented credential is ever logged — not the password,
		// and not the username, which for a single-operator daemon is very
		// nearly as sensitive.
		if failures := s.loginLimiter.RecordFailure(ip); failures >= loginFailureLogThreshold {
			util.GetLogger().WarnS("Repeated failed login attempts",
				"component", "api",
				"ip", ip,
				"consecutive_failures", failures)
		}

		// Deliberately generic: never reveal whether the username or the
		// password was the wrong half.
		time.Sleep(s.failedLoginDelay)
		respondError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	s.loginLimiter.RecordSuccess(ip)

	token, err := s.sessions.Issue(req.Username)
	if err != nil {
		util.GetLogger().ErrorS("Failed to issue session",
			"component", "api",
			"error", err)
		respondError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	setSessionCookie(w, r, token, s.sessionTTL, s.secureCookies)

	respondJSON(w, http.StatusOK, identityResponse{Username: req.Username})
}

// handleLogout revokes the current session. Revoking closes the session's live
// WebSockets, so a logged-out browser stops receiving job logs immediately
// rather than streaming until it happens to disconnect.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if token := sessionTokenFromRequest(r); token != "" {
		s.sessions.Revoke(token)
	}

	clearSessionCookie(w, r, s.secureCookies)

	respondSuccess(w, "Logged out")
}

// handleMe reports the authenticated identity. The dashboard uses it as its
// auth guard, since an httpOnly cookie is invisible to JavaScript.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	auth, ok := authFromContext(r.Context())
	if !ok {
		// Unreachable behind authMiddleware; defensive.
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	respondJSON(w, http.StatusOK, identityResponse{Username: auth.username})
}
