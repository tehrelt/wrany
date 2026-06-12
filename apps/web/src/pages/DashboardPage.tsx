import { useState, useCallback } from 'react'
import { useQuery, keepPreviousData } from '@tanstack/react-query'
import { AppLayout } from '@/components/layout/AppLayout'
import { RouteMap } from '@/components/map/RouteMap'
import { SummaryCards } from '@/components/tracking/SummaryCards'
import { PointsTable } from '@/components/tracking/PointsTable'
import { TrackingFilters, defaultFrom, defaultTo, defaultTrackSettings, type TrackDisplaySettings } from '@/features/tracking/TrackingFilters'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { getPoints, getSummary, deletePoint, getTrack, type PointsResponse, type TrackingSummary, type TrackSegment } from '@/features/tracking/trackingApi'
import { useAuth } from '@/features/auth/useAuth'

interface Props {
  onLogout: () => void
}

export function DashboardPage({ onLogout }: Props) {
  const { token } = useAuth()
  const [deviceId, setDeviceId] = useState('')
  const [from, setFrom] = useState(defaultFrom)
  const [to, setTo] = useState(defaultTo)
  const [rev, setRev] = useState(0)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [trackSettings, setTrackSettings] = useState<TrackDisplaySettings>(defaultTrackSettings)

  const filter = {
    device_id: deviceId || undefined,
    from,
    to,
  }

  const trackQuery = useQuery<TrackSegment[]>({
    queryKey: ['track', filter, trackSettings, rev],
    queryFn: () => getTrack({
      ...filter,
      speed_threshold_mps: trackSettings.speedThresholdMps,
      min_stay_sec: trackSettings.minStaySec,
      min_move_sec: trackSettings.minMoveSec,
    }),
    enabled: !!filter.from && !!filter.to,
    placeholderData: keepPreviousData,
  })

  const pointsQuery = useQuery<PointsResponse>({
    queryKey: ['points', filter, rev],
    queryFn: () => getPoints({ ...filter, limit: 1000 }),
    enabled: !!filter.from && !!filter.to,
  })

  const summaryQuery = useQuery<TrackingSummary>({
    queryKey: ['summary', filter, rev],
    queryFn: () => getSummary(filter),
    enabled: !!filter.from && !!filter.to,
  })

  const refresh = useCallback(() => setRev(r => r + 1), [])

  const handleDelete = useCallback(async (id: string) => {
    await deletePoint(id)
    refresh()
  }, [refresh])

  const segments = trackQuery.data ?? []
  const points = pointsQuery.data?.items ?? []
  const mapPoints = segments.map(segment => ({
    lat: segment.lat,
    lon: segment.lon,
    recordedAt: segment.recorded_at,
  }))
  const selectedSegment = selectedId
    ? segments.find(segment => segment.event_id === selectedId)
    : undefined
  const isLoading = trackQuery.isLoading || summaryQuery.isLoading

  let userEmail = ''
  if (token) {
    try {
      userEmail = (JSON.parse(atob(token.split('.')[1])) as { sub?: string }).sub ?? ''
    } catch {
      // ignore malformed token
    }
  }

  return (
    <AppLayout
      userEmail={userEmail}
      onLogout={onLogout}
      sidebar={
        <TrackingFilters
          deviceId={deviceId}
          from={from}
          to={to}
          loading={isLoading}
          settings={trackSettings}
          onDeviceChange={setDeviceId}
          onFromChange={setFrom}
          onToChange={setTo}
          onSettingsChange={setTrackSettings}
          onRefresh={refresh}
        />
      }
    >
      {(trackQuery.isError || pointsQuery.isError || summaryQuery.isError) && (
        <Alert variant="destructive" className="m-4">
          <AlertDescription>Failed to load tracking data. Check filters or try again.</AlertDescription>
        </Alert>
      )}

      <SummaryCards summary={summaryQuery.data} loading={summaryQuery.isLoading} />

      <div className="flex-1 min-h-0">
        <RouteMap
          points={mapPoints}
          selectedPoint={
            selectedSegment
              ? {
                  lat: selectedSegment.lat,
                  lon: selectedSegment.lon,
                  recordedAt: selectedSegment.recorded_at,
                }
              : null
          }
        />
      </div>

      <PointsTable
        points={points}
        loading={pointsQuery.isLoading}
        selectedId={selectedId}
        onSelect={setSelectedId}
        onDelete={handleDelete}
      />
    </AppLayout>
  )
}
