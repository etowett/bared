import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/utils'

interface StatCardProps {
  title: string
  value: string | number
  className?: string
}

export function StatCard({ title, value, className }: StatCardProps) {
  return (
    <Card className={cn('', className)}>
      <CardHeader className="pb-2">
        <CardTitle className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {/*
          Mono for anything the daemon produced. It is the one typographic rule
          this dashboard keeps: sans frames, mono reports.
        */}
        <div className="text-metric font-mono font-semibold tracking-tight tabular-nums">
          {value}
        </div>
      </CardContent>
    </Card>
  )
}
