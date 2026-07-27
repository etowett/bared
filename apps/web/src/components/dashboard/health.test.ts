import type { Target } from '@/types'
import { describe, expect, it } from 'vitest'
import { isHealthy, isUnreported, rankTargets, summarizeFleet } from './health'

const target = (overrides: Partial<Target> = {}): Target => ({
  name: 'db',
  type: 'postgres',
  database: 'app',
  is_running: false,
  ...overrides,
})

const now = new Date('2026-07-28T12:00:00Z')

describe('summarizeFleet', () => {
  it('does not count a target with no health fields as healthy', () => {
    // An older daemon omits `last_backup_status` entirely. Absence is not a
    // clean bill of health.
    const fleet = summarizeFleet([target({ name: 'legacy' })], now)

    expect(fleet.healthy).toBe(0)
    expect(fleet.unreported).toBe(1)
    expect(fleet.attention).toHaveLength(0)
    expect(isUnreported(target({ name: 'legacy' }))).toBe(true)
  })

  it('does not count a target the daemon could not read as healthy', () => {
    // `last_backup_status: 'unknown'` is the daemon saying its job store was
    // unreachable (#134). Landing in no bucket at all would let the banner
    // claim "All current" during a persistence outage — the false all-clear
    // the backend contract exists to prevent.
    const unreadable = target({ name: 'orders', last_backup_status: 'unknown' })
    const fleet = summarizeFleet([unreadable], now)

    expect(isHealthy(unreadable)).toBe(false)
    expect(isUnreported(unreadable)).toBe(true)
    expect(fleet.healthy).toBe(0)
    expect(fleet.unreported).toBe(1)
  })

  it('keeps a running target healthy while its backup is in flight', () => {
    expect(isHealthy(target({ last_backup_status: 'success', is_running: true }))).toBe(true)
  })

  it('lists a failure streak with the streak length', () => {
    const fleet = summarizeFleet(
      [target({ name: 'orders', last_backup_status: 'failed', consecutive_failures: 3 })],
      now
    )

    expect(fleet.attention).toEqual([
      expect.objectContaining({ reason: 'failing', detail: '3 failed runs in a row' }),
    ])
    expect(fleet.healthy).toBe(0)
  })

  it('names a single failure without pluralising it', () => {
    const fleet = summarizeFleet(
      [target({ last_backup_status: 'failed', consecutive_failures: 1 })],
      now
    )

    expect(fleet.attention[0].detail).toBe('last backup failed')
  })

  it('ages the last success of an overdue target', () => {
    const fleet = summarizeFleet(
      [
        target({
          last_backup_status: 'success',
          overdue: true,
          last_backup: '2026-07-26T12:00:00Z',
        }),
      ],
      now
    )

    expect(fleet.attention[0]).toMatchObject({
      reason: 'overdue',
      detail: 'last success 2 days ago',
    })
    expect(fleet.overdue).toBe(1)
  })

  it('flags a target that has never been backed up', () => {
    const fleet = summarizeFleet([target({ last_backup_status: 'never' })], now)

    expect(fleet.attention[0]).toMatchObject({ reason: 'never', detail: 'never backed up' })
  })

  it('orders attention by urgency: failing, then overdue, then never run', () => {
    const fleet = summarizeFleet(
      [
        target({ name: 'c', last_backup_status: 'never' }),
        target({ name: 'b', last_backup_status: 'success', overdue: true }),
        target({ name: 'a', last_backup_status: 'failed', consecutive_failures: 2 }),
        target({ name: 'ok', last_backup_status: 'success' }),
      ],
      now
    )

    expect(fleet.attention.map((entry) => entry.target.name)).toEqual(['a', 'b', 'c'])
    expect(fleet.total).toBe(4)
    expect(fleet.healthy).toBe(1)
  })
})

describe('rankTargets', () => {
  it('sorts by severity, then by name', () => {
    const ranked = rankTargets([
      target({ name: 'healthy-b', last_backup_status: 'success' }),
      target({ name: 'unreported' }),
      target({ name: 'running', last_backup_status: 'success', is_running: true }),
      target({ name: 'never', last_backup_status: 'never' }),
      target({ name: 'overdue', last_backup_status: 'success', overdue: true }),
      target({ name: 'failing', last_backup_status: 'failed', consecutive_failures: 1 }),
      target({ name: 'healthy-a', last_backup_status: 'success' }),
    ])

    expect(ranked.map((entry) => entry.name)).toEqual([
      'failing',
      'overdue',
      'never',
      'running',
      'unreported',
      'healthy-a',
      'healthy-b',
    ])
  })

  it('ranks a failing target above an overdue one even while it is running', () => {
    const ranked = rankTargets([
      target({ name: 'overdue', last_backup_status: 'success', overdue: true }),
      target({
        name: 'failing',
        last_backup_status: 'failed',
        consecutive_failures: 4,
        is_running: true,
      }),
    ])

    expect(ranked[0].name).toBe('failing')
  })
})
