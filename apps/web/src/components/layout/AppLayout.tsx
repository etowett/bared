import { useEffect, useState, ReactNode } from 'react'
import { useLocation } from '@tanstack/react-router'
import { TopHeader } from './TopHeader'
import { Sidebar } from './Sidebar'
import { MobileNav } from './MobileNav'
import { cn } from '@/lib/utils'

interface AppLayoutProps {
  children: ReactNode
  onLogout: () => void
}

export function AppLayout({ children, onLogout }: AppLayoutProps) {
  const [isCollapsed, setIsCollapsed] = useState(() => {
    const stored = localStorage.getItem('sidebar-collapsed')
    return stored === 'true'
  })
  const [isMobileNavOpen, setIsMobileNavOpen] = useState(false)
  const location = useLocation()
  const [lastPathname, setLastPathname] = useState(location.pathname)

  useEffect(() => {
    localStorage.setItem('sidebar-collapsed', String(isCollapsed))
  }, [isCollapsed])

  // A route change from anywhere — a link inside the sheet, the browser's back
  // button — dismisses the sheet. Adjusting during render rather than in an
  // effect keeps it to a single pass, so the sheet never paints over the page
  // the user just asked for.
  if (lastPathname !== location.pathname) {
    setLastPathname(location.pathname)
    setIsMobileNavOpen(false)
  }

  const toggleCollapse = () => {
    setIsCollapsed((prev) => !prev)
  }

  return (
    <div className="min-h-screen">
      <TopHeader onLogout={onLogout} onOpenNav={() => setIsMobileNavOpen(true)} />
      <Sidebar isCollapsed={isCollapsed} onToggleCollapse={toggleCollapse} />
      <MobileNav open={isMobileNavOpen} onOpenChange={setIsMobileNavOpen} />

      {/*
        The rail only exists at `lg` and up, so the offset only exists there.
        Below that the content owns the full width — at 320px the old
        unconditional `pl-64` left about 64px of usable page.
      */}
      <main
        className={cn(
          'pt-16 motion-safe:transition-all motion-safe:duration-300',
          isCollapsed ? 'lg:pl-16' : 'lg:pl-64'
        )}
      >
        <div className="container mx-auto p-4 sm:p-6">{children}</div>
      </main>
    </div>
  )
}
