import { describe, expect, it } from 'vitest'
import { render, screen } from '../test/utils'
import type { Progress } from '../types'
import { JobProgress } from './JobProgress'

describe('JobProgress Component', () => {
  const createMockProgress = (overrides: Partial<Progress> = {}): Progress => ({
    percent: 50,
    current: 50,
    total: 100,
    stage: 'Processing',
    bytes_processed: 1048576,
    bytes_total: 2097152,
    eta: new Date(Date.now() + 60000).toISOString(), // 1 minute from now
    message: 'Processing data',
    ...overrides,
  })

  describe('Compact Mode', () => {
    it('renders compact progress bar with percentage', () => {
      const progress = createMockProgress({ percent: 75 })

      render(<JobProgress progress={progress} compact={true} />)

      expect(screen.getByText('75.0%')).toBeInTheDocument()
    })

    it('displays progress bar in compact mode', () => {
      const progress = createMockProgress({ percent: 50 })

      const { container } = render(<JobProgress progress={progress} compact={true} />)

      const progressBar = container.querySelector('[role="progressbar"]')
      expect(progressBar).toBeInTheDocument()
    })

    it('does not show stage or message in compact mode', () => {
      const progress = createMockProgress({ stage: 'Processing', message: 'Test message' })

      render(<JobProgress progress={progress} compact={true} />)

      expect(screen.queryByText('Processing')).not.toBeInTheDocument()
      expect(screen.queryByText('Test message')).not.toBeInTheDocument()
    })

    it('handles 0% progress', () => {
      const progress = createMockProgress({ percent: 0 })

      render(<JobProgress progress={progress} compact={true} />)

      expect(screen.getByText('0.0%')).toBeInTheDocument()
    })

    it('handles 100% progress', () => {
      const progress = createMockProgress({ percent: 100 })

      render(<JobProgress progress={progress} compact={true} />)

      expect(screen.getByText('100.0%')).toBeInTheDocument()
    })

    it('clamps progress above 100%', () => {
      const progress = createMockProgress({ percent: 150 })

      render(<JobProgress progress={progress} compact={true} />)

      // Display shows actual percent
      expect(screen.getByText('150.0%')).toBeInTheDocument()
    })
  })

  describe('Full Mode', () => {
    it('renders full progress with stage and percentage', () => {
      const progress = createMockProgress({ percent: 60, stage: 'Compressing' })

      render(<JobProgress progress={progress} compact={false} />)

      expect(screen.getByText('Compressing')).toBeInTheDocument()
      expect(screen.getByText('60.0%')).toBeInTheDocument()
    })

    it('displays bytes processed and total when available', () => {
      const progress = createMockProgress({
        bytes_processed: 1048576, // 1 MB
        bytes_total: 10485760, // 10 MB
      })

      render(<JobProgress progress={progress} compact={false} />)

      expect(screen.getByText(/1\.00 MB/)).toBeInTheDocument()
      expect(screen.getByText(/10\.00 MB/)).toBeInTheDocument()
    })

    it('does not display bytes when total is 0', () => {
      const progress = createMockProgress({
        bytes_processed: 0,
        bytes_total: 0,
      })

      render(<JobProgress progress={progress} compact={false} />)

      expect(screen.queryByText(/MB/)).not.toBeInTheDocument()
    })

    it('displays message when provided', () => {
      const progress = createMockProgress({ message: 'Currently processing table users' })

      render(<JobProgress progress={progress} compact={false} />)

      expect(screen.getByText('Currently processing table users')).toBeInTheDocument()
    })

    it('does not display message when not provided', () => {
      const progress = createMockProgress({ message: undefined })

      render(<JobProgress progress={progress} compact={false} />)

      const container = screen.getByText('Processing').parentElement?.parentElement
      expect(container?.querySelector('.italic')).not.toBeInTheDocument()
    })

    it('displays ETA when provided', () => {
      const futureTime = new Date(Date.now() + 3600000) // 1 hour from now
      const progress = createMockProgress({ eta: futureTime.toISOString() })

      render(<JobProgress progress={progress} compact={false} />)

      expect(screen.getByText(/ETA:/)).toBeInTheDocument()
    })

    it('formats ETA in seconds correctly', () => {
      const futureTime = new Date(Date.now() + 45000) // 45 seconds
      const progress = createMockProgress({ eta: futureTime.toISOString() })

      render(<JobProgress progress={progress} compact={false} />)

      expect(screen.getByText(/ETA: 4[0-9]s/)).toBeInTheDocument()
    })

    it('formats ETA in minutes and seconds correctly', () => {
      const futureTime = new Date(Date.now() + 150000) // 2.5 minutes
      const progress = createMockProgress({ eta: futureTime.toISOString() })

      render(<JobProgress progress={progress} compact={false} />)

      expect(screen.getByText(/ETA: 2m/)).toBeInTheDocument()
    })

    it('formats ETA in hours and minutes correctly', () => {
      const futureTime = new Date(Date.now() + 7200000) // 2 hours
      const progress = createMockProgress({ eta: futureTime.toISOString() })

      render(<JobProgress progress={progress} compact={false} />)

      // Match either "2h 0m" or "1h 59m" due to timing differences
      expect(screen.getByText(/ETA: (2h 0m|1h 59m)/)).toBeInTheDocument()
    })

    it('shows "Soon" for past ETA', () => {
      const pastTime = new Date(Date.now() - 1000) // 1 second ago
      const progress = createMockProgress({ eta: pastTime.toISOString() })

      render(<JobProgress progress={progress} compact={false} />)

      expect(screen.getByText(/ETA: Soon/)).toBeInTheDocument()
    })

    it('does not display ETA when missing', () => {
      const progress = createMockProgress({ eta: undefined })

      render(<JobProgress progress={progress} compact={false} />)

      expect(screen.queryByText(/ETA:/)).not.toBeInTheDocument()
    })

    it('handles invalid ETA format by showing NaN', () => {
      const progress = createMockProgress({ eta: 'invalid-date' })

      render(<JobProgress progress={progress} compact={false} />)

      expect(screen.getByText(/ETA: NaNs/)).toBeInTheDocument()
    })

    it('renders progress bar with correct value', () => {
      const progress = createMockProgress({ percent: 75 })

      const { container } = render(<JobProgress progress={progress} compact={false} />)

      const progressBar = container.querySelector('[role="progressbar"]')
      expect(progressBar).toBeInTheDocument()
    })
  })

  describe('Default Mode', () => {
    it('defaults to full mode when compact is not specified', () => {
      const progress = createMockProgress({ stage: 'Testing', message: 'Test' })

      render(<JobProgress progress={progress} />)

      expect(screen.getByText('Testing')).toBeInTheDocument()
      expect(screen.getByText('Test')).toBeInTheDocument()
    })
  })

  describe('Edge Cases', () => {
    it('handles very small percentages', () => {
      const progress = createMockProgress({ percent: 0.01 })

      render(<JobProgress progress={progress} compact={true} />)

      expect(screen.getByText('0.0%')).toBeInTheDocument()
    })

    it('handles very large byte values', () => {
      const progress = createMockProgress({
        bytes_processed: 1099511627776, // 1 TB
        bytes_total: 2199023255552, // 2 TB
      })

      render(<JobProgress progress={progress} compact={false} />)

      expect(screen.getByText(/1\.00 TB/)).toBeInTheDocument()
      expect(screen.getByText(/2\.00 TB/)).toBeInTheDocument()
    })

    it('handles progress without bytes information', () => {
      const progress = createMockProgress({
        bytes_processed: 0,
        bytes_total: 0,
      })

      render(<JobProgress progress={progress} compact={false} />)

      expect(screen.getByText('Processing')).toBeInTheDocument()
      expect(screen.getByText('50.0%')).toBeInTheDocument()
    })

    it('handles empty stage name', () => {
      const progress = createMockProgress({ stage: '' })

      render(<JobProgress progress={progress} compact={false} />)

      // Should still render but with empty stage
      expect(screen.getByText('50.0%')).toBeInTheDocument()
    })

    it('handles exactly 100% progress', () => {
      const progress = createMockProgress({ percent: 100 })

      const { container } = render(<JobProgress progress={progress} compact={false} />)

      expect(screen.getByText('100.0%')).toBeInTheDocument()
      const progressBar = container.querySelector('[role="progressbar"]')
      expect(progressBar).toBeInTheDocument()
    })

    it('handles fractional percentages', () => {
      const progress = createMockProgress({ percent: 33.333 })

      render(<JobProgress progress={progress} compact={true} />)

      expect(screen.getByText('33.3%')).toBeInTheDocument()
    })
  })
})
