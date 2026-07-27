import * as React from 'react'
import { ArrowDown, ArrowUp, ChevronsUpDown } from 'lucide-react'

import { cn } from '@/lib/utils'

/**
 * How much air the rows get.
 *
 * `comfortable` is the default; `compact` is for operators scanning a long
 * history on a big screen. The choice is set on the `<table>` as a data
 * attribute and read by the cells, so a caller sets it in one place.
 */
export type TableDensity = 'comfortable' | 'compact'

/**
 * The breakpoint below which a column is hidden.
 *
 * Eight columns cannot be read on a phone, and horizontal scrolling hides the
 * fact that there is anything to the right at all. Give every column a
 * priority instead: the identifying ones stay at 320px, the rest reappear as
 * the viewport earns them. A hidden column must never be the only place a
 * value is shown — `JobList` folds the hidden ones into the row's first cell.
 */
export type ColumnPriority = 'sm' | 'md' | 'lg' | 'xl'

const priorityClass: Record<ColumnPriority, string> = {
  sm: 'hidden sm:table-cell',
  md: 'hidden md:table-cell',
  lg: 'hidden lg:table-cell',
  xl: 'hidden xl:table-cell',
}

/** Tailwind cannot see through a data attribute, so the variant is spelled out. */
const compactCell = '[[data-density=compact]_&]:px-2 [[data-density=compact]_&]:py-1.5'

interface TableProps extends React.HTMLAttributes<HTMLTableElement> {
  density?: TableDensity
}

const Table = React.forwardRef<HTMLTableElement, TableProps>(
  ({ className, density = 'comfortable', ...props }, ref) => (
    <div className="relative w-full overflow-auto">
      <table
        ref={ref}
        data-density={density}
        className={cn('w-full caption-bottom text-sm', className)}
        {...props}
      />
    </div>
  )
)
Table.displayName = 'Table'

const TableHeader = React.forwardRef<
  HTMLTableSectionElement,
  React.HTMLAttributes<HTMLTableSectionElement>
>(({ className, ...props }, ref) => (
  <thead ref={ref} className={cn('[&_tr]:border-b', className)} {...props} />
))
TableHeader.displayName = 'TableHeader'

const TableBody = React.forwardRef<
  HTMLTableSectionElement,
  React.HTMLAttributes<HTMLTableSectionElement>
>(({ className, ...props }, ref) => (
  <tbody ref={ref} className={cn('[&_tr:last-child]:border-0', className)} {...props} />
))
TableBody.displayName = 'TableBody'

const TableFooter = React.forwardRef<
  HTMLTableSectionElement,
  React.HTMLAttributes<HTMLTableSectionElement>
>(({ className, ...props }, ref) => (
  <tfoot
    ref={ref}
    className={cn('border-t bg-muted/50 font-medium last:[&>tr]:border-b-0', className)}
    {...props}
  />
))
TableFooter.displayName = 'TableFooter'

const TableRow = React.forwardRef<HTMLTableRowElement, React.HTMLAttributes<HTMLTableRowElement>>(
  ({ className, ...props }, ref) => (
    <tr
      ref={ref}
      className={cn(
        'border-b transition-colors hover:bg-muted/50 data-[state=selected]:bg-muted',
        className
      )}
      {...props}
    />
  )
)
TableRow.displayName = 'TableRow'

interface TableHeadProps extends React.ThHTMLAttributes<HTMLTableCellElement> {
  /** Hide this header below the given breakpoint. Pair with the matching cell. */
  priority?: ColumnPriority
}

const TableHead = React.forwardRef<HTMLTableCellElement, TableHeadProps>(
  ({ className, priority, ...props }, ref) => (
    <th
      ref={ref}
      className={cn(
        'h-12 px-4 text-left align-middle font-medium text-muted-foreground has-[[role=checkbox]]:pr-0',
        '[[data-density=compact]_&]:h-9',
        compactCell,
        priority && priorityClass[priority],
        className
      )}
      {...props}
    />
  )
)
TableHead.displayName = 'TableHead'

interface TableCellProps extends React.TdHTMLAttributes<HTMLTableCellElement> {
  /** Hide this cell below the given breakpoint. Pair with the matching header. */
  priority?: ColumnPriority
}

const TableCell = React.forwardRef<HTMLTableCellElement, TableCellProps>(
  ({ className, priority, ...props }, ref) => (
    <td
      ref={ref}
      className={cn(
        'p-4 align-middle has-[[role=checkbox]]:pr-0',
        compactCell,
        priority && priorityClass[priority],
        className
      )}
      {...props}
    />
  )
)
TableCell.displayName = 'TableCell'

const TableCaption = React.forwardRef<
  HTMLTableCaptionElement,
  React.HTMLAttributes<HTMLTableCaptionElement>
>(({ className, ...props }, ref) => (
  <caption ref={ref} className={cn('mt-4 text-sm text-muted-foreground', className)} {...props} />
))
TableCaption.displayName = 'TableCaption'

export type SortDirection = 'asc' | 'desc'

interface SortableTableHeadProps extends Omit<TableHeadProps, 'onClick' | 'children'> {
  children: React.ReactNode
  /** The direction this column is currently sorted, or false when it is not. */
  sorted: SortDirection | false
  /** Which direction a fresh click on this column should ask for. */
  defaultDirection?: SortDirection
  onSort: (_direction: SortDirection) => void
}

/**
 * A column header that also sorts.
 *
 * `aria-sort` lives on the `<th>` — that is the attribute screen readers
 * announce — while the click target is a real `<button>` so the column is
 * reachable by keyboard without inventing key handlers. The glyph is not
 * decoration: with `aria-sort` alone, a sighted user has no indication which
 * column is active, and colour is not used to say it.
 */
const SortableTableHead = React.forwardRef<HTMLTableCellElement, SortableTableHeadProps>(
  ({ children, sorted, defaultDirection = 'desc', onSort, className, ...props }, ref) => {
    const next: SortDirection =
      sorted === 'asc' ? 'desc' : sorted === 'desc' ? 'asc' : defaultDirection
    const Icon = sorted === 'asc' ? ArrowUp : sorted === 'desc' ? ArrowDown : ChevronsUpDown

    return (
      <TableHead
        ref={ref}
        aria-sort={sorted === 'asc' ? 'ascending' : sorted === 'desc' ? 'descending' : 'none'}
        className={cn('p-0', className)}
        {...props}
      >
        <button
          type="button"
          onClick={() => onSort(next)}
          className={cn(
            'flex h-12 w-full items-center gap-1.5 px-4 text-left font-medium',
            '[[data-density=compact]_&]:h-9 [[data-density=compact]_&]:px-2',
            'transition-colors hover:text-foreground',
            'focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset',
            sorted && 'text-foreground'
          )}
        >
          {children}
          <Icon
            aria-hidden="true"
            className={cn('h-3.5 w-3.5 shrink-0', !sorted && 'opacity-40')}
          />
        </button>
      </TableHead>
    )
  }
)
SortableTableHead.displayName = 'SortableTableHead'

interface TableDensityToggleProps {
  value: TableDensity
  onChange: (_density: TableDensity) => void
  className?: string
}

/**
 * A two-option segmented control for row height.
 *
 * It is a radio group rather than a toggle button: "compact" and "comfortable"
 * are two named choices, and a lone pressed/unpressed button leaves a screen
 * reader user guessing what the unpressed state means.
 */
function TableDensityToggle({ value, onChange, className }: TableDensityToggleProps) {
  const options: { density: TableDensity; label: string }[] = [
    { density: 'comfortable', label: 'Comfortable' },
    { density: 'compact', label: 'Compact' },
  ]

  return (
    <div
      role="radiogroup"
      aria-label="Row density"
      className={cn('inline-flex items-center rounded-md border p-0.5', className)}
    >
      {options.map((option) => (
        <button
          key={option.density}
          type="button"
          role="radio"
          aria-checked={value === option.density}
          onClick={() => onChange(option.density)}
          className={cn(
            'rounded-sm px-2 py-1 text-xs font-medium transition-colors',
            'focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring',
            value === option.density
              ? 'bg-accent text-accent-foreground'
              : 'text-muted-foreground hover:text-foreground'
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  )
}

export {
  SortableTableHead,
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableDensityToggle,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
}
