import { JobDetailContent } from '@/components/JobDetailContent'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { PageHeader } from '@/components/ui/page-header'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge } from '@/components/ui/status-badge'
import { useJob } from '@/hooks/useJobs'
import { createLazyFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'

export const Route = createLazyFileRoute('/jobs/$id')({
  component: JobDetailPage,
})

export function JobDetailPage() {
  const { id } = Route.useParams()
  const { data: job, isPending, error } = useJob(id)
  const shortId = id.slice(0, 8)

  if (isPending) {
    return (
      <div className="space-y-6">
        <PageHeader
          breadcrumbs={[
            { label: 'Jobs', to: '/jobs' },
            { label: shortId, mono: true },
          ]}
          title={`Job ${shortId}`}
        />
        <Card>
          <CardContent className="space-y-4 pt-6">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              {Array.from({ length: 6 }).map((_, index) => (
                <div key={index} className="space-y-2">
                  <Skeleton className="h-3 w-20" />
                  <Skeleton className="h-4 w-40" />
                </div>
              ))}
            </div>
            <Skeleton className="h-64 w-full" />
          </CardContent>
        </Card>
      </div>
    )
  }

  if (error || !job) {
    return (
      <div className="space-y-6">
        <PageHeader
          breadcrumbs={[
            { label: 'Jobs', to: '/jobs' },
            { label: shortId, mono: true },
          ]}
          title="Job not found"
          description="The job may have been deleted, or the id in the address is wrong."
          actions={
            <Button asChild variant="outline">
              <Link to="/jobs">
                <ArrowLeft aria-hidden="true" className="mr-2 h-4 w-4" />
                All jobs
              </Link>
            </Button>
          }
        />
      </div>
    )
  }

  // Breadcrumbs, not history: a back button lands wherever the user came from,
  // which after a deep link is somewhere else entirely.
  const historyCrumb =
    job.type === 'restore'
      ? ({ label: 'Restore history', to: '/restore/jobs' } as const)
      : ({ label: 'Backup history', to: '/backup/jobs' } as const)

  return (
    <div className="space-y-6">
      <PageHeader
        breadcrumbs={[{ label: 'Jobs', to: '/jobs' }, historyCrumb, { label: shortId, mono: true }]}
        title={`Job ${shortId}`}
        description={`${job.type === 'restore' ? 'Restore' : 'Backup'} of ${job.target}`}
        status={<StatusBadge status={job.status} />}
        actions={
          <Button asChild variant="outline">
            <Link to={historyCrumb.to}>
              <ArrowLeft aria-hidden="true" className="mr-2 h-4 w-4" />
              Back to Job History
            </Link>
          </Button>
        }
      />

      <Card>
        <CardContent className="pt-6">
          <JobDetailContent job={job} compact={false} />
        </CardContent>
      </Card>
    </div>
  )
}
