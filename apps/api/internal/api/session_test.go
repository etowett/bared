package api

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStore_IssueAndValidate(t *testing.T) {
	store := newSessionStore(time.Hour)

	token, err := store.Issue("admin")
	require.NoError(t, err)
	require.NotEmpty(t, token)

	sess, ok := store.Validate(token)
	require.True(t, ok)
	assert.Equal(t, "admin", sess.username)
}

func TestSessionStore_TokensAreUnique(t *testing.T) {
	store := newSessionStore(time.Hour)

	seen := make(map[string]struct{})
	for range 100 {
		token, err := store.Issue("admin")
		require.NoError(t, err)

		_, dup := seen[token]
		require.False(t, dup, "session tokens must not repeat")
		seen[token] = struct{}{}
	}
}

func TestSessionStore_UnknownToken(t *testing.T) {
	store := newSessionStore(time.Hour)

	_, ok := store.Validate("not-a-real-token")
	assert.False(t, ok)

	_, ok = store.Validate("")
	assert.False(t, ok)
}

func TestSessionStore_Revoke(t *testing.T) {
	store := newSessionStore(time.Hour)

	token, err := store.Issue("admin")
	require.NoError(t, err)
	sess, ok := store.Validate(token)
	require.True(t, ok)

	store.Revoke(token)

	_, ok = store.Validate(token)
	assert.False(t, ok)
	assert.Equal(t, 0, store.count())

	select {
	case <-sess.Done():
	default:
		t.Fatal("revoking a session must close its connections")
	}

	// Revoking twice must not panic on the already-closed channel.
	store.Revoke(token)
}

func TestSessionStore_ExpiryBoundary(t *testing.T) {
	store := newSessionStore(time.Hour)

	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	token, err := store.Issue("admin")
	require.NoError(t, err)

	// One nanosecond before expiry the session is still good.
	now = now.Add(time.Hour - time.Nanosecond)
	_, ok := store.Validate(token)
	assert.True(t, ok, "session should be valid right up to its expiry")

	// Exactly at expiry it is not.
	now = now.Add(time.Nanosecond)
	_, ok = store.Validate(token)
	assert.False(t, ok, "session should expire at its deadline")
	assert.Equal(t, 0, store.count(), "validating an expired session should reap it")
}

func TestSessionStore_SweepClosesExpiredSessions(t *testing.T) {
	store := newSessionStore(time.Hour)

	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }

	expiring, err := store.Issue("admin")
	require.NoError(t, err)
	expiringSession, ok := store.Validate(expiring)
	require.True(t, ok)

	now = now.Add(2 * time.Hour)

	// Issued after the clock moved, so it outlives the sweep.
	fresh, err := store.Issue("admin")
	require.NoError(t, err)

	store.sweep()

	assert.Equal(t, 1, store.count())
	_, ok = store.Validate(fresh)
	assert.True(t, ok)

	select {
	case <-expiringSession.Done():
	default:
		t.Fatal("sweeping an expired session must close its connections")
	}
}

func TestSessionStore_SweeperStops(t *testing.T) {
	store := newSessionStore(time.Hour)
	store.startSweeper(time.Millisecond)

	store.stopSweeper()
	// Stopping twice must not panic.
	store.stopSweeper()
}

// A nil store is what a Server built without sessions has; it must degrade to
// "no sessions exist" rather than panicking.
func TestSessionStore_NilIsSafe(t *testing.T) {
	var store *sessionStore

	_, err := store.Issue("admin")
	assert.Error(t, err)

	_, ok := store.Validate("anything")
	assert.False(t, ok)

	assert.Equal(t, 0, store.count())
	assert.NotPanics(t, func() {
		store.Revoke("anything")
		store.sweep()
		store.startSweeper(time.Millisecond)
		store.stopSweeper()
	})
}

func TestSessionStore_ConcurrentAccess(t *testing.T) {
	store := newSessionStore(time.Hour)

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()

			token, err := store.Issue("admin")
			if err != nil {
				t.Error(err)
				return
			}
			if _, ok := store.Validate(token); !ok {
				t.Error("freshly issued session should validate")
				return
			}
			store.sweep()
			store.Revoke(token)
		}()
	}
	wg.Wait()

	assert.Equal(t, 0, store.count())
}
