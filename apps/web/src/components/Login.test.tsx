import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { login as loginRequest } from '../api/client'
import { useAuthStore } from '../stores/auth'
import { render, screen, waitFor } from '../test/utils'
import { Login } from './Login'

// Mock the API client
vi.mock('../api/client', () => ({
  login: vi.fn(),
  logout: vi.fn(),
  fetchCurrentUser: vi.fn(),
}))

const mockLogin = vi.mocked(loginRequest)

describe('Login Component', () => {
  const mockOnLogin = vi.fn()
  const mockFetch = vi.fn()

  beforeEach(() => {
    vi.clearAllMocks()
    globalThis.fetch = mockFetch
    useAuthStore.setState({ status: 'unknown', username: null })
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

  it('logs in through the login endpoint', async () => {
    const user = userEvent.setup()
    mockLogin.mockResolvedValueOnce({ username: 'admin' })

    render(<Login onLogin={mockOnLogin} />)

    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'password')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(mockLogin).toHaveBeenCalledWith('admin', 'password')
      expect(mockOnLogin).toHaveBeenCalled()
    })

    // The credential probe against /api/dashboard is gone.
    expect(mockFetch).not.toHaveBeenCalled()
    expect(useAuthStore.getState().status).toBe('authenticated')
  })

  it('handles invalid credentials', async () => {
    const user = userEvent.setup()
    mockLogin.mockRejectedValueOnce(new Error('Invalid username or password'))

    render(<Login onLogin={mockOnLogin} />)

    await user.type(screen.getByLabelText(/username/i), 'wronguser')
    await user.type(screen.getByLabelText(/password/i), 'wrongpass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByText(/invalid username or password/i)).toBeInTheDocument()
      expect(mockOnLogin).not.toHaveBeenCalled()
    })

    // Both inputs keep what the user typed (#124)
    expect(screen.getByLabelText(/username/i)).toHaveValue('wronguser')
    expect(screen.getByLabelText(/password/i)).toHaveValue('wrongpass')
  })

  it('retains both credentials after a network failure', async () => {
    const user = userEvent.setup()
    mockLogin.mockRejectedValueOnce(new Error('Failed to connect to server'))

    render(<Login onLogin={mockOnLogin} />)

    await user.type(screen.getByLabelText(/username/i), 'testuser')
    await user.type(screen.getByLabelText(/password/i), 'testpass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent(/failed to connect to server/i)
    })

    expect(screen.getByLabelText(/username/i)).toHaveValue('testuser')
    expect(screen.getByLabelText(/password/i)).toHaveValue('testpass')
  })

  it('focuses and selects the password field after a failed sign-in', async () => {
    const user = userEvent.setup()
    mockLogin.mockRejectedValueOnce(new Error('Invalid username or password'))

    render(<Login onLogin={mockOnLogin} />)

    await user.type(screen.getByLabelText(/username/i), 'wronguser')
    await user.type(screen.getByLabelText(/password/i), 'wrongpass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    const passwordInput = screen.getByLabelText(/password/i) as HTMLInputElement
    await waitFor(() => {
      expect(passwordInput).toHaveFocus()
    })

    // Contents are selected, so retyping is a single keystroke away.
    expect(passwordInput.selectionStart).toBe(0)
    expect(passwordInput.selectionEnd).toBe('wrongpass'.length)
  })

  it('handles network errors', async () => {
    const user = userEvent.setup()
    mockLogin.mockRejectedValueOnce(new Error('Failed to connect to server'))

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
    let resolveLogin: (value: { username: string }) => void
    mockLogin.mockReturnValueOnce(
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
    resolveLogin!({ username: 'testuser' })

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
    mockLogin.mockRejectedValueOnce(new Error('Invalid username or password'))

    render(<Login onLogin={mockOnLogin} />)

    // First failed login
    await user.type(screen.getByLabelText(/username/i), 'wronguser')
    await user.type(screen.getByLabelText(/password/i), 'wrongpass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.getByText(/invalid username or password/i)).toBeInTheDocument()
    })

    // Second login attempt - error should be cleared
    mockLogin.mockResolvedValueOnce({ username: 'correctuser' })
    await user.type(screen.getByLabelText(/username/i), 'correctuser')
    await user.type(screen.getByLabelText(/password/i), 'correctpass')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() => {
      expect(screen.queryByText(/invalid username or password/i)).not.toBeInTheDocument()
    })
  })
})
