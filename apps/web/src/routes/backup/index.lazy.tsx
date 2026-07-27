import { JobDetail } from '@/components/JobDetail'
import { JobHistoryTable } from '@/components/JobHistoryTable'
import { TargetList } from '@/components/TargetList'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { PageHeader } from '@/components/ui/page-header'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useTargets } from '@/hooks/useTargets'
import type { JobSearch } from '@/lib/job-search'
import type { Job } from '@/types'
import { createLazyFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { History } from 'lucide-react'
import { useMemo, useState } from 'react'

export const Route = createLazyFileRoute('/backup/')({
  component: BackupPage,
})

export function BackupPage() {
  const navigate = useNavigate()
  const search = Route.useSearch() as JobSearch
  const { data: dashboard } = useTargets()
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)
  const [targetSearch, setTargetSearch] = useState<string>('')
  const [typeFilter, setTypeFilter] = useState<string>('all')

  const targets = useMemo(() => dashboard?.targets || [], [dashboard?.targets])

  // Derive unique db types for filter dropdown
  const dbTypes = useMemo(() => {
    const types = new Set(targets.map((t) => t.type))
    return Array.from(types).sort()
  }, [targets])

  // The target list is bounded by the number of configured targets — a handful
  // — and `/api/targets` returns all of them in one unpaged response, so
  // filtering it in the browser costs nothing. The job history below does not
  // get that luxury and filters server-side.
  const filteredTargets = useMemo(() => {
    let result = targets
    if (targetSearch) {
      result = result.filter((t) => t.name.toLowerCase().includes(targetSearch.toLowerCase()))
    }
    if (typeFilter !== 'all') {
      result = result.filter((t) => t.type === typeFilter)
    }
    return result
  }, [targets, targetSearch, typeFilter])

  return (
    <div className="space-y-6">
      <PageHeader
        title="Backup"
        description="Run a backup now, or review what the schedule has already produced."
        actions={
          <Button asChild variant="outline">
            <Link to="/backup/jobs">
              <History aria-hidden="true" className="mr-2 h-4 w-4" />
              Job history
            </Link>
          </Button>
        }
      />

      <Card>
        <CardHeader className="flex flex-col gap-4 space-y-0 pb-4 sm:flex-row sm:items-center sm:justify-between">
          <CardTitle>Backup Targets ({filteredTargets.length})</CardTitle>
          <div className="flex flex-wrap gap-3">
            <Input
              placeholder="Search targets..."
              aria-label="Search targets"
              value={targetSearch}
              onChange={(e) => setTargetSearch(e.target.value)}
              className="w-full sm:w-[200px]"
            />
            <Select value={typeFilter} onValueChange={setTypeFilter}>
              <SelectTrigger className="w-full sm:w-[150px]" aria-label="Filter targets by type">
                <SelectValue placeholder="All Types" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All Types</SelectItem>
                {dbTypes.map((type) => (
                  <SelectItem key={type} value={type}>
                    {type}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardHeader>
        <CardContent>
          <TargetList targets={filteredTargets} />
        </CardContent>
      </Card>

      <JobHistoryTable
        title="Backup Job History"
        type="backup"
        search={search}
        onSearchChange={(next) => navigate({ to: '.', search: { ...search, ...next } })}
        targetOptions={targets.map((target) => target.name)}
        onSelectJob={setSelectedJob}
        selectedJobId={selectedJob?.id}
        emptyMessage="No backup jobs found."
      />

      {selectedJob && <JobDetail job={selectedJob} onClose={() => setSelectedJob(null)} />}
    </div>
  )
}
