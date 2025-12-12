import { HardDrive, LogOut } from 'lucide-react';
import { ThemeToggle } from '@/components/ThemeToggle';
import { Button } from '@/components/ui/button';

interface TopHeaderProps {
  onLogout: () => void;
}

export function TopHeader({ onLogout }: TopHeaderProps) {
  return (
    <header className="fixed top-0 left-0 right-0 z-50 h-16 border-b bg-card">
      <div className="flex h-full items-center justify-between px-6">
        {/* Left: Logo & Branding */}
        <div className="flex items-center gap-3">
          <HardDrive className="h-6 w-6 text-primary" />
          <h1 className="text-xl font-semibold tracking-tight">
            <span className="text-primary">BareD</span>
            <span className="ml-2 text-muted-foreground text-sm font-normal">
              Backup Dashboard
            </span>
          </h1>
        </div>

        {/* Right: Theme Toggle & Logout */}
        <div className="flex items-center gap-2">
          <ThemeToggle />
          <Button
            variant="ghost"
            size="sm"
            onClick={onLogout}
            className="gap-2"
          >
            <LogOut className="h-4 w-4" />
            Logout
          </Button>
        </div>
      </div>
    </header>
  );
}
