/**
 * Formats a past instant as an age — "2 hours ago".
 *
 * The Overview is scanned for staleness, not read for timestamps: "3 days ago"
 * answers "is this backup current?" in one glance, where "12/09/2025, 2:00:00
 * AM" makes the reader do the subtraction. `formatNextRun` in `utils/cron.ts`
 * is the mirror of this for instants in the future.
 *
 * Returns `null` when there is nothing to format, so the caller decides what an
 * absent value means. "Never backed up" and "this daemon does not report it"
 * are different claims and must not collapse into one string here.
 *
 * @param timestamp ISO 8601 instant, or absent
 * @param now Injectable for tests; defaults to the current time
 */
export function formatAge(timestamp?: string | null, now: Date = new Date()): string | null {
  if (!timestamp) return null

  const then = new Date(timestamp)
  if (isNaN(then.getTime())) return null

  const seconds = Math.floor((now.getTime() - then.getTime()) / 1000)
  // A daemon clock a little ahead of the browser's is ordinary; a negative age
  // is not worth surfacing as "in -3 minutes".
  if (seconds < 60) return 'just now'

  // Floor at every step, matching `formatNextRun`: a backup that finished 119
  // minutes ago is "1 hour ago", never the flattering "2 hours ago".
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return pluralize(minutes, 'minute')

  const hours = Math.floor(minutes / 60)
  if (hours < 24) return pluralize(hours, 'hour')

  return pluralize(Math.floor(hours / 24), 'day')
}

function pluralize(count: number, unit: string): string {
  return `${count} ${unit}${count === 1 ? '' : 's'} ago`
}
