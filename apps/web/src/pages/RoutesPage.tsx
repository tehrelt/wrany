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
  getRouteTrips,
  getRoutePoints,
  formatDistance,
  formatDuration,
  type Route,
  type RoutePoint,
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

export function RoutesPage({ onLogout }: Props) {
  const { token } = useAuth()
  const [selectedRoute, setSelectedRoute] = useState<Route | null>(null)

  const routesQuery = useQuery({
    queryKey: ['routes'],
    queryFn: () => listRoutes({ limit: 50 }),
  })

  const tripsQuery = useQuery({
    queryKey: ['route-trips', selectedRoute?.id],
    queryFn: () => getRouteTrips(selectedRoute!.id, { limit: 50 }),
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

        {/* Trips table */}
        {selectedRoute && (
          <div className="h-56 border-t overflow-y-auto shrink-0">
            <div className="p-3">
              <h3 className="text-sm font-semibold mb-2">
                Trips — {selectedRoute.name ?? `Route ${selectedRoute.id.slice(0, 8)}`}
              </h3>

              {tripsQuery.isLoading && <Skeleton className="h-8 w-full" />}

              {tripsQuery.isError && (
                <Alert variant="destructive">
                  <AlertDescription>Failed to load trips.</AlertDescription>
                </Alert>
              )}

              {tripsQuery.data && tripsQuery.data.items.length === 0 && (
                <p className="text-xs text-muted-foreground">No trips yet.</p>
              )}

              {tripsQuery.data && tripsQuery.data.items.length > 0 && (
                <table className="w-full text-xs">
                  <thead>
                    <tr className="text-muted-foreground border-b">
                      <th className="text-left pb-1 pr-3 font-medium">Date</th>
                      <th className="text-right pb-1 pr-3 font-medium">Distance</th>
                      <th className="text-right pb-1 pr-3 font-medium">Duration</th>
                      <th className="text-right pb-1 font-medium">Score</th>
                    </tr>
                  </thead>
                  <tbody>
                    {tripsQuery.data.items.map(rt => {
                      const date = new Date(rt.started_at)
                      const dateStr = date.toLocaleDateString(undefined, {
                        month: 'short',
                        day: 'numeric',
                        hour: '2-digit',
                        minute: '2-digit',
                      })
                      return (
                        <tr key={rt.trip_id} className="border-b last:border-0 hover:bg-accent/50">
                          <td className="py-1 pr-3 text-muted-foreground">{dateStr}</td>
                          <td className="py-1 pr-3 text-right">{formatDistance(rt.distance_m)}</td>
                          <td className="py-1 pr-3 text-right">{formatDuration(rt.duration_sec)}</td>
                          <td className="py-1 text-right">{(rt.match_score * 100).toFixed(0)}%</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        )}
      </div>
    </AppLayout>
  )
}
