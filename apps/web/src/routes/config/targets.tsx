import { SourceBadge } from '@/components/config/SourceBadge'
import { TargetForm } from '@/components/config/TargetForm'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  useCreateTargetConfig,
  useDeleteTargetConfig,
  useTargetsConfig,
  useUpdateTargetConfig,
} from '@/hooks/useConfig'
import { useConfirm } from '@/hooks/useConfirm'
import type { ConfigSource, ConnectionConfig, TargetConfig, TargetConfigRequest } from '@/types'
import { createFileRoute } from '@tanstack/react-router'
import { Calendar, Database, Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'

export const Route = createFileRoute('/config/targets')({
  component: TargetsPage,
})

function TargetsPage() {
  const [isFormOpen, setIsFormOpen] = useState(false)
  const [editingTarget, setEditingTarget] = useState<TargetConfig | undefined>(undefined)

  const { data, isLoading, error } = useTargetsConfig()
  const createMutation = useCreateTargetConfig()
  const updateMutation = useUpdateTargetConfig()
  const deleteMutation = useDeleteTargetConfig()
  const { confirm } = useConfirm()

  const targets = data?.targets ?? []
  const source: ConfigSource = (data?.source as ConfigSource) ?? 'yaml'

  const handleCreate = () => {
    setEditingTarget(undefined)
    setIsFormOpen(true)
  }

  const handleEdit = (target: TargetConfig) => {
    setEditingTarget(target)
    setIsFormOpen(true)
  }

  const handleDelete = async (target: TargetConfig) => {
    const confirmed = await confirm({
      title: 'Delete Target',
      description: `Are you sure you want to delete "${target.name}"? This action cannot be undone.`,
    })

    if (confirmed) {
      try {
        await deleteMutation.mutateAsync(target.name)
      } catch (err) {
        console.error('Failed to delete target:', err)
      }
    }
  }

  const handleSubmit = async (targetData: TargetConfigRequest) => {
    if (editingTarget) {
      await updateMutation.mutateAsync({
        name: editingTarget.name,
        target: targetData,
      })
    } else {
      await createMutation.mutateAsync(targetData)
    }
  }

  const getTypeBadgeColor = (type: string) => {
    switch (type) {
      case 'mysql':
        return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
      case 'postgres':
        return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400'
      case 'redis':
        return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
      default:
        return ''
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Target Management</h2>
          <p className="text-sm text-gray-500 mt-1">
            Configure backup targets with database connections and schedules
          </p>
        </div>
        <Button onClick={handleCreate}>
          <Plus className="mr-2 h-4 w-4" />
          Add Target
        </Button>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle>
            {targets.length} Target{targets.length !== 1 ? 's' : ''}
          </CardTitle>
          <SourceBadge source={source} />
        </CardHeader>
        <CardContent>
          {error ? (
            <div className="text-center py-12">
              <div className="text-destructive mb-4">
                Failed to load targets: {error instanceof Error ? error.message : String(error)}
              </div>
            </div>
          ) : isLoading ? (
            <div className="text-center py-6 text-muted-foreground">Loading targets...</div>
          ) : targets.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              <Database className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>No backup targets configured</p>
              <Button onClick={handleCreate} variant="outline" className="mt-4">
                <Plus className="mr-2 h-4 w-4" />
                Add Your First Target
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Connection</TableHead>
                  <TableHead>Storage</TableHead>
                  <TableHead>Schedule</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {targets.map((target) => {
                  const conn = target.connection as ConnectionConfig
                  return (
                    <TableRow key={target.name}>
                      <TableCell className="font-medium">{target.name}</TableCell>
                      <TableCell>
                        <Badge className={getTypeBadgeColor(conn.type)}>
                          {conn.type?.toUpperCase()}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="text-sm text-gray-500">
                          {conn.type === 'redis' ? (
                            <span>
                              {conn.host}:{conn.port}
                            </span>
                          ) : (
                            <span>
                              {conn.user}@{conn.host}:{conn.port}/{conn.database}
                            </span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <span className="text-sm">
                          {target.storage_name || <span className="text-gray-500">Default</span>}
                        </span>
                      </TableCell>
                      <TableCell>
                        {target.schedule ? (
                          <div className="flex items-center gap-2 text-sm">
                            <Calendar className="h-3 w-3" />
                            <code className="bg-gray-100 dark:bg-gray-800 px-1 rounded text-xs">
                              {target.schedule}
                            </code>
                          </div>
                        ) : (
                          <span className="text-sm text-gray-500">Manual only</span>
                        )}
                      </TableCell>
                      <TableCell>
                        {target.enabled ? (
                          <Badge className="bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400">
                            Enabled
                          </Badge>
                        ) : (
                          <Badge className="bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400">
                            Disabled
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex items-center justify-end gap-2">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleEdit(target)}
                            disabled={source === 'yaml'}
                            title={source === 'yaml' ? 'Cannot edit YAML-sourced configs' : 'Edit'}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleDelete(target)}
                            disabled={source === 'yaml' || deleteMutation.isPending}
                            title={
                              source === 'yaml' ? 'Cannot delete YAML-sourced configs' : 'Delete'
                            }
                          >
                            <Trash2 className="h-4 w-4 text-red-500" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <TargetForm
        open={isFormOpen}
        onOpenChange={setIsFormOpen}
        target={editingTarget}
        onSubmit={handleSubmit}
      />
    </div>
  )
}
