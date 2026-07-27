import { describe, expect, it } from 'vitest'
import { render, screen, within } from '../../test/utils'
import { PageHeader } from './page-header'
import { StatusBadge } from './status-badge'

// The router is real: `test/utils.tsx` supplies one, so the breadcrumb renders
// a real `<Link>` and the href it resolves to is the thing under test. Add a
// path to that wrapper's stub tree when a crumb needs a new destination.

describe('PageHeader', () => {
  it('renders the title as the single level-2 heading', () => {
    render(<PageHeader title="All Jobs" />)

    const headings = screen.getAllByRole('heading', { level: 2 })
    expect(headings).toHaveLength(1)
    expect(headings[0]).toHaveTextContent('All Jobs')
  })

  it('renders the description when given', () => {
    render(<PageHeader title="Backup" description="Run a backup now." />)

    expect(screen.getByText('Run a backup now.')).toBeInTheDocument()
  })

  it('omits the description element entirely when not given', () => {
    render(<PageHeader title="Backup" />)

    expect(screen.queryByText(/run a backup/i)).not.toBeInTheDocument()
  })

  it('omits the breadcrumb nav when there are no crumbs', () => {
    render(<PageHeader title="Overview" />)

    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
  })

  describe('breadcrumbs', () => {
    it('links every crumb that has a destination', () => {
      render(
        <PageHeader
          title="Job a1b2c3d4"
          breadcrumbs={[
            { label: 'Jobs', to: '/jobs' },
            { label: 'Backup history', to: '/backup/jobs' },
            { label: 'a1b2c3d4', mono: true },
          ]}
        />
      )

      const nav = screen.getByRole('navigation', { name: 'Breadcrumb' })
      expect(within(nav).getByRole('link', { name: 'Jobs' })).toHaveAttribute('href', '/jobs')
      expect(within(nav).getByRole('link', { name: 'Backup history' })).toHaveAttribute(
        'href',
        '/backup/jobs'
      )
    })

    it('marks the final crumb as the current page rather than a link', () => {
      render(
        <PageHeader
          title="Job a1b2c3d4"
          breadcrumbs={[{ label: 'Jobs', to: '/jobs' }, { label: 'a1b2c3d4' }]}
        />
      )

      const nav = screen.getByRole('navigation', { name: 'Breadcrumb' })
      expect(within(nav).queryByRole('link', { name: 'a1b2c3d4' })).not.toBeInTheDocument()
      expect(within(nav).getByText('a1b2c3d4')).toHaveAttribute('aria-current', 'page')
    })

    it('renders machine values in the mono face', () => {
      render(<PageHeader title="Job a1b2c3d4" breadcrumbs={[{ label: 'a1b2c3d4', mono: true }]} />)

      expect(screen.getByText('a1b2c3d4')).toHaveClass('font-mono')
    })
  })

  it('renders a status badge beside the title', () => {
    render(<PageHeader title="Job a1b2c3d4" status={<StatusBadge status="running" />} />)

    expect(screen.getByText('Running')).toBeInTheDocument()
  })

  it('renders the actions slot', () => {
    render(<PageHeader title="Backup" actions={<button type="button">Job history</button>} />)

    expect(screen.getByRole('button', { name: 'Job history' })).toBeInTheDocument()
  })
})
