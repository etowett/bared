import { createFileRoute, Link } from '@tanstack/react-router'
import { RestoreForm } from '@/components/RestoreForm'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { ArrowRight } from 'lucide-react'

export const Route = createFileRoute('/restore')({
  component: RestorePage,
})

function RestorePage() {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-semibold">Restore Database</h2>
        <Link to="/restore/jobs">
          <Button variant="outline">
            View Job History
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        </Link>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Restore Database</CardTitle>
        </CardHeader>
        <CardContent>
          <RestoreForm />
        </CardContent>
      </Card>
    </div>
  )
}
