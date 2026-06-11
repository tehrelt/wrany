import type { FeatureCollection, Feature, LineString, Point } from 'geojson'
import type { TrackingPoint } from '@/features/tracking/trackingApi'

export function pointsToGeoJSON(points: TrackingPoint[]): FeatureCollection {
  if (points.length === 0) {
    return { type: 'FeatureCollection', features: [] }
  }

  const line: Feature<LineString> = {
    type: 'Feature',
    geometry: {
      type: 'LineString',
      coordinates: points.map(p => [p.lon, p.lat]),
    },
    properties: {},
  }

  const dots: Feature<Point>[] = points.map(p => ({
    type: 'Feature',
    geometry: { type: 'Point', coordinates: [p.lon, p.lat] },
    properties: {
      event_id: p.event_id,
      recorded_at: p.recorded_at,
      speed_mps: p.speed_mps,
      accuracy_m: p.accuracy_m,
      activity_type: p.activity_type,
    },
  }))

  return { type: 'FeatureCollection', features: [line, ...dots] }
}

export function getBounds(
  points: TrackingPoint[]
): [[number, number], [number, number]] | null {
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
