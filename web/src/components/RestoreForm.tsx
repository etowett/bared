import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { AlertTriangle, Info } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { useTriggerRestore } from '../hooks/useJobs'
import { useRestoreTargets } from '../hooks/useRestoreTargets'
import type { RestoreTarget } from '../types'

interface RestoreFormProps {
  onSuccess?: () => void
}

const STORAGE_KEY = 'bared_backup_paths'
const MAX_HISTORY = 20

function loadBackupPathHistory(): string[] {
  if (typeof window === 'undefined' || !window.localStorage) {
    return []
  }
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY)
    return stored ? JSON.parse(stored) : []
  } catch {
    return []
  }
}

function saveBackupPath(path: string) {
  if (typeof window === 'undefined' || !window.localStorage) {
    return
  }
  try {
    const history = loadBackupPathHistory()
    const filtered = history.filter((p) => p !== path)
    const updated = [path, ...filtered].slice(0, MAX_HISTORY)
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(updated))
  } catch {
    // Ignore localStorage errors
  }
}

export function RestoreForm({ onSuccess }: RestoreFormProps) {
  const [selectedTarget, setSelectedTarget] = useState<string>('')
  const [backupPath, setBackupPath] = useState<string>('')
  const [dryRun, setDryRun] = useState<boolean>(true)
  const [showConfirm, setShowConfirm] = useState<boolean>(false)
  const [showSuggestions, setShowSuggestions] = useState<boolean>(false)
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [selectedIndex, setSelectedIndex] = useState<number>(-1)
  const inputRef = useRef<HTMLInputElement>(null)
  const suggestionsRef = useRef<HTMLDivElement>(null)

  const { data: restoreTargets, isLoading } = useRestoreTargets()
  const triggerRestore = useTriggerRestore()

  const selectedTargetInfo = restoreTargets?.restore_targets.find(
    (t: RestoreTarget) => t.name === selectedTarget
  )

  // Load and filter suggestions based on input
  useEffect(() => {
    if (!backupPath.trim()) {
      setSuggestions([])
      setShowSuggestions(false)
      return
    }

    const history = loadBackupPathHistory()
    const filtered = history.filter((path) => path.toLowerCase().includes(backupPath.toLowerCase()))
    // Don't show suggestions if the input exactly matches a suggestion (user selected it)
    const exactMatch = filtered.some((path) => path === backupPath)
    setSuggestions(filtered)
    setShowSuggestions(filtered.length > 0 && !exactMatch)
    setSelectedIndex(-1)
  }, [backupPath])

  // Handle keyboard navigation
  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (!showSuggestions || suggestions.length === 0) return

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        setSelectedIndex((prev) => (prev < suggestions.length - 1 ? prev + 1 : prev))
        break
      case 'ArrowUp':
        e.preventDefault()
        setSelectedIndex((prev) => (prev > 0 ? prev - 1 : -1))
        break
      case 'Enter':
        if (selectedIndex >= 0) {
          e.preventDefault()
          setBackupPath(suggestions[selectedIndex])
          setShowSuggestions(false)
          setSelectedIndex(-1)
        }
        break
      case 'Escape':
        setShowSuggestions(false)
        setSelectedIndex(-1)
        break
    }
  }

  const handleSuggestionClick = (suggestion: string) => {
    setBackupPath(suggestion)
    setShowSuggestions(false)
    setSelectedIndex(-1)
    inputRef.current?.focus()
  }

  // Close suggestions when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        suggestionsRef.current &&
        !suggestionsRef.current.contains(event.target as Node) &&
        inputRef.current &&
        !inputRef.current.contains(event.target as Node)
      ) {
        setShowSuggestions(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    if (!selectedTarget || !backupPath) {
      toast.error('Validation Error', {
        description: 'Please select a restore target and enter a backup path',
      })
      return
    }

    if (!dryRun) {
      setShowConfirm(true)
      return
    }

    executeRestore()
  }

  const executeRestore = async () => {
    try {
      await triggerRestore.mutateAsync({
        target: selectedTarget,
        backup_path: backupPath,
        dry_run: dryRun,
      })

      toast.success(
        dryRun ? 'Restore validation job queued successfully!' : 'Restore job queued successfully!'
      )

      saveBackupPath(backupPath)

      setBackupPath('')
      setDryRun(true)
      setShowConfirm(false)
      setShowSuggestions(false)

      if (onSuccess) onSuccess()
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to trigger restore'
      toast.error('Failed to trigger restore', {
        description: errorMessage,
      })
    }
  }

  const handleConfirm = () => {
    setShowConfirm(false)
    executeRestore()
  }

  if (isLoading) {
    return <div className="text-center py-8 text-muted-foreground">Loading restore targets...</div>
  }

  return (
    <>
      <form onSubmit={handleSubmit} className="space-y-6">
        <div className="space-y-2">
          <Label htmlFor="restore-target">Restore Target</Label>
          <Select value={selectedTarget} onValueChange={setSelectedTarget}>
            <SelectTrigger id="restore-target">
              <SelectValue placeholder="-- Select Restore Target --" />
            </SelectTrigger>
            <SelectContent>
              {restoreTargets?.restore_targets.map((target: RestoreTarget) => (
                <SelectItem key={target.name} value={target.name}>
                  {target.name} ({target.type} - {target.database}@{target.host})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {selectedTargetInfo?.description && (
          <Alert>
            <Info className="h-4 w-4" />
            <AlertDescription>
              <strong>Description:</strong> {selectedTargetInfo.description}
            </AlertDescription>
          </Alert>
        )}

        {selectedTargetInfo?.source_target && (
          <Alert>
            <Info className="h-4 w-4" />
            <AlertDescription>
              <strong>Source Target:</strong> {selectedTargetInfo.source_target}
            </AlertDescription>
          </Alert>
        )}

        <div className="space-y-2">
          <Label htmlFor="backup-path">Backup Path</Label>
          <div className="relative">
            <Input
              ref={inputRef}
              id="backup-path"
              type="text"
              value={backupPath}
              onChange={(e) => setBackupPath(e.target.value)}
              onKeyDown={handleKeyDown}
              onFocus={() => {
                if (suggestions.length > 0) {
                  setShowSuggestions(true)
                }
              }}
              placeholder="e.g., et-backups/athena_local_db/athena-postgres-2025-12-03T06-28-21Z.tar.gz or 'latest'"
              required
              autoComplete="off"
            />
            {showSuggestions && suggestions.length > 0 && (
              <div
                ref={suggestionsRef}
                className="absolute top-full left-0 right-0 mt-1 z-50 border border-input bg-popover rounded-md shadow-md max-h-[200px] overflow-y-auto"
              >
                {suggestions.map((suggestion, index) => (
                  <div
                    key={suggestion}
                    onClick={() => handleSuggestionClick(suggestion)}
                    onMouseEnter={() => setSelectedIndex(index)}
                    className={`px-3 py-2 cursor-pointer transition-colors border-b last:border-b-0 ${
                      index === selectedIndex ? 'bg-accent' : 'hover:bg-accent/50'
                    }`}
                  >
                    {suggestion}
                  </div>
                ))}
              </div>
            )}
          </div>
          <p className="text-sm text-muted-foreground">
            Enter the backup file path or 'latest' to restore the most recent backup
          </p>
        </div>

        <div className="flex items-center space-x-2">
          <Checkbox
            id="dry-run"
            checked={dryRun}
            onCheckedChange={(checked) => setDryRun(checked === true)}
          />
          <div className="space-y-1">
            <Label htmlFor="dry-run" className="font-normal cursor-pointer">
              Dry-run (validate only, do not restore)
            </Label>
            <p className="text-sm text-muted-foreground">
              Recommended: Run validation first before actual restore
            </p>
          </div>
        </div>

        <div className="flex justify-end">
          <Button
            type="submit"
            disabled={triggerRestore.isPending || !selectedTarget || !backupPath}
            variant={dryRun ? 'secondary' : 'destructive'}
          >
            {triggerRestore.isPending
              ? 'Submitting...'
              : dryRun
                ? 'Validate Restore'
                : 'Execute Restore'}
          </Button>
        </div>
      </form>

      <Dialog open={showConfirm} onOpenChange={setShowConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-destructive">
              <AlertTriangle className="h-5 w-5" />
              Confirm Restore
            </DialogTitle>
            <DialogDescription asChild>
              <div className="space-y-2 pt-4">
                <div>
                  You are about to restore database <strong>{selectedTargetInfo?.database}</strong>{' '}
                  on <strong>{selectedTargetInfo?.host}</strong>.
                </div>
                <div className="font-semibold">This will overwrite the existing database!</div>
                <div className="text-sm">Backup path: {backupPath}</div>
              </div>
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setShowConfirm(false)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleConfirm}>
              Yes, Restore Database
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
