import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '../test/utils'
import userEvent from '@testing-library/user-event'
import { Login } from './Login'
import * as apiClient from '../api/client'

// Mock the API client
vi.mock('../api/client', () => ({
  setAuth: vi.fn(),
}))

describe('Login Component', () => {
  const mockOnLogin = vi.fn()
  const mockFetch = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    globalThis.fetch = mockFetch
  })

  it('renders login form with username and password fields', () => {
    render(<Login onLogin={mockOnLogin} />)

    expect(screen.getByRole('heading', { name: /BareD/i })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: /Backup Dashboard/i })).toBeInTheDocument()
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
  })

  it('updates input values when user types', async () => {
    const user = userEvent.setup()
    render(<Login onLogin={mockOnLogin} />)

    const usernameInput = screen.getByLabelText(/username/i)
    const passwordInput = screen.getByLabelText(/password/i)

    await user.type(usernameInput, 'testuser')
    await user.type(passwordInput, 'testpass')

    expect(usernameInput).toHaveValue('testuser')
    expect(passwordInput).toHaveValue('testpass')
  })

  it('handles successful login', async () => {
    const user = userEvent.setup()
    mockFetch.mockResolvedValueOnce({ ok: true })

    render(<Login onLogin={mockOnLogin} />)

    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'password')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(apiClient.setAuth).toHaveBeenCalledWith('admin', 'password')
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/dashboard',
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: expect.stringContaining('Basic'),
          }),
        })
      )
      expect(mockOnLogin).toHaveBeenCalled()
    })
  })

  it('handles invalid credentials', async () => {
    const user = userEvent.setup()
    mockFetch.mockResolvedValueOnce({ ok: false })

    render(<Login onLogin={mockOnLogin} />)

    await user.type(screen.getByLabelText(/username/i), 'wronguser')
    await user.type(screen.getByLabelText(/password/i), 'wrongpass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByText(/invalid username or password/i)).toBeInTheDocument()
      expect(mockOnLogin).not.toHaveBeenCalled()
    })

    // Verify inputs are cleared on error
    expect(screen.getByLabelText(/username/i)).toHaveValue('')
    expect(screen.getByLabelText(/password/i)).toHaveValue('')
  })

  it('handles network errors', async () => {
    const user = userEvent.setup()
    mockFetch.mockRejectedValueOnce(new Error('Network error'))

    render(<Login onLogin={mockOnLogin} />)

    await user.type(screen.getByLabelText(/username/i), 'testuser')
    await user.type(screen.getByLabelText(/password/i), 'testpass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByText(/failed to connect to server/i)).toBeInTheDocument()
      expect(mockOnLogin).not.toHaveBeenCalled()
    })
  })

  it('shows loading state during authentication', async () => {
    const user = userEvent.setup()
    let resolveLogin: (value: { ok: boolean }) => void
    mockFetch.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveLogin = resolve
      })
    )

    render(<Login onLogin={mockOnLogin} />)

    await user.type(screen.getByLabelText(/username/i), 'testuser')
    await user.type(screen.getByLabelText(/password/i), 'testpass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    // Button should show loading state
    expect(screen.getByRole('button', { name: /signing in.../i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /signing in.../i })).toBeDisabled()

    // Inputs should be disabled
    expect(screen.getByLabelText(/username/i)).toBeDisabled()
    expect(screen.getByLabelText(/password/i)).toBeDisabled()

    // Resolve the login
    resolveLogin!({ ok: true })

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
    })
  })

  it('prevents form submission when inputs are empty', () => {
    render(<Login onLogin={mockOnLogin} />)

    const usernameInput = screen.getByLabelText(/username/i)
    const passwordInput = screen.getByLabelText(/password/i)

    // HTML5 validation should prevent submission
    expect(usernameInput).toBeRequired()
    expect(passwordInput).toBeRequired()
  })

  it('clears error message when starting a new login attempt', async () => {
    const user = userEvent.setup()
    mockFetch.mockResolvedValueOnce({ ok: false })

    render(<Login onLogin={mockOnLogin} />)

    // First failed login
    await user.type(screen.getByLabelText(/username/i), 'wronguser')
    await user.type(screen.getByLabelText(/password/i), 'wrongpass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByText(/invalid username or password/i)).toBeInTheDocument()
    })

    // Second login attempt - error should be cleared
    mockFetch.mockResolvedValueOnce({ ok: true })
    await user.type(screen.getByLabelText(/username/i), 'correctuser')
    await user.type(screen.getByLabelText(/password/i), 'correctpass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.queryByText(/invalid username or password/i)).not.toBeInTheDocument()
    })
  })
})
