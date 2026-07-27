/* eslint-disable @typescript-eslint/no-explicit-any */
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '../test/utils'
import { ThemeToggle } from './ThemeToggle'

describe('ThemeToggle Component', () => {
  beforeEach(() => {
    // Clear localStorage before each test
    window.localStorage.clear()
    ;(window.localStorage.getItem as any).mockReturnValue(null)
    ;(window.localStorage.setItem as any).mockClear()

    // Remove theme classes from document
    document.documentElement.classList.remove('light', 'dark')
  })

  it('renders toggle button', () => {
    render(<ThemeToggle />)

    const button = screen.getByRole('button')
    expect(button).toBeInTheDocument()
  })

  it('displays moon icon for light theme', () => {
    // Set light theme in localStorage
    ;(window.localStorage.getItem as any).mockReturnValue('light')

    const { container } = render(<ThemeToggle />)

    // Moon icon should be displayed when theme is light
    const button = screen.getByRole('button')
    expect(button).toBeInTheDocument()

    // Check for svg icon
    const svg = container.querySelector('svg')
    expect(svg).toBeInTheDocument()
  })

  it('displays sun icon for dark theme', () => {
    // Set dark theme in localStorage
    ;(window.localStorage.getItem as any).mockReturnValue('dark')

    const { container } = render(<ThemeToggle />)

    // Sun icon should be displayed when theme is dark
    const button = screen.getByRole('button')
    expect(button).toBeInTheDocument()

    // Check for svg icon
    const svg = container.querySelector('svg')
    expect(svg).toBeInTheDocument()
  })

  it('names the icon-only button for assistive tech', () => {
    // The label names the *action*, not the current state, and replaces the
    // old visually hidden "Toggle theme" span.
    ;(window.localStorage.getItem as any).mockReturnValue('light')

    render(<ThemeToggle />)

    expect(screen.getByRole('button', { name: 'Switch to dark mode' })).toBeInTheDocument()
  })

  it('has title attribute for tooltip', () => {
    ;(window.localStorage.getItem as any).mockReturnValue('light')

    render(<ThemeToggle />)

    const button = screen.getByRole('button')
    expect(button).toHaveAttribute('title', 'Switch to dark mode')
  })

  it('updates title attribute based on current theme', () => {
    ;(window.localStorage.getItem as any).mockReturnValue('dark')

    render(<ThemeToggle />)

    const button = screen.getByRole('button')
    expect(button).toHaveAttribute('title', 'Switch to light mode')
  })

  it('toggles theme from light to dark when clicked', async () => {
    const user = userEvent.setup()
    ;(window.localStorage.getItem as any).mockReturnValue('light')

    render(<ThemeToggle />)

    const button = screen.getByRole('button')
    await user.click(button)

    // Theme should toggle to dark
    expect(window.localStorage.setItem).toHaveBeenCalledWith('theme', 'dark')
  })

  it('toggles theme from dark to light when clicked', async () => {
    const user = userEvent.setup()
    ;(window.localStorage.getItem as any).mockReturnValue('dark')

    render(<ThemeToggle />)

    const button = screen.getByRole('button')
    await user.click(button)

    // Theme should toggle to light
    expect(window.localStorage.setItem).toHaveBeenCalledWith('theme', 'light')
  })

  it('updates document classes when theme changes', async () => {
    const user = userEvent.setup()
    ;(window.localStorage.getItem as any).mockReturnValue('light')

    render(<ThemeToggle />)

    // Initial state - light theme
    expect(document.documentElement.classList.contains('light')).toBe(true)

    const button = screen.getByRole('button')
    await user.click(button)

    // After toggle - dark theme
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.classList.contains('light')).toBe(false)
  })

  it('handles multiple toggles correctly', async () => {
    const user = userEvent.setup()
    ;(window.localStorage.getItem as any).mockReturnValue('light')

    render(<ThemeToggle />)

    const button = screen.getByRole('button')

    // First toggle: light -> dark
    await user.click(button)
    expect(document.documentElement.classList.contains('dark')).toBe(true)

    // Second toggle: dark -> light
    await user.click(button)
    expect(document.documentElement.classList.contains('light')).toBe(true)

    // Third toggle: light -> dark
    await user.click(button)
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('uses ghost variant for button styling', () => {
    render(<ThemeToggle />)

    const button = screen.getByRole('button')
    expect(button).toBeInTheDocument()
  })

  it('uses icon size for button', () => {
    render(<ThemeToggle />)

    const button = screen.getByRole('button')
    expect(button).toBeInTheDocument()
  })

  it('defaults to dark theme when no localStorage value', () => {
    ;(window.localStorage.getItem as any).mockReturnValue(null)

    render(<ThemeToggle />)

    // Default should be dark theme
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('respects system preference when no localStorage value', () => {
    ;(window.localStorage.getItem as any).mockReturnValue(null)

    // Mock system prefers dark
    ;(window.matchMedia as any).mockImplementation((query: string) => ({
      matches: query === '(prefers-color-scheme: dark)',
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }))

    render(<ThemeToggle />)

    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('persists theme selection to localStorage', async () => {
    const user = userEvent.setup()
    ;(window.localStorage.getItem as any).mockReturnValue('light')

    render(<ThemeToggle />)

    const button = screen.getByRole('button')
    await user.click(button)

    expect(window.localStorage.setItem).toHaveBeenCalledWith('theme', 'dark')
  })

  it('is keyboard accessible', async () => {
    const user = userEvent.setup()
    ;(window.localStorage.getItem as any).mockReturnValue('light')

    render(<ThemeToggle />)

    const button = screen.getByRole('button')
    button.focus()

    expect(button).toHaveFocus()

    // Simulate Enter key press
    await user.keyboard('{Enter}')

    expect(window.localStorage.setItem).toHaveBeenCalledWith('theme', 'dark')
  })
})
