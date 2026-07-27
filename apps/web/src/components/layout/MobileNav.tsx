import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { HardDrive } from 'lucide-react'
import { NavLinks } from './Sidebar'

interface MobileNavProps {
  open: boolean
  onOpenChange: (_open: boolean) => void
}

/**
 * Navigation below `lg`, where the persistent rail would eat the page.
 *
 * Choosing a destination closes the sheet — leaving it open over the page the
 * user just asked for would mean two taps for every move.
 */
export function MobileNav({ open, onOpenChange }: MobileNavProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      {/* `max-w-[80%]` keeps a tappable strip of overlay at 320px. */}
      <SheetContent side="left" className="w-72 max-w-[80%] p-0 sm:max-w-xs">
        <SheetHeader className="border-b px-4 py-4 text-left">
          <SheetTitle className="flex items-center gap-2 text-base">
            <HardDrive aria-hidden="true" className="h-5 w-5 text-primary" />
            BareD
          </SheetTitle>
          <SheetDescription className="sr-only">Dashboard sections</SheetDescription>
        </SheetHeader>

        <nav aria-label="Main" className="flex flex-col py-2">
          <NavLinks onNavigate={() => onOpenChange(false)} />
        </nav>
      </SheetContent>
    </Sheet>
  )
}
