import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Clock3, Gauge, Route as RouteIcon, Trash2, Trophy } from 'lucide-react'
import { toast } from 'sonner'
import { AppLayout } from '@/components/layout/AppLayout'
import { RouteMap } from '@/components/map/RouteMap'
import {
  ComparisonDelta,
  EmptyState,
  ErrorState,
  LoadingSkeleton,
  MetricCard,
  RouteTypeBadge,
  SectionHeader,
  StatusBadge,
} from '@/components/analytics/AnalyticsUi'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/features/auth/useAuth'
import {
  deleteRoute,
  formatDistance,
  formatDuration,
  getRouteAttempts,
  getRoutePoints,
  getRouteResults,
  listRoutes,
  type Route,
  type TripAttemptItem,
} from '@/features/routes/routesApi'
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

function endpointDistance(route: Route): number {
  const radius = 6_371_000
  const lat1 = route.start_lat * Math.PI / 180
  const lat2 = route.end_lat * Math.PI / 180
  const deltaLat = (route.end_lat - route.start_lat) * Math.PI / 180
  const deltaLon = (route.end_lon - route.start_lon) * Math.PI / 180
  const value = Math.sin(deltaLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(deltaLon / 2) ** 2
  return radius * 2 * Math.atan2(Math.sqrt(value), Math.sqrt(1 - value))
}

function isCircuit(route: Route): boolean {
  return endpointDistance(route) <= 150
}

function routeName(route: Route): string {
  return route.name?.trim() || `Circuit ${route.id.slice(0, 6).toUpperCase()}`
}

function formatDate(value?: string): string {
  if (!value) return 'No timestamp'
  return new Date(value).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

function RouteEntry({ route, position, selected, onClick }: { route: Route; position: number; selected: boolean; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} aria-pressed={selected} className={cn(
      'grid w-full cursor-pointer grid-cols-[42px_minmax(0,1fr)_auto] items-center gap-3 border-b px-3 py-4 text-left transition-colors',
      selected ? 'border-l-2 border-l-primary bg-primary/10' : 'border-l-2 border-l-transparent hover:bg-sidebar-accent',
    )}>
      <span className="mono-data font-mono text-lg font-black text-muted-foreground">{String(position).padStart(2, '0')}</span>
      <span className="min-w-0">
        <strong className="block truncate text-xs font-bold uppercase">{routeName(route)}</strong>
        <span className="mt-1 flex items-center gap-2 font-mono text-[9px] text-muted-foreground">
          {formatDistance(route.distance_m)} <span className="text-border">/</span> {route.trips_count} runs
        </span>
      </span>
      <RouteTypeBadge isLoop={isCircuit(route)} />
    </button>
  )
}

function Leaderboard({ attempts }: { attempts: TripAttemptItem[] }) {
  if (attempts.length === 0) {
    return <EmptyState compact title="No timed runs" description="Matched attempts appear after route recognition." />
  }
  const durations = attempts.map(item => item.duration_sec).filter((value): value is number => value != null)
  const bestDuration = durations.length ? Math.min(...durations) : 0
  const sorted = [...attempts].sort((a, b) => (a.duration_sec ?? Number.MAX_SAFE_INTEGER) - (b.duration_sec ?? Number.MAX_SAFE_INTEGER))

  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[720px] border-collapse text-left">
        <thead>
          <tr className="border-b bg-muted/35 font-mono text-[9px] uppercase tracking-[0.14em] text-muted-foreground">
            <th className="px-4 py-3">Pos</th>
            <th className="px-4 py-3">Attempt</th>
            <th className="px-4 py-3 text-right">Time</th>
            <th className="px-4 py-3 text-right">Delta</th>
            <th className="px-4 py-3 text-right">Avg speed</th>
            <th className="px-4 py-3 text-right">Match</th>
          </tr>
        </thead>
        <tbody>
          {sorted.map((attempt, index) => {
            const duration = attempt.duration_sec ?? 0
            const best = attempt.is_best || duration === bestDuration
            return (
              <tr key={attempt.trip_id ?? `${attempt.started_at}-${index}`} className={cn('border-b transition-colors hover:bg-muted/25', best && 'bg-primary/7')}>
                <td className="mono-data px-4 py-4 font-mono text-xl font-black">{String(index + 1).padStart(2, '0')}</td>
                <td className="px-4 py-4">
                  <div className="flex items-center gap-3">
                    {best ? <Trophy className="size-4 text-primary" aria-label="Personal best" /> : <span className="size-4" />}
                    <div>
                      <p className="text-xs font-bold uppercase">Run {String(sorted.length - index).padStart(2, '0')}</p>
                      <p className="mt-1 font-mono text-[9px] text-muted-foreground">{formatDate(attempt.started_at)}</p>
                    </div>
                  </div>
                </td>
                <td className="mono-data px-4 py-4 text-right font-mono text-base font-bold">{formatDuration(duration)}</td>
                <td className="px-4 py-4 text-right"><ComparisonDelta seconds={duration - bestDuration} isBest={best} /></td>
                <td className="mono-data px-4 py-4 text-right font-mono text-xs">{(attempt.avg_speed_mps ?? 0).toFixed(2)} m/s</td>
                <td className="mono-data px-4 py-4 text-right font-mono text-xs">{Math.round((attempt.match_score ?? 0) * 100)}%</td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

export function RoutesPage({ onLogout }: Props) {
  const { token } = useAuth()
  const queryClient = useQueryClient()
  const [selectedId, setSelectedId] = useSelectedId('route')
  const routesQuery = useQuery({ queryKey: ['routes'], queryFn: () => listRoutes({ limit: 50 }) })

  const routes = routesQuery.data?.items ?? []
  const selectedRoute = routes.find(route => route.id === selectedId) ?? null

  const deleteMutation = useMutation({
    mutationFn: (routeId: string) => deleteRoute(routeId),
    onSuccess: () => {
      toast.success('Circuit deleted')
      setSelectedId(null)
      void queryClient.invalidateQueries({ queryKey: ['routes'] })
    },
  })

  function handleDelete(route: Route) {
    if (window.confirm('Delete this circuit permanently? Recorded sessions are kept, only the circuit and its leaderboard are removed.')) {
      deleteMutation.mutate(route.id)
    }
  }
  const resultsQuery = useQuery({
    queryKey: ['route-results', selectedRoute?.id],
    queryFn: () => getRouteResults(selectedRoute!.id),
    enabled: Boolean(selectedRoute),
  })
  const attemptsQuery = useQuery({
    queryKey: ['route-attempts', selectedRoute?.id],
    queryFn: () => getRouteAttempts(selectedRoute!.id, { limit: 50 }),
    enabled: Boolean(selectedRoute),
  })
  const pointsQuery = useQuery({
    queryKey: ['route-points', selectedRoute?.id],
    queryFn: () => getRoutePoints(selectedRoute!.id),
    enabled: Boolean(selectedRoute),
  })

  const result = resultsQuery.data
  const best = result?.best
  const latest = result?.latest
  const routeList = (
    <div>
      <div className="mb-3 flex items-center justify-between">
        <SectionHeader title="Circuit index" description={`${routes.length} recognized routes`} />
        <StatusBadge tone="success">Ready</StatusBadge>
      </div>
      <div className="border">
        {routesQuery.isLoading ? <div className="p-3"><LoadingSkeleton rows={5} /></div> : null}
        {routesQuery.isError ? <ErrorState title="Index unavailable" description="Route list failed to load." onRetry={() => routesQuery.refetch()} /> : null}
        {routes.map((route, index) => (
          <RouteEntry key={route.id} route={route} position={index + 1} selected={selectedRoute?.id === route.id} onClick={() => setSelectedId(route.id)} />
        ))}
      </div>
    </div>
  )

  return (
    <AppLayout userEmail={getUserEmail(token)} onLogout={onLogout} sidebar={routeList}>
      <div className="h-full overflow-y-auto">
        <div className="p-4 sm:p-6 lg:p-8">
          <div className="race-panel mb-4 p-4 lg:hidden">{routeList}</div>
          {!selectedRoute ? (
            <EmptyState title="Select a circuit" description="Choose a recognized route to open its track map and performance leaderboard." />
          ) : (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
                <MetricCard icon={<RouteIcon className="size-3.5" />} label="Circuit length" value={formatDistance(selectedRoute.distance_m)} detail={isCircuit(selectedRoute) ? 'Closed circuit' : 'Point-to-point sprint'} />
                <MetricCard icon={<Trophy className="size-3.5" />} label="Personal best" value={best ? formatDuration(best.duration_sec ?? 0) : '--:--'} detail={`${result?.attempts_count ?? selectedRoute.trips_count} classified attempts`} />
                <MetricCard icon={<Clock3 className="size-3.5" />} label="Latest run" value={latest ? formatDuration(latest.duration_sec ?? 0) : '--:--'} detail={formatDate(latest?.started_at)} accent="amber" />
                <MetricCard icon={<Gauge className="size-3.5" />} label="Best average" value={`${(best?.avg_speed_mps ?? 0).toFixed(2)}`} detail="Meters / second" accent="cyan" />
              </div>

              <div className="grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(500px,1.5fr)]">
                <section className="race-panel overflow-hidden">
                  <div className="flex items-center justify-between border-b px-4 py-3">
                    <SectionHeader title="Circuit geometry" description="Canonical route model" />
                    <RouteTypeBadge isLoop={isCircuit(selectedRoute)} />
                  </div>
                  {pointsQuery.isError ? (
                    <ErrorState title="Map unavailable" description="Circuit points failed to load." onRetry={() => pointsQuery.refetch()} />
                  ) : (
                    <div className="h-[420px]">
                      <RouteMap points={pointsQuery.data ?? []} startPoint={{ lat: selectedRoute.start_lat, lon: selectedRoute.start_lon }} finishPoint={{ lat: selectedRoute.end_lat, lon: selectedRoute.end_lon }} />
                    </div>
                  )}
                </section>

                <section className="race-panel overflow-hidden">
                  <div className="flex items-center justify-between border-b px-4 py-3">
                    <SectionHeader title="Attempt leaderboard" description="Fastest classified run starts P01" />
                    {result?.comparison ? <ComparisonDelta seconds={result.comparison.latest_vs_best_sec ?? 0} isBest={result.comparison.latest_vs_best_sec === 0} /> : null}
                  </div>
                  {(resultsQuery.isLoading || attemptsQuery.isLoading) ? <div className="p-4"><LoadingSkeleton rows={5} /></div> : null}
                  {(resultsQuery.isError || attemptsQuery.isError) ? <ErrorState title="Timing unavailable" description="Attempt classification failed." onRetry={() => { resultsQuery.refetch(); attemptsQuery.refetch() }} /> : null}
                  {attemptsQuery.data ? <Leaderboard attempts={attemptsQuery.data.items} /> : null}
                </section>
              </div>
              <div className="flex items-center justify-between gap-3 border border-dashed px-4 py-3 font-mono text-[9px] uppercase tracking-[0.1em] text-muted-foreground">
                <span>Deleting a circuit keeps its recorded sessions intact.</span>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleDelete(selectedRoute)}
                  disabled={deleteMutation.isPending}
                  className="gap-2 border-destructive/40 text-destructive hover:bg-destructive/10 hover:text-destructive"
                >
                  <Trash2 className="size-3.5" /> Delete circuit
                </Button>
              </div>
            </div>
          )}
        </div>
      </div>
    </AppLayout>
  )
}
