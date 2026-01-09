import { useState } from 'react'
import { createFileRoute } from '@tanstack/react-router'
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
import { StorageForm } from '@/components/config/StorageForm'
import { SourceBadge } from '@/components/config/SourceBadge'
import {
  useStorages,
  useCreateStorage,
  useUpdateStorage,
  useDeleteStorage,
} from '@/hooks/useConfig'
import { useConfirm } from '@/hooks/useConfirm'
import { Plus, Pencil, Trash2, HardDrive, Cloud, Server } from 'lucide-react'
import type { Storage, StorageRequest } from '@/types'

export const Route = createFileRoute('/config/storages')({
  component: StoragesPage,
})

function StoragesPage() {
  const [isFormOpen, setIsFormOpen] = useState(false)
  const [editingStorage, setEditingStorage] = useState<Storage | undefined>(undefined)

  const { data, isLoading, error } = useStorages()
  const createMutation = useCreateStorage()
  const updateMutation = useUpdateStorage()
  const deleteMutation = useDeleteStorage()
  const { confirm } = useConfirm()

  const storages = data?.storages ?? []
  const source = data?.source ?? 'yaml'

  const handleCreate = () => {
    setEditingStorage(undefined)
    setIsFormOpen(true)
  }

  const handleEdit = (storage: Storage) => {
    setEditingStorage(storage)
    setIsFormOpen(true)
  }

  const handleDelete = async (storage: Storage) => {
    const confirmed = await confirm({
      title: 'Delete Storage',
      description: `Are you sure you want to delete "${storage.name}"? This action cannot be undone.`,
    })

    if (confirmed) {
      try {
        await deleteMutation.mutateAsync(storage.name)
      } catch (err) {
        console.error('Failed to delete storage:', err)
      }
    }
  }

  const handleSubmit = async (storageData: StorageRequest) => {
    if (editingStorage) {
      await updateMutation.mutateAsync({
        name: editingStorage.name,
        storage: storageData,
      })
    } else {
      await createMutation.mutateAsync(storageData)
    }
  }

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'local':
        return <HardDrive className="h-4 w-4" />
      case 's3':
        return <Cloud className="h-4 w-4" />
      case 'sftp':
        return <Server className="h-4 w-4" />
      default:
        return null
    }
  }

  const getTypeBadgeColor = (type: string) => {
    switch (type) {
      case 'local':
        return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
      case 's3':
        return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
      case 'sftp':
        return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400'
      default:
        return ''
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Storage Management</h2>
          <p className="text-sm text-gray-500 mt-1">
            Configure storage backends for backup destinations
          </p>
        </div>
        <Button onClick={handleCreate}>
          <Plus className="mr-2 h-4 w-4" />
          Add Storage
        </Button>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle>
            {storages.length} Storage Backend{storages.length !== 1 ? 's' : ''}
          </CardTitle>
          <SourceBadge source={source as any} />
        </CardHeader>
        <CardContent>
          {error ? (
            <div className="text-center py-12">
              <div className="text-destructive mb-4">
                Failed to load storages: {error instanceof Error ? error.message : String(error)}
              </div>
            </div>
          ) : isLoading ? (
            <div className="text-center py-6 text-muted-foreground">Loading storages...</div>
          ) : storages.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              <HardDrive className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>No storage backends configured</p>
              <Button onClick={handleCreate} variant="outline" className="mt-4">
                <Plus className="mr-2 h-4 w-4" />
                Add Your First Storage
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Configuration</TableHead>
                  <TableHead>Retention</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {storages.map((storage) => (
                  <TableRow key={storage.name}>
                    <TableCell className="font-medium">{storage.name}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        {getTypeIcon(storage.type)}
                        <Badge className={getTypeBadgeColor(storage.type)}>
                          {storage.type.toUpperCase()}
                        </Badge>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="text-sm text-gray-500">
                        {storage.type === 'local' && (
                          <span>Path: {storage.config.path}</span>
                        )}
                        {storage.type === 's3' && (
                          <span>
                            Bucket: {storage.config.bucket} ({storage.config.region})
                          </span>
                        )}
                        {storage.type === 'sftp' && (
                          <span>
                            {storage.config.host}:{storage.config.port}
                          </span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <span className="text-sm">{storage.keep} days</span>
                    </TableCell>
                    <TableCell>
                      {storage.enabled ? (
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
                          onClick={() => handleEdit(storage)}
                          disabled={source === 'yaml'}
                          title={source === 'yaml' ? 'Cannot edit YAML-sourced configs' : 'Edit'}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleDelete(storage)}
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
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <StorageForm
        open={isFormOpen}
        onOpenChange={setIsFormOpen}
        storage={editingStorage}
        onSubmit={handleSubmit}
      />
    </div>
  )
}
