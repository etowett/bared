import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fetchCurrentUser, login as loginRequest, logout as logoutRequest } from '../api/client'
import { useAuthStore } from './auth'

vi.mock('../api/client', () => ({
  fetchCurrentUser: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(),
}))

const mockFetchCurrentUser = vi.mocked(fetchCurrentUser)
const mockLogin = vi.mocked(loginRequest)
const mockLogout = vi.mocked(logoutRequest)

describe('auth store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useAuthStore.setState({ status: 'unknown', username: null })
  })

  it('resolves an authenticated session', async () => {
    mockFetchCurrentUser.mockResolvedValueOnce({ username: 'admin' })

    await expect(useAuthStore.getState().check()).resolves.toBe(true)

    expect(useAuthStore.getState().status).toBe('authenticated')
    expect(useAuthStore.getState().username).toBe('admin')
  })

  it('resolves an anonymous session', async () => {
    mockFetchCurrentUser.mockResolvedValueOnce(null)

    await expect(useAuthStore.getState().check()).resolves.toBe(false)

    expect(useAuthStore.getState().status).toBe('anonymous')
    expect(useAuthStore.getState().username).toBeNull()
  })

  it('treats a failed check as anonymous rather than hanging', async () => {
    mockFetchCurrentUser.mockRejectedValueOnce(new Error('network down'))

    await expect(useAuthStore.getState().check()).resolves.toBe(false)
    expect(useAuthStore.getState().status).toBe('anonymous')
  })

  // beforeLoad runs per route, so a burst of navigations must not each hit
  // /api/me.
  it('caches the answer and shares one in-flight request', async () => {
    mockFetchCurrentUser.mockResolvedValue({ username: 'admin' })

    const [a, b] = await Promise.all([
      useAuthStore.getState().check(),
      useAuthStore.getState().check(),
    ])
    expect(a).toBe(true)
    expect(b).toBe(true)

    await useAuthStore.getState().check()

    expect(mockFetchCurrentUser).toHaveBeenCalledTimes(1)
  })

  it('signs in and records the identity', async () => {
    mockLogin.mockResolvedValueOnce({ username: 'admin' })

    await useAuthStore.getState().signIn('admin', 'secret')

    expect(mockLogin).toHaveBeenCalledWith('admin', 'secret')
    expect(useAuthStore.getState().status).toBe('authenticated')
    expect(useAuthStore.getState().username).toBe('admin')
  })

  it('leaves the store untouched when sign-in fails', async () => {
    mockLogin.mockRejectedValueOnce(new Error('Invalid username or password'))

    await expect(useAuthStore.getState().signIn('admin', 'wrong')).rejects.toThrow()

    expect(useAuthStore.getState().status).toBe('unknown')
    expect(useAuthStore.getState().username).toBeNull()
  })

  it('signs out through the API', async () => {
    useAuthStore.setState({ status: 'authenticated', username: 'admin' })
    mockLogout.mockResolvedValueOnce(undefined)

    await useAuthStore.getState().signOut()

    expect(mockLogout).toHaveBeenCalled()
    expect(useAuthStore.getState().status).toBe('anonymous')
    expect(useAuthStore.getState().username).toBeNull()
  })

  it('marks the session anonymous without a request', () => {
    useAuthStore.setState({ status: 'authenticated', username: 'admin' })

    useAuthStore.getState().markAnonymous()

    expect(useAuthStore.getState().status).toBe('anonymous')
    expect(mockLogout).not.toHaveBeenCalled()
  })

  // After a 401 marks the session dead, the next guard must re-ask the server
  // rather than trusting the cached 'anonymous'.
  it('re-checks after being reset to unknown', async () => {
    useAuthStore.getState().markAnonymous()
    expect(useAuthStore.getState().status).toBe('anonymous')

    useAuthStore.setState({ status: 'unknown' })
    mockFetchCurrentUser.mockResolvedValueOnce({ username: 'admin' })

    await expect(useAuthStore.getState().check()).resolves.toBe(true)
  })
})
