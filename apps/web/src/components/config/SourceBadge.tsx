import type { ConfigSource } from '../../types'

interface SourceBadgeProps {
  source: ConfigSource
  className?: string
}

export function SourceBadge({ source, className = '' }: SourceBadgeProps) {
  const isDatabase = source === 'database'

  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-1 text-xs font-medium ${
        isDatabase
          ? 'border border-success/25 bg-success-subtle text-success'
          : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
      } ${className}`}
    >
      {isDatabase ? 'DB' : 'YAML'}
    </span>
  )
}
