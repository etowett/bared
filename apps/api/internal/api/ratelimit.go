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

	limit      rate.Limit
	burst      int
	ttl        time.Duration
	maxEntries int

	// now is swappable so tests can advance past the window without sleeping.
	now func() time.Time

	stopOnce sync.Once
	stop     chan struct{}
}

func newIPRateLimiter(limit rate.Limit, burst int, ttl time.Duration, maxEntries int) *ipRateLimiter {
	return &ipRateLimiter{
		entries:    make(map[string]*limiterEntry),
		limit:      limit,
		burst:      burst,
		ttl:        ttl,
		maxEntries: maxEntries,
		now:        time.Now,
		stop:       make(chan struct{}),
	}
}

// newLoginRateLimiter builds the limiter guarding POST /api/login.
func newLoginRateLimiter() *ipRateLimiter {
	return newIPRateLimiter(
		rate.Every(loginRateInterval),
		loginRateBurst,
		loginLimiterTTL,
		loginLimiterMaxEntries,
	)
}

// Allow consumes a token for key, reporting whether the attempt may proceed.
func (l *ipRateLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	entry := l.entryLocked(key, now)

	return entry.limiter.AllowN(now, 1)
}

// RecordFailure counts a failed attempt for key and returns the running total,
// so the caller can log a brute-force attempt without keeping state itself.
func (l *ipRateLimiter) RecordFailure(key string) int {
	if l == nil {
		return 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.entryLocked(key, l.now())
	entry.failures++

	return entry.failures
}

// RecordSuccess clears the failure streak for key. The token bucket is left
// alone: a successful login is still an attempt.
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
		// Evicting the least-recently-seen entry is the right trade: an
		// attacker cycling addresses would otherwise push out nothing, and a
		// hard refusal here would deny logins to everyone.
		for len(l.entries) >= l.maxEntries {
			l.evictOldestLocked()
		}
	}

	entry := &limiterEntry{
		limiter:  rate.NewLimiter(l.limit, l.burst),
		lastSeen: now,
	}
	l.entries[key] = entry

	return entry
}

// evictOldestLocked drops the least-recently-seen entry. Callers must hold l.mu
// and must not call it on an empty map.
func (l *ipRateLimiter) evictOldestLocked() {
	var oldestKey string
	var oldestSeen time.Time

	for key, entry := range l.entries {
		if oldestKey == "" || entry.lastSeen.Before(oldestSeen) {
			oldestKey, oldestSeen = key, entry.lastSeen
		}
	}

	delete(l.entries, oldestKey)
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
