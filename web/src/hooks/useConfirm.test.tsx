import userEvent from '@testing-library/user-event'
import { act, renderHook } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { useConfirm } from './useConfirm'
import { render, screen, waitFor } from '../test/utils'

describe('useConfirm Hook', () => {
  it('returns confirm function and null dialog initially', () => {
    const { result } = renderHook(() => useConfirm())

    expect(result.current.confirm).toBeInstanceOf(Function)
    expect(result.current.ConfirmDialog).toBeNull()
  })

  it('displays confirmation dialog when confirm is called', async () => {
    const { result } = renderHook(() => useConfirm())

    act(() => {
      result.current.confirm({
        title: 'Test Title',
        description: 'Test Description',
      })
    })

    const TestComponent = () => <>{result.current.ConfirmDialog}</>
    render(<TestComponent />)

    await waitFor(() => {
      expect(screen.getByText('Test Title')).toBeInTheDocument()
      expect(screen.getByText('Test Description')).toBeInTheDocument()
    })
  })

  it('resolves promise with true when confirmed', async () => {
    const user = userEvent.setup()
    const { result } = renderHook(() => useConfirm())

    let promiseResult: boolean | undefined

    act(() => {
      result.current
        .confirm({
          title: 'Confirm Action',
          description: 'Are you sure?',
        })
        .then((res) => {
          promiseResult = res
        })
    })

    const TestComponent = () => <>{result.current.ConfirmDialog}</>
    render(<TestComponent />)

    await waitFor(() => {
      expect(screen.getByText('Confirm Action')).toBeInTheDocument()
    })

    const confirmButton = screen.getByRole('button', { name: /confirm/i })
    await user.click(confirmButton)

    await waitFor(() => {
      expect(promiseResult).toBe(true)
    })
  })

  it('resolves promise with false when cancelled', async () => {
    const user = userEvent.setup()
    const { result } = renderHook(() => useConfirm())

    let promiseResult: boolean | undefined

    act(() => {
      result.current
        .confirm({
          title: 'Confirm Action',
          description: 'Are you sure?',
        })
        .then((res) => {
          promiseResult = res
        })
    })

    const TestComponent = () => <>{result.current.ConfirmDialog}</>
    render(<TestComponent />)

    await waitFor(() => {
      expect(screen.getByText('Confirm Action')).toBeInTheDocument()
    })

    const cancelButton = screen.getByRole('button', { name: /cancel/i })
    await user.click(cancelButton)

    await waitFor(() => {
      expect(promiseResult).toBe(false)
    })
  })

  it('uses custom confirm label when provided', async () => {
    const { result } = renderHook(() => useConfirm())

    act(() => {
      result.current.confirm({
        title: 'Delete Item',
        description: 'This cannot be undone',
        confirmLabel: 'Yes, Delete',
      })
    })

    const TestComponent = () => <>{result.current.ConfirmDialog}</>
    render(<TestComponent />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /yes, delete/i })).toBeInTheDocument()
    })
  })

  it('uses custom cancel label when provided', async () => {
    const { result } = renderHook(() => useConfirm())

    act(() => {
      result.current.confirm({
        title: 'Confirm',
        description: 'Proceed?',
        cancelLabel: 'No, Go Back',
      })
    })

    const TestComponent = () => <>{result.current.ConfirmDialog}</>
    render(<TestComponent />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /no, go back/i })).toBeInTheDocument()
    })
  })

  it('passes variant to dialog component', async () => {
    const { result } = renderHook(() => useConfirm())

    act(() => {
      result.current.confirm({
        title: 'Delete',
        description: 'Are you sure?',
        variant: 'destructive',
      })
    })

    const TestComponent = () => <>{result.current.ConfirmDialog}</>
    render(<TestComponent />)

    await waitFor(() => {
      expect(screen.getByText('Delete')).toBeInTheDocument()
    })
  })

  it('hides dialog after confirmation', async () => {
    const user = userEvent.setup()
    const { result } = renderHook(() => useConfirm())

    act(() => {
      result.current.confirm({
        title: 'Confirm',
        description: 'Proceed?',
      })
    })

    const TestComponent = () => <>{result.current.ConfirmDialog}</>
    const { rerender } = render(<TestComponent />)

    await waitFor(() => {
      expect(screen.getByText('Confirm')).toBeInTheDocument()
    })

    const confirmButton = screen.getByRole('button', { name: /confirm/i })
    await user.click(confirmButton)

    rerender(<TestComponent />)

    await waitFor(() => {
      expect(screen.queryByText('Confirm')).not.toBeInTheDocument()
    })
  })

  it('hides dialog after cancellation', async () => {
    const user = userEvent.setup()
    const { result } = renderHook(() => useConfirm())

    act(() => {
      result.current.confirm({
        title: 'Confirm',
        description: 'Proceed?',
      })
    })

    const TestComponent = () => <>{result.current.ConfirmDialog}</>
    const { rerender } = render(<TestComponent />)

    await waitFor(() => {
      expect(screen.getByText('Confirm')).toBeInTheDocument()
    })

    const cancelButton = screen.getByRole('button', { name: /cancel/i })
    await user.click(cancelButton)

    rerender(<TestComponent />)

    await waitFor(() => {
      expect(screen.queryByText('Confirm')).not.toBeInTheDocument()
    })
  })

  it('handles multiple confirm calls sequentially', async () => {
    const user = userEvent.setup()
    const { result } = renderHook(() => useConfirm())

    // First confirm
    let firstResult: boolean | undefined
    act(() => {
      result.current
        .confirm({
          title: 'First Confirm',
          description: 'First action',
        })
        .then((res) => {
          firstResult = res
        })
    })

    const TestComponent = () => <>{result.current.ConfirmDialog}</>
    const { rerender } = render(<TestComponent />)

    await waitFor(() => {
      expect(screen.getByText('First Confirm')).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /confirm/i }))

    await waitFor(() => {
      expect(firstResult).toBe(true)
    })

    // Second confirm
    let secondResult: boolean | undefined
    act(() => {
      result.current
        .confirm({
          title: 'Second Confirm',
          description: 'Second action',
        })
        .then((res) => {
          secondResult = res
        })
    })

    rerender(<TestComponent />)

    await waitFor(() => {
      expect(screen.getByText('Second Confirm')).toBeInTheDocument()
    })

    await user.click(screen.getByRole('button', { name: /cancel/i }))

    await waitFor(() => {
      expect(secondResult).toBe(false)
    })
  })

  it('handles dialog close via onOpenChange', async () => {
    const { result } = renderHook(() => useConfirm())

    let promiseResult: boolean | undefined

    act(() => {
      result.current
        .confirm({
          title: 'Confirm',
          description: 'Test',
        })
        .then((res) => {
          promiseResult = res
        })
    })

    const TestComponent = () => <>{result.current.ConfirmDialog}</>
    const { rerender } = render(<TestComponent />)

    await waitFor(() => {
      expect(screen.getByText('Confirm')).toBeInTheDocument()
    })

    // Simulate closing via dialog's onOpenChange
    act(() => {
      if (result.current.ConfirmDialog?.props?.onOpenChange) {
        result.current.ConfirmDialog.props.onOpenChange(false)
      }
    })

    rerender(<TestComponent />)

    await waitFor(() => {
      expect(result.current.ConfirmDialog).toBeNull()
    })
  })

  it('passes all options correctly to dialog component', async () => {
    const { result } = renderHook(() => useConfirm())

    act(() => {
      result.current.confirm({
        title: 'Custom Title',
        description: 'Custom Description',
        confirmLabel: 'Custom Confirm',
        cancelLabel: 'Custom Cancel',
        variant: 'destructive',
      })
    })

    const TestComponent = () => <>{result.current.ConfirmDialog}</>
    render(<TestComponent />)

    await waitFor(() => {
      expect(screen.getByText('Custom Title')).toBeInTheDocument()
      expect(screen.getByText('Custom Description')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /custom confirm/i })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: /custom cancel/i })).toBeInTheDocument()
    })
  })
})
