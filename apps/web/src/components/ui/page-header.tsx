import { cn } from '@/lib/utils'
import { Link, type LinkProps } from '@tanstack/react-router'
import { ChevronRight } from 'lucide-react'
import type { ReactNode } from 'react'

export interface Breadcrumb {
  label: string
  /** Omit on the final crumb — the page you are already on is not a link. */
  to?: LinkProps['to']
  params?: LinkProps['params']
  /** Render in the mono face. Use for machine values: ids, paths, hashes. */
  mono?: boolean
}

export interface PageHeaderProps {
  title: string
  description?: string
  breadcrumbs?: Breadcrumb[]
  /** A `StatusBadge` describing the whole page's subject. */
  status?: ReactNode
  /** Buttons and links. Stacks under the title below `sm`. */
  actions?: ReactNode
  className?: string
}

/**
 * The single page header for the dashboard.
 *
 * The `<h2>` is the page's identity — `routes/routes.test.tsx` asserts exactly
 * one level-2 heading per page, which is what makes "the parent route rendered
 * instead of the child" detectable. Do not add a second one to a page.
 */
export function PageHeader({
  title,
  description,
  breadcrumbs,
  status,
  actions,
  className,
}: PageHeaderProps) {
  return (
    <div className={cn('space-y-3 border-b pb-5', className)}>
      {breadcrumbs && breadcrumbs.length > 0 && (
        <nav aria-label="Breadcrumb">
          <ol className="flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
            {breadcrumbs.map((crumb, index) => (
              <li key={`${crumb.label}-${index}`} className="flex items-center gap-1">
                {index > 0 && (
                  <ChevronRight aria-hidden="true" className="h-3 w-3 shrink-0 opacity-60" />
                )}
                {crumb.to ? (
                  <Link
                    to={crumb.to}
                    params={crumb.params}
                    className={cn(
                      'rounded-xs underline-offset-4 hover:text-foreground hover:underline focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring',
                      crumb.mono && 'font-mono'
                    )}
                  >
                    {crumb.label}
                  </Link>
                ) : (
                  <span
                    aria-current="page"
                    className={cn('text-foreground', crumb.mono && 'font-mono')}
                  >
                    {crumb.label}
                  </span>
                )}
              </li>
            ))}
          </ol>
        </nav>
      )}

      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 space-y-1.5">
          <div className="flex flex-wrap items-center gap-3">
            <h2 className="text-page-title font-semibold tracking-tight">{title}</h2>
            {status}
          </div>
          {description && <p className="max-w-2xl text-sm text-muted-foreground">{description}</p>}
        </div>

        {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
      </div>
    </div>
  )
}
