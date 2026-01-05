import { describe, expect, it } from 'vitest'
import { cn, formatBytes, formatDate, formatDuration } from './utils'

describe('Utility Functions', () => {
  describe('cn (className merge)', () => {
    it('merges class names correctly', () => {
      expect(cn('foo', 'bar')).toBe('foo bar')
    })

    it('handles conditional classes', () => {
      expect(cn('foo', false && 'bar', 'baz')).toBe('foo baz')
    })

    it('handles tailwind conflicting classes', () => {
      // twMerge should handle conflicting tailwind classes
      const result = cn('px-2 py-1', 'px-4')
      expect(result).toContain('px-4')
      expect(result).not.toContain('px-2')
    })

    it('handles empty input', () => {
      expect(cn()).toBe('')
    })
  })

  describe('formatBytes', () => {
    it('returns N/A for undefined', () => {
      expect(formatBytes(undefined)).toBe('N/A')
    })

    it('returns N/A for 0', () => {
      expect(formatBytes(0)).toBe('N/A')
    })

    it('formats bytes correctly', () => {
      expect(formatBytes(500)).toBe('500.00 B')
    })

    it('formats kilobytes correctly', () => {
      expect(formatBytes(1024)).toBe('1.00 KB')
      expect(formatBytes(2048)).toBe('2.00 KB')
      expect(formatBytes(1536)).toBe('1.50 KB')
    })

    it('formats megabytes correctly', () => {
      expect(formatBytes(1048576)).toBe('1.00 MB')
      expect(formatBytes(2097152)).toBe('2.00 MB')
      expect(formatBytes(5242880)).toBe('5.00 MB')
    })

    it('formats gigabytes correctly', () => {
      expect(formatBytes(1073741824)).toBe('1.00 GB')
      expect(formatBytes(2147483648)).toBe('2.00 GB')
    })

    it('formats terabytes correctly', () => {
      expect(formatBytes(1099511627776)).toBe('1.00 TB')
      expect(formatBytes(2199023255552)).toBe('2.00 TB')
    })

    it('handles fractional values', () => {
      expect(formatBytes(1536)).toBe('1.50 KB')
      expect(formatBytes(1610612736)).toBe('1.50 GB')
    })

    it('does not exceed TB unit', () => {
      const result = formatBytes(1099511627776 * 1024) // 1024 TB
      expect(result).toContain('TB')
    })
  })

  describe('formatDate', () => {
    it('returns N/A for undefined', () => {
      expect(formatDate(undefined)).toBe('N/A')
    })

    it('returns N/A for empty string', () => {
      expect(formatDate('')).toBe('N/A')
    })

    it('formats valid ISO date string', () => {
      const date = '2025-12-09T10:00:00Z'
      const result = formatDate(date)
      // The result will vary based on locale, but should contain the date
      expect(result).not.toBe('N/A')
      expect(result).not.toBe(date) // Should be formatted
    })

    it('formats date with milliseconds', () => {
      const date = '2025-12-09T10:00:00.123Z'
      const result = formatDate(date)
      expect(result).not.toBe('N/A')
    })

    it('handles local date strings', () => {
      const date = '2025-12-09T10:00:00'
      const result = formatDate(date)
      expect(result).not.toBe('N/A')
    })

    it('handles invalid date format gracefully', () => {
      const invalidDate = 'not-a-date'
      const result = formatDate(invalidDate)
      // Should return some string representation (may be "Invalid Date" or the original string)
      expect(result).toBeTruthy()
      expect(typeof result).toBe('string')
    })

    it('handles date objects converted to strings', () => {
      const date = new Date('2025-12-09T10:00:00Z').toISOString()
      const result = formatDate(date)
      expect(result).not.toBe('N/A')
    })
  })

  describe('formatDuration', () => {
    it('returns N/A for undefined', () => {
      expect(formatDuration(undefined)).toBe('N/A')
    })

    it('returns N/A for 0', () => {
      expect(formatDuration(0)).toBe('N/A')
    })

    it('formats seconds only', () => {
      expect(formatDuration(1)).toBe('1s')
      expect(formatDuration(30)).toBe('30s')
      expect(formatDuration(59)).toBe('59s')
    })

    it('formats minutes and seconds', () => {
      expect(formatDuration(60)).toBe('1m 0s')
      expect(formatDuration(90)).toBe('1m 30s')
      expect(formatDuration(120)).toBe('2m 0s')
      expect(formatDuration(125)).toBe('2m 5s')
    })

    it('formats hours, minutes, and seconds', () => {
      expect(formatDuration(3600)).toBe('1h 0m 0s')
      expect(formatDuration(3661)).toBe('1h 1m 1s')
      expect(formatDuration(7200)).toBe('2h 0m 0s')
      expect(formatDuration(7325)).toBe('2h 2m 5s')
    })

    it('handles large durations', () => {
      expect(formatDuration(86400)).toBe('24h 0m 0s') // 1 day
      expect(formatDuration(90061)).toBe('25h 1m 1s')
    })

    it('floors fractional seconds', () => {
      expect(formatDuration(90.9)).toBe('1m 30s')
      expect(formatDuration(3661.7)).toBe('1h 1m 1s')
    })

    it('handles zero hours with minutes', () => {
      expect(formatDuration(3599)).toBe('59m 59s')
    })

    it('handles zero minutes with hours', () => {
      expect(formatDuration(3605)).toBe('1h 0m 5s')
    })

    it('handles exact hour boundaries', () => {
      expect(formatDuration(3600)).toBe('1h 0m 0s')
      expect(formatDuration(7200)).toBe('2h 0m 0s')
    })

    it('handles exact minute boundaries', () => {
      expect(formatDuration(60)).toBe('1m 0s')
      expect(formatDuration(180)).toBe('3m 0s')
    })
  })
})
