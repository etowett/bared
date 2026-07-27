import type { LinkProps } from '@tanstack/react-router'
import { Database, LayoutDashboard, ListTodo, RotateCcw, Settings } from 'lucide-react'

export interface NavItem {
  id: string
  label: string
  icon: React.ComponentType<{ className?: string }>
  to: LinkProps['to']
  /** Extra paths that should light this item up, beyond `to` and its children. */
  matchPaths?: string[]
}

/**
 * The one navigation list. The desktop sidebar and the mobile sheet both read
 * it, so a new section appears in both or in neither.
 */
export const navItems: NavItem[] = [
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
    matchPaths: [
      '/config',
      '/config/storages',
      '/config/notifiers',
      '/config/targets',
      '/config/restore-targets',
      '/config/import',
    ],
  },
]

/** Whether `pathname` sits inside `item`'s section. */
export function isNavItemActive(item: NavItem, pathname: string): boolean {
  if (item.to === '/') {
    return pathname === '/'
  }
  const paths = item.matchPaths ?? [item.to as string]
  return paths.some((path) => pathname === path || pathname.startsWith(path + '/'))
}
