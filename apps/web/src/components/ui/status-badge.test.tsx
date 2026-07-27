import { describe, expect, it } from 'vitest'
import { Zap } from 'lucide-react'
import { render, screen } from '../../test/utils'
import { StatusBadge } from './status-badge'

describe('StatusBadge', () => {
  describe('job state', () => {
    it.each([
      ['running', 'Running', 'text-info'],
      ['queued', 'Queued', 'text-warning'],
      ['completed', 'Completed', 'text-success'],
      ['failed', 'Failed', 'text-danger'],
      ['cancelling', 'Cancelling', 'text-warning'],
      ['cancelled', 'Cancelled', 'text-muted-foreground'],
      ['idle', 'Idle', 'text-muted-foreground'],
    ] as const)('renders %s with its label and tone', (status, label, toneClass) => {
      const { container } = render(<StatusBadge status={status} />)

      expect(screen.getByText(label)).toBeInTheDocument()
      expect(container.firstElementChild).toHaveClass(toneClass)
    })

    it('defaults to the job kind when none is given', () => {
      render(<StatusBadge status="completed" />)
      expect(screen.getByText('Completed')).toBeInTheDocument()
    })

    it('spins the glyph only while work is in flight', () => {
      const { container: running } = render(<StatusBadge status="running" />)
      expect(running.querySelector('svg')).toHaveClass('motion-safe:animate-spin')

      const { container: completed } = render(<StatusBadge status="completed" />)
      expect(completed.querySelector('svg')).not.toHaveClass('motion-safe:animate-spin')
    })
  })

  describe('colour independence', () => {
    it('pairs every status with a glyph and a word', () => {
      const { container } = render(<StatusBadge status="failed" />)

      // The glyph is decorative — the word carries the meaning for screen
      // readers, and the two together carry it for colour-blind viewers.
      const icon = container.querySelector('svg')
      expect(icon).toBeInTheDocument()
      expect(icon).toHaveAttribute('aria-hidden', 'true')
      expect(container.firstElementChild).toHaveTextContent('Failed')
    })
  })

  describe('enabled state', () => {
    it('renders Enabled for true', () => {
      const { container } = render(<StatusBadge kind="enabled" status={true} />)

      expect(screen.getByText('Enabled')).toBeInTheDocument()
      expect(container.firstElementChild).toHaveClass('text-success')
    })

    it('renders Disabled for false', () => {
      const { container } = render(<StatusBadge kind="enabled" status={false} />)

      expect(screen.getByText('Disabled')).toBeInTheDocument()
      expect(container.firstElementChild).toHaveClass('text-muted-foreground')
    })
  })

  describe('database type', () => {
    it('shows the engine name and stays neutral', () => {
      const { container } = render(<StatusBadge kind="database" status="postgresql" />)

      expect(screen.getByText('postgresql')).toBeInTheDocument()
      expect(container.firstElementChild).toHaveClass('text-muted-foreground')
    })
  })

  describe('trigger', () => {
    it.each([
      ['schedule', 'Scheduled'],
      ['manual', 'Manual'],
      ['api', 'API'],
    ] as const)('renders the %s trigger', (trigger, label) => {
      render(<StatusBadge kind="trigger" status={trigger} />)
      expect(screen.getByText(label)).toBeInTheDocument()
    })
  })

  describe('target health', () => {
    it.each([
      ['running', 'Running', 'text-info'],
      ['failing', 'Failing', 'text-danger'],
      ['overdue', 'Overdue', 'text-warning'],
      ['healthy', 'Healthy', 'text-success'],
      ['never', 'Never run', 'text-muted-foreground'],
      ['unknown', 'Idle', 'text-muted-foreground'],
    ] as const)('renders %s with its label and tone', (health, label, toneClass) => {
      const { container } = render(<StatusBadge kind="target" status={health} />)

      expect(screen.getByText(label)).toBeInTheDocument()
      expect(container.firstElementChild).toHaveClass(toneClass)
    })
  })

  describe('custom', () => {
    it('accepts an explicit tone, label and icon for unmapped subjects', () => {
      const { container } = render(
        <StatusBadge kind="custom" tone="info" label="Live" icon={Zap} />
      )

      expect(screen.getByText('Live')).toBeInTheDocument()
      expect(container.firstElementChild).toHaveClass('text-info')
      expect(container.querySelector('svg')).toBeInTheDocument()
    })
  })

  it('merges a caller className without dropping the tone', () => {
    const { container } = render(<StatusBadge status="completed" className="ml-2" />)

    expect(container.firstElementChild).toHaveClass('ml-2')
    expect(container.firstElementChild).toHaveClass('text-success')
  })
})
