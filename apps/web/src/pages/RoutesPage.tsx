import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import Map, { Source, Layer, type LayerProps } from 'react-map-gl/maplibre'
import type { StyleSpecification } from 'maplibre-gl'
import type { FeatureCollection } from 'geojson'
import { AppLayout } from '@/components/layout/AppLayout'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useAuth } from '@/features/auth/useAuth'
import {
  listRoutes,
  getRoutePoints,
  getRouteResults,
  getRouteAttempts,
  formatDistance,
  formatDuration,
  type Route,
  type RoutePoint,
  type RouteResultResponse,
  type TripAttemptItem,
} from '@/features/routes/routesApi'
import { MAP_STYLE_URL } from '@/config/env'

interface Props {
  onLogout: () => void
}

const OSM_FALLBACK: StyleSpecification = {
  version: 8,
  sources: {
    osm: {
      type: 'raster',
      tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'],
      tileSize: 256,
      attribution: '© OpenStreetMap contributors',
    },
  },
  layers: [{ id: 'osm-tiles', type: 'raster', source: 'osm' }],
}

const routeLineLayer: LayerProps = {
  id: 'route-line',
  type: 'line',
  filter: ['==', '$type', 'LineString'],
  paint: { 'line-color': '#3b82f6', 'line-width': 3 },
}

const startDotLayer: LayerProps = {
  id: 'route-start',
  type: 'circle',
  filter: ['==', ['get', 'role'], 'start'],
  paint: { 'circle-radius': 7, 'circle-color': '#22c55e', 'circle-stroke-width': 2, 'circle-stroke-color': '#fff' },
}

const endDotLayer: LayerProps = {
  id: 'route-end',
  type: 'circle',
  filter: ['==', ['get', 'role'], 'end'],
  paint: { 'circle-radius': 7, 'circle-color': '#ef4444', 'circle-stroke-width': 2, 'circle-stroke-color': '#fff' },
}

function pointsToGeoJSON(pts: RoutePoint[]): FeatureCollection {
  if (pts.length === 0) return { type: 'FeatureCollection', features: [] }
  const coords = pts.map(p => [p.lon, p.lat] as [number, number])
  return {
    type: 'FeatureCollection',
    features: [
      {
        type: 'Feature',
        geometry: { type: 'LineString', coordinates: coords },
        properties: {},
      },
      {
        type: 'Feature',
        geometry: { type: 'Point', coordinates: coords[0] },
        properties: { role: 'start' },
      },
      {
        type: 'Feature',
        geometry: { type: 'Point', coordinates: coords[coords.length - 1] },
        properties: { role: 'end' },
      },
    ],
  }
}

function RouteCard({
  route,
  selected,
  onClick,
}: {
  route: Route
  selected: boolean
  onClick: () => void
}) {
  const date = new Date(route.updated_at)
  const dateStr = date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })

  return (
    <button
      onClick={onClick}
      className={[
        'w-full text-left rounded-lg border p-3 transition-colors hover:bg-accent',
        selected ? 'border-primary bg-accent' : 'border-border',
      ].join(' ')}
    >
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs text-muted-foreground">{dateStr}</span>
        <Badge variant="secondary" className="text-xs">
          {route.trips_count} trip{route.trips_count !== 1 ? 's' : ''}
        </Badge>
      </div>
      <div className="flex gap-4 text-sm font-medium">
        <span>{formatDistance(route.distance_m)}</span>
      </div>
      <div className="text-xs text-muted-foreground mt-1 truncate">
        {route.name ?? `Route ${route.id.slice(0, 8)}`}
      </div>
    </button>
  )
}

function formatSpeed(mps: number | undefined): string {
  if (!mps) return '—'
  return `${(mps * 3.6).toFixed(1)} km/h`
}

function TripResultCard({ label, trip }: { label: string; trip: { trip_id?: string; started_at?: string; duration_sec?: number; distance_m?: number; avg_speed_mps?: number } }) {
  const date = trip.started_at ? new Date(trip.started_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) : '—'
  return (
    <div className="rounded-lg border p-2 flex-1 text-xs">
      <div className="text-muted-foreground font-medium mb-1">{label}</div>
      <div className="font-semibold text-sm">{formatDuration(trip.duration_sec ?? 0)}</div>
      <div className="text-muted-foreground">{date} · {formatDistance(trip.distance_m ?? 0)} · {formatSpeed(trip.avg_speed_mps)}</div>
    </div>
  )
}

function PersonalRecordsSection({ result }: { result: RouteResultResponse }) {
  const { best, latest, comparison, attempts_count } = result

  if (!attempts_count) {
    return <p className="text-xs text-muted-foreground">No completed attempts yet.</p>
  }

  const isPersonalRecord = comparison?.latest_vs_best_sec === 0
  const diff = comparison?.latest_vs_best_sec ?? 0

  return (
    <div className="space-y-2">
      <div className="flex gap-2">
        {best && <TripResultCard label="Best" trip={best} />}
        {latest && <TripResultCard label="Latest" trip={latest} />}
        <div className="rounded-lg border p-2 text-xs flex flex-col justify-center items-center gap-1 min-w-[80px]">
          <div className="text-muted-foreground font-medium">vs Best</div>
          {isPersonalRecord ? (
            <Badge variant="default" className="text-xs">Personal Record</Badge>
          ) : (
            <span className={diff > 0 ? 'text-orange-500 font-semibold' : 'text-green-600 font-semibold'}>
              {diff > 0 ? `+${diff}s` : `${diff}s`}
            </span>
          )}
          <div className="text-muted-foreground">{attempts_count} attempt{attempts_count !== 1 ? 's' : ''}</div>
        </div>
      </div>
    </div>
  )
}

function AttemptsTable({ attempts }: { attempts: TripAttemptItem[] }) {
  if (attempts.length === 0) {
    return <p className="text-xs text-muted-foreground">No attempts yet.</p>
  }
  return (
    <table className="w-full text-xs">
      <thead>
        <tr className="text-muted-foreground border-b">
          <th className="text-left pb-1 pr-3 font-medium">Date</th>
          <th className="text-right pb-1 pr-3 font-medium">Distance</th>
          <th className="text-right pb-1 pr-3 font-medium">Duration</th>
          <th className="text-right pb-1 pr-3 font-medium">Speed</th>
          <th className="text-right pb-1 font-medium">Score</th>
        </tr>
      </thead>
      <tbody>
        {attempts.map(a => {
          const date = a.started_at
            ? new Date(a.started_at).toLocaleDateString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
            : '—'
          return (
            <tr
              key={a.trip_id}
              className={[
                'border-b last:border-0 hover:bg-accent/50',
                a.is_best ? 'bg-yellow-50 dark:bg-yellow-950/20' : '',
              ].join(' ')}
            >
              <td className="py-1 pr-3 text-muted-foreground">
                {a.is_best && <span className="text-yellow-600 mr-1">★</span>}
                {date}
              </td>
              <td className="py-1 pr-3 text-right">{formatDistance(a.distance_m ?? 0)}</td>
              <td className="py-1 pr-3 text-right font-medium">{formatDuration(a.duration_sec ?? 0)}</td>
              <td className="py-1 pr-3 text-right">{formatSpeed(a.avg_speed_mps)}</td>
              <td className="py-1 text-right">{((a.match_score ?? 0) * 100).toFixed(0)}%</td>
            </tr>
          )
        })}
      </tbody>
    </table>
  )
}

export function RoutesPage({ onLogout }: Props) {
  const { token } = useAuth()
  const [selectedRoute, setSelectedRoute] = useState<Route | null>(null)

  const routesQuery = useQuery({
    queryKey: ['routes'],
    queryFn: () => listRoutes({ limit: 50 }),
  })

  const resultsQuery = useQuery({
    queryKey: ['route-results', selectedRoute?.id],
    queryFn: () => getRouteResults(selectedRoute!.id),
    enabled: !!selectedRoute,
  })

  const attemptsQuery = useQuery({
    queryKey: ['route-attempts', selectedRoute?.id],
    queryFn: () => getRouteAttempts(selectedRoute!.id, { limit: 50 }),
    enabled: !!selectedRoute,
  })

  const pointsQuery = useQuery({
    queryKey: ['route-points', selectedRoute?.id],
    queryFn: () => getRoutePoints(selectedRoute!.id),
    enabled: !!selectedRoute,
  })

  let userEmail = ''
  if (token) {
    try {
      userEmail = (JSON.parse(atob(token.split('.')[1])) as { sub?: string }).sub ?? ''
    } catch {
      // ignore decode errors
    }
  }

  const geoJSON = pointsQuery.data ? pointsToGeoJSON(pointsQuery.data) : pointsToGeoJSON([])

  const routes = routesQuery.data?.items ?? []

  const sidebar = (
    <div className="flex flex-col gap-3 h-full">
      <div className="shrink-0">
        <h2 className="text-sm font-semibold">Routes</h2>
        <p className="text-xs text-muted-foreground mt-0.5">
          {routes.length} route{routes.length !== 1 ? 's' : ''}
        </p>
      </div>

      {routesQuery.isLoading && (
        <div className="flex flex-col gap-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-lg" />
          ))}
        </div>
      )}

      {routesQuery.isError && (
        <Alert variant="destructive">
          <AlertDescription>Failed to load routes.</AlertDescription>
        </Alert>
      )}

      {!routesQuery.isLoading && routes.length === 0 && (
        <p className="text-xs text-muted-foreground">No routes yet.</p>
      )}

      <div className="flex flex-col gap-2 overflow-y-auto flex-1">
        {routes.map(r => (
          <RouteCard
            key={r.id}
            route={r}
            selected={selectedRoute?.id === r.id}
            onClick={() => setSelectedRoute(r)}
          />
        ))}
      </div>
    </div>
  )

  return (
    <AppLayout userEmail={userEmail} onLogout={onLogout} sidebar={sidebar}>
      <div className="flex flex-col flex-1 overflow-hidden">
        {/* Map */}
        <div className="flex-1 min-h-0">
          <Map
            initialViewState={{ longitude: 37.618, latitude: 55.751, zoom: 11 }}
            style={{ width: '100%', height: '100%' }}
            mapStyle={MAP_STYLE_URL ?? OSM_FALLBACK}
          >
            <Source id="route" type="geojson" data={geoJSON}>
              <Layer {...routeLineLayer} />
              <Layer {...startDotLayer} />
              <Layer {...endDotLayer} />
            </Source>
          </Map>
        </div>

        {/* Personal Records + Attempts */}
        {selectedRoute && (
          <div className="border-t shrink-0 overflow-y-auto" style={{ maxHeight: '320px' }}>
            <div className="p-3 space-y-3">
              <h3 className="text-sm font-semibold">
                {selectedRoute.name ?? `Route ${selectedRoute.id.slice(0, 8)}`}
              </h3>

              {(resultsQuery.isLoading || attemptsQuery.isLoading) && (
                <Skeleton className="h-16 w-full" />
              )}

              {(resultsQuery.isError || attemptsQuery.isError) && (
                <Alert variant="destructive">
                  <AlertDescription>Failed to load results.</AlertDescription>
                </Alert>
              )}

              {resultsQuery.data && (
                <PersonalRecordsSection result={resultsQuery.data} />
              )}

              {attemptsQuery.data && (
                <AttemptsTable attempts={attemptsQuery.data.items} />
              )}
            </div>
          </div>
        )}
      </div>
    </AppLayout>
  )
}
