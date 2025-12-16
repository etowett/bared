import { createFileRoute, Link } from '@tanstack/react-router'
import { useTargets } from '@/hooks/useTargets'
import { TargetList } from '@/components/TargetList'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { ArrowRight } from 'lucide-react'

export const Route = createFileRoute('/backup')({
  component: BackupPage,
})

function BackupPage() {
  const { data: dashboard } = useTargets()

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-semibold">Backup Targets</h2>
        <Link to="/backup/jobs">
          <Button variant="outline">
            View Job History
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        </Link>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Backup Targets</CardTitle>
        </CardHeader>
        <CardContent>
          <TargetList targets={dashboard?.targets || []} />
        </CardContent>
      </Card>
    </div>
  )
}
