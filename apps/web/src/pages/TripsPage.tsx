import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Clock3, Gauge, MapPin, Route as RouteIcon, Satellite, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { AppLayout } from '@/components/layout/AppLayout'
import { RouteMap } from '@/components/map/RouteMap'
import {
  ActivityTypeBadge,
  EmptyState,
  ErrorState,
  LoadingSkeleton,
  MetricCard,
  SectionHeader,
  StatusBadge,
} from '@/components/analytics/AnalyticsUi'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useAuth } from '@/features/auth/useAuth'
import { deleteTrip, formatDistance, formatDuration, getTripPoints, listTrips, type Trip, type TripPoint } from '@/features/trips/tripsApi'
import { useSelectedId } from '@/lib/useSelectedId'
import { cn } from '@/lib/utils'

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

function formatDate(value: string): string {
  return new Date(value).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

function SessionEntry({ trip, position, selected, onClick }: { trip: Trip; position: number; selected: boolean; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} aria-pressed={selected} className={cn(
      'grid w-full cursor-pointer grid-cols-[42px_minmax(0,1fr)_auto] items-center gap-3 border-b px-3 py-4 text-left transition-colors',
      selected ? 'border-l-2 border-l-primary bg-primary/10' : 'border-l-2 border-l-transparent hover:bg-sidebar-accent',
    )}>
      <span className="mono-data font-mono text-lg font-black text-muted-foreground">{String(position).padStart(2, '0')}</span>
      <span className="min-w-0">
        <strong className="block truncate text-xs font-bold uppercase">{formatDate(trip.started_at)}</strong>
        <span className="mt-1 block font-mono text-[9px] text-muted-foreground">{formatDistance(trip.distance_m)} / {formatDuration(trip.duration_sec)}</span>
      </span>
      <StatusBadge tone={trip.status === 'TRIP_ACTIVE' ? 'warning' : 'success'}>{trip.status === 'TRIP_ACTIVE' ? 'Live' : 'Done'}</StatusBadge>
    </button>
  )
}

export function TripsPage({ onLogout }: Props) {
  const { token } = useAuth()
  const queryClient = useQueryClient()
  const [status, setStatus] = useState('all')
  const [selectedId, setSelectedId] = useSelectedId('trip')
  const tripsQuery = useQuery({
    queryKey: ['trips', status],
    queryFn: () => listTrips({ status: status === 'all' ? undefined : status, limit: 50 }),
  })

  const trips = tripsQuery.data?.items ?? []
  const selectedTrip = trips.find(trip => trip.id === selectedId) ?? null

  const deleteMutation = useMutation({
    mutationFn: (tripId: string) => deleteTrip(tripId),
    onSuccess: () => {
      toast.success('Session deleted')
      setSelectedId(null)
      void queryClient.invalidateQueries({ queryKey: ['trips'] })
      void queryClient.invalidateQueries({ queryKey: ['routes'] })
    },
  })

  function handleDelete(trip: Trip) {
    if (window.confirm('Delete this session permanently? Its trace and route matches will be removed.')) {
      deleteMutation.mutate(trip.id)
    }
  }
  const pointsQuery = useQuery<{ items: TripPoint[] }>({
    queryKey: ['trip-points', selectedTrip?.id],
    queryFn: () => getTripPoints(selectedTrip!.id, { limit: 5000 }),
    enabled: Boolean(selectedTrip),
  })

  const points = pointsQuery.data?.items ?? []
  const averageSpeed = selectedTrip?.duration_sec ? selectedTrip.distance_m / selectedTrip.duration_sec : 0
  const sessionList = (
    <div>
      <div className="mb-3">
        <SectionHeader
          title="Session log"
          description={`${trips.length} detected runs`}
          action={
            <Select value={status} onValueChange={setStatus}>
              <SelectTrigger className="h-8 w-28 font-mono text-[9px]" aria-label="Filter sessions">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">All runs</SelectItem>
                <SelectItem value="TRIP_COMPLETED">Finished</SelectItem>
                <SelectItem value="TRIP_ACTIVE">Live</SelectItem>
              </SelectContent>
            </Select>
          }
        />
      </div>
      <div className="border">
        {tripsQuery.isLoading ? <div className="p-3"><LoadingSkeleton rows={5} /></div> : null}
        {tripsQuery.isError ? <ErrorState title="Log unavailable" description="Sessions failed to load." onRetry={() => tripsQuery.refetch()} /> : null}
        {trips.map((trip, index) => <SessionEntry key={trip.id} trip={trip} position={index + 1} selected={selectedTrip?.id === trip.id} onClick={() => setSelectedId(trip.id)} />)}
      </div>
    </div>
  )

  return (
    <AppLayout userEmail={getUserEmail(token)} onLogout={onLogout} sidebar={sessionList}>
      <div className="h-full overflow-y-auto">
        <div className="p-4 sm:p-6 lg:p-8">
          <div className="race-panel mb-4 p-4 lg:hidden">{sessionList}</div>
          {!selectedTrip ? (
            <EmptyState title="Select a session" description="Choose an automatically detected run to inspect its telemetry trace." />
          ) : (
            <div className="space-y-4">
              <div className="flex items-center justify-between gap-4">
                <SectionHeader title={formatDate(selectedTrip.started_at)} description="Detected session" />
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleDelete(selectedTrip)}
                  disabled={deleteMutation.isPending}
                  className="gap-2 border-destructive/40 text-destructive hover:bg-destructive/10 hover:text-destructive"
                >
                  <Trash2 className="size-3.5" /> Delete session
                </Button>
              </div>
              <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
                <MetricCard icon={<RouteIcon className="size-3.5" />} label="Distance" value={formatDistance(selectedTrip.distance_m)} detail="Detected route length" />
                <MetricCard icon={<Clock3 className="size-3.5" />} label="Elapsed" value={formatDuration(selectedTrip.duration_sec)} detail="Start to finish" />
                <MetricCard icon={<Gauge className="size-3.5" />} label="Average pace" value={averageSpeed.toFixed(2)} detail="Meters / second" accent="cyan" />
                <MetricCard icon={<Satellite className="size-3.5" />} label="Samples" value={String(selectedTrip.points_count)} detail="Position packets" accent="amber" />
              </div>

              <div className="grid gap-4 xl:grid-cols-[minmax(0,1.5fr)_380px]">
                <section className="race-panel overflow-hidden">
                  <div className="flex items-center justify-between border-b px-4 py-3">
                    <SectionHeader title="Session trace" description={`${points.length} GPS samples rendered`} />
                    <StatusBadge tone="info">Route model</StatusBadge>
                  </div>
                  {pointsQuery.isError ? (
                    <ErrorState title="Trace unavailable" description="Position samples failed to load." onRetry={() => pointsQuery.refetch()} />
                  ) : (
                    <div className="h-[480px]">
                      <RouteMap
                        points={points}
                        startPoint={{ lat: selectedTrip.start_lat, lon: selectedTrip.start_lon }}
                        finishPoint={selectedTrip.end_lat != null && selectedTrip.end_lon != null ? { lat: selectedTrip.end_lat, lon: selectedTrip.end_lon } : undefined}
                      />
                    </div>
                  )}
                </section>

                <div className="space-y-4">
                  <section className="race-panel p-4">
                    <SectionHeader title="Timing record" description="Detection timestamps" />
                    <dl className="mt-5 space-y-4">
                      <div className="border-b pb-3">
                        <dt className="font-mono text-[9px] uppercase text-muted-foreground">Run start</dt>
                        <dd className="mono-data mt-1 font-mono text-sm font-bold">{formatDate(selectedTrip.started_at)}</dd>
                      </div>
                      <div className="border-b pb-3">
                        <dt className="font-mono text-[9px] uppercase text-muted-foreground">Run finish</dt>
                        <dd className="mono-data mt-1 font-mono text-sm font-bold">{selectedTrip.ended_at ? formatDate(selectedTrip.ended_at) : 'IN PROGRESS'}</dd>
                      </div>
                      <div>
                        <dt className="font-mono text-[9px] uppercase text-muted-foreground">Classification</dt>
                        <dd className="mt-2"><ActivityTypeBadge /></dd>
                      </div>
                    </dl>
                  </section>
                  <section className="race-panel p-4">
                    <SectionHeader title="Coordinates" description="Detected timing gates" />
                    <div className="mt-5 space-y-4 font-mono text-[10px]">
                      <div className="flex items-start gap-3"><MapPin className="mt-0.5 size-4 text-primary" /><span><strong className="block uppercase">Start gate</strong><span className="text-muted-foreground">{selectedTrip.start_lat.toFixed(5)} / {selectedTrip.start_lon.toFixed(5)}</span></span></div>
                      <div className="flex items-start gap-3"><MapPin className="mt-0.5 size-4 text-accent" /><span><strong className="block uppercase">Finish gate</strong><span className="text-muted-foreground">{selectedTrip.end_lat?.toFixed(5) ?? '--'} / {selectedTrip.end_lon?.toFixed(5) ?? '--'}</span></span></div>
                    </div>
                  </section>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </AppLayout>
  )
}
