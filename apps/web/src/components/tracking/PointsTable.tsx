import { format } from 'date-fns'
import { Crosshair, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import type { TrackingPoint } from '@/features/tracking/trackingApi'
import { cn } from '@/lib/utils'

interface Props {
  points: TrackingPoint[]
  loading?: boolean
  selectedId?: string | null
  onSelect?: (id: string) => void
  onDelete?: (id: string) => Promise<void>
}

export function PointsTable({ points, loading, selectedId, onSelect, onDelete }: Props) {
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const rows = points.slice(0, 40)

  const handleDelete = async (id: string) => {
    if (!onDelete) return
    setDeletingId(id)
    try {
      await onDelete(id)
    } finally {
      setDeletingId(null)
    }
  }

  if (loading) {
    return <div className="space-y-1 p-3">{Array.from({ length: 7 }).map((_, index) => <Skeleton key={index} className="h-12" />)}</div>
  }

  if (rows.length === 0) {
    return <p className="p-8 text-center text-xs text-muted-foreground">No signal packets inside window.</p>
  }

  return (
    <div className="max-h-[610px] overflow-y-auto">
      {rows.map((point, index) => (
        <div key={point.event_id} className={cn(
          'grid grid-cols-[38px_minmax(0,1fr)_auto] items-center gap-3 border-b px-3 py-3 transition-colors',
          selectedId === point.event_id ? 'bg-primary/10' : 'hover:bg-muted/35',
        )}>
          <span className="mono-data font-mono text-[9px] text-muted-foreground">{String(index + 1).padStart(2, '0')}</span>
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <strong className="mono-data font-mono text-xs">{format(new Date(point.recorded_at), 'HH:mm:ss.SSS')}</strong>
              <span className="font-mono text-[8px] uppercase text-primary">{point.activity_type || 'RAW'}</span>
            </div>
            <p className="mono-data mt-1 truncate font-mono text-[9px] text-muted-foreground">
              {point.lat.toFixed(5)} / {point.lon.toFixed(5)} / {(point.speed_mps ?? 0).toFixed(2)} m/s
            </p>
          </div>
          <div className="flex">
            <Button variant="ghost" size="icon" onClick={() => onSelect?.(point.event_id)} aria-label="Focus point on map" className="size-8 text-muted-foreground hover:text-primary">
              <Crosshair className="size-3.5" />
            </Button>
            {onDelete ? (
              <Button variant="ghost" size="icon" onClick={() => handleDelete(point.event_id)} disabled={deletingId === point.event_id} aria-label="Delete tracking point" className="size-8 text-muted-foreground hover:text-destructive">
                <Trash2 className="size-3.5" />
              </Button>
            ) : null}
          </div>
        </div>
      ))}
    </div>
  )
}
