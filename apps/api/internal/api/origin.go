package api

import (
	"net/url"
	"slices"
	"strings"
)

// canonicalOrigin normalises an Origin header value to "scheme://host:port"
// with the default port made explicit, so comparisons cannot be fooled by
// "http://host" vs "http://host:80". It returns "" if the value is not a
// usable origin.
//
// Everything origin-related — CORS, CSRF, and the WebSocket upgrader — goes
// through here. String prefix matching on origins is how lookalike hosts
// ("evil-localhost" matching "localhost") get through.
func canonicalOrigin(origin string) string {
	if origin == "" || origin == "null" {
		return ""
	}

	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return ""
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}

	return scheme + "://" + canonicalHost(u.Host, scheme)
}

// canonicalHost lowercases a host and gives it an explicit port.
func canonicalHost(host, scheme string) string {
	host = strings.ToLower(host)
	if strings.Contains(host, ":") && !strings.HasSuffix(host, "]") {
		return host
	}

	switch scheme {
	case "https":
		return host + ":443"
	default:
		return host + ":80"
	}
}

// originAllowed reports whether origin may talk to this server.
//
// Same-origin is decided on host:port alone, ignoring scheme — matching
// gorilla/websocket's own default. Comparing schemes here would break every
// deployment behind a TLS-terminating proxy, where the browser sends
// "https://…" but the daemon sees a plain HTTP request.
//
// The configured allowlist is compared as a full canonical origin, because an
// operator passing --http-allowed-origin wrote the scheme deliberately.
func (s *Server) originAllowed(origin string, requestHost string) bool {
	canonical := canonicalOrigin(origin)
	if canonical == "" {
		return false
	}

	u, err := url.Parse(origin)
	if err == nil && u.Host != "" && requestHost != "" {
		scheme := strings.ToLower(u.Scheme)
		if canonicalHost(u.Host, scheme) == canonicalHost(requestHost, scheme) {
			return true
		}
	}

	return slices.Contains(s.allowedOrigins, canonical)
}

// normaliseAllowedOrigins canonicalises the configured allowlist once, at
// construction, so request handling is a plain string comparison. Entries that
// aren't usable origins are dropped.
func normaliseAllowedOrigins(origins []string) []string {
	if len(origins) == 0 {
		return nil
	}

	out := make([]string, 0, len(origins))
	for _, o := range origins {
		if canonical := canonicalOrigin(strings.TrimSpace(o)); canonical != "" {
			out = append(out, canonical)
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}
