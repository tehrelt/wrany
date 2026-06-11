import { useRef, useEffect, useCallback } from 'react'
import Map, {
  Source,
  Layer,
  MapRef,
  type LayerProps,
  type MapLayerMouseEvent,
} from 'react-map-gl/maplibre'
import type { StyleSpecification, LngLatBoundsLike } from 'maplibre-gl'
import { MAP_STYLE_URL } from '@/config/env'
import { Skeleton } from '@/components/ui/skeleton'
import { pointsToGeoJSON, getBounds } from './trackingGeoJson'
import type { TrackingPoint } from '@/features/tracking/trackingApi'

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

const trackLineLayer: LayerProps = {
  id: 'track-line',
  type: 'line',
  filter: ['==', '$type', 'LineString'],
  paint: { 'line-color': '#3b82f6', 'line-width': 2 },
}

const pointsLayer: LayerProps = {
  id: 'points-circle',
  type: 'circle',
  filter: ['==', '$type', 'Point'],
  paint: {
    'circle-radius': 4,
    'circle-color': '#2563eb',
    'circle-stroke-width': 1,
    'circle-stroke-color': '#fff',
  },
}

function selectedLayer(id: string): LayerProps {
  return {
    id: 'point-selected',
    type: 'circle',
    filter: ['all', ['==', '$type', 'Point'], ['==', ['get', 'event_id'], id]],
    paint: {
      'circle-radius': 8,
      'circle-color': '#dc2626',
      'circle-stroke-width': 2,
      'circle-stroke-color': '#fff',
    },
  }
}

interface Props {
  points: TrackingPoint[]
  loading?: boolean
  selectedId?: string | null
  onSelect?: (id: string | null) => void
}

export function TrackingMap({ points, loading, selectedId = null, onSelect }: Props) {
  const mapRef = useRef<MapRef>(null)

  const geoData = pointsToGeoJSON(points)
  const mapStyle = MAP_STYLE_URL ?? OSM_FALLBACK

  useEffect(() => {
    const bounds = getBounds(points)
    if (!bounds || !mapRef.current) return
    mapRef.current.fitBounds(bounds as LngLatBoundsLike, { padding: 48, duration: 600 })
  }, [points])

  useEffect(() => {
    if (!selectedId || !mapRef.current) return
    const pt = points.find(p => p.event_id === selectedId)
    if (!pt) return
    mapRef.current.flyTo({ center: [pt.lon, pt.lat], zoom: 21, duration: 500 })
  }, [selectedId]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleClick = useCallback((e: MapLayerMouseEvent) => {
    const feature = e.features?.[0]
    if (feature?.properties) {
      onSelect?.(feature.properties.event_id as string)
    }
  }, [onSelect])

  if (loading) {
    return <Skeleton className="w-full h-full rounded-none" />
  }

  return (
    <div className="relative w-full h-full">
      {points.length === 0 && (
        <div className="absolute inset-0 flex items-center justify-center z-10 pointer-events-none">
          <div className="bg-background/80 rounded-lg px-4 py-2 text-sm text-muted-foreground">
            No points in selected range
          </div>
        </div>
      )}

      <Map
        ref={mapRef}
        mapStyle={mapStyle}
        initialViewState={{ longitude: 0, latitude: 20, zoom: 1 }}
        style={{ width: '100%', height: '100%' }}
        interactiveLayerIds={['points-circle']}
        onClick={handleClick}
        cursor="default"
      >
        <Source id="tracking" type="geojson" data={geoData}>
          <Layer {...trackLineLayer} />
          <Layer {...pointsLayer} />
          {selectedId && <Layer {...selectedLayer(selectedId)} />}
        </Source>
      </Map>

    </div>
  )
}
