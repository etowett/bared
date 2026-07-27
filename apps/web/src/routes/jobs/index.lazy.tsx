import { JobList } from '@/components/JobList'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Pagination } from '@/components/ui/pagination'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useJobs } from '@/hooks/useJobs'
import { createLazyFileRoute, useNavigate } from '@tanstack/react-router'
import { useMemo } from 'react'
import type { JobsSearch } from './index'

export const Route = createLazyFileRoute('/jobs/')({
  component: UnifiedJobsPage,
})

export function UnifiedJobsPage() {
  const navigate = useNavigate()
  const search = Route.useSearch() as JobsSearch

  // Build filters for API
  const filters = useMemo(() => {
    const f: Record<string, string | number> = {
      page: search.page || 1,
      limit: search.limit || 20,
    }
    if (search.status) f.status = search.status
    if (search.type) f.type = search.type
    if (search.target) f.target = search.target
    return f
  }, [search.page, search.limit, search.status, search.type, search.target])

  const { data, isLoading, isError, error } = useJobs(filters)

  const handlePageChange = (newPage: number) => {
    navigate({
      to: '.',
      search: { ...search, page: newPage },
    })
  }

  const handleFilterChange = (key: string, value: string) => {
    // Reset to page 1 when filters change
    navigate({
      to: '.',
      search: { ...search, [key]: value === '' ? undefined : value, page: 1 },
    })
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-semibold">All Jobs</h2>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle>
            {data?.total !== undefined ? (
              <>
                {data.total} Job{data.total !== 1 ? 's' : ''}
              </>
            ) : (
              'Jobs'
            )}
          </CardTitle>

          {/* Filters */}
          <div className="flex gap-3">
            <Input
              placeholder="Filter by target..."
              value={search.target || ''}
              onChange={(e) => handleFilterChange('target', e.target.value)}
              className="w-[200px]"
            />

            <Select
              value={search.type || 'all'}
              onValueChange={(v) => handleFilterChange('type', v)}
            >
              <SelectTrigger className="w-[150px]">
                <SelectValue placeholder="All Types" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Types</SelectItem>
                <SelectItem value="backup">Backup</SelectItem>
                <SelectItem value="restore">Restore</SelectItem>
              </SelectContent>
            </Select>

            <Select
              value={search.status || 'all'}
              onValueChange={(v) => handleFilterChange('status', v)}
            >
              <SelectTrigger className="w-[180px]">
                <SelectValue placeholder="All Status" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Status</SelectItem>
                <SelectItem value="queued">Queued</SelectItem>
                <SelectItem value="running">Running</SelectItem>
                <SelectItem value="completed">Completed</SelectItem>
                <SelectItem value="failed">Failed</SelectItem>
                <SelectItem value="cancelled">Cancelled</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardHeader>

        <CardContent className="space-y-4">
          {/* Loading State */}
          {isLoading ? (
            <div className="text-center py-6 text-muted-foreground">Loading jobs...</div>
          ) : isError ? (
            /* Error State */
            <div className="text-center py-12 text-destructive">
              Error loading jobs: {error instanceof Error ? error.message : 'Unknown error'}
            </div>
          ) : data?.jobs.length === 0 ? (
            /* Empty State */
            <div className="text-center py-12 text-muted-foreground">
              No jobs found.{' '}
              {(search.status || search.type || search.target) && 'Try adjusting your filters.'}
            </div>
          ) : (
            /* Job List with Navigation Mode */
            <>
              <JobList jobs={data?.jobs || []} navigationMode={true} />

              {/* Pagination */}
              {data?.pagination && data.pagination.total_pages > 1 && (
                <div className="flex justify-center pt-4">
                  <Pagination
                    currentPage={data.pagination.page}
                    totalPages={data.pagination.total_pages}
                    hasNext={data.pagination.has_next}
                    hasPrev={data.pagination.has_prev}
                    onPageChange={handlePageChange}
                  />
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
