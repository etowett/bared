/**
 * Converts a cron expression to a human-readable description
 * @param cronExpr - Standard cron expression (minute hour day month weekday)
 * @returns Human-readable description
 */
export function cronToHuman(cronExpr: string): string {
  if (!cronExpr || cronExpr.trim() === '') {
    return 'Manual only'
  }

  try {
    const parts = cronExpr.trim().split(/\s+/)
    if (parts.length < 5) {
      return cronExpr // Return as-is if invalid format
    }

    const [minute, hour, day, month, weekday] = parts

    // Every minute
    if (minute === '*' && hour === '*' && day === '*' && month === '*' && weekday === '*') {
      return 'Every minute'
    }

    // Hourly patterns
    if (hour === '*' && day === '*' && month === '*' && weekday === '*') {
      if (minute === '0') return 'Hourly'
      if (minute === '*/15') return 'Every 15 minutes'
      if (minute === '*/30') return 'Every 30 minutes'
      return `Every hour at minute ${minute}`
    }

    // Daily patterns
    if (day === '*' && month === '*' && weekday === '*') {
      if (minute === '0' && hour !== '*') {
        if (hour.includes('*/')) {
          const interval = hour.split('*/')[1]
          return `Every ${interval} hours`
        }
        const hourNum = parseInt(hour)
        if (!isNaN(hourNum)) {
          const time = formatTime(hourNum, 0)
          return `Daily at ${time}`
        }
      }
      if (hour !== '*') {
        const hourNum = parseInt(hour)
        const minuteNum = parseInt(minute)
        if (!isNaN(hourNum) && !isNaN(minuteNum)) {
          const time = formatTime(hourNum, minuteNum)
          return `Daily at ${time}`
        }
      }
    }

    // Weekly patterns
    if (day === '*' && month === '*' && weekday !== '*') {
      const hourNum = parseInt(hour)
      const minuteNum = parseInt(minute)
      const dayName = weekdayToName(weekday)
      if (!isNaN(hourNum) && !isNaN(minuteNum) && dayName) {
        const time = formatTime(hourNum, minuteNum)
        return `Weekly on ${dayName} at ${time}`
      }
    }

    // Monthly patterns
    if (day !== '*' && month === '*' && weekday === '*') {
      const hourNum = parseInt(hour)
      const minuteNum = parseInt(minute)
      const dayNum = parseInt(day)
      if (!isNaN(hourNum) && !isNaN(minuteNum) && !isNaN(dayNum)) {
        const time = formatTime(hourNum, minuteNum)
        const ordinal = getOrdinal(dayNum)
        return `Monthly on the ${ordinal} at ${time}`
      }
    }

    // Fallback to showing the cron expression
    return cronExpr
  } catch {
    return cronExpr
  }
}

/**
 * Extracts the timezone the daemon interprets its cron expressions in.
 *
 * A cron expression carries no timezone of its own — `0 2 * * *` is bare
 * numbers, and the daemon resolves them against its own `time.Local` (UTC in
 * the Docker image, the host zone under `make run-daemon`). The browser has no
 * way to know which, so `cronToHuman` alone can only ever print an unqualified
 * "2:00 AM" that the viewer naturally misreads as their own time.
 *
 * `next_scheduled` is the one place that leaks it: the API formats it with Go's
 * RFC3339 from a `time.Local` instant, so it arrives carrying the daemon's own
 * offset — "Z" from the container, "+03:00" from a Nairobi host. Reading it back
 * off the wire is exact, so no extra API field is needed.
 *
 * @returns A label like "UTC", "UTC+3" or "UTC-5:30", or null when the offset
 * can't be read.
 */
export function serverZoneLabel(nextScheduled?: string | null): string | null {
  if (!nextScheduled) return null

  const match = /(Z|[+-]\d{2}:\d{2})$/.exec(nextScheduled.trim())
  if (!match) return null

  const offset = match[1]
  if (offset === 'Z' || offset === '+00:00' || offset === '-00:00') return 'UTC'

  const sign = offset[0]
  const hours = parseInt(offset.slice(1, 3), 10)
  const minutes = parseInt(offset.slice(4, 6), 10)
  if (isNaN(hours) || isNaN(minutes)) return null

  // Whole-hour zones read better without ":00"; the half- and quarter-hour ones
  // (+05:30, +05:45) need the minutes to stay meaningful.
  return minutes === 0 ? `UTC${sign}${hours}` : `UTC${sign}${hours}:${offset.slice(4, 6)}`
}

/**
 * Describes a cron expression, qualified with the daemon's timezone when the
 * description names a wall-clock time.
 *
 * @param cronExpr - Standard cron expression
 * @param nextScheduled - The target's `next_scheduled`, which supplies the zone
 * @returns e.g. "Daily at 2:00 AM UTC", or "Hourly" unqualified
 */
export function describeSchedule(cronExpr: string, nextScheduled?: string | null): string {
  const human = cronToHuman(cronExpr)
  const zone = serverZoneLabel(nextScheduled)
  if (!zone) return human

  // Only wall-clock descriptions are ambiguous. "Hourly", "Every 15 minutes" and
  // "Every 6 hours" name intervals, which land on the same instants in every
  // zone — a suffix there would be noise. formatTime is this module's only
  // producer of clock times, so its "h:mm AM/PM" tail identifies them exactly.
  if (!/\d:\d{2} (AM|PM)$/.test(human)) return human

  return `${human} ${zone}`
}

/**
 * Formats a time as HH:MM AM/PM
 */
function formatTime(hour: number, minute: number): string {
  const period = hour >= 12 ? 'PM' : 'AM'
  const displayHour = hour % 12 || 12
  const displayMinute = minute.toString().padStart(2, '0')
  return `${displayHour}:${displayMinute} ${period}`
}

/**
 * Converts a weekday number (0-7) to name
 */
function weekdayToName(weekday: string): string | null {
  const days: Record<string, string> = {
    '0': 'Sunday',
    '1': 'Monday',
    '2': 'Tuesday',
    '3': 'Wednesday',
    '4': 'Thursday',
    '5': 'Friday',
    '6': 'Saturday',
    '7': 'Sunday',
  }
  return days[weekday] || null
}

/**
 * Converts a number to its ordinal form (1st, 2nd, 3rd, etc.)
 */
function getOrdinal(num: number): string {
  const j = num % 10
  const k = num % 100
  if (j === 1 && k !== 11) return `${num}st`
  if (j === 2 && k !== 12) return `${num}nd`
  if (j === 3 && k !== 13) return `${num}rd`
  return `${num}th`
}

/**
 * Formats a next run date/time in both relative and absolute format.
 *
 * This is the timezone-honest half of the schedule display: `next_scheduled` is
 * an absolute instant, so `Date` renders it in the viewer's own zone with DST
 * handled, whatever zone the daemon scheduled it in. Shifting the cron fields
 * themselves cannot match that — an offset rolls the day over (23:00 Monday UTC
 * is Tuesday in Nairobi, so "Weekly on Monday" would be wrong), step values and
 * hour lists don't map to one local hour, and a fixed offset drifts across a DST
 * transition. Showing the concrete next occurrence sidesteps all of it.
 *
 * @param nextRun - ISO 8601 timestamp or Date object
 * @returns Formatted string like "in 5 hours · Tomorrow at 2:00 AM"
 */
export function formatNextRun(nextRun: string | Date | null | undefined): string {
  if (!nextRun) return 'Not scheduled'

  try {
    const date = typeof nextRun === 'string' ? new Date(nextRun) : nextRun
    if (isNaN(date.getTime())) return 'Invalid date'

    const now = new Date()
    const diff = date.getTime() - now.getTime()

    // If in the past, return "Overdue"
    if (diff < 0) return 'Overdue'

    const relative = formatRelativeTime(diff)
    const absolute = formatAbsoluteTime(date, now)

    return `${relative} · ${absolute}`
  } catch {
    return 'Invalid date'
  }
}

/**
 * Formats milliseconds difference to relative time
 */
function formatRelativeTime(diffMs: number): string {
  const seconds = Math.floor(diffMs / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (days > 0) {
    return days === 1 ? 'in 1 day' : `in ${days} days`
  }
  if (hours > 0) {
    return hours === 1 ? 'in 1 hour' : `in ${hours} hours`
  }
  if (minutes > 0) {
    return minutes === 1 ? 'in 1 minute' : `in ${minutes} minutes`
  }
  return 'in less than a minute'
}

/**
 * Formats absolute time considering if it's today, tomorrow, or another day
 */
function formatAbsoluteTime(date: Date, now: Date): string {
  const isToday = date.toDateString() === now.toDateString()

  const tomorrow = new Date(now)
  tomorrow.setDate(tomorrow.getDate() + 1)
  const isTomorrow = date.toDateString() === tomorrow.toDateString()

  const time = date.toLocaleTimeString('en-US', {
    hour: 'numeric',
    minute: '2-digit',
    hour12: true,
  })

  if (isToday) {
    return `Today at ${time}`
  }
  if (isTomorrow) {
    return `Tomorrow at ${time}`
  }

  // For dates further out, show day and date
  const dayName = date.toLocaleDateString('en-US', { weekday: 'short' })
  const monthDay = date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
  })

  return `${dayName}, ${monthDay} at ${time}`
}
