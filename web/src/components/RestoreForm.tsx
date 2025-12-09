import { useEffect, useRef, useState } from 'react'
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
    // Remove if already exists and add to front
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
    setSuggestions(filtered)
    setShowSuggestions(filtered.length > 0)
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
      alert('Please select a restore target and enter a backup path')
      return
    }

    // Show confirmation for non-dry-run restores
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

      alert(
        dryRun ? 'Restore validation job queued successfully!' : 'Restore job queued successfully!'
      )

      // Save successful backup path to history
      saveBackupPath(backupPath)

      // Only clear form fields on success
      setBackupPath('')
      setDryRun(true)
      setShowConfirm(false)
      setShowSuggestions(false)

      if (onSuccess) onSuccess()
    } catch (error) {
      // On error, preserve all form values so user doesn't have to retype
      alert(`Failed to trigger restore: ${error}`)
      // backupPath, selectedTarget, and dryRun are preserved automatically
    }
  }

  const handleConfirm = () => {
    setShowConfirm(false)
    executeRestore()
  }

  const handleCancel = () => {
    setShowConfirm(false)
  }

  if (isLoading) {
    return <div className="restore-form loading">Loading restore targets...</div>
  }

  return (
    <div className="restore-form">
      <h3>Restore Database</h3>

      <form onSubmit={handleSubmit}>
        <div className="form-group">
          <label htmlFor="restore-target">Restore Target:</label>
          <select
            id="restore-target"
            value={selectedTarget}
            onChange={(e) => setSelectedTarget(e.target.value)}
            className="form-select"
            required
          >
            <option value="">-- Select Restore Target --</option>
            {restoreTargets?.restore_targets.map((target: RestoreTarget) => (
              <option key={target.name} value={target.name}>
                {target.name} ({target.type} - {target.database}@{target.host})
              </option>
            ))}
          </select>
        </div>

        {selectedTargetInfo && selectedTargetInfo.description && (
          <div className="form-info">
            <strong>Description:</strong> {selectedTargetInfo.description}
          </div>
        )}

        {selectedTargetInfo && selectedTargetInfo.source_target && (
          <div className="form-info">
            <strong>Source Target:</strong> {selectedTargetInfo.source_target}
          </div>
        )}

        <div className="form-group" style={{ position: 'relative' }}>
          <label htmlFor="backup-path">Backup Path:</label>
          <input
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
            className="form-input"
            placeholder="e.g., et-backups/athena_local_db/athena-postgres-2025-12-03T06-28-21Z.tar.gz or 'latest'"
            required
            autoComplete="off"
          />
          {showSuggestions && suggestions.length > 0 && (
            <div ref={suggestionsRef} className="autocomplete-suggestions">
              {suggestions.map((suggestion, index) => (
                <div
                  key={suggestion}
                  onClick={() => handleSuggestionClick(suggestion)}
                  onMouseEnter={() => setSelectedIndex(index)}
                  className={index === selectedIndex ? 'selected' : ''}
                >
                  {suggestion}
                </div>
              ))}
            </div>
          )}
          <small className="form-help">
            Enter the backup file path or 'latest' to restore the most recent backup
          </small>
        </div>

        <div className="form-group checkbox-group">
          <label>
            <input type="checkbox" checked={dryRun} onChange={(e) => setDryRun(e.target.checked)} />
            <span>Dry-run (validate only, do not restore)</span>
          </label>
          <small className="form-help">
            Recommended: Run validation first before actual restore
          </small>
        </div>

        <div className="form-actions">
          <button
            type="submit"
            disabled={triggerRestore.isPending || !selectedTarget || !backupPath}
            className={dryRun ? 'btn-secondary' : 'btn-danger'}
          >
            {triggerRestore.isPending
              ? 'Submitting...'
              : dryRun
                ? 'Validate Restore'
                : 'Execute Restore'}
          </button>
        </div>
      </form>

      {showConfirm && (
        <div className="modal-overlay">
          <div className="modal-content confirm-dialog">
            <h3>⚠️ Confirm Restore</h3>
            <p>
              You are about to restore database <strong>{selectedTargetInfo?.database}</strong> on{' '}
              <strong>{selectedTargetInfo?.host}</strong>.
            </p>
            <p>
              <strong>This will overwrite the existing database!</strong>
            </p>
            <p>Backup path: {backupPath}</p>
            <div className="modal-actions">
              <button onClick={handleCancel} className="btn-secondary">
                Cancel
              </button>
              <button onClick={handleConfirm} className="btn-danger">
                Yes, Restore Database
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
