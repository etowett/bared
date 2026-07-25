package api

import "net/http"

// csrfMiddleware rejects cross-site state-changing requests that authenticated
// with a session cookie.
//
// Introducing a cookie introduces CSRF: unlike an Authorization header, the
// browser attaches a cookie to any request a foreign page provokes. SameSite=Strict
// on the cookie is the first line of defence, but it is a browser behaviour we
// cannot verify server-side, so unsafe methods are checked against the Origin
// header here as well.
//
// Basic-authenticated requests are exempt — nothing is attached ambiently, so a
// foreign page cannot forge them, and CLI clients send no Origin.
//
// Must be installed after authMiddleware, which supplies the identity.
func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		auth, ok := authFromContext(r.Context())
		if !ok || auth.sess == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Browsers send Origin on every unsafe method, same-origin included,
		// so a cookie-authenticated request without one did not come from a
		// browser context we can vouch for.
		origin := r.Header.Get("Origin")
		if origin == "" || !s.originAllowed(origin, r.Host) {
			respondError(w, http.StatusForbidden, "Cross-site request rejected")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isSafeMethod reports whether a method is read-only and therefore outside the
// CSRF threat model.
func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
