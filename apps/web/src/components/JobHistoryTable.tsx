import { JobList } from '@/components/JobList'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Pagination } from '@/components/ui/pagination'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { TableSkeleton } from '@/components/ui/skeleton'
import { TableDensityToggle, type SortDirection } from '@/components/ui/table'
import { useJobs } from '@/hooks/useJobs'
import { JOB_STATUSES, PAGE_SIZES, type JobSearch, type JobSortKey } from '@/lib/job-search'
import type { Job } from '@/types'
import { useEffect, useId, useMemo } from 'react'
import { toast } from 'sonner'

const ALL = '__all__'

interface JobHistoryTableProps {
  /** Card title, without the count — the count is appended here. */
  title: string
  /**
   * Pins the job type. When set the type filter is hidden and the URL's `type`
   * is ignored, because the page itself already means "backup" or "restore".
   */
  type?: 'backup' | 'restore'
  search: JobSearch
  onSearchChange: (_next: Partial<JobSearch>) => void
  /**
   * The target names the filter offers.
   *
   * `/api/jobs` matches `target` exactly, so a free-text box would quietly
   * return nothing for a partial name. A list of the names that exist is both
   * honest about the API and easier to use.
   */
  targetOptions: string[]
  /** Provide to open a job in a dialog; omit to link through to `/jobs/$id`. */
  onSelectJob?: (_job: Job) => void
  selectedJobId?: string
  emptyMessage?: string
}

/**
 * The one filtered, sorted, paginated job table.
 *
 * All of its state is in the URL, so a filtered view is a link somebody can
 * paste into an incident channel and it survives a reload. Filtering and
 * paging happen on the server; sorting cannot — see the note it renders.
 */
export function JobHistoryTable({
  title,
  type,
  search,
  onSearchChange,
  targetOptions,
  onSelectJob,
  selectedJobId,
  emptyMessage = 'No jobs found.',
}: JobHistoryTableProps) {
  const controlId = useId()
  const density = search.density ?? 'comfortable'
  const page = search.page ?? 1
  const limit = search.limit ?? 20

  const filters = useMemo(() => {
    const next: Record<string, string | number> = { page, limit }
    const effectiveType = type ?? search.type
    if (effectiveType) next.type = effectiveType
    if (search.status) next.status = search.status
    if (search.target) next.target = search.target
    return next
  }, [type, page, limit, search.type, search.status, search.target])

  const { data, isPending, isError, error } = useJobs(filters)

  // A failed background refresh must not blank the table that is already on
  // screen, and it must not stack up a toast per poll — a fixed id makes
  // sonner replace the previous one instead.
  const refreshFailed = isError && !!data
  useEffect(() => {
    if (!refreshFailed) return
    toast.error('Could not refresh jobs', {
      id: 'jobs-refresh',
      description: error instanceof Error ? error.message : 'The daemon did not answer.',
    })
  }, [refreshFailed, error])

  const jobs = data?.jobs ?? []
  const total = data?.total
  const filtered = !!(search.status || search.target || (!type && search.type))

  // A target that only exists in the URL still has to be selectable, or the
  // control would silently show "All targets" while filtering by something.
  const targets = useMemo(() => {
    const names = new Set(targetOptions)
    if (search.target) names.add(search.target)
    return Array.from(names).sort()
  }, [targetOptions, search.target])

  const handleSortChange = (sort: JobSortKey, order: SortDirection) =>
    onSearchChange({ sort, order })

  return (
    <Card>
      <CardHeader className="flex flex-col gap-4 space-y-0 pb-4 lg:flex-row lg:items-end lg:justify-between">
        <CardTitle>
          {total !== undefined ? `${title} (${total} job${total === 1 ? '' : 's'})` : title}
        </CardTitle>

        {/* Filters wrap rather than overflow — at 320px they stack. */}
        <div className="flex flex-wrap items-end gap-3">
          <div className="grid w-full gap-1.5 sm:w-[200px]">
            <Label htmlFor={`${controlId}-target`} className="text-xs text-muted-foreground">
              Target
            </Label>
            <Select
              value={search.target ?? ALL}
              onValueChange={(value) =>
                onSearchChange({ target: value === ALL ? undefined : value, page: 1 })
              }
            >
              <SelectTrigger id={`${controlId}-target`}>
                <SelectValue placeholder="All targets" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL}>All targets</SelectItem>
                {targets.map((name) => (
                  <SelectItem key={name} value={name}>
                    {name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {!type && (
            <div className="grid w-[calc(50%-0.375rem)] gap-1.5 sm:w-[150px]">
              <Label htmlFor={`${controlId}-type`} className="text-xs text-muted-foreground">
                Type
              </Label>
              <Select
                value={search.type ?? ALL}
                onValueChange={(value) =>
                  onSearchChange({ type: value === ALL ? undefined : value, page: 1 })
                }
              >
                <SelectTrigger id={`${controlId}-type`}>
                  <SelectValue placeholder="All types" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ALL}>All types</SelectItem>
                  <SelectItem value="backup">Backup</SelectItem>
                  <SelectItem value="restore">Restore</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}

          <div className="grid w-[calc(50%-0.375rem)] gap-1.5 sm:w-[180px]">
            <Label htmlFor={`${controlId}-status`} className="text-xs text-muted-foreground">
              Status
            </Label>
            <Select
              value={search.status ?? ALL}
              onValueChange={(value) =>
                onSearchChange({ status: value === ALL ? undefined : value, page: 1 })
              }
            >
              <SelectTrigger id={`${controlId}-status`}>
                <SelectValue placeholder="All statuses" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL}>All statuses</SelectItem>
                {JOB_STATUSES.map((status) => (
                  <SelectItem key={status} value={status}>
                    {status[0].toUpperCase() + status.slice(1)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      </CardHeader>

      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="text-xs text-muted-foreground">
            {/*
              The daemon returns newest first and offers no sort parameter, so
              saying "sorted by duration" without this caveat would be a lie
              about which rows you are looking at. Server-side sort is #141.
            */}
            Column sorting reorders this page only; the daemon always returns the newest jobs first.
          </p>

          <div className="flex flex-wrap items-center gap-3">
            <div className="flex items-center gap-2">
              <Label
                htmlFor={`${controlId}-limit`}
                className="text-xs font-normal text-muted-foreground"
              >
                Rows
              </Label>
              <Select
                value={String(limit)}
                onValueChange={(value) => onSearchChange({ limit: Number(value), page: 1 })}
              >
                <SelectTrigger id={`${controlId}-limit`} className="h-8 w-[4.5rem]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PAGE_SIZES.map((size) => (
                    <SelectItem key={size} value={String(size)}>
                      {size}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <TableDensityToggle
              value={density}
              onChange={(next) => onSearchChange({ density: next })}
            />
          </div>
        </div>

        {/* First load holds the table's shape; polls keep the rows on screen. */}
        {isPending ? (
          <TableSkeleton rows={6} columns={7} />
        ) : isError && !data ? (
          <div className="py-12 text-center text-destructive">
            Error loading jobs: {error instanceof Error ? error.message : 'Unknown error'}
          </div>
        ) : jobs.length === 0 ? (
          <div className="py-12 text-center text-muted-foreground">
            {emptyMessage} {filtered && 'Try adjusting your filters.'}
          </div>
        ) : (
          <>
            <JobList
              jobs={jobs}
              navigationMode={!onSelectJob}
              onSelectJob={onSelectJob}
              selectedJobId={selectedJobId}
              density={density}
              sort={search.sort}
              order={search.order}
              onSortChange={handleSortChange}
            />

            {data?.pagination && data.pagination.total_pages > 1 && (
              <div className="flex justify-center pt-4">
                <Pagination
                  currentPage={data.pagination.page}
                  totalPages={data.pagination.total_pages}
                  hasNext={data.pagination.has_next}
                  hasPrev={data.pagination.has_prev}
                  onPageChange={(page) => onSearchChange({ page })}
                />
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
