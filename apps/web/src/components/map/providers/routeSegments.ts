import type { MapPoint } from './MapProvider'
import { shouldBreakSegment, splitByGap } from '../geoGap'

function toSample(point: MapPoint) {
  return { lat: point.lat, lon: point.lon, recordedAt: point.recordedAt }
}

// buildRoutePolylines groups points into continuous runs, breaking the line at
// GPS teleports / long pauses so a single LineString never jumps across the
// city. Returns coordinate arrays ready for a (Multi)LineString geometry.
export function buildRoutePolylines(points: MapPoint[]): number[][][] {
  return splitByGap(points, toSample)
    .filter(run => run.length >= 2)
    .map(run => run.map(point => [point.lon, point.lat]))
}

export interface RouteSegmentStyle {
  from: MapPoint
  to: MapPoint
  color: string
  opacity: number
}

const MIN_OPACITY = 0.18

function clamp(value: number): number {
  return Math.min(1, Math.max(0, value))
}

function timestamp(point: MapPoint, fallback: number): number {
  const value = point.recordedAt ? Date.parse(point.recordedAt) : Number.NaN
  return Number.isFinite(value) ? value : fallback
}

function speedColor(ratio: number): string {
  const hue = 210 - clamp(ratio) * 210
  return `hsl(${Math.round(hue)} 88% 54%)`
}

export function buildTelemetrySegments(points: MapPoint[]): RouteSegmentStyle[] {
  if (points.length < 2) return []

  const timestamps = points.map((point, index) => timestamp(point, index))
  const speeds = points
    .map(point => point.speedMps)
    .filter((speed): speed is number => speed != null && Number.isFinite(speed))
  const oldest = Math.min(...timestamps)
  const newest = Math.max(...timestamps)
  const minSpeed = speeds.length > 0 ? Math.min(...speeds) : 0
  const maxSpeed = speeds.length > 0 ? Math.max(...speeds) : minSpeed

  return points.slice(1).flatMap((to, index) => {
    const from = points[index]
    // Do not draw a colored segment across a GPS teleport / long pause: it would
    // be a fake straight line through the city. The gap stays visually empty.
    if (shouldBreakSegment(toSample(from), toSample(to))) return []
    const ageRatio = newest === oldest
      ? (index + 1) / (points.length - 1)
      : (timestamps[index] + timestamps[index + 1]) / 2 - oldest
    const normalizedAge = newest === oldest
      ? ageRatio
      : ageRatio / (newest - oldest)
    const segmentSpeed = to.speedMps ?? from.speedMps ?? minSpeed
    const speedRatio = maxSpeed === minSpeed
      ? 0
      : (segmentSpeed - minSpeed) / (maxSpeed - minSpeed)

    return [{
      from,
      to,
      color: speedColor(speedRatio),
      opacity: MIN_OPACITY + clamp(normalizedAge) * (1 - MIN_OPACITY),
    }]
  })
}
