import { format } from 'date-fns'
import { Activity, Clock, Gauge, MapPin, Timer, TrendingUp } from 'lucide-react'
import { MetricCard } from '@/components/analytics/AnalyticsUi'
import { Skeleton } from '@/components/ui/skeleton'
import type { TrackingSummary } from '@/features/tracking/trackingApi'

interface Props {
  summary?: TrackingSummary
  loading?: boolean
}

function fmtTime(value: string | null | undefined): string {
  if (!value) return 'Not available'
  const date = new Date(value)
  const isToday = date.toDateString() === new Date().toDateString()
  return isToday ? format(date, 'HH:mm:ss') : format(date, 'MMM d, HH:mm')
}

function fmtDuration(seconds: number): string {
  if (seconds === 0) return '0s'
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remaining = seconds % 60
  return hours > 0 ? `${hours}h ${minutes}m` : minutes > 0 ? `${minutes}m ${remaining}s` : `${remaining}s`
}

export function SummaryCards({ summary, loading }: Props) {
  if (loading) {
    return (
      <div className="grid shrink-0 grid-cols-2 gap-3 p-4 sm:grid-cols-3 xl:grid-cols-6">
        {Array.from({ length: 6 }).map((_, index) => (
          <Skeleton key={index} className="h-32 rounded-[14px]" />
        ))}
      </div>
    )
  }

  return (
    <div className="grid shrink-0 grid-cols-2 gap-3 p-4 sm:grid-cols-3 xl:grid-cols-6">
      <MetricCard icon={<MapPin className="size-4" />} label="GPS points" value={String(summary?.points_count ?? 0)} detail="Captured samples" />
      <MetricCard icon={<Timer className="size-4" />} label="Duration" value={fmtDuration(summary?.duration_sec ?? 0)} detail="Observed window" />
      <MetricCard icon={<Gauge className="size-4" />} label="Avg speed" value={`${summary?.avg_speed_mps ?? 0} m/s`} detail="Moving average" accent="cyan" />
      <MetricCard icon={<TrendingUp className="size-4" />} label="Top speed" value={`${summary?.max_speed_mps ?? 0} m/s`} detail="Peak detected speed" accent="amber" />
      <MetricCard icon={<Clock className="size-4" />} label="First point" value={fmtTime(summary?.first_recorded_at)} detail="Window start" />
      <MetricCard icon={<Activity className="size-4" />} label="Last point" value={fmtTime(summary?.last_recorded_at)} detail="Latest signal" />
    </div>
  )
}
