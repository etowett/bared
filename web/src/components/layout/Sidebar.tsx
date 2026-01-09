import { Link, useLocation } from '@tanstack/react-router'
import { LayoutDashboard, RotateCcw, Database, ChevronLeft, ListTodo, Settings } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'

interface NavItem {
  id: string
  label: string
  icon: React.ComponentType<{ className?: string }>
  to: string
  matchPaths?: string[]
}

const navItems: NavItem[] = [
  {
    id: 'overview',
    label: 'Overview',
    icon: LayoutDashboard,
    to: '/',
  },
  {
    id: 'jobs',
    label: 'Jobs',
    icon: ListTodo,
    to: '/jobs',
  },
  {
    id: 'backup',
    label: 'Backup',
    icon: Database,
    to: '/backup',
    matchPaths: ['/backup', '/backup/jobs'],
  },
  {
    id: 'restore',
    label: 'Restore',
    icon: RotateCcw,
    to: '/restore',
    matchPaths: ['/restore', '/restore/jobs'],
  },
  {
    id: 'config',
    label: 'Configuration',
    icon: Settings,
    to: '/config',
    matchPaths: ['/config', '/config/storages', '/config/notifiers', '/config/targets', '/config/restore-targets'],
  },
]

interface SidebarProps {
  isCollapsed: boolean
  onToggleCollapse: () => void
}

export function Sidebar({ isCollapsed, onToggleCollapse }: SidebarProps) {
  const location = useLocation()

  const isActive = (item: NavItem) => {
    if (item.to === '/') {
      return location.pathname === '/'
    }
    if (item.matchPaths) {
      return item.matchPaths.some(
        (path) => location.pathname === path || location.pathname.startsWith(path + '/')
      )
    }
    return location.pathname === item.to || location.pathname.startsWith(item.to + '/')
  }

  return (
    <aside
      className={cn(
        'fixed left-0 top-16 bottom-0 z-40 border-r bg-card transition-all duration-300',
        isCollapsed ? 'w-16' : 'w-64'
      )}
    >
      <nav className="flex h-full flex-col">
        {/* Navigation Items */}
        <div className="flex-1 space-y-1 p-3">
          {navItems.map((item) => {
            const Icon = item.icon
            const active = isActive(item)

            return (
              <Link
                key={item.id}
                to={item.to}
                className={cn(
                  'flex w-full items-center gap-3 rounded-md px-3 py-2.5 text-sm font-medium transition-colors',
                  'hover:bg-accent hover:text-accent-foreground',
                  active ? 'bg-primary text-primary-foreground' : 'text-muted-foreground'
                )}
                title={isCollapsed ? item.label : undefined}
              >
                <Icon
                  className={cn('h-5 w-5 flex-shrink-0', active && 'text-primary-foreground')}
                />
                {!isCollapsed && <span>{item.label}</span>}
              </Link>
            )
          })}
        </div>

        {/* Collapse Toggle Button */}
        <div className="border-t p-3">
          <Button
            variant="ghost"
            size="sm"
            onClick={onToggleCollapse}
            className="w-full justify-center"
            title={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            <ChevronLeft
              className={cn(
                'h-4 w-4 transition-transform duration-300',
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
