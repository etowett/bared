import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Link } from '@tanstack/react-router'
import { ExternalLink, X } from 'lucide-react'
import type { Job } from '../types'
import { JobDetailContent } from './JobDetailContent'

interface JobDetailProps {
  job: Job
  onClose: () => void
}

export function JobDetail({ job, onClose }: JobDetailProps) {
  return (
    <Dialog open={true} onOpenChange={onClose}>
      <DialogContent className="max-w-4xl max-h-[90vh] flex flex-col">
        <DialogHeader>
          <div className="flex items-center justify-between gap-2">
            <DialogTitle>Job Details</DialogTitle>
            <div className="flex items-center gap-2">
              <Button asChild variant="ghost" size="sm">
                <Link to="/jobs/$id" params={{ id: job.id }}>
                  <ExternalLink className="h-4 w-4 mr-1" />
                  Full Page
                </Link>
              </Button>
              <Button variant="ghost" size="icon" onClick={onClose}>
                <X className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </DialogHeader>

        <JobDetailContent job={job} compact={true} />
      </DialogContent>
    </Dialog>
  )
}
