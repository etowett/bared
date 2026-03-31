import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { useImportConfig } from '@/hooks/useConfig'
import { useConfirm } from '@/hooks/useConfirm'
import type { ConfigImportResponse, ResourceImportSummary } from '@/types'
import { createFileRoute, Link } from '@tanstack/react-router'
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle,
  AlertTriangle,
  Loader2,
  Eye,
  Upload,
  XCircle,
} from 'lucide-react'
import { useState } from 'react'

export const Route = createFileRoute('/config/import')({
  component: ConfigImportPage,
})

function ConfigImportPage() {
  const [yamlContent, setYamlContent] = useState('')
  const [conflictMode, setConflictMode] = useState<'override' | 'skip'>('skip')
  const [result, setResult] = useState<ConfigImportResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  const importMutation = useImportConfig()
  const { confirm, ConfirmDialog } = useConfirm()

  const handleValidate = async () => {
    setError(null)
    setResult(null)
    try {
      const res = await importMutation.mutateAsync({
        yaml_content: yamlContent,
        conflict_mode: conflictMode,
        dry_run: true,
      })
      setResult(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Validation failed')
    }
  }

  const handleImport = async () => {
    const confirmed = await confirm({
      title: 'Import Configuration',
      description: `This will import the YAML configuration with "${conflictMode}" conflict mode. Existing resources will be ${conflictMode === 'override' ? 'updated' : 'skipped'}. Continue?`,
    })

    if (!confirmed) return

    setError(null)
    setResult(null)
    try {
      const res = await importMutation.mutateAsync({
        yaml_content: yamlContent,
        conflict_mode: conflictMode,
        dry_run: false,
      })
      setResult(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Import failed')
    }
  }

  const hasContent = yamlContent.trim().length > 0

  return (
    <>
    {ConfirmDialog}
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button asChild variant="ghost" size="sm">
          <Link to="/config">
            <ArrowLeft className="mr-2 h-4 w-4" />
            Back
          </Link>
        </Button>
        <div>
          <h2 className="text-2xl font-semibold">Import Configuration</h2>
          <p className="text-sm text-muted-foreground mt-1">
            Paste YAML configuration to import storages, notifiers, targets, and restore targets
          </p>
        </div>
      </div>

      {error && (
        <Card className="border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-900/10">
          <CardContent className="pt-6">
            <div className="flex items-start gap-3">
              <XCircle className="h-5 w-5 text-red-600 dark:text-red-400 mt-0.5 shrink-0" />
              <div>
                <h3 className="font-medium text-red-900 dark:text-red-100">Error</h3>
                <p className="text-sm text-red-700 dark:text-red-300 mt-1 whitespace-pre-wrap">
                  {error}
                </p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>YAML Configuration</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="yaml-content">
              Paste your YAML configuration below
            </Label>
            <Textarea
              id="yaml-content"
              value={yamlContent}
              onChange={(e) => setYamlContent(e.target.value)}
              placeholder={`# Example configuration
storages:
  local-backup:
    type: local
    path: /backups
    keep: 5

targets:
  - name: my-database
    conn:
      type: mysql
      host: localhost
      port: 3306
      user: root
      password: secret
      database: myapp
    storage:
      enabled: true
      name: local-backup
    schedule: "0 2 * * *"`}
              className="font-mono text-sm min-h-[400px] resize-y"
              disabled={importMutation.isPending}
            />
            <p className="text-xs text-muted-foreground">
              Environment variables (<code className="text-xs">{'${VAR}'}</code>) will not be
              expanded. Replace them with actual values before importing.
            </p>
          </div>

          <div className="flex flex-col sm:flex-row gap-4 items-start sm:items-end">
            <div className="space-y-2 w-full sm:w-48">
              <Label htmlFor="conflict-mode">Conflict Mode</Label>
              <Select
                value={conflictMode}
                onValueChange={(v) => setConflictMode(v as 'override' | 'skip')}
                disabled={importMutation.isPending}
              >
                <SelectTrigger id="conflict-mode">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="skip">Skip existing</SelectItem>
                  <SelectItem value="override">Override existing</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                {conflictMode === 'skip'
                  ? 'Existing resources will be kept unchanged'
                  : 'Existing resources will be updated'}
              </p>
            </div>

            <div className="flex gap-2 ml-auto">
              <Button
                variant="outline"
                onClick={handleValidate}
                disabled={!hasContent || importMutation.isPending}
              >
                {importMutation.isPending ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <Eye className="mr-2 h-4 w-4" />
                )}
                Validate (Dry Run)
              </Button>
              <Button
                onClick={handleImport}
                disabled={!hasContent || importMutation.isPending}
              >
                {importMutation.isPending ? (
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                ) : (
                  <Upload className="mr-2 h-4 w-4" />
                )}
                Import
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {result && <ImportResults result={result} />}
    </div>
    </>
  )
}

function ImportResults({ result }: { result: ConfigImportResponse }) {
  const isDryRun = result.dry_run
  const totalProcessed = result.total_created + result.total_updated + result.total_skipped + result.total_failed

  const bannerType = result.has_errors
    ? totalProcessed === result.total_failed
      ? 'error'
      : 'warning'
    : 'success'

  const bannerConfig = {
    success: {
      border: 'border-green-200 dark:border-green-800',
      bg: 'bg-green-50 dark:bg-green-900/10',
      icon: <CheckCircle className="h-5 w-5 text-green-600 dark:text-green-400 mt-0.5 shrink-0" />,
      titleColor: 'text-green-900 dark:text-green-100',
      textColor: 'text-green-700 dark:text-green-300',
      title: isDryRun ? 'Dry Run Complete' : 'Import Successful',
    },
    warning: {
      border: 'border-amber-200 dark:border-amber-800',
      bg: 'bg-amber-50 dark:bg-amber-900/10',
      icon: <AlertTriangle className="h-5 w-5 text-amber-600 dark:text-amber-400 mt-0.5 shrink-0" />,
      titleColor: 'text-amber-900 dark:text-amber-100',
      textColor: 'text-amber-700 dark:text-amber-300',
      title: isDryRun ? 'Dry Run Complete (with errors)' : 'Import Completed with Errors',
    },
    error: {
      border: 'border-red-200 dark:border-red-800',
      bg: 'bg-red-50 dark:bg-red-900/10',
      icon: <XCircle className="h-5 w-5 text-red-600 dark:text-red-400 mt-0.5 shrink-0" />,
      titleColor: 'text-red-900 dark:text-red-100',
      textColor: 'text-red-700 dark:text-red-300',
      title: isDryRun ? 'Dry Run Failed' : 'Import Failed',
    },
  }[bannerType]

  const verb = isDryRun ? 'Would be' : ''

  return (
    <div className="space-y-4">
      <Card className={`${bannerConfig.border} ${bannerConfig.bg}`}>
        <CardContent className="pt-6">
          <div className="flex items-start gap-3">
            {bannerConfig.icon}
            <div>
              <h3 className={`font-medium ${bannerConfig.titleColor}`}>{bannerConfig.title}</h3>
              <div className={`text-sm ${bannerConfig.textColor} mt-2 flex flex-wrap gap-x-6 gap-y-1`}>
                {result.total_created > 0 && (
                  <span>{verb ? `${verb} created` : 'Created'}: {result.total_created}</span>
                )}
                {result.total_updated > 0 && (
                  <span>{verb ? `${verb} updated` : 'Updated'}: {result.total_updated}</span>
                )}
                {result.total_skipped > 0 && (
                  <span>{verb ? `${verb} skipped` : 'Skipped'}: {result.total_skipped}</span>
                )}
                {result.total_failed > 0 && (
                  <span>Failed: {result.total_failed}</span>
                )}
                {totalProcessed === 0 && <span>No resources found in the YAML configuration.</span>}
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <ResourceSectionCard
          title="Storages"
          summary={result.storages}
          isDryRun={isDryRun}
        />
        <ResourceSectionCard
          title="Notifiers"
          summary={result.notifiers}
          isDryRun={isDryRun}
        />
        <ResourceSectionCard
          title="Targets"
          summary={result.targets}
          isDryRun={isDryRun}
        />
        <ResourceSectionCard
          title="Restore Targets"
          summary={result.restore_targets}
          isDryRun={isDryRun}
        />
      </div>

      {(result.global_config.updated.length > 0 ||
        result.global_config.failed.length > 0) && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Global Config</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2 text-sm">
              {result.global_config.updated.map((key) => (
                <div key={key} className="flex items-center gap-2">
                  <CheckCircle className="h-3.5 w-3.5 text-green-600 dark:text-green-400 shrink-0" />
                  <span>
                    <code className="text-xs bg-muted px-1 py-0.5 rounded">{key}</code>{' '}
                    {isDryRun ? 'would be updated' : 'updated'}
                  </span>
                </div>
              ))}
              {result.global_config.failed.map((item) => (
                <div key={item.key} className="flex items-start gap-2">
                  <XCircle className="h-3.5 w-3.5 text-red-600 dark:text-red-400 mt-0.5 shrink-0" />
                  <span>
                    <code className="text-xs bg-muted px-1 py-0.5 rounded">{item.key}</code>{' '}
                    <span className="text-red-600 dark:text-red-400">{item.error}</span>
                  </span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function ResourceSectionCard({
  title,
  summary,
  isDryRun,
}: {
  title: string
  summary: ResourceImportSummary
  isDryRun: boolean
}) {
  const total = summary.created.length + summary.updated.length + summary.skipped.length + summary.failed.length
  if (total === 0) return null

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base flex items-center justify-between">
          <span>{title}</span>
          <span className="text-sm font-normal text-muted-foreground">{total} total</span>
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-2 text-sm">
          {summary.created.map((name) => (
            <div key={name} className="flex items-center gap-2">
              <CheckCircle className="h-3.5 w-3.5 text-green-600 dark:text-green-400 shrink-0" />
              <span className="truncate">{name}</span>
              <span className="text-muted-foreground text-xs ml-auto shrink-0">
                {isDryRun ? 'would create' : 'created'}
              </span>
            </div>
          ))}
          {summary.updated.map((name) => (
            <div key={name} className="flex items-center gap-2">
              <AlertCircle className="h-3.5 w-3.5 text-blue-600 dark:text-blue-400 shrink-0" />
              <span className="truncate">{name}</span>
              <span className="text-muted-foreground text-xs ml-auto shrink-0">
                {isDryRun ? 'would update' : 'updated'}
              </span>
            </div>
          ))}
          {summary.skipped.map((name) => (
            <div key={name} className="flex items-center gap-2">
              <AlertTriangle className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              <span className="truncate text-muted-foreground">{name}</span>
              <span className="text-muted-foreground text-xs ml-auto shrink-0">skipped</span>
            </div>
          ))}
          {summary.failed.map((item) => (
            <div key={item.name} className="flex items-start gap-2">
              <XCircle className="h-3.5 w-3.5 text-red-600 dark:text-red-400 mt-0.5 shrink-0" />
              <div className="min-w-0">
                <span className="truncate block">{item.name}</span>
                <span className="text-xs text-red-600 dark:text-red-400 block">{item.error}</span>
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
