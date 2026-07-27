import { useState } from 'react'
import { createLazyFileRoute } from '@tanstack/react-router'
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
import { Badge } from '@/components/ui/badge'
import { RestoreTargetForm } from '@/components/config/RestoreTargetForm'
import { SourceBadge } from '@/components/config/SourceBadge'
import {
  useRestoreTargetsConfig,
  useCreateRestoreTargetConfig,
  useUpdateRestoreTargetConfig,
  useDeleteRestoreTargetConfig,
} from '@/hooks/useConfig'
import { useConfirm } from '@/hooks/useConfirm'
import { Plus, Pencil, Trash2, ArrowDownToLine } from 'lucide-react'
import type {
  RestoreTargetConfig,
  RestoreTargetConfigRequest,
  ConfigSource,
  ConnectionConfig,
} from '@/types'

export const Route = createLazyFileRoute('/config/restore-targets')({
  component: RestoreTargetsPage,
})

export function RestoreTargetsPage() {
  const [isFormOpen, setIsFormOpen] = useState(false)
  const [editingTarget, setEditingTarget] = useState<RestoreTargetConfig | undefined>(undefined)

  const { data, isLoading, error } = useRestoreTargetsConfig()
  const createMutation = useCreateRestoreTargetConfig()
  const updateMutation = useUpdateRestoreTargetConfig()
  const deleteMutation = useDeleteRestoreTargetConfig()
  const { confirm } = useConfirm()

  const targets = data?.restore_targets ?? []
  const source: ConfigSource = (data?.source as ConfigSource) ?? 'yaml'

  const handleCreate = () => {
    setEditingTarget(undefined)
    setIsFormOpen(true)
  }

  const handleEdit = (target: RestoreTargetConfig) => {
    setEditingTarget(target)
    setIsFormOpen(true)
  }

  const handleDelete = async (target: RestoreTargetConfig) => {
    const confirmed = await confirm({
      title: 'Delete Restore Target',
      description: `Are you sure you want to delete "${target.name}"? This action cannot be undone.`,
    })

    if (confirmed) {
      try {
        await deleteMutation.mutateAsync(target.name)
      } catch (err) {
        console.error('Failed to delete restore target:', err)
      }
    }
  }

  const handleSubmit = async (targetData: RestoreTargetConfigRequest) => {
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
          <h2 className="text-2xl font-semibold">Restore Target Management</h2>
          <p className="text-sm text-gray-500 mt-1">
            Configure restore destinations for testing and recovery
          </p>
        </div>
        <Button onClick={handleCreate}>
          <Plus className="mr-2 h-4 w-4" />
          Add Restore Target
        </Button>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle>
            {targets.length} Restore Target{targets.length !== 1 ? 's' : ''}
          </CardTitle>
          <SourceBadge source={source} />
        </CardHeader>
        <CardContent>
          {error ? (
            <div className="text-center py-12">
              <div className="text-destructive mb-4">
                Failed to load restore targets:{' '}
                {error instanceof Error ? error.message : String(error)}
              </div>
            </div>
          ) : isLoading ? (
            <div className="text-center py-6 text-muted-foreground">Loading restore targets...</div>
          ) : targets.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              <ArrowDownToLine className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>No restore targets configured</p>
              <Button onClick={handleCreate} variant="outline" className="mt-4">
                <Plus className="mr-2 h-4 w-4" />
                Add Your First Restore Target
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Connection</TableHead>
                  <TableHead>Source Target</TableHead>
                  <TableHead>Storage</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {targets.map((target) => {
                  const conn = target.connection as ConnectionConfig
                  return (
                    <TableRow key={target.name}>
                      <TableCell className="font-medium">
                        <div>
                          <div>{target.name}</div>
                          {target.description && (
                            <div className="text-xs text-gray-500 mt-1">{target.description}</div>
                          )}
                        </div>
                      </TableCell>
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
                        {target.source_target ? (
                          <Badge variant="outline" className="text-xs">
                            {target.source_target}
                          </Badge>
                        ) : (
                          <span className="text-sm text-gray-500">Any</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <span className="text-sm">
                          {target.storage_name || <span className="text-gray-500">Default</span>}
                        </span>
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

      <RestoreTargetForm
        open={isFormOpen}
        onOpenChange={setIsFormOpen}
        target={editingTarget}
        onSubmit={handleSubmit}
      />
    </div>
  )
}
