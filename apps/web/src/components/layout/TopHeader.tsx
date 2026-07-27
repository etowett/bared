import { HardDrive, LoaderCircle, LogOut, Menu, PlugZap, RefreshCw, WifiOff } from 'lucide-react'
import { ThemeToggle } from '@/components/ThemeToggle'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/ui/status-badge'
import { useDaemonStatus } from '@/hooks/useDaemonStatus'

interface TopHeaderProps {
  onLogout: () => void
  /** Opens the navigation sheet. Only reachable below `lg`. */
  onOpenNav: () => void
}

export function TopHeader({ onLogout, onOpenNav }: TopHeaderProps) {
  const { reachable, checking, version, refetch } = useDaemonStatus()

  return (
    <header className="fixed left-0 right-0 top-0 z-50 h-16 border-b bg-card">
      <div className="flex h-full items-center justify-between gap-2 px-4 sm:px-6">
        {/* Left: menu (below lg) + branding */}
        <div className="flex min-w-0 items-center gap-2 sm:gap-3">
          <Button
            variant="ghost"
            size="icon"
            className="lg:hidden"
            onClick={onOpenNav}
            aria-label="Open navigation menu"
          >
            <Menu aria-hidden="true" className="h-5 w-5" />
          </Button>

          <HardDrive aria-hidden="true" className="h-6 w-6 shrink-0 text-primary" />
          <h1 className="truncate text-xl font-semibold tracking-tight">
            <span className="text-primary">BareD</span>
            <span className="ml-2 hidden text-sm font-normal text-muted-foreground sm:inline">
              Backup Dashboard
            </span>
          </h1>
        </div>

        {/* Right: daemon reachability, theme, sign out */}
        <div className="flex shrink-0 items-center gap-2">
          {reachable ? (
            // Reassurance is optional at narrow widths; the alarm below is not.
            <StatusBadge
              kind="custom"
              tone="success"
              label="Connected"
              icon={PlugZap}
              className="hidden sm:inline-flex"
              title={version ? `Daemon v${version}` : 'Daemon reachable'}
            />
          ) : checking ? (
            <StatusBadge
              kind="custom"
              tone="warning"
              label="Checking"
              icon={LoaderCircle}
              className="hidden sm:inline-flex [&>svg]:motion-safe:animate-spin"
            />
          ) : (
            <>
              <StatusBadge kind="custom" tone="danger" label="Unreachable" icon={WifiOff} />
              <Button
                variant="outline"
                size="icon"
                onClick={() => refetch()}
                aria-label="Retry connecting to the daemon"
                title="Retry connecting to the daemon"
              >
                <RefreshCw aria-hidden="true" className="h-4 w-4" />
              </Button>
            </>
          )}

          <ThemeToggle />

          <Button
            variant="ghost"
            size="sm"
            onClick={onLogout}
            className="gap-2"
            aria-label="Sign out"
          >
            <LogOut aria-hidden="true" className="h-4 w-4" />
            <span className="hidden sm:inline">Logout</span>
          </Button>
        </div>
      </div>
    </header>
  )
}
