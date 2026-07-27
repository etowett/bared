import { Link, useLocation } from '@tanstack/react-router'
import { ChevronLeft } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { isNavItemActive, navItems } from './nav-items'

interface NavLinksProps {
  /** Hide the labels — only the persistent desktop rail ever does this. */
  iconsOnly?: boolean
  /** Called after a destination is chosen, so the mobile sheet can close. */
  onNavigate?: () => void
}

/** The shared link list, rendered by the desktop rail and the mobile sheet. */
export function NavLinks({ iconsOnly = false, onNavigate }: NavLinksProps) {
  const location = useLocation()

  return (
    <div className="flex-1 space-y-1 p-3">
      {navItems.map((item) => {
        const Icon = item.icon
        const active = isNavItemActive(item, location.pathname)

        return (
          <Link
            key={item.id}
            to={item.to}
            onClick={onNavigate}
            aria-current={active ? 'page' : undefined}
            className={cn(
              'flex w-full items-center gap-3 rounded-md px-3 py-2.5 text-sm font-medium transition-colors',
              'hover:bg-accent hover:text-accent-foreground',
              'focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring',
              active ? 'bg-primary text-primary-foreground' : 'text-muted-foreground'
            )}
            title={iconsOnly ? item.label : undefined}
          >
            <Icon className={cn('h-5 w-5 shrink-0', active && 'text-primary-foreground')} />
            {iconsOnly ? <span className="sr-only">{item.label}</span> : <span>{item.label}</span>}
          </Link>
        )
      })}
    </div>
  )
}

interface SidebarProps {
  isCollapsed: boolean
  onToggleCollapse: () => void
}

/**
 * The persistent desktop rail.
 *
 * Hidden below `lg`, where navigation moves into the sheet `AppLayout` opens
 * from the header — an expanded 256px rail leaves roughly 64px of content at
 * 320px wide.
 */
export function Sidebar({ isCollapsed, onToggleCollapse }: SidebarProps) {
  return (
    <aside
      className={cn(
        'fixed bottom-0 left-0 top-16 z-40 hidden border-r bg-card transition-all duration-300 lg:block',
        isCollapsed ? 'w-16' : 'w-64'
      )}
    >
      <nav aria-label="Main" className="flex h-full flex-col">
        <NavLinks iconsOnly={isCollapsed} />

        <div className="border-t p-3">
          <Button
            variant="ghost"
            size="sm"
            onClick={onToggleCollapse}
            className="w-full justify-center"
            aria-label={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            title={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            <ChevronLeft
              aria-hidden="true"
              className={cn(
                'h-4 w-4 motion-safe:transition-transform motion-safe:duration-300',
                isCollapsed && 'rotate-180'
              )}
            />
            {!isCollapsed && <span className="ml-2">Collapse</span>}
          </Button>
        </div>
      </nav>
    </aside>
  )
}
