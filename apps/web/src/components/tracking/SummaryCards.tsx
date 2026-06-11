import { format } from 'date-fns'
import { Activity, Clock, Gauge, MapPin, Timer, TrendingUp } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import type { TrackingSummary } from '@/features/tracking/trackingApi'

interface Props {
  summary?: TrackingSummary
  loading?: boolean
}

export function SummaryCards({ summary, loading }: Props) {
  if (loading) {
    return (
      <div className="grid grid-cols-2 lg:grid-cols-3 gap-3 p-4">
        {Array.from({ length: 6 }).map((_, i) => (
          <Skeleton key={i} className="h-20 rounded-lg" />
        ))}
      </div>
    )
  }

  const fmtSpeed = (v: number | null | undefined) =>
    v != null ? `${v.toFixed(1)} m/s` : '—'
  const fmtTime = (v: string | null | undefined) =>
    v ? format(new Date(v), 'HH:mm:ss') : '—'
  const fmtDuration = (sec: number) => {
    if (sec === 0) return '—'
    const h = Math.floor(sec / 3600)
    const m = Math.floor((sec % 3600) / 60)
    const s = sec % 60
    return h > 0 ? `${h}h ${m}m` : m > 0 ? `${m}m ${s}s` : `${s}s`
  }

  return (
    <div className="grid grid-cols-2 lg:grid-cols-3 gap-3 p-4">
      <StatCard icon={<MapPin className="h-4 w-4" />} label="Points" value={String(summary?.points_count ?? 0)} />
      <StatCard icon={<Timer className="h-4 w-4" />} label="Duration" value={fmtDuration(summary?.duration_sec ?? 0)} />
      <StatCard icon={<Gauge className="h-4 w-4" />} label="Avg Speed" value={fmtSpeed(summary?.avg_speed_mps)} />
      <StatCard icon={<TrendingUp className="h-4 w-4" />} label="Max Speed" value={fmtSpeed(summary?.max_speed_mps)} />
      <StatCard icon={<Clock className="h-4 w-4" />} label="First Point" value={fmtTime(summary?.first_recorded_at)} />
      <StatCard icon={<Activity className="h-4 w-4" />} label="Last Point" value={fmtTime(summary?.last_recorded_at)} />
    </div>
  )
}

function StatCard({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <Card>
      <CardHeader className="p-3 pb-1">
        <CardTitle className="text-xs font-medium text-muted-foreground flex items-center gap-1.5">
          {icon}
          {label}
        </CardTitle>
      </CardHeader>
      <CardContent className="p-3 pt-0">
        <div className="text-xl font-bold">{value}</div>
      </CardContent>
    </Card>
  )
}
