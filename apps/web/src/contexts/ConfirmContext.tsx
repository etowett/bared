import { ConfirmDialog } from '@/components/ui/confirm-dialog'
import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from 'react'

export interface ConfirmOptions {
  title: string
  description: string
  confirmLabel?: string
  cancelLabel?: string
  variant?: 'default' | 'destructive'
}

export type ConfirmFn = (_options: ConfirmOptions) => Promise<boolean>

const ConfirmContext = createContext<ConfirmFn | undefined>(undefined)

/**
 * Owns the single confirmation dialog for the whole app.
 *
 * The previous shape of this API returned the dialog element alongside
 * `confirm()` and relied on every caller remembering to render it. Five of the
 * eight call sites did not, so `confirm()` handed back a promise that nothing
 * could ever settle and the destructive action became a silent no-op (#125).
 * Mounting the dialog here — once, in `__root.tsx` — removes the thing callers
 * could forget: `useConfirm()` now returns nothing but the function.
 */
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [options, setOptions] = useState<ConfirmOptions | null>(null)

  // The resolver lives in a ref rather than in state so that settling is
  // exactly-once: a single interaction fires several close paths (the Confirm
  // button calls `onConfirm` *and* `onOpenChange(false)`; Escape and the
  // overlay only call the latter). Whichever arrives first consumes the
  // resolver and the rest become no-ops.
  const resolverRef = useRef<((_value: boolean) => void) | null>(null)

  const settle = useCallback((answer: boolean) => {
    const resolve = resolverRef.current
    resolverRef.current = null
    setOptions(null)
    resolve?.(answer)
  }, [])

  const confirm = useCallback<ConfirmFn>((nextOptions) => {
    return new Promise<boolean>((resolve) => {
      // A second confirm() while one is still open would otherwise strand the
      // first caller's `await` forever. Answer it "no" and take over.
      resolverRef.current?.(false)
      resolverRef.current = resolve
      setOptions(nextOptions)
    })
  }, [])

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      {options && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open) {
              settle(false)
            }
          }}
          title={options.title}
          description={options.description}
          confirmLabel={options.confirmLabel}
          cancelLabel={options.cancelLabel}
          variant={options.variant}
          onConfirm={() => settle(true)}
          onCancel={() => settle(false)}
        />
      )}
    </ConfirmContext.Provider>
  )
}

/**
 * Returns `confirm(options)`, which resolves `true` when the user confirms and
 * `false` on every dismissal path (Cancel, Escape, overlay click).
 */
// eslint-disable-next-line react-refresh/only-export-components
export function useConfirm(): ConfirmFn {
  const confirm = useContext(ConfirmContext)
  if (confirm === undefined) {
    throw new Error('useConfirm must be used within a ConfirmProvider')
  }
  return confirm
}
