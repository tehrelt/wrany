import { useRef, useEffect, useCallback, useState, useMemo } from 'react'
import Map, {
  Source,
  Layer,
  Popup,
  MapRef,
  type LayerProps,
  type MapLayerMouseEvent,
} from 'react-map-gl/maplibre'
import type { StyleSpecification, LngLatBoundsLike } from 'maplibre-gl'
import { MAP_STYLE_URL } from '@/config/env'
import { Skeleton } from '@/components/ui/skeleton'
import { trackToGeoJSON, getBoundsFromSegments } from './trackingGeoJson'
import type { TrackSegment } from '@/features/tracking/trackingApi'

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

const movePointsLayer: LayerProps = {
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

const stayPointsLayer: LayerProps = {
  id: 'stay-circle',
  type: 'circle',
  paint: {
    'circle-radius': 12,
    'circle-color': '#f97316',
    'circle-stroke-width': 2,
    'circle-stroke-color': '#fff',
    'circle-opacity': 0.85,
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

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}с`
  if (seconds < 3600) return `${Math.floor(seconds / 60)} мин`
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return m > 0 ? `${h}ч ${m}мин` : `${h}ч`
}

interface HoverInfo {
  lon: number
  lat: number
  stayDurationSec: number
  mergedCount: number
}

interface Props {
  segments: TrackSegment[]
  fitKey: string   // fitBounds fires only when this changes (use filter identity, not display settings)
  loading?: boolean
  selectedId?: string | null
  onSelect?: (id: string | null) => void
}

export function TrackingMap({ segments, fitKey, loading, selectedId = null, onSelect }: Props) {
  const mapRef = useRef<MapRef>(null)
  const [hoverInfo, setHoverInfo] = useState<HoverInfo | null>(null)

  const { moveGeo, stayGeo } = useMemo(() => trackToGeoJSON(segments), [segments])
  const mapStyle = MAP_STYLE_URL ?? OSM_FALLBACK

  useEffect(() => {
    const bounds = getBoundsFromSegments(segments)
    if (!bounds || !mapRef.current) return
    mapRef.current.fitBounds(bounds as LngLatBoundsLike, { padding: 48, duration: 600 })
  }, [fitKey]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!selectedId || !mapRef.current) return
    const seg = segments.find(s => s.kind === 'move' && s.event_id === selectedId)
    if (!seg) return
    mapRef.current.flyTo({ center: [seg.lon, seg.lat], zoom: 21, duration: 500 })
  }, [selectedId]) // eslint-disable-line react-hooks/exhaustive-deps

  const handleClick = useCallback((e: MapLayerMouseEvent) => {
    const feature = e.features?.[0]
    if (feature?.properties?.kind === 'move') {
      onSelect?.(feature.properties.event_id as string)
    }
  }, [onSelect])

  const handleMouseMove = useCallback((e: MapLayerMouseEvent) => {
    const stayFeature = e.features?.find(f => f.layer?.id === 'stay-circle')
    if (stayFeature?.properties) {
      const [lon, lat] = (stayFeature.geometry as GeoJSON.Point).coordinates
      setHoverInfo({
        lon,
        lat,
        stayDurationSec: stayFeature.properties.stay_duration_sec as number,
        mergedCount: stayFeature.properties.merged_count as number,
      })
    } else {
      setHoverInfo(null)
    }
  }, [])

  if (loading && segments.length === 0) {
    return <Skeleton className="w-full h-full rounded-none" />
  }

  return (
    <div className="relative w-full h-full">
      {segments.length === 0 && (
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
        interactiveLayerIds={['points-circle', 'stay-circle']}
        onClick={handleClick}
        onMouseMove={handleMouseMove}
        cursor="default"
      >
        <Source id="tracking-move" type="geojson" data={moveGeo}>
          <Layer {...trackLineLayer} />
          <Layer {...movePointsLayer} />
          {selectedId && <Layer {...selectedLayer(selectedId)} />}
        </Source>

        <Source id="tracking-stay" type="geojson" data={stayGeo}>
          <Layer {...stayPointsLayer} />
        </Source>

        {hoverInfo && (
          <Popup
            longitude={hoverInfo.lon}
            latitude={hoverInfo.lat}
            closeButton={false}
            offset={16}
          >
            <div className="text-xs font-medium">
              Здесь {formatDuration(hoverInfo.stayDurationSec)}
              <span className="text-muted-foreground ml-1">({hoverInfo.mergedCount} точек)</span>
            </div>
          </Popup>
        )}
      </Map>
    </div>
  )
}
