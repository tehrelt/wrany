import { useRef, useEffect } from 'react'
import Map, { Source, Layer, MapRef, type LayerProps } from 'react-map-gl/maplibre'
import type { StyleSpecification, LngLatBoundsLike } from 'maplibre-gl'
import type { FeatureCollection } from 'geojson'
import { MAP_STYLE_URL } from '@/config/env'
import { Skeleton } from '@/components/ui/skeleton'
import type { TripPoint } from '@/features/trips/tripsApi'

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

const lineLayer: LayerProps = {
  id: 'trip-line',
  type: 'line',
  filter: ['==', '$type', 'LineString'],
  paint: { 'line-color': '#22c55e', 'line-width': 3 },
}

const startDotLayer: LayerProps = {
  id: 'trip-start',
  type: 'circle',
  filter: ['==', ['get', 'role'], 'start'],
  paint: {
    'circle-radius': 7,
    'circle-color': '#22c55e',
    'circle-stroke-width': 2,
    'circle-stroke-color': '#fff',
  },
}

const endDotLayer: LayerProps = {
  id: 'trip-end',
  type: 'circle',
  filter: ['==', ['get', 'role'], 'end'],
  paint: {
    'circle-radius': 7,
    'circle-color': '#ef4444',
    'circle-stroke-width': 2,
    'circle-stroke-color': '#fff',
  },
}

function tripToGeoJSON(points: TripPoint[]): FeatureCollection {
  if (points.length === 0) return { type: 'FeatureCollection', features: [] }
  const coords = points.map(p => [p.lon, p.lat] as [number, number])
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

function getBounds(points: TripPoint[]): [[number, number], [number, number]] | null {
  if (points.length === 0) return null
  let minLon = Infinity, maxLon = -Infinity, minLat = Infinity, maxLat = -Infinity
  for (const p of points) {
    if (p.lon < minLon) minLon = p.lon
    if (p.lon > maxLon) maxLon = p.lon
    if (p.lat < minLat) minLat = p.lat
    if (p.lat > maxLat) maxLat = p.lat
  }
  const pad = 0.001
  return [[minLon - pad, minLat - pad], [maxLon + pad, maxLat + pad]]
}

interface Props {
  points: TripPoint[]
  loading?: boolean
}

export function TripMap({ points, loading }: Props) {
  const mapRef = useRef<MapRef>(null)
  const geoData = tripToGeoJSON(points)
  const mapStyle = MAP_STYLE_URL ?? OSM_FALLBACK

  useEffect(() => {
    const bounds = getBounds(points)
    if (!bounds || !mapRef.current) return
    mapRef.current.fitBounds(bounds as LngLatBoundsLike, { padding: 56, duration: 600 })
  }, [points])

  if (loading) return <Skeleton className="w-full h-full rounded-none" />

  return (
    <div className="relative w-full h-full">
      {points.length === 0 && (
        <div className="absolute inset-0 flex items-center justify-center z-10 pointer-events-none">
          <div className="bg-background/80 rounded-lg px-4 py-2 text-sm text-muted-foreground">
            Select a trip to view its route
          </div>
        </div>
      )}
      <Map
        ref={mapRef}
        mapStyle={mapStyle}
        initialViewState={{ longitude: 0, latitude: 20, zoom: 1 }}
        style={{ width: '100%', height: '100%' }}
      >
        <Source id="trip" type="geojson" data={geoData}>
          <Layer {...lineLayer} />
          <Layer {...startDotLayer} />
          <Layer {...endDotLayer} />
        </Source>
      </Map>
    </div>
  )
}
