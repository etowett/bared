import { describe, expect, it } from 'vitest'
import { describeTargetHealth } from './status'
import type { Target } from '@/types'

const target = (overrides: Partial<Target> = {}): Target => ({
  name: 'orders_primary',
  type: 'mysql',
  database: 'orders',
  is_running: false,
  ...overrides,
})

describe('describeTargetHealth', () => {
  it('reports work in flight ahead of any health verdict', () => {
    expect(
      describeTargetHealth(
        target({ is_running: true, last_backup_status: 'failed', overdue: true })
      )
    ).toBe('running')
  })

  it('reports a failed last run ahead of being overdue', () => {
    expect(describeTargetHealth(target({ last_backup_status: 'failed', overdue: true }))).toBe(
      'failing'
    )
  })

  it('reports overdue when the last run succeeded but the next one is late', () => {
    expect(describeTargetHealth(target({ last_backup_status: 'success', overdue: true }))).toBe(
      'overdue'
    )
  })

  it('reports healthy when the last run succeeded and nothing is late', () => {
    expect(describeTargetHealth(target({ last_backup_status: 'success', overdue: false }))).toBe(
      'healthy'
    )
  })

  it('distinguishes a target that has never run from a healthy one', () => {
    expect(describeTargetHealth(target({ last_backup_status: 'never' }))).toBe('never')
  })

  it('says unknown rather than healthy when the daemon disclaims the history', () => {
    // The daemon sends "unknown" when its job store was unreachable (#134).
    // Falling through to healthy here would restore the false all-clear the
    // backend went out of its way to avoid.
    expect(describeTargetHealth(target({ last_backup_status: 'unknown', overdue: false }))).toBe(
      'unknown'
    )
  })

  it('says unknown rather than healthy when the daemon reports no health at all', () => {
    // An older daemon predates the health fields (#127). Treating absence as
    // success would show a green tick the backend never claimed.
    expect(describeTargetHealth(target())).toBe('unknown')
  })
})
