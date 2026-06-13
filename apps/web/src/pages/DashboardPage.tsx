import { useCallback, useState } from 'react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Activity, Crosshair, Gauge, Radio, Satellite, Timer } from 'lucide-react'
import { AppLayout } from '@/components/layout/AppLayout'
import { RouteMap } from '@/components/map/RouteMap'
import { PointsTable } from '@/components/tracking/PointsTable'
import {
  defaultFrom,
  defaultTo,
  TrackingFilters,
} from '@/features/tracking/TrackingFilters'
import { useTrackSettings } from '@/features/tracking/useTrackSettings'
import { ErrorState, MetricCard, SectionHeader } from '@/components/analytics/AnalyticsUi'
import {
  deletePoint,
  getPoints,
  getSummary,
  getTrack,
  type PointsResponse,
  type TrackingSummary,
  type TrackSegment,
} from '@/features/tracking/trackingApi'
import { useAuth } from '@/features/auth/useAuth'

interface Props {
  onLogout: () => void
}

function getUserEmail(token: string | null): string {
  if (!token) return ''
  try {
    return (JSON.parse(atob(token.split('.')[1])) as { sub?: string }).sub ?? ''
  } catch {
    return ''
  }
}

function formatDuration(seconds = 0): string {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remainder = seconds % 60
  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(remainder).padStart(2, '0')}`
}

export function DashboardPage({ onLogout }: Props) {
  const { token } = useAuth()
  const [deviceId, setDeviceId] = useState('')
  const [from, setFrom] = useState(defaultFrom)
  const [to, setTo] = useState(defaultTo)
  const [revision, setRevision] = useState(0)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [trackSettings, setTrackSettings] = useTrackSettings()

  const filter = { device_id: deviceId || undefined, from, to }
  const trackQuery = useQuery<TrackSegment[]>({
    queryKey: ['track', filter, trackSettings, revision],
    queryFn: () => getTrack({
      ...filter,
      speed_threshold_mps: trackSettings.speedThresholdMps,
      min_stay_sec: trackSettings.minStaySec,
      min_move_sec: trackSettings.minMoveSec,
    }),
    enabled: Boolean(from && to),
    placeholderData: keepPreviousData,
  })
  const pointsQuery = useQuery<PointsResponse>({
    queryKey: ['points', filter, revision],
    queryFn: () => getPoints({ ...filter, limit: 1000 }),
    enabled: Boolean(from && to),
  })
  const summaryQuery = useQuery<TrackingSummary>({
    queryKey: ['summary', filter, revision],
    queryFn: () => getSummary(filter),
    enabled: Boolean(from && to),
  })

  const refresh = useCallback(() => setRevision(value => value + 1), [])
  const handleDelete = useCallback(async (id: string) => {
    await deletePoint(id)
    refresh()
  }, [refresh])

  const segments = trackQuery.data ?? []
  const points = pointsQuery.data?.items ?? []
  const summary = summaryQuery.data
  const selectedSegment = selectedId ? segments.find(segment => segment.event_id === selectedId) : undefined
  const mapPoints = segments.map(segment => ({
    lat: segment.lat,
    lon: segment.lon,
    recordedAt: segment.recorded_at,
  }))
  const loading = trackQuery.isLoading || summaryQuery.isLoading
  const filters = (
    <TrackingFilters
      deviceId={deviceId}
      from={from}
      to={to}
      loading={loading}
      settings={trackSettings}
      onDeviceChange={setDeviceId}
      onFromChange={setFrom}
      onToChange={setTo}
      onSettingsChange={setTrackSettings}
      onRefresh={refresh}
    />
  )

  return (
    <AppLayout userEmail={getUserEmail(token)} onLogout={onLogout} sidebar={filters}>
      <div className="h-full overflow-y-auto">
        {(trackQuery.isError || pointsQuery.isError || summaryQuery.isError) ? (
          <ErrorState title="Telemetry link lost" description="Check device and selected time window." onRetry={refresh} />
        ) : null}

        <div className="grid gap-px bg-border xl:grid-cols-[minmax(0,1fr)_360px]">
          <section className="min-w-0 bg-background p-4 sm:p-6">
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-5">
              <MetricCard icon={<Satellite className="size-3.5" />} label="Samples" value={String(summary?.points_count ?? 0)} detail="GPS packets" />
              <MetricCard icon={<Timer className="size-3.5" />} label="Session time" value={formatDuration(summary?.duration_sec)} detail="Observed window" />
              <MetricCard icon={<Gauge className="size-3.5" />} label="Average" value={`${(summary?.avg_speed_mps ?? 0).toFixed(2)}`} detail="Meters / second" accent="cyan" />
              <MetricCard icon={<Activity className="size-3.5" />} label="Maximum" value={`${(summary?.max_speed_mps ?? 0).toFixed(2)}`} detail="Peak velocity" accent="amber" />
              <MetricCard icon={<Crosshair className="size-3.5" />} label="Segments" value={String(segments.length)} detail="Processed trace" />
            </div>

            <section className="race-panel mt-4 overflow-hidden">
              <div className="flex items-center justify-between border-b px-4 py-3">
                <SectionHeader title="Track trace" description={`${mapPoints.length} processed route nodes`} />
                <div className="flex items-center gap-2 font-mono text-[9px] uppercase text-muted-foreground">
                  <Radio className="size-3.5 text-primary" /> Auto follow
                </div>
              </div>
              <div className="h-[420px] lg:h-[520px]">
                <RouteMap
                  points={mapPoints}
                  selectedPoint={selectedSegment ? {
                    lat: selectedSegment.lat,
                    lon: selectedSegment.lon,
                    recordedAt: selectedSegment.recorded_at,
                  } : null}
                />
              </div>
            </section>
          </section>

          <aside className="bg-background p-4 sm:p-6">
            <div className="race-panel p-4 lg:hidden">
              <SectionHeader title="Strategy controls" description="Telemetry processing window" />
              <div className="mt-5">{filters}</div>
            </div>
            <div className="race-panel mt-4 overflow-hidden lg:mt-0">
              <div className="border-b px-4 py-3">
                <SectionHeader title="Signal feed" description="Latest raw positioning packets" />
              </div>
              <PointsTable
                points={points}
                loading={pointsQuery.isLoading}
                selectedId={selectedId}
                onSelect={setSelectedId}
                onDelete={handleDelete}
              />
            </div>
          </aside>
        </div>
      </div>
    </AppLayout>
  )
}
