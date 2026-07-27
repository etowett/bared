/* eslint-disable @typescript-eslint/no-explicit-any */
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '../../test/utils'

// The router is real — `test/utils.tsx` supplies one at `/`, which is what the
// nav's active state and the sheet's links resolve against. Only the daemon
// probe is stubbed, so these tests are about the shell's layout and the sheet.
vi.mock('@/hooks/useDaemonStatus', () => ({
  useDaemonStatus: () => ({
    reachable: true,
    checking: false,
    version: '1.2.3',
    refetch: vi.fn(),
  }),
}))

import { AppLayout } from './AppLayout'

describe('AppLayout', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(window.localStorage.getItem as any).mockReturnValue(null)
  })

  const renderLayout = () =>
    render(
      <AppLayout onLogout={vi.fn()}>
        <p>Page content</p>
      </AppLayout>
    )

  it('renders the page content', () => {
    renderLayout()
    expect(screen.getByText('Page content')).toBeInTheDocument()
  })

  it('only offsets the main region at the lg breakpoint', () => {
    const { container } = renderLayout()

    const main = container.querySelector('main')
    expect(main).toHaveClass('lg:pl-64')
    // The unconditional `pl-64` left ~64px of content at 320px wide.
    expect(main?.className).not.toMatch(/(^|\s)pl-(16|64)(\s|$)/)
  })

  it('hides the persistent sidebar below lg', () => {
    const { container } = renderLayout()

    const aside = container.querySelector('aside')
    expect(aside).toHaveClass('hidden')
    expect(aside).toHaveClass('lg:block')
  })

  describe('mobile navigation sheet', () => {
    it('has a labelled menu button that opens the sheet', async () => {
      const user = userEvent.setup()
      renderLayout()

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

      await user.click(screen.getByRole('button', { name: 'Open navigation menu' }))

      const sheet = await screen.findByRole('dialog')
      expect(within(sheet).getByRole('link', { name: 'Overview' })).toBeInTheDocument()
      expect(within(sheet).getByRole('link', { name: 'Configuration' })).toBeInTheDocument()
    })

    it('closes when a destination is chosen', async () => {
      const user = userEvent.setup()
      renderLayout()

      await user.click(screen.getByRole('button', { name: 'Open navigation menu' }))
      const sheet = await screen.findByRole('dialog')

      await user.click(within(sheet).getByRole('link', { name: 'Jobs' }))

      await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      })
    })

    it('closes on the sheet close button', async () => {
      const user = userEvent.setup()
      renderLayout()

      await user.click(screen.getByRole('button', { name: 'Open navigation menu' }))
      const sheet = await screen.findByRole('dialog')

      await user.click(within(sheet).getByRole('button', { name: 'Close' }))

      await waitFor(() => {
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
      })
    })
  })
})
