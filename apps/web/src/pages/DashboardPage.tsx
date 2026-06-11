import { useState, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppLayout } from '@/components/layout/AppLayout'
import { TrackingMap } from '@/components/map/TrackingMap'
import { SummaryCards } from '@/components/tracking/SummaryCards'
import { PointsTable } from '@/components/tracking/PointsTable'
import { TrackingFilters, defaultFrom, defaultTo } from '@/features/tracking/TrackingFilters'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { getPoints, getSummary, deletePoint, type PointsResponse, type TrackingSummary } from '@/features/tracking/trackingApi'
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

  const filter = {
    device_id: deviceId || undefined,
    from,
    to,
  }

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

  const points = pointsQuery.data?.items ?? []
  const isLoading = pointsQuery.isLoading || summaryQuery.isLoading

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
          onDeviceChange={setDeviceId}
          onFromChange={setFrom}
          onToChange={setTo}
          onRefresh={refresh}
        />
      }
    >
      {(pointsQuery.isError || summaryQuery.isError) && (
        <Alert variant="destructive" className="m-4">
          <AlertDescription>Failed to load tracking data. Check filters or try again.</AlertDescription>
        </Alert>
      )}

      <SummaryCards summary={summaryQuery.data} loading={summaryQuery.isLoading} />

      <div className="flex-1 min-h-0">
        <TrackingMap
          points={points}
          loading={pointsQuery.isLoading}
          selectedId={selectedId}
          onSelect={setSelectedId}
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
