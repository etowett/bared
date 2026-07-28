import { DialogContent, DialogFooter, DialogHeader } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
import { ChevronRight } from 'lucide-react'
import { useId, type ReactNode } from 'react'

/**
 * The dialog shell every config form uses.
 *
 * The old shell put `max-h-[90vh] overflow-y-auto` on the content itself, so a
 * long form scrolled its own Save button off the bottom of the viewport and
 * the title with it. Here the content is a flex column that never scrolls; the
 * fields do, between a pinned header and a pinned footer.
 */
export function FormDialogContent({
  className,
  children,
}: {
  className?: string
  children: ReactNode
}) {
  return (
    <DialogContent
      className={cn('flex max-h-[90vh] flex-col gap-0 overflow-hidden p-0', className)}
    >
      {children}
    </DialogContent>
  )
}

/** The pinned title block. Pass `DialogTitle`/`DialogDescription` as children. */
export function FormDialogHeader({ children }: { children: ReactNode }) {
  return <DialogHeader className="shrink-0 space-y-1 border-b px-6 py-4">{children}</DialogHeader>
}

/** The scrolling middle. Everything between the header and the buttons. */
export function FormDialogBody({
  className,
  children,
}: {
  className?: string
  children: ReactNode
}) {
  return (
    <div className={cn('min-h-0 flex-1 space-y-6 overflow-y-auto px-6 py-5', className)}>
      {children}
    </div>
  )
}

/** The pinned button row. Always reachable, however long the form gets. */
export function FormDialogFooter({ children }: { children: ReactNode }) {
  return (
    <DialogFooter className="shrink-0 gap-2 border-t bg-background px-6 py-4">
      {children}
    </DialogFooter>
  )
}

interface FormSectionProps {
  title: string
  description?: string
  children: ReactNode
}

/**
 * A titled group of fields.
 *
 * `aria-labelledby` on the section is what makes the grouping real rather than
 * a visual rhythm: a screen reader announces which part of the form the cursor
 * is in. The heading is an `<h3>` — pages own the single `<h2>`.
 */
export function FormSection({ title, description, children }: FormSectionProps) {
  const headingId = useId()

  return (
    <section aria-labelledby={headingId} className="space-y-4">
      <div className="space-y-1">
        <h3 id={headingId} className="text-section-title font-semibold tracking-tight">
          {title}
        </h3>
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
      </div>
      {children}
    </section>
  )
}

interface DisclosureSectionProps {
  title: string
  description?: string
  open: boolean
  onOpenChange: (_open: boolean) => void
  children: ReactNode
}

/**
 * A section that starts folded away.
 *
 * Uses the real `hidden` attribute rather than a class, so collapsed fields
 * leave the accessibility tree and the tab order entirely — a "hidden" section
 * you can still tab into is worse than no disclosure at all.
 */
export function DisclosureSection({
  title,
  description,
  open,
  onOpenChange,
  children,
}: DisclosureSectionProps) {
  const contentId = useId()

  return (
    <section className="space-y-4 border-t pt-5">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={contentId}
        onClick={() => onOpenChange(!open)}
        className="flex w-full items-center gap-2 rounded-sm text-left focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring"
      >
        <ChevronRight
          aria-hidden="true"
          className={cn('h-4 w-4 shrink-0 transition-transform', open && 'rotate-90')}
        />
        <span className="text-section-title font-semibold tracking-tight">{title}</span>
      </button>
      {description && !open && <p className="pl-6 text-sm text-muted-foreground">{description}</p>}
      <div id={contentId} hidden={!open} className="space-y-4">
        {children}
      </div>
    </section>
  )
}

/**
 * The message a field's `aria-describedby` points at when it is invalid.
 *
 * `role="alert"` so the reason is announced when it appears, rather than only
 * when focus happens to land back on the field.
 */
export function FieldError({ id, children }: { id: string; children: ReactNode }) {
  return (
    <p id={id} role="alert" className="text-xs font-medium text-danger">
      {children}
    </p>
  )
}

/** A field's static help text. Also referenced by `aria-describedby`. */
export function FieldHint({ id, children }: { id: string; children: ReactNode }) {
  return (
    <p id={id} className="text-xs text-muted-foreground">
      {children}
    </p>
  )
}

/** The banner for a failure the server reported, above the fields. */
export function FormError({ children }: { children: ReactNode }) {
  return (
    <div
      role="alert"
      className="rounded-md border border-danger/25 bg-danger-subtle p-3 text-sm text-danger"
    >
      {children}
    </div>
  )
}
