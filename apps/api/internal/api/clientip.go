package api

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// parseTrustedProxies canonicalises the configured proxy list once, at
// construction. Entries may be a single address ("10.1.2.3") or a CIDR
// ("10.0.0.0/8"); anything unparseable is dropped, the same way
// normaliseAllowedOrigins drops unusable origins.
func parseTrustedProxies(values []string) []netip.Prefix {
	if len(values) == 0 {
		return nil
	}

	out := make([]netip.Prefix, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}

		if prefix, err := netip.ParsePrefix(v); err == nil {
			out = append(out, prefix.Masked())
			continue
		}
		if addr, err := netip.ParseAddr(v); err == nil {
			out = append(out, netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()))
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

// clientIP returns the address a request is attributed to for rate limiting.
//
// X-Forwarded-For is honoured ONLY when the immediate peer is a configured
// trusted proxy. The header is client-controlled: trusting it unconditionally
// would let an attacker mint a fresh identity per request and walk straight
// through a per-IP limiter. This is the same reasoning that keeps
// --http-secure-cookies explicit instead of inferring TLS from
// X-Forwarded-Proto (see isSecureRequest).
func (s *Server) clientIP(r *http.Request) string {
	return clientIP(r, s.trustedProxies)
}

func clientIP(r *http.Request, trusted []netip.Prefix) string {
	peer := remoteAddr(r)
	if !peer.IsValid() {
		// Unparseable RemoteAddr (httptest sometimes, odd transports
		// otherwise). Fall back to the raw value so distinct peers still get
		// distinct buckets.
		return r.RemoteAddr
	}

	if len(trusted) == 0 || !isTrustedProxy(peer, trusted) {
		return peer.String()
	}

	// Walk right to left: the rightmost entry was appended by our own trusted
	// proxy, so the first address that is not itself a trusted proxy is the
	// furthest one we can still vouch for.
	forwarded := r.Header.Values("X-Forwarded-For")
	for i := len(forwarded) - 1; i >= 0; i-- {
		parts := strings.Split(forwarded[i], ",")
		for j := len(parts) - 1; j >= 0; j-- {
			addr, err := netip.ParseAddr(strings.TrimSpace(parts[j]))
			if err != nil {
				// A malformed hop means we can no longer trust anything to its
				// left; stop here rather than skipping past it.
				return peer.String()
			}
			addr = addr.Unmap()
			if !isTrustedProxy(addr, trusted) {
				return addr.String()
			}
		}
	}

	return peer.String()
}

// remoteAddr parses r.RemoteAddr into an address, dropping the port.
func remoteAddr(r *http.Request) netip.Addr {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

func isTrustedProxy(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
