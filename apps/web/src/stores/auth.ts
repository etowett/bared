import { create } from 'zustand'
import { fetchCurrentUser, login as loginRequest, logout as logoutRequest } from '@/api/client'

export type AuthStatus = 'unknown' | 'authenticated' | 'anonymous'

interface AuthState {
  status: AuthStatus
  username: string | null

  /**
   * Resolves the session against the server.
   *
   * The session cookie is httpOnly, so unlike the old sessionStorage check this
   * cannot be answered locally. The result is cached in the store so the route
   * guard doesn't round-trip on every navigation; concurrent callers share one
   * in-flight request.
   */
  check: () => Promise<boolean>

  signIn: (username: string, password: string) => Promise<void>
  signOut: () => Promise<void>

  /** Marks the session dead without a request — used by the 401 handler. */
  markAnonymous: () => void
}

// Shared across concurrent check() callers so a burst of route loads issues a
// single /api/me request.
let inFlight: Promise<boolean> | null = null

export const useAuthStore = create<AuthState>((set, get) => ({
  status: 'unknown',
  username: null,

  check: async () => {
    const { status } = get()
    if (status !== 'unknown') {
      return status === 'authenticated'
    }

    inFlight ??= fetchCurrentUser()
      .then((user) => {
        set({
          status: user ? 'authenticated' : 'anonymous',
          username: user?.username ?? null,
        })
        return Boolean(user)
      })
      .catch(() => {
        // Network failure is not proof of a dead session, but the app cannot
        // proceed without one either.
        set({ status: 'anonymous', username: null })
        return false
      })
      .finally(() => {
        inFlight = null
      })

    return inFlight
  },

  signIn: async (username, password) => {
    const user = await loginRequest(username, password)
    set({ status: 'authenticated', username: user.username })
  },

  signOut: async () => {
    await logoutRequest()
    set({ status: 'anonymous', username: null })
  },

  markAnonymous: () => set({ status: 'anonymous', username: null }),
}))
