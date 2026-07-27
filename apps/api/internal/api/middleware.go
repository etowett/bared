package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"bared/internal/util"
)

// authContext is the authenticated identity attached to a request.
type authContext struct {
	username string

	// sess is the dashboard session backing this request, or nil when the
	// request authenticated with Basic auth (CLI/API clients). Handlers that
	// hold a connection open — the WebSocket log stream — select on its Done
	// channel so revocation and expiry reach already-established connections.
	sess *session
}

type ctxKey int

const authCtxKey ctxKey = iota

func withAuth(ctx context.Context, auth *authContext) context.Context {
	return context.WithValue(ctx, authCtxKey, auth)
}

// authFromContext returns the identity attached by authMiddleware.
func authFromContext(ctx context.Context) (*authContext, bool) {
	auth, ok := ctx.Value(authCtxKey).(*authContext)
	return auth, ok && auth != nil
}

// authMiddleware authenticates a request by session cookie, falling back to
// HTTP Basic Auth.
//
// The cookie is what the dashboard uses: the browser attaches it automatically,
// including on the WebSocket handshake, which is the header the browser cannot
// set. Basic auth stays first-class for CLI and API clients (see
// internal/client).
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		staleCookie := false

		if token := sessionTokenFromRequest(r); token != "" {
			if sess, ok := s.sessions.Validate(token); ok {
				ctx := withAuth(r.Context(), &authContext{username: sess.username, sess: sess})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			staleCookie = true
		}

		// Basic auth checks the same static credential pair /api/login does, on
		// every authenticated route, so it has to spend from the same per-IP
		// budget. Rate limiting only the login form would leave the identical
		// password oracle wide open behind GET /api/jobs.
		//
		// Only failures are charged, so a working CLI client — which presents
		// Basic auth on every single request — is never throttled. An
		// unauthenticated probe with no Authorization header costs nothing
		// either, so the dashboard's 401-then-login flow is unaffected.
		if user, pass, ok := r.BasicAuth(); ok {
			ip := s.clientIP(r)
			if !s.loginLimiter.Permit(ip) {
				util.GetLogger().WarnS("Basic auth rate limit exceeded",
					"component", "api",
					"ip", ip)
				w.Header().Set("Retry-After", retryAfterSeconds)
				respondError(w, http.StatusTooManyRequests, "Too many authentication attempts. Try again later.")
				return
			}

			if s.credentialsValid(user, pass) {
				s.loginLimiter.RecordSuccess(ip)
				ctx := withAuth(r.Context(), &authContext{username: user})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if failures := s.loginLimiter.RecordFailure(ip); failures >= loginFailureLogThreshold {
				util.GetLogger().WarnS("Repeated failed authentication attempts",
					"component", "api",
					"ip", ip,
					"consecutive_failures", failures)
			}
		}

		if staleCookie {
			// Stop the browser re-sending a session we no longer honour.
			clearSessionCookie(w, r, s.secureCookies)
		}

		if !isBrowserRequest(r) {
			// Only offer Basic to non-browser clients. Sending it to the SPA
			// makes a failed XHR pop the browser's native credential dialog,
			// which this app's login form is meant to replace.
			w.Header().Set("WWW-Authenticate", `Basic realm="BareD API"`)
		}

		respondError(w, http.StatusUnauthorized, "Authentication required")
	})
}

// isBrowserRequest reports whether a request came from a browser. Fetch
// metadata headers are sent by every browser that can run this dashboard and by
// no CLI client.
func isBrowserRequest(r *http.Request) bool {
	return r.Header.Get("Sec-Fetch-Mode") != "" ||
		r.Header.Get("Sec-Fetch-Site") != "" ||
		sessionTokenFromRequest(r) != ""
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := util.GetLogger()
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		logger.InfoS("HTTP request",
			"component", "api",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.statusCode,
			"duration", duration)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// corsMiddleware echoes CORS headers for explicitly allowed origins.
//
// This emits headers and nothing more — it is not an access control. A browser
// consults CORS before letting a *script* read a response; it does not stop the
// request from being made, so CORS can never stand in for CSRF protection. That
// job belongs to csrfMiddleware.
//
// The previous wildcard ("Access-Control-Allow-Origin: *") is gone: it is
// incompatible with credentialed requests and advertised the API to every page
// on the internet. The dashboard is served from this same binary and the Vite
// dev server proxies /api, so both are same-origin and need no CORS at all.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin, r.Host) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		// Origin varies the response even when it isn't allowed, so caches
		// must not serve one origin's response to another.
		w.Header().Add("Vary", "Origin")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, code int, data interface{}) {
	logger := util.GetLogger()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			logger.ErrorS("Failed to encode JSON response",
				"component", "api",
				"error", err)
		}
	}
}

// respondError writes an error response
func respondError(w http.ResponseWriter, code int, message string) {
	respondJSON(w, code, ErrorResponse{
		Error:   http.StatusText(code),
		Message: message,
	})
}

// respondSuccess writes a success message
func respondSuccess(w http.ResponseWriter, message string) {
	respondJSON(w, http.StatusOK, map[string]string{
		"message": message,
	})
}
