import { format } from 'date-fns'
import { Activity, Clock, Gauge, MapPin, Timer, TrendingUp } from 'lucide-react'
import { Skeleton } from '@/components/ui/skeleton'
import type { TrackingSummary } from '@/features/tracking/trackingApi'

interface Props {
  summary?: TrackingSummary
  loading?: boolean
}

function mpsToKmh(mps: number | null | undefined): string {
  if (mps == null) return '—'
  return `${(mps * 3.6).toFixed(1)} km/h`
}

function fmtTime(v: string | null | undefined): string {
  if (!v) return '—'
  const d = new Date(v)
  const today = new Date()
  const isToday = d.toDateString() === today.toDateString()
  return isToday ? format(d, 'HH:mm:ss') : format(d, 'MMM d, HH:mm')
}

function fmtDuration(sec: number): string {
  if (sec === 0) return '—'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = sec % 60
  return h > 0 ? `${h}h ${m}m` : m > 0 ? `${m}m ${s}s` : `${s}s`
}

export function SummaryCards({ summary, loading }: Props) {
  if (loading) {
    return (
      <div className="grid grid-cols-2 lg:grid-cols-3 gap-3 p-4 shrink-0">
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton key={i} className="h-20 rounded-lg" />
        ))}
      </div>
    )
  }

  return (
    <div className="grid grid-cols-2 lg:grid-cols-3 gap-3 p-4 shrink-0">
      <StatCard
        icon={<MapPin className="h-4 w-4" />}
        label="GPS points"
        value={summary ? String(summary.points_count) : '—'}
      />
      <StatCard
        icon={<Timer className="h-4 w-4" />}
        label="Duration"
        value={fmtDuration(summary?.duration_sec ?? 0)}
      />
      <StatCard
        icon={<Gauge className="h-4 w-4" />}
        label="Avg speed"
        value={mpsToKmh(summary?.avg_speed_mps)}
      />
      <StatCard
        icon={<TrendingUp className="h-4 w-4" />}
        label="Top speed"
        value={mpsToKmh(summary?.max_speed_mps)}
      />
      <StatCard
        icon={<Clock className="h-4 w-4" />}
        label="First point"
        value={fmtTime(summary?.first_recorded_at)}
      />
      <StatCard
        icon={<Activity className="h-4 w-4" />}
        label="Last point"
        value={fmtTime(summary?.last_recorded_at)}
      />
    </div>
  )
}

function StatCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="rounded-lg bg-muted/40 p-3 flex flex-col gap-1">
      <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
        {icon}
        <span>{label}</span>
      </div>
      <div className="text-xl font-semibold tabular-nums">{value}</div>
    </div>
  )
}
