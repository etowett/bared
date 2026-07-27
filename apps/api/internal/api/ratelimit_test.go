package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// fakeClock drives a limiter's notion of time so a test can cross the refill
// window without sleeping.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// newTestLoginServer returns a server whose login limiter runs on a fake clock.
func newTestLoginServer(t *testing.T, opts ...func(*ServerOptions)) (*Server, *fakeClock) {
	t.Helper()

	server := newAuthTestServer(t, opts...)
	clock := newFakeClock()
	server.loginLimiter.now = clock.Now
	// The constant per-failure penalty is real behaviour but would make a
	// burst-sized table take seconds.
	server.failedLoginDelay = 0

	return server, clock
}

// postLogin submits a login attempt as remoteAddr. The username is always the
// configured one: these tests vary the password and the client address, since
// a wrong username and a wrong password are deliberately indistinguishable.
func postLogin(t *testing.T, server *Server, remoteAddr, password string,
	headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/login", loginRequestBody(t, "admin", password))
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rr := httptest.NewRecorder()
	server.handleLogin(rr, req)

	return rr
}

// The headline requirement for #88: a burst of failures from one IP stops
// getting 401s and starts getting 429s.
func TestHandleLogin_RateLimitedAfterRepeatedFailures(t *testing.T) {
	server, _ := newTestLoginServer(t)

	for i := range loginRateBurst {
		rr := postLogin(t, server, "203.0.113.10:5555", "wrong", nil)
		require.Equal(t, http.StatusUnauthorized, rr.Code, "attempt %d should still be evaluated", i+1)
	}

	rr := postLogin(t, server, "203.0.113.10:5555", "wrong", nil)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, strconv.Itoa(int(loginRateInterval.Seconds())), rr.Header().Get("Retry-After"))

	// Even the right password is refused while the bucket is empty: the limit
	// is on the attempt, not on the outcome.
	rr = postLogin(t, server, "203.0.113.10:5555", "secret", nil)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Nil(t, sessionCookie(rr))
	assert.Equal(t, 0, server.sessions.count())
}

// Once the window has passed, a legitimate login from the same IP works again —
// a lockout that never lifts is a denial of service against the operator.
func TestHandleLogin_SucceedsAfterWindowElapses(t *testing.T) {
	server, clock := newTestLoginServer(t)

	for range loginRateBurst + 1 {
		postLogin(t, server, "203.0.113.10:5555", "wrong", nil)
	}
	require.Equal(t, http.StatusTooManyRequests,
		postLogin(t, server, "203.0.113.10:5555", "secret", nil).Code)

	clock.Advance(loginRateInterval)

	rr := postLogin(t, server, "203.0.113.10:5555", "secret", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.NotNil(t, sessionCookie(rr), "a valid login after the window must still issue a session")
}

// One IP exhausting its bucket must not lock out anybody else.
func TestHandleLogin_RateLimitIsPerIP(t *testing.T) {
	server, _ := newTestLoginServer(t)

	for range loginRateBurst + 1 {
		postLogin(t, server, "203.0.113.10:5555", "wrong", nil)
	}
	require.Equal(t, http.StatusTooManyRequests,
		postLogin(t, server, "203.0.113.10:5555", "secret", nil).Code)

	rr := postLogin(t, server, "198.51.100.7:5555", "secret", nil)
	assert.Equal(t, http.StatusOK, rr.Code, "a different IP has its own budget")
}

// X-Forwarded-For is only believed from a configured trusted proxy. Believing
// it unconditionally would let an attacker mint a new identity per request.
func TestHandleLogin_ForwardedForOnlyFromTrustedProxies(t *testing.T) {
	tests := []struct {
		name           string
		trustedProxies []string
		// spoofed is the X-Forwarded-For each attempt claims; each attempt
		// claims a different one, which is what an attacker would do.
		spoofPerRequest bool
		wantFinalStatus int
	}{
		{
			name:            "untrusted peer spoofing a new IP per request is still limited",
			spoofPerRequest: true,
			wantFinalStatus: http.StatusTooManyRequests,
		},
		{
			name:            "trusted proxy forwarding distinct clients gives each its own budget",
			trustedProxies:  []string{"192.0.2.0/24"},
			spoofPerRequest: true,
			wantFinalStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := newTestLoginServer(t, func(o *ServerOptions) { o.TrustedProxies = tt.trustedProxies })

			var last *httptest.ResponseRecorder
			for i := range loginRateBurst + 1 {
				headers := map[string]string{}
				if tt.spoofPerRequest {
					headers["X-Forwarded-For"] = fmt.Sprintf("203.0.113.%d", i+1)
				}
				last = postLogin(t, server, "192.0.2.50:4444", "wrong", headers)
			}

			assert.Equal(t, tt.wantFinalStatus, last.Code)
		})
	}
}

func TestClientIP(t *testing.T) {
	trusted := []string{"192.0.2.0/24", "10.1.2.3"}

	tests := []struct {
		name       string
		remoteAddr string
		forwarded  []string
		trusted    []string
		want       string
	}{
		{
			name:       "no trusted proxies configured ignores the header",
			remoteAddr: "203.0.113.9:1234",
			forwarded:  []string{"198.51.100.1"},
			want:       "203.0.113.9",
		},
		{
			name:       "untrusted peer ignores the header",
			remoteAddr: "203.0.113.9:1234",
			forwarded:  []string{"198.51.100.1"},
			trusted:    trusted,
			want:       "203.0.113.9",
		},
		{
			name:       "trusted peer takes the rightmost untrusted hop",
			remoteAddr: "192.0.2.50:1234",
			forwarded:  []string{"198.51.100.1, 203.0.113.9"},
			trusted:    trusted,
			want:       "203.0.113.9",
		},
		{
			name:       "trusted peer skips its own trusted hops",
			remoteAddr: "192.0.2.50:1234",
			forwarded:  []string{"203.0.113.9, 192.0.2.60, 10.1.2.3"},
			trusted:    trusted,
			want:       "203.0.113.9",
		},
		{
			name:       "repeated headers are read right to left",
			remoteAddr: "192.0.2.50:1234",
			forwarded:  []string{"198.51.100.1", "203.0.113.9"},
			trusted:    trusted,
			want:       "203.0.113.9",
		},
		{
			name:       "a malformed hop stops the walk at the peer",
			remoteAddr: "192.0.2.50:1234",
			forwarded:  []string{"203.0.113.9, not-an-ip"},
			trusted:    trusted,
			want:       "192.0.2.50",
		},
		{
			name:       "all hops trusted falls back to the peer",
			remoteAddr: "192.0.2.50:1234",
			forwarded:  []string{"192.0.2.60"},
			trusted:    trusted,
			want:       "192.0.2.50",
		},
		{
			name:       "IPv6 peer",
			remoteAddr: "[2001:db8::1]:1234",
			want:       "2001:db8::1",
		},
		{
			name:       "unparseable remote address falls back to the raw value",
			remoteAddr: "pipe",
			want:       "pipe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader("{}"))
			req.RemoteAddr = tt.remoteAddr
			for _, v := range tt.forwarded {
				req.Header.Add("X-Forwarded-For", v)
			}

			assert.Equal(t, tt.want, clientIP(req, parseTrustedProxies(tt.trusted)))
		})
	}
}

func TestParseTrustedProxies(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		// probe is an address expected to be inside the parsed set, or "" when
		// nothing should parse.
		probe    string
		wantLen  int
		probeHit bool
	}{
		{name: "empty", input: nil, wantLen: 0},
		{name: "cidr", input: []string{"10.0.0.0/8"}, wantLen: 1, probe: "10.4.5.6", probeHit: true},
		{name: "bare address", input: []string{"192.0.2.7"}, wantLen: 1, probe: "192.0.2.7", probeHit: true},
		{name: "bare address is exact", input: []string{"192.0.2.7"}, wantLen: 1, probe: "192.0.2.8"},
		{name: "ipv6 cidr", input: []string{"2001:db8::/32"}, wantLen: 1, probe: "2001:db8::99", probeHit: true},
		{name: "whitespace is trimmed", input: []string{"  10.0.0.0/8  "}, wantLen: 1, probe: "10.0.0.1", probeHit: true},
		{name: "garbage is dropped", input: []string{"not-an-ip", ""}, wantLen: 0},
		{name: "mixed keeps the good ones", input: []string{"nope", "10.0.0.0/8"}, wantLen: 1, probe: "10.0.0.1", probeHit: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTrustedProxies(tt.input)

			assert.Len(t, got, tt.wantLen)
			if tt.probe != "" {
				req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader("{}"))
				req.RemoteAddr = tt.probe + ":1234"
				req.Header.Set("X-Forwarded-For", "203.0.113.200")

				// A trusted probe yields the forwarded address; an untrusted
				// one yields the peer.
				want := tt.probe
				if tt.probeHit {
					want = "203.0.113.200"
				}
				assert.Equal(t, want, clientIP(req, got))
			}
		})
	}
}

// The map must not grow without bound: it is fed by unauthenticated requests
// from arbitrary addresses.
func TestIPRateLimiter_MapStaysBounded(t *testing.T) {
	clock := newFakeClock()
	limiter := newIPRateLimiter(rate.Every(loginRateInterval), loginRateBurst, loginLimiterTTL, 32)
	limiter.now = clock.Now

	for i := range 5000 {
		limiter.Allow(fmt.Sprintf("198.51.100.%d", i))
	}

	assert.LessOrEqual(t, limiter.count(), 32, "the entry cap must hold under a flood of distinct IPs")
	assert.Positive(t, limiter.count())
}

// Idle buckets are reaped, but only after the TTL — evicting sooner would hand
// an attacker a free reset.
func TestIPRateLimiter_SweepsIdleEntries(t *testing.T) {
	clock := newFakeClock()
	limiter := newIPRateLimiter(rate.Every(loginRateInterval), loginRateBurst, loginLimiterTTL, loginLimiterMaxEntries)
	limiter.now = clock.Now

	limiter.Allow("203.0.113.1")
	limiter.Allow("203.0.113.2")
	require.Equal(t, 2, limiter.count())

	clock.Advance(loginLimiterTTL / 2)
	limiter.sweep()
	assert.Equal(t, 2, limiter.count(), "buckets must survive within the TTL")

	// Keep one alive, let the other go idle.
	limiter.Allow("203.0.113.1")
	clock.Advance(loginLimiterTTL + time.Second)
	limiter.sweep()

	assert.Equal(t, 0, limiter.count(), "everything idle past the TTL is reaped")
}

func TestIPRateLimiter_FailureCounter(t *testing.T) {
	limiter := newLoginRateLimiter()

	assert.Equal(t, 1, limiter.RecordFailure("203.0.113.1"))
	assert.Equal(t, 2, limiter.RecordFailure("203.0.113.1"))
	assert.Equal(t, 1, limiter.RecordFailure("203.0.113.2"), "counters are per IP")

	limiter.RecordSuccess("203.0.113.1")
	assert.Equal(t, 1, limiter.RecordFailure("203.0.113.1"), "a success clears the streak")
}

// A nil limiter must not rate limit and must not panic, matching sessionStore's
// nil-safety.
func TestIPRateLimiter_NilIsPermissive(t *testing.T) {
	var limiter *ipRateLimiter

	assert.True(t, limiter.Allow("203.0.113.1"))
	assert.Equal(t, 0, limiter.RecordFailure("203.0.113.1"))
	assert.Equal(t, 0, limiter.count())
	limiter.RecordSuccess("203.0.113.1")
	limiter.sweep()
	limiter.startSweeper(time.Millisecond)
	limiter.stopSweeper()
}

func TestIPRateLimiter_ConcurrentUse(t *testing.T) {
	limiter := newLoginRateLimiter()

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("203.0.113.%d", n%10)
			limiter.Allow(key)
			limiter.RecordFailure(key)
			limiter.RecordSuccess(key)
			limiter.sweep()
		}(i)
	}
	wg.Wait()

	assert.LessOrEqual(t, limiter.count(), 10)
}
