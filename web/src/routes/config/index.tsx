import { SourceBadge } from '@/components/config/SourceBadge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatCard } from '@/components/ui/stat-card'
import {
  useConfigSource,
  useMigrateConfig,
  useNotifiers,
  useReloadConfig,
  useRestoreTargetsConfig,
  useStorages,
  useTargetsConfig,
} from '@/hooks/useConfig'
import { useConfirm } from '@/hooks/useConfirm'
import type { ConfigSource, MigrateConfigResult, ReloadConfigResult } from '@/types'
import { createFileRoute, Link } from '@tanstack/react-router'
import {
  AlertCircle,
  ArrowDownToLine,
  ArrowRight,
  Bell,
  CheckCircle,
  Database,
  HardDrive,
  RefreshCw,
  Upload,
} from 'lucide-react'
import { useState } from 'react'

export const Route = createFileRoute('/config/')({
  component: ConfigDashboardPage,
})

function ConfigDashboardPage() {
  const { data: storagesData, isLoading: storagesLoading, error: storagesError } = useStorages()
  const { data: notifiersData, isLoading: notifiersLoading, error: notifiersError } = useNotifiers()
  const { data: targetsData, isLoading: targetsLoading, error: targetsError } = useTargetsConfig()
  const {
    data: restoreTargetsData,
    isLoading: restoreTargetsLoading,
    error: restoreTargetsError,
  } = useRestoreTargetsConfig()
  const { data: configSourceData, isLoading: sourceLoading, error: sourceError } = useConfigSource()

  const migrateMutation = useMigrateConfig()
  const reloadMutation = useReloadConfig()
  const { confirm } = useConfirm()

  const [migrateResult, setMigrateResult] = useState<MigrateConfigResult | null>(null)
  const [reloadResult, setReloadResult] = useState<ReloadConfigResult | null>(null)

  const source: ConfigSource = (configSourceData?.source as ConfigSource) || 'yaml'
  const storageCount = storagesData?.storages?.length ?? 0
  const notifierCount = notifiersData?.notifiers?.length ?? 0
  const targetCount = targetsData?.targets?.length ?? 0
  const restoreTargetCount = restoreTargetsData?.restore_targets?.length ?? 0

  const isLoading =
    storagesLoading || notifiersLoading || targetsLoading || restoreTargetsLoading || sourceLoading
  const hasError =
    storagesError || notifiersError || targetsError || restoreTargetsError || sourceError

  const handleMigrate = async () => {
    const confirmed = await confirm({
      title: 'Migrate Configuration',
      description:
        'This will import all YAML configuration into the database. Existing database configs will not be overwritten. Continue?',
    })

    if (confirmed) {
      try {
        const result = await migrateMutation.mutateAsync()
        setMigrateResult(result)
        setReloadResult(null)
      } catch (err) {
        console.error('Migration failed:', err)
      }
    }
  }

  const handleReload = async () => {
    const confirmed = await confirm({
      title: 'Reload Configuration',
      description:
        'This will reload the configuration from the database and reschedule all jobs. This may cause brief interruptions. Continue?',
    })

    if (confirmed) {
      try {
        const result = await reloadMutation.mutateAsync()
        setReloadResult(result as ReloadConfigResult)
        setMigrateResult(null)
      } catch (err) {
        console.error('Reload failed:', err)
      }
    }
  }

  if (isLoading) {
    return <div className="text-center py-12 text-muted-foreground">Loading configuration...</div>
  }

  if (hasError) {
    return (
      <div className="space-y-6">
        <h2 className="text-2xl font-semibold">Configuration Management</h2>
        <Card className="border-red-200 bg-red-50 dark:bg-red-900/10">
          <CardContent className="pt-6">
            <div className="flex items-start gap-3">
              <AlertCircle className="h-5 w-5 text-red-600 dark:text-red-400 mt-0.5" />
              <div>
                <h3 className="font-medium text-red-900 dark:text-red-100">
                  Configuration Not Available
                </h3>
                <p className="text-sm text-red-700 dark:text-red-300 mt-1">
                  Configuration management is not enabled. The daemon may need to be restarted with
                  database config support, or config management might not be available in your
                  deployment.
                </p>
                <p className="text-sm text-red-700 dark:text-red-300 mt-2">
                  Error details: {String(hasError)}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-semibold">Configuration Management</h2>
          <p className="text-sm text-gray-500 mt-1">
            Manage storage backends, notifiers, targets, and restore targets
          </p>
        </div>
        <div className="flex items-center gap-2">
          <SourceBadge source={source} />
        </div>
      </div>

      {migrateResult && (
        <Card className="border-green-200 bg-green-50 dark:bg-green-900/10">
          <CardContent className="pt-6">
            <div className="flex items-start gap-3">
              <CheckCircle className="h-5 w-5 text-green-600 dark:text-green-400 mt-0.5" />
              <div>
                <h3 className="font-medium text-green-900 dark:text-green-100">
                  Migration Successful
                </h3>
                <p className="text-sm text-green-700 dark:text-green-300 mt-1">
                  Imported {migrateResult.storages_count} storage(s),{' '}
                  {migrateResult.notifiers_count} notifier(s), and {migrateResult.targets_count}{' '}
                  target(s) from YAML to database.
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {reloadResult && (
        <Card className="border-blue-200 bg-blue-50 dark:bg-blue-900/10">
          <CardContent className="pt-6">
            <div className="flex items-start gap-3">
              <CheckCircle className="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5" />
              <div>
                <h3 className="font-medium text-blue-900 dark:text-blue-100">
                  Configuration Reloaded
                </h3>
                <p className="text-sm text-blue-700 dark:text-blue-300 mt-1">
                  Configuration has been reloaded from {reloadResult.source}. All scheduled jobs
                  have been updated.
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      {source === 'yaml' && (
        <Card className="border-amber-200 bg-amber-50 dark:bg-amber-900/10">
          <CardContent className="pt-6">
            <div className="flex items-start gap-3">
              <AlertCircle className="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5" />
              <div className="flex-1">
                <h3 className="font-medium text-amber-900 dark:text-amber-100">
                  Using YAML Configuration
                </h3>
                <p className="text-sm text-amber-700 dark:text-amber-300 mt-1">
                  Configuration is currently loaded from YAML file. To manage configs through the
                  UI, migrate to database storage.
                </p>
                <Button
                  onClick={handleMigrate}
                  disabled={migrateMutation.isPending}
                  variant="outline"
                  size="sm"
                  className="mt-3"
                >
                  <Upload className="mr-2 h-4 w-4" />
                  {migrateMutation.isPending ? 'Migrating...' : 'Migrate to Database'}
                </Button>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
        <StatCard title="Storages" value={storageCount} />
        <StatCard title="Notifiers" value={notifierCount} />
        <StatCard title="Targets" value={targetCount} />
        <StatCard title="Restore Targets" value={restoreTargetCount} />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <HardDrive className="h-5 w-5" />
              Storage Backends
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-gray-500 mb-4">
              Configure where backup files are stored (local, S3, SFTP)
            </p>
            <Button asChild variant="outline" className="w-full">
              <Link to="/config/storages">
                Manage Storages
                <ArrowRight className="ml-2 h-4 w-4" />
              </Link>
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Bell className="h-5 w-5" />
              Notifiers
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-gray-500 mb-4">
              Configure notification channels for backup alerts (Slack, Email, Webhooks)
            </p>
            <Button asChild variant="outline" className="w-full">
              <Link to="/config/notifiers">
                Manage Notifiers
                <ArrowRight className="ml-2 h-4 w-4" />
              </Link>
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Database className="h-5 w-5" />
              Backup Targets
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-gray-500 mb-4">
              Configure databases to backup with schedules and retention policies
            </p>
            <Button asChild variant="outline" className="w-full">
              <Link to="/config/targets">
                Manage Targets
                <ArrowRight className="ml-2 h-4 w-4" />
              </Link>
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <ArrowDownToLine className="h-5 w-5" />
              Restore Targets
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-gray-500 mb-4">
              Configure destinations for restoring backups to test environments
            </p>
            <Button asChild variant="outline" className="w-full">
              <Link to="/config/restore-targets">
                Manage Restore Targets
                <ArrowRight className="ml-2 h-4 w-4" />
              </Link>
            </Button>
          </CardContent>
        </Card>
      </div>

      {source === 'database' && (
        <Card>
          <CardHeader>
            <CardTitle>Configuration Actions</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex gap-4">
              <Button onClick={handleReload} disabled={reloadMutation.isPending} variant="outline">
                <RefreshCw
                  className={`mr-2 h-4 w-4 ${reloadMutation.isPending ? 'animate-spin' : ''}`}
                />
                {reloadMutation.isPending ? 'Reloading...' : 'Reload Configuration'}
              </Button>
              <p className="text-sm text-gray-500 flex items-center">
                Apply configuration changes without restarting the daemon
              </p>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
