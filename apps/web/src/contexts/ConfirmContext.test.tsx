import { render, screen, waitFor } from '@/test/utils'
import { renderHook } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { useConfirm, type ConfirmOptions } from './ConfirmContext'

/**
 * Drives `confirm()` the way a page does — from a click handler that awaits the
 * answer — and reports every settlement so double-resolves are visible.
 */
function Harness({
  options,
  onSettled,
}: {
  options: ConfirmOptions
  onSettled: (_answer: boolean) => void
}) {
  const confirm = useConfirm()
  return (
    <button
      onClick={async () => {
        onSettled(await confirm(options))
      }}
    >
      Trigger
    </button>
  )
}

const deleteOptions: ConfirmOptions = {
  title: 'Delete Target',
  description: 'Are you sure you want to delete "prod-db"?',
  confirmLabel: 'Delete Target',
  variant: 'destructive',
}

describe('ConfirmProvider / useConfirm', () => {
  it('throws when used outside a provider', () => {
    // React logs the thrown error; silence it so the run stays readable.
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => renderHook(() => useConfirm())).toThrow(/must be used within a ConfirmProvider/)
    consoleError.mockRestore()
  })

  it('renders no dialog until confirm() is called', () => {
    render(<Harness options={deleteOptions} onSettled={vi.fn()} />)

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows the dialog and resolves true when confirmed', async () => {
    const user = userEvent.setup()
    const onSettled = vi.fn()

    render(<Harness options={deleteOptions} onSettled={onSettled} />)
    await user.click(screen.getByRole('button', { name: 'Trigger' }))

    expect(await screen.findByText('Delete Target', { selector: 'h2' })).toBeInTheDocument()
    expect(screen.getByText('Are you sure you want to delete "prod-db"?')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Delete Target' }))

    await waitFor(() => expect(onSettled).toHaveBeenCalledWith(true))
    // Confirming runs `onConfirm` *and* `onOpenChange(false)`; only one of them
    // may settle the promise.
    expect(onSettled).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('resolves false when cancelled', async () => {
    const user = userEvent.setup()
    const onSettled = vi.fn()

    render(<Harness options={deleteOptions} onSettled={onSettled} />)
    await user.click(screen.getByRole('button', { name: 'Trigger' }))
    await screen.findByRole('dialog')

    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(onSettled).toHaveBeenCalledWith(false))
    expect(onSettled).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('resolves false when dismissed with Escape', async () => {
    const user = userEvent.setup()
    const onSettled = vi.fn()

    render(<Harness options={deleteOptions} onSettled={onSettled} />)
    await user.click(screen.getByRole('button', { name: 'Trigger' }))
    await screen.findByRole('dialog')

    await user.keyboard('{Escape}')

    await waitFor(() => expect(onSettled).toHaveBeenCalledWith(false))
    expect(onSettled).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('resolves false when dismissed by clicking the close button', async () => {
    const user = userEvent.setup()
    const onSettled = vi.fn()

    render(<Harness options={deleteOptions} onSettled={onSettled} />)
    await user.click(screen.getByRole('button', { name: 'Trigger' }))
    await screen.findByRole('dialog')

    await user.click(screen.getByRole('button', { name: /close/i }))

    await waitFor(() => expect(onSettled).toHaveBeenCalledWith(false))
    expect(onSettled).toHaveBeenCalledTimes(1)
  })

  it('answers a superseded prompt false instead of leaving it pending', async () => {
    const user = userEvent.setup()
    const first = vi.fn()
    const second = vi.fn()

    function DoubleHarness() {
      const confirm = useConfirm()
      return (
        <button
          onClick={() => {
            void confirm({ title: 'First', description: 'First?' }).then(first)
            void confirm({ title: 'Second', description: 'Second?' }).then(second)
          }}
        >
          Trigger
        </button>
      )
    }

    render(<DoubleHarness />)
    await user.click(screen.getByRole('button', { name: 'Trigger' }))

    await waitFor(() => expect(first).toHaveBeenCalledWith(false))
    expect(await screen.findByText('Second?')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Confirm' }))
    await waitFor(() => expect(second).toHaveBeenCalledWith(true))
  })

  it('can be reused after a dismissal', async () => {
    const user = userEvent.setup()
    const onSettled = vi.fn()

    render(<Harness options={deleteOptions} onSettled={onSettled} />)

    await user.click(screen.getByRole('button', { name: 'Trigger' }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(onSettled).toHaveBeenNthCalledWith(1, false))

    await user.click(screen.getByRole('button', { name: 'Trigger' }))
    await screen.findByRole('dialog')
    await user.click(screen.getByRole('button', { name: 'Delete Target' }))
    await waitFor(() => expect(onSettled).toHaveBeenNthCalledWith(2, true))
  })

  it('uses the supplied labels', async () => {
    const user = userEvent.setup()

    render(
      <Harness
        options={{
          title: 'Cancel Job',
          description: 'Stop this job?',
          confirmLabel: 'Cancel Job',
          cancelLabel: 'Keep Running',
        }}
        onSettled={vi.fn()}
      />
    )
    await user.click(screen.getByRole('button', { name: 'Trigger' }))

    expect(await screen.findByRole('button', { name: 'Keep Running' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Cancel Job' })).toBeInTheDocument()
  })
})
