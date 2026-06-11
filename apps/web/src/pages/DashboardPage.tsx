import { useState, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { subHours, formatISO } from 'date-fns'
import { DeviceSelector } from '../features/devices/DeviceSelector'
import { DateRangePicker } from '../features/tracking/DateRangePicker'
import { getPoints, getSummary } from '../features/tracking/trackingApi'
import { MapView } from '../components/MapView'
import { PointsTable } from '../components/PointsTable'
import { SummaryCards } from '../components/SummaryCards'
import { LoadingState } from '../components/LoadingState'
import { ErrorState } from '../components/ErrorState'
import { EmptyState } from '../components/EmptyState'

function toLocal(iso: string): string {
  return iso.replace('Z', '')
}

function toISO(local: string): string {
  return local ? new Date(local).toISOString() : ''
}

interface Props {
  onLogout: () => void
}

export function DashboardPage({ onLogout }: Props) {
  const [deviceId, setDeviceId] = useState('')
  const [from, setFrom] = useState(() => toLocal(formatISO(subHours(new Date(), 24))))
  const [to, setTo] = useState(() => toLocal(formatISO(new Date())))
  const [queryKey, setQueryKey] = useState(0)

  const filter = {
    device_id: deviceId || undefined,
    from: toISO(from),
    to: toISO(to),
  }

  const pointsQuery = useQuery({
    queryKey: ['points', filter, queryKey],
    queryFn: () => getPoints({ ...filter, limit: 1000 }),
    enabled: !!filter.from && !!filter.to,
  })

  const summaryQuery = useQuery({
    queryKey: ['summary', filter, queryKey],
    queryFn: () => getSummary(filter),
    enabled: !!filter.from && !!filter.to,
  })

  const refresh = useCallback(() => setQueryKey((k) => k + 1), [])

  const points = pointsQuery.data?.items ?? []

  return (
    <div style={{ padding: 16, fontFamily: 'system-ui, sans-serif' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <h1 style={{ margin: 0, fontSize: 20 }}>WR any% — Tracking Viewer</h1>
        <button onClick={onLogout} style={{ padding: '4px 12px', cursor: 'pointer' }}>
          Logout
        </button>
      </div>

      <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', marginBottom: 16, alignItems: 'center' }}>
        <DeviceSelector value={deviceId} onChange={setDeviceId} />
        <DateRangePicker from={from} to={to} onFromChange={setFrom} onToChange={setTo} onRefresh={refresh} />
      </div>

      {summaryQuery.isLoading && <LoadingState message="Loading summary…" />}
      {summaryQuery.isError && <ErrorState message="Failed to load summary" />}
      {summaryQuery.data && <SummaryCards summary={summaryQuery.data} />}

      <div style={{ marginTop: 16 }}>
        {pointsQuery.isLoading && <LoadingState message="Loading points…" />}
        {pointsQuery.isError && <ErrorState message="Failed to load points" />}
        {!pointsQuery.isLoading && !pointsQuery.isError && points.length === 0 && (
          <EmptyState />
        )}
        {points.length > 0 && (
          <>
            <div style={{ marginBottom: 16 }}>
              <MapView points={points} />
            </div>
            <PointsTable points={points} />
          </>
        )}
      </div>
    </div>
  )
}
