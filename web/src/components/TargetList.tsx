import { useTriggerBackup } from '../hooks/useJobs'
import type { Target } from '../types'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatusBadge } from '@/components/ui/status-badge'
import { formatDate } from '@/lib/utils'

interface TargetListProps {
  targets: Target[]
}

export function TargetList({ targets }: TargetListProps) {
  const triggerBackup = useTriggerBackup()

  const handleBackup = async (targetName: string) => {
    if (confirm(`Start backup for ${targetName}?`)) {
      try {
        await triggerBackup.mutateAsync(targetName)
      } catch (error) {
        alert(`Failed to trigger backup: ${error}`)
      }
    }
  }

  if (targets.length === 0) {
    return <div className="text-center py-12 text-muted-foreground">No targets configured</div>
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
      {targets.map((target) => (
        <Card key={target.name} className="border-border/50 hover:border-border transition-colors">
          <CardHeader>
            <div className="flex justify-between items-center">
              <CardTitle className="text-xl">{target.name}</CardTitle>
              <StatusBadge status={target.is_running ? 'running' : 'idle'} />
            </div>
          </CardHeader>

          <CardContent className="space-y-3">
            <div className="space-y-2">
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground font-medium">Type:</span>
                <span className="text-foreground">{target.type}</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground font-medium">Database:</span>
                <span className="text-foreground font-mono">{target.database}</span>
              </div>
              <div className="flex justify-between text-sm">
                <span className="text-muted-foreground font-medium">Last Backup:</span>
                <span className="text-foreground font-mono">
                  {target.last_backup ? formatDate(target.last_backup) : 'Never'}
                </span>
              </div>
              {target.schedule && (
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground font-medium">Schedule:</span>
                  <span className="text-foreground font-mono">{target.schedule}</span>
                </div>
              )}
              {target.next_scheduled && (
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground font-medium">Next Run:</span>
                  <span className="text-foreground font-mono">
                    {formatDate(target.next_scheduled)}
                  </span>
                </div>
              )}
            </div>

            <Button
              onClick={() => handleBackup(target.name)}
              disabled={target.is_running || triggerBackup.isPending}
              className="w-full"
            >
              {target.is_running ? 'Backup Running...' : 'Backup Now'}
            </Button>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}
