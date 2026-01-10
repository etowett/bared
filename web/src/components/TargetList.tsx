import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatusBadge } from '@/components/ui/status-badge'
import { formatDate } from '@/lib/utils'
import { cronToHuman, formatNextRun } from '@/utils/cron'
import { toast } from 'sonner'
import { useConfirm } from '../hooks/useConfirm'
import { useTriggerBackup } from '../hooks/useJobs'
import type { Target } from '../types'

interface TargetListProps {
  targets: Target[]
}

export function TargetList({ targets }: TargetListProps) {
  const triggerBackup = useTriggerBackup()
  const { confirm, ConfirmDialog } = useConfirm()

  const handleBackup = async (targetName: string) => {
    const confirmed = await confirm({
      title: 'Start Backup',
      description: `Are you sure you want to start a backup for ${targetName}?`,
      confirmLabel: 'Start Backup',
      variant: 'default',
    })

    if (confirmed) {
      try {
        await triggerBackup.mutateAsync(targetName)
        toast.success('Backup started successfully')
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Failed to trigger backup'
        toast.error('Failed to trigger backup', {
          description: errorMessage,
        })
      }
    }
  }

  if (targets.length === 0) {
    return <div className="text-center py-12 text-muted-foreground">No targets configured</div>
  }

  return (
    <>
      {ConfirmDialog}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {targets.map((target) => (
          <Card
            key={target.name}
            className="border-border/50 hover:border-border transition-colors"
          >
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
                  <div className="flex flex-col gap-1 text-sm">
                    <span className="text-muted-foreground font-medium">Schedule:</span>
                    <span className="text-foreground">{cronToHuman(target.schedule)}</span>
                    <span className="text-muted-foreground text-xs font-mono">
                      {target.schedule}
                    </span>
                  </div>
                )}
                {target.next_scheduled && (
                  <div className="flex flex-col gap-1 text-sm">
                    <span className="text-muted-foreground font-medium">Next Run:</span>
                    <span className="text-foreground text-sm">
                      {formatNextRun(target.next_scheduled)}
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
    </>
  )
}
