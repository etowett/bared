import { JobDetailContent } from '@/components/JobDetailContent'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { useJob } from '@/hooks/useJobs'
import { createLazyFileRoute, useNavigate } from '@tanstack/react-router'
import { ArrowLeft } from 'lucide-react'

export const Route = createLazyFileRoute('/jobs/$id')({
  component: JobDetailPage,
})

export function JobDetailPage() {
  const { id } = Route.useParams()
  const navigate = useNavigate()
  const { data: job, isLoading, error } = useJob(id)

  if (isLoading) {
    return <div className="text-center py-12 text-muted-foreground">Loading job details...</div>
  }

  if (error || !job) {
    return (
      <div className="space-y-6">
        <div className="flex items-center gap-4">
          <Button variant="outline" onClick={() => navigate({ to: '/' })}>
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back
          </Button>
        </div>
        <Card>
          <CardContent className="py-12">
            <div className="text-center text-muted-foreground">
              Job not found. The job may have been deleted or the ID is incorrect.
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  const getBackLink = () => {
    if (job.type === 'backup') {
      return '/backup/jobs'
    } else if (job.type === 'restore') {
      return '/restore/jobs'
    }
    return '/'
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="outline" onClick={() => navigate({ to: getBackLink() })}>
          <ArrowLeft className="mr-2 h-4 w-4" />
          Back to Job History
        </Button>
        <h2 className="text-2xl font-semibold">Job Details: {job.id.slice(0, 8)}...</h2>
      </div>

      <Card>
        <CardContent className="pt-6">
          <JobDetailContent job={job} compact={false} />
        </CardContent>
      </Card>
    </div>
  )
}
