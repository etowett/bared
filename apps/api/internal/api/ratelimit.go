package api

import (
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// loginRateBurst is how many login attempts a single IP may make back to
	// back before it has to wait. Enough for a person fat-fingering their
	// password, nowhere near enough for a dictionary.
	loginRateBurst = 5

	// loginRateInterval is the sustained rate an IP refills at — one attempt
	// per interval once the burst is spent.
	loginRateInterval = 15 * time.Second

	// loginLimiterTTL is how long an idle IP's bucket is kept. Dropping it
	// early would hand an attacker a free reset; keeping it forever would be an
	// unbounded map fed by unauthenticated requests.
	loginLimiterTTL = 30 * time.Minute

	// loginLimiterMaxEntries caps the map regardless of TTL, so a spoofed or
	// distributed flood cannot grow it without bound. Once it is hit, the
	// least-recently-seen entries are evicted.
	loginLimiterMaxEntries = 4096

	// loginLimiterSweepInterval is how often idle buckets are reaped.
	loginLimiterSweepInterval = time.Minute
)

// retryAfterSeconds is the Retry-After value sent with a 429: the interval a
// rate-limited IP refills one token in.
var retryAfterSeconds = strconv.Itoa(int(loginRateInterval.Seconds()))

// limiterEntry is one IP's token bucket plus the failure counter used to make
// a brute-force attempt visible in the log.
type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
	failures int
}

// ipRateLimiter is a bounded map of per-IP token buckets.
//
// It is modelled on sessionStore: a swept map behind a mutex, with a swappable
// clock so tests can cross the window boundary without sleeping, and nil-safe
// methods so a Server built without one (tests, embedded uses) simply does not
// rate limit.
type ipRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*limiterEntry

	// overflow is the shared bucket handed to new keys once the map is full
	// and every entry in it is throttled. See overflowLocked.
	overflow *limiterEntry

	limit      rate.Limit
	burst      int
	ttl        time.Duration
	maxEntries int

	// now is swappable so tests can advance past the window without sleeping.
	now func() time.Time

	stopOnce sync.Once
	stop     chan struct{}
}

// newIPRateLimiter builds a limiter with the authentication policy above.
// maxEntries is a parameter only so tests can force eviction without standing
// up 4096 entries.
func newIPRateLimiter(maxEntries int) *ipRateLimiter {
	return &ipRateLimiter{
		entries:    make(map[string]*limiterEntry),
		limit:      rate.Every(loginRateInterval),
		burst:      loginRateBurst,
		ttl:        loginLimiterTTL,
		maxEntries: maxEntries,
		now:        time.Now,
		stop:       make(chan struct{}),
	}
}

// newLoginRateLimiter builds the limiter guarding every path that checks the
// operator's password: POST /api/login and the Basic auth fallback.
func newLoginRateLimiter() *ipRateLimiter {
	return newIPRateLimiter(loginLimiterMaxEntries)
}

// Permit reports whether key may make another authentication attempt. It does
// NOT consume a token — only RecordFailure does.
//
// Spending on every attempt would rate limit success as well as failure, which
// would throttle the CLI (internal/client authenticates with Basic auth on
// every request) down to a handful of calls a minute. Charging only for
// failures still bounds guessing, because a guess that is wrong is the only
// kind an attacker can make: once the bucket is empty the credentials are no
// longer evaluated at all, so a correct guess arriving later is refused too.
func (l *ipRateLimiter) Permit(key string) bool {
	if l == nil {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	entry := l.entryLocked(key, now)

	return entry.limiter.TokensAt(now) >= 1
}

// RecordFailure charges key a token for a failed attempt and returns its
// running failure count, so the caller can log a brute force without keeping
// state itself.
func (l *ipRateLimiter) RecordFailure(key string) int {
	if l == nil {
		return 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	entry := l.entryLocked(key, now)
	entry.limiter.AllowN(now, 1)
	entry.failures++

	return entry.failures
}

// RecordSuccess clears the failure streak for key. No token is charged — a
// successful authentication is not what the limiter exists to stop.
func (l *ipRateLimiter) RecordSuccess(key string) {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if entry, ok := l.entries[key]; ok {
		entry.failures = 0
	}
}

// entryLocked returns key's bucket, creating it if needed and enforcing the
// entry cap on the way. Callers must hold l.mu.
func (l *ipRateLimiter) entryLocked(key string, now time.Time) *limiterEntry {
	if entry, ok := l.entries[key]; ok {
		entry.lastSeen = now
		return entry
	}

	if l.maxEntries > 0 && len(l.entries) >= l.maxEntries {
		l.sweepLocked(now)
		for len(l.entries) >= l.maxEntries {
			if !l.evictOneLocked(now) {
				// Every tracked bucket is still throttled, so there is nothing
				// safe to drop. Share the overflow bucket rather than evict a
				// throttled attacker into a fresh one — cycling addresses would
				// otherwise be a free reset.
				return l.overflowLocked(now)
			}
		}
	}

	entry := &limiterEntry{
		limiter:  rate.NewLimiter(l.limit, l.burst),
		lastSeen: now,
	}
	l.entries[key] = entry

	return entry
}

// overflowLocked returns the single bucket shared by every key that arrives
// once the map is full and nothing in it can be evicted. It lives outside the
// map so it can never be evicted itself, and so this path cannot grow memory.
// Callers must hold l.mu.
func (l *ipRateLimiter) overflowLocked(now time.Time) *limiterEntry {
	if l.overflow == nil {
		l.overflow = &limiterEntry{limiter: rate.NewLimiter(l.limit, l.burst)}
	}
	l.overflow.lastSeen = now

	return l.overflow
}

// evictOneLocked drops the least-recently-seen bucket that still has tokens to
// spare, reporting whether it found one. Buckets that are currently throttled
// are deliberately never evicted: re-creating one hands its owner a full burst
// back, which is exactly what an attacker rotating addresses wants. Callers
// must hold l.mu.
func (l *ipRateLimiter) evictOneLocked(now time.Time) bool {
	var victim string
	var victimSeen time.Time
	found := false

	for key, entry := range l.entries {
		if entry.limiter.TokensAt(now) < 1 {
			continue
		}
		if !found || entry.lastSeen.Before(victimSeen) {
			victim, victimSeen, found = key, entry.lastSeen, true
		}
	}

	if !found {
		return false
	}

	delete(l.entries, victim)
	return true
}

// sweep reaps buckets that have been idle for longer than the TTL.
func (l *ipRateLimiter) sweep() {
	if l == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweepLocked(l.now())
}

func (l *ipRateLimiter) sweepLocked(now time.Time) {
	cutoff := now.Add(-l.ttl)
	for key, entry := range l.entries {
		if entry.lastSeen.Before(cutoff) {
			delete(l.entries, key)
		}
	}
}

// count reports the number of tracked IPs (used by tests).
func (l *ipRateLimiter) count() int {
	if l == nil {
		return 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.entries)
}

// startSweeper runs the reaper until stopSweeper is called.
func (l *ipRateLimiter) startSweeper(interval time.Duration) {
	if l == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				l.sweep()
			case <-l.stop:
				return
			}
		}
	}()
}

// stopSweeper halts the reaper. Safe to call more than once.
func (l *ipRateLimiter) stopSweeper() {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() { close(l.stop) })
}
