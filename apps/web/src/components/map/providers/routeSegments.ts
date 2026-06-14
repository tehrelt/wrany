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

export interface ActivityRun {
  runId: number
  activity: string
  startAt?: string
  endAt?: string
  pointCount: number
  // Gap-split polylines so a run never draws a fake line across a teleport.
  lines: number[][][]
}

function activityOf(point: MapPoint): string {
  const value = point.activityType?.trim()
  return value ? value : 'unknown'
}

// Groups consecutive points sharing the same activity type into runs, so the
// whole span of one activity (start -> end) can be highlighted together. Each
// run's geometry is further split at GPS gaps.
export function buildActivityRuns(points: MapPoint[]): ActivityRun[] {
  const runs: ActivityRun[] = []
  let current: MapPoint[] = []
  let currentActivity = ''

  const flush = () => {
    if (current.length === 0) return
    const lines = splitByGap(current, toSample)
      .filter(run => run.length >= 2)
      .map(run => run.map(point => [point.lon, point.lat]))
    if (lines.length > 0) {
      runs.push({
        runId: runs.length,
        activity: currentActivity || 'unknown',
        startAt: current[0].recordedAt ?? undefined,
        endAt: current[current.length - 1].recordedAt ?? undefined,
        pointCount: current.length,
        lines,
      })
    }
  }

  for (const point of points) {
    const activity = activityOf(point)
    if (current.length === 0) {
      currentActivity = activity
      current = [point]
    } else if (activity === currentActivity) {
      current.push(point)
    } else {
      flush()
      currentActivity = activity
      current = [point]
    }
  }
  flush()
  return runs
}

const MIN_OPACITY = 0.18

function clamp(value: number): number {
  return Math.min(1, Math.max(0, value))
}

function timestamp(point: MapPoint, fallback: number): number {
  const value = point.recordedAt ? Date.parse(point.recordedAt) : Number.NaN
  return Number.isFinite(value) ? value : fallback
}

// Vivid, perceptually-spread speed ramp (slow -> fast). A multi-stop ramp gives
// far more local contrast than a single HSL hue sweep, so neighbouring speeds
// stay distinguishable. Shared with the map legend so both stay in sync.
export const SPEED_RAMP: readonly (readonly [number, readonly [number, number, number]])[] = [
  [0.0, [37, 99, 235]],   // blue   — slowest
  [0.25, [6, 182, 212]],  // cyan
  [0.5, [34, 197, 94]],   // green
  [0.75, [234, 179, 8]],  // amber
  [1.0, [239, 68, 68]],   // red    — fastest
]

export const SPEED_RAMP_CSS = `linear-gradient(90deg, ${SPEED_RAMP.map(
  ([, [r, g, b]]) => `rgb(${r}, ${g}, ${b})`,
).join(', ')})`

export function speedColor(ratio: number): string {
  const x = clamp(ratio)
  for (let i = 1; i < SPEED_RAMP.length; i++) {
    const [p1, c1] = SPEED_RAMP[i]
    const [p0, c0] = SPEED_RAMP[i - 1]
    if (x <= p1) {
      const f = p1 === p0 ? 0 : (x - p0) / (p1 - p0)
      const mix = (a: number, b: number) => Math.round(a + (b - a) * f)
      return `rgb(${mix(c0[0], c1[0])}, ${mix(c0[1], c1[1])}, ${mix(c0[2], c1[2])})`
    }
  }
  const [, last] = SPEED_RAMP[SPEED_RAMP.length - 1]
  return `rgb(${last[0]}, ${last[1]}, ${last[2]})`
}

// Empirical-CDF (percentile) position of `value` within a sorted ascending list.
// This spreads colors by the data's distribution, so clustered speeds (e.g. a
// mostly-slow walk with a few fast bursts) still use the whole ramp instead of
// collapsing to one hue under linear min-max scaling.
export function percentileRank(sorted: number[], value: number): number {
  const n = sorted.length
  if (n <= 1) return 0
  // Binary search for the first index >= value.
  let lo = 0
  let hi = n
  while (lo < hi) {
    const mid = (lo + hi) >> 1
    if (sorted[mid] < value) lo = mid + 1
    else hi = mid
  }
  return lo / (n - 1)
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
  const sortedSpeeds = [...speeds].sort((a, b) => a - b)

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
    const speedRatio = percentileRank(sortedSpeeds, segmentSpeed)

    return [{
      from,
      to,
      color: speedColor(speedRatio),
      opacity: MIN_OPACITY + clamp(normalizedAge) * (1 - MIN_OPACITY),
    }]
  })
}
