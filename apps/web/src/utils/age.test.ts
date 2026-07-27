import { describe, expect, it } from 'vitest'
import { formatAge } from './age'

describe('formatAge', () => {
  const now = new Date('2026-07-28T12:00:00Z')

  const ago = (ms: number) => new Date(now.getTime() - ms).toISOString()

  it('returns null when there is no timestamp', () => {
    expect(formatAge(undefined, now)).toBeNull()
    expect(formatAge(null, now)).toBeNull()
    expect(formatAge('', now)).toBeNull()
  })

  it('returns null for an unparseable timestamp', () => {
    expect(formatAge('not a date', now)).toBeNull()
  })

  it('collapses the last minute to "just now"', () => {
    expect(formatAge(ago(0), now)).toBe('just now')
    expect(formatAge(ago(59_000), now)).toBe('just now')
  })

  it('reports a future instant as "just now" rather than a negative age', () => {
    expect(formatAge(new Date(now.getTime() + 30_000).toISOString(), now)).toBe('just now')
  })

  it('counts minutes, hours and days', () => {
    expect(formatAge(ago(60_000), now)).toBe('1 minute ago')
    expect(formatAge(ago(5 * 60_000), now)).toBe('5 minutes ago')
    expect(formatAge(ago(60 * 60_000), now)).toBe('1 hour ago')
    expect(formatAge(ago(5 * 60 * 60_000), now)).toBe('5 hours ago')
    expect(formatAge(ago(24 * 60 * 60_000), now)).toBe('1 day ago')
    expect(formatAge(ago(9 * 24 * 60 * 60_000), now)).toBe('9 days ago')
  })

  it('floors rather than rounds up, so a stale backup never reads fresher', () => {
    expect(formatAge(ago(119 * 60_000), now)).toBe('1 hour ago')
    expect(formatAge(ago(47 * 60 * 60_000), now)).toBe('1 day ago')
  })
})
