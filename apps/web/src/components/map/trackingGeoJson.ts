import type { FeatureCollection, Feature, LineString, Point } from 'geojson'
import type { TrackingPoint, TrackSegment } from '@/features/tracking/trackingApi'
import { splitByGap } from './geoGap'

function lineFeatures<T extends { lat: number; lon: number; recorded_at: string }>(
  items: T[],
): Feature<LineString>[] {
  // Break the trace at GPS teleports / long pauses so a single line never jumps
  // across the map. Each continuous run becomes its own LineString.
  return splitByGap(items, item => ({ lat: item.lat, lon: item.lon, recordedAt: item.recorded_at }))
    .filter(run => run.length >= 2)
    .map(run => ({
      type: 'Feature',
      geometry: { type: 'LineString', coordinates: run.map(item => [item.lon, item.lat]) },
      properties: {},
    }))
}

export function pointsToGeoJSON(points: TrackingPoint[]): FeatureCollection {
  if (points.length === 0) {
    return { type: 'FeatureCollection', features: [] }
  }

  const lines = lineFeatures(points)

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

  return { type: 'FeatureCollection', features: [...lines, ...dots] }
}

// trackToGeoJSON converts simplified track segments into two GeoJSON collections:
// - moveGeo: LineString of ALL centroids (preserves visual continuity) + move point dots
// - stayGeo: stay centroid dots only
export function trackToGeoJSON(segments: TrackSegment[]): {
  moveGeo: FeatureCollection
  stayGeo: FeatureCollection
} {
  if (segments.length === 0) {
    const empty: FeatureCollection = { type: 'FeatureCollection', features: [] }
    return { moveGeo: empty, stayGeo: empty }
  }

  const lines = lineFeatures(segments)

  const moveDots: Feature<Point>[] = segments
    .filter(s => s.kind === 'move')
    .map(s => ({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [s.lon, s.lat] },
      properties: {
        event_id: s.event_id,
        kind: 'move',
        recorded_at: s.recorded_at,
        speed_mps: s.speed_mps,
        accuracy_m: s.accuracy_m,
      },
    }))

  const stayDots: Feature<Point>[] = segments
    .filter(s => s.kind === 'stay')
    .map(s => ({
      type: 'Feature',
      geometry: { type: 'Point', coordinates: [s.lon, s.lat] },
      properties: {
        kind: 'stay',
        recorded_at: s.recorded_at,
        period_end: s.period_end,
        stay_duration_sec: s.stay_duration_sec,
        merged_count: s.merged_count,
      },
    }))

  return {
    moveGeo: { type: 'FeatureCollection', features: [...lines, ...moveDots] },
    stayGeo: { type: 'FeatureCollection', features: stayDots },
  }
}

export function getBoundsFromSegments(
  segments: TrackSegment[]
): [[number, number], [number, number]] | null {
  if (segments.length === 0) return null
  let minLon = Infinity, maxLon = -Infinity, minLat = Infinity, maxLat = -Infinity
  for (const s of segments) {
    if (s.lon < minLon) minLon = s.lon
    if (s.lon > maxLon) maxLon = s.lon
    if (s.lat < minLat) minLat = s.lat
    if (s.lat > maxLat) maxLat = s.lat
  }
  const pad = 0.001
  return [[minLon - pad, minLat - pad], [maxLon + pad, maxLat + pad]]
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
