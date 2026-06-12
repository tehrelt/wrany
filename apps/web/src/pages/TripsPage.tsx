import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AppLayout } from '@/components/layout/AppLayout'
import { TripMap } from '@/components/map/TripMap'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useAuth } from '@/features/auth/useAuth'
import {
  listTrips,
  getTripPoints,
  formatDuration,
  formatDistance,
  type Trip,
  type TripPoint,
} from '@/features/trips/tripsApi'

interface Props {
  onLogout: () => void
}

function TripCard({
  trip,
  selected,
  onClick,
}: {
  trip: Trip
  selected: boolean
  onClick: () => void
}) {
  const isActive = trip.status === 'TRIP_ACTIVE'
  const date = new Date(trip.started_at)
  const dateStr = date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  const timeStr = date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })

  return (
    <button
      onClick={onClick}
      className={[
        'w-full text-left rounded-lg border p-3 transition-colors hover:bg-accent',
        selected ? 'border-primary bg-accent' : 'border-border',
      ].join(' ')}
    >
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs text-muted-foreground">{dateStr} · {timeStr}</span>
        {isActive && (
          <Badge variant="default" className="text-xs">Active</Badge>
        )}
      </div>
      <div className="flex gap-4 text-sm font-medium">
        <span>{formatDistance(trip.distance_m)}</span>
        <span className="text-muted-foreground">{formatDuration(trip.duration_sec)}</span>
      </div>
    </button>
  )
}

export function TripsPage({ onLogout }: Props) {
  const { token } = useAuth()
  const [status, setStatus] = useState<string>('all')
  const [selectedTrip, setSelectedTrip] = useState<Trip | null>(null)

  const tripsQuery = useQuery({
    queryKey: ['trips', status],
    queryFn: () => listTrips({ status: status === 'all' ? undefined : status, limit: 50 }),
  })

  const pointsQuery = useQuery<{ items: TripPoint[] }>({
    queryKey: ['trip-points', selectedTrip?.id],
    queryFn: () => getTripPoints(selectedTrip!.id, { limit: 5000 }),
    enabled: !!selectedTrip,
  })

  let userEmail = ''
  if (token) {
    try {
      userEmail = (JSON.parse(atob(token.split('.')[1])) as { sub?: string }).sub ?? ''
    } catch {
      // ignore
    }
  }

  const trips = tripsQuery.data?.items ?? []
  const tripPoints = pointsQuery.data?.items ?? []

  const sidebar = (
    <div className="flex flex-col gap-3 h-full">
      <div className="flex items-center justify-between shrink-0">
        <span className="text-sm font-medium">Trips</span>
        <Select value={status} onValueChange={setStatus}>
          <SelectTrigger className="h-7 w-28 text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All</SelectItem>
            <SelectItem value="TRIP_COMPLETED">Completed</SelectItem>
            <SelectItem value="TRIP_ACTIVE">Active</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {tripsQuery.isLoading && (
        <div className="flex flex-col gap-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-lg" />
          ))}
        </div>
      )}

      {tripsQuery.isError && (
        <Alert variant="destructive">
          <AlertDescription>Could not load trips. Try again.</AlertDescription>
        </Alert>
      )}

      {!tripsQuery.isLoading && !tripsQuery.isError && trips.length === 0 && (
        <div className="flex flex-col items-center gap-2 py-10 px-2 text-center">
          <p className="text-sm font-medium text-muted-foreground">No trips detected yet</p>
          <p className="text-xs text-muted-foreground leading-relaxed">
            The tracker automatically detects trips when you move. Start tracking to see results here.
          </p>
        </div>
      )}

      <div className="flex flex-col gap-2 overflow-y-auto flex-1 min-h-0">
        {trips.map(trip => (
          <TripCard
            key={trip.id}
            trip={trip}
            selected={selectedTrip?.id === trip.id}
            onClick={() => setSelectedTrip(trip)}
          />
        ))}

        {tripsQuery.data?.next_cursor && (
          <Button
            variant="outline"
            size="sm"
            className="mt-1"
            onClick={() => {/* pagination not needed for MVP */}}
          >
            Load more
          </Button>
        )}
      </div>
    </div>
  )

  return (
    <AppLayout userEmail={userEmail} onLogout={onLogout} sidebar={sidebar}>
      <div className="flex-1 min-h-0">
        <TripMap
          points={tripPoints}
          loading={pointsQuery.isFetching}
        />
      </div>

      {selectedTrip && (
        <div className="shrink-0 border-t px-4 py-2 flex gap-6 text-sm text-muted-foreground bg-background">
          <span>
            <span className="font-medium text-foreground">{formatDistance(selectedTrip.distance_m)}</span>
            {' '}distance
          </span>
          <span>
            <span className="font-medium text-foreground">{formatDuration(selectedTrip.duration_sec)}</span>
            {' '}duration
          </span>
          <span>
            <span className="font-medium text-foreground">{selectedTrip.points_count}</span>
            {' '}points
          </span>
        </div>
      )}
    </AppLayout>
  )
}
