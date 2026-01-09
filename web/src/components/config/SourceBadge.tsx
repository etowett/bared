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
          ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
          : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400'
      } ${className}`}
    >
      {isDatabase ? 'DB' : 'YAML'}
    </span>
  )
}
