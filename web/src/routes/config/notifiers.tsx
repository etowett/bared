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
import { NotifierForm } from '@/components/config/NotifierForm'
import { SourceBadge } from '@/components/config/SourceBadge'
import {
  useNotifiers,
  useCreateNotifier,
  useUpdateNotifier,
  useDeleteNotifier,
} from '@/hooks/useConfig'
import { useConfirm } from '@/hooks/useConfirm'
import { Plus, Pencil, Trash2, Bell, Mail, Webhook } from 'lucide-react'
import type { Notifier, NotifierRequest } from '@/types'

export const Route = createFileRoute('/config/notifiers')({
  component: NotifiersPage,
})

function NotifiersPage() {
  const [isFormOpen, setIsFormOpen] = useState(false)
  const [editingNotifier, setEditingNotifier] = useState<Notifier | undefined>(undefined)

  const { data, isLoading, error } = useNotifiers()
  const createMutation = useCreateNotifier()
  const updateMutation = useUpdateNotifier()
  const deleteMutation = useDeleteNotifier()
  const { confirm } = useConfirm()

  const notifiers = data?.notifiers ?? []
  const source = data?.source ?? 'yaml'

  const handleCreate = () => {
    setEditingNotifier(undefined)
    setIsFormOpen(true)
  }

  const handleEdit = (notifier: Notifier) => {
    setEditingNotifier(notifier)
    setIsFormOpen(true)
  }

  const handleDelete = async (notifier: Notifier) => {
    const confirmed = await confirm({
      title: 'Delete Notifier',
      description: `Are you sure you want to delete "${notifier.name}"? This action cannot be undone.`,
    })

    if (confirmed) {
      try {
        await deleteMutation.mutateAsync(notifier.name)
      } catch (err) {
        console.error('Failed to delete notifier:', err)
      }
    }
  }

  const handleSubmit = async (notifierData: NotifierRequest) => {
    if (editingNotifier) {
      await updateMutation.mutateAsync({
        name: editingNotifier.name,
        notifier: notifierData,
      })
    } else {
      await createMutation.mutateAsync(notifierData)
    }
  }

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'slack':
        return <Bell className="h-4 w-4" />
      case 'email':
        return <Mail className="h-4 w-4" />
      case 'webhook':
        return <Webhook className="h-4 w-4" />
      default:
        return null
    }
  }

  const getTypeBadgeColor = (type: string) => {
    switch (type) {
      case 'slack':
        return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400'
      case 'email':
        return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
      case 'webhook':
        return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
      default:
        return ''
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Notifier Management</h2>
          <p className="text-sm text-gray-500 mt-1">
            Configure notification channels for backup alerts
          </p>
        </div>
        <Button onClick={handleCreate}>
          <Plus className="mr-2 h-4 w-4" />
          Add Notifier
        </Button>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
          <CardTitle>
            {notifiers.length} Notifier{notifiers.length !== 1 ? 's' : ''}
          </CardTitle>
          <SourceBadge source={source as any} />
        </CardHeader>
        <CardContent>
          {error ? (
            <div className="text-center py-12">
              <div className="text-destructive mb-4">
                Failed to load notifiers: {error instanceof Error ? error.message : String(error)}
              </div>
            </div>
          ) : isLoading ? (
            <div className="text-center py-6 text-muted-foreground">Loading notifiers...</div>
          ) : notifiers.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              <Bell className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p>No notifiers configured</p>
              <Button onClick={handleCreate} variant="outline" className="mt-4">
                <Plus className="mr-2 h-4 w-4" />
                Add Your First Notifier
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Configuration</TableHead>
                  <TableHead>Notify On</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {notifiers.map((notifier) => (
                  <TableRow key={notifier.name}>
                    <TableCell className="font-medium">{notifier.name}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        {getTypeIcon(notifier.type)}
                        <Badge className={getTypeBadgeColor(notifier.type)}>
                          {notifier.type.toUpperCase()}
                        </Badge>
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="text-sm text-gray-500">
                        {notifier.type === 'slack' && (
                          <span>Channel: {notifier.config.channel || 'default'}</span>
                        )}
                        {notifier.type === 'email' && (
                          <span>To: {notifier.config.to_email}</span>
                        )}
                        {notifier.type === 'webhook' && (
                          <span>{notifier.config.method} {new URL(notifier.config.url).hostname}</span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      <div className="flex gap-2">
                        <Badge variant="outline" className="text-xs">
                          Failures
                        </Badge>
                        {notifier.on_success && (
                          <Badge variant="outline" className="text-xs">
                            Success
                          </Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell>
                      {notifier.enabled ? (
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
                          onClick={() => handleEdit(notifier)}
                          disabled={source === 'yaml'}
                          title={source === 'yaml' ? 'Cannot edit YAML-sourced configs' : 'Edit'}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleDelete(notifier)}
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

      <NotifierForm
        open={isFormOpen}
        onOpenChange={setIsFormOpen}
        notifier={editingNotifier}
        onSubmit={handleSubmit}
      />
    </div>
  )
}
