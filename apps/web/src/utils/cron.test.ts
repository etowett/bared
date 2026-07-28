import { describe, expect, it, vi } from 'vitest'
import { describeSchedule, formatNextRun, serverZoneLabel } from './cron'

describe('serverZoneLabel', () => {
  // The offset is read straight off next_scheduled because the API formats it
  // with Go's RFC3339 from a time.Local instant — "Z" from the UTC container,
  // "+03:00" from a Nairobi host. Both forms are verified against the real
  // daemon output, not assumed.
  it('reads UTC from a Z-suffixed timestamp', () => {
    expect(serverZoneLabel('2026-07-28T02:00:00Z')).toBe('UTC')
  })

  it('treats an explicit zero offset as UTC', () => {
    expect(serverZoneLabel('2026-07-28T02:00:00+00:00')).toBe('UTC')
    expect(serverZoneLabel('2026-07-28T02:00:00-00:00')).toBe('UTC')
  })

  it('drops ":00" for whole-hour zones', () => {
    expect(serverZoneLabel('2026-07-28T02:00:00+03:00')).toBe('UTC+3')
    expect(serverZoneLabel('2026-07-27T21:00:00-05:00')).toBe('UTC-5')
  })

  it('keeps minutes for half- and quarter-hour zones', () => {
    expect(serverZoneLabel('2026-07-28T02:00:00+05:30')).toBe('UTC+5:30')
    expect(serverZoneLabel('2026-07-28T02:00:00+05:45')).toBe('UTC+5:45')
  })

  it('returns null when no offset is present', () => {
    expect(serverZoneLabel('2026-07-28T02:00:00')).toBeNull()
    expect(serverZoneLabel(undefined)).toBeNull()
    expect(serverZoneLabel(null)).toBeNull()
    expect(serverZoneLabel('')).toBeNull()
  })
})

describe('describeSchedule', () => {
  it('qualifies wall-clock schedules with the daemon zone', () => {
    expect(describeSchedule('0 2 * * *', '2026-07-28T02:00:00Z')).toBe('Daily at 2:00 AM UTC')
    expect(describeSchedule('30 14 * * 1', '2026-07-27T14:30:00+03:00')).toBe(
      'Weekly on Monday at 2:30 PM UTC+3'
    )
    expect(describeSchedule('0 3 15 * *', '2026-08-15T03:00:00Z')).toBe(
      'Monthly on the 15th at 3:00 AM UTC'
    )
  })

  it('leaves interval schedules unqualified', () => {
    // These fire at identical instants in every zone, so a zone suffix would
    // imply a distinction that does not exist.
    expect(describeSchedule('0 * * * *', '2026-07-27T11:00:00Z')).toBe('Hourly')
    expect(describeSchedule('*/15 * * * *', '2026-07-27T10:45:00Z')).toBe('Every 15 minutes')
    expect(describeSchedule('0 */6 * * *', '2026-07-27T12:00:00Z')).toBe('Every 6 hours')
  })

  it('falls back to the bare description when the zone is unknown', () => {
    expect(describeSchedule('0 2 * * *', undefined)).toBe('Daily at 2:00 AM')
  })

  it('passes non-schedules through unchanged', () => {
    expect(describeSchedule('', '2026-07-28T02:00:00Z')).toBe('Manual only')
    expect(describeSchedule('not a cron', '2026-07-28T02:00:00Z')).toBe('not a cron')
  })
})

describe('formatNextRun', () => {
  it('renders the absolute instant in the viewer zone, not the daemon zone', () => {
    // 02:00 UTC is 05:00 for a UTC+3 viewer. Asserting on toLocaleTimeString
    // output means the assertion tracks the environment's zone the same way the
    // component does.
    //
    // The clock has to be pinned. `formatNextRun` measures against the real
    // `new Date()` and short-circuits to "Overdue" for any past instant, so a
    // hardcoded date here is a time bomb: this assertion passed until 02:00 UTC
    // on 2026-07-28 and then failed on every branch at once. See #145.
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-07-28T01:00:00Z'))

    try {
      const nextRun = new Date('2026-07-28T02:00:00Z')
      const expectedTime = nextRun.toLocaleTimeString('en-US', {
        hour: 'numeric',
        minute: '2-digit',
        hour12: true,
      })

      expect(formatNextRun(nextRun.toISOString())).toContain(expectedTime)
    } finally {
      // A failure must not leak fake timers into the rest of the file.
      vi.useRealTimers()
    }
  })

  it('reports a past instant as overdue', () => {
    expect(formatNextRun('2020-01-01T00:00:00Z')).toBe('Overdue')
  })

  it('handles missing and malformed input', () => {
    expect(formatNextRun(null)).toBe('Not scheduled')
    expect(formatNextRun(undefined)).toBe('Not scheduled')
    expect(formatNextRun('nonsense')).toBe('Invalid date')
  })
})
