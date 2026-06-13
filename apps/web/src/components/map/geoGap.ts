// Shared geometry helpers that decide where a GPS trace must be broken into
// separate line segments. A continuous polyline is only correct while
// consecutive samples really are consecutive in space and time. When two
// samples are separated by a long pause or an impossible jump (GPS teleport),
// connecting them would draw a fake straight line across the map, so the line
// must be split instead.

export interface GeoSample {
  lat: number
  lon: number
  // ISO timestamp; optional because some callers only carry coordinates.
  recordedAt?: string | null
}

const EARTH_RADIUS_M = 6_371_000
// Implied speed above this (~252 km/h) is physically impossible -> GPS teleport.
const MAX_IMPLIED_SPEED_MPS = 70
// Absolute jump treated as a teleport (also the fallback when timestamps are
// missing). A long *pause* is not a break: standing still keeps the samples
// spatially close, so the line stays continuous through it.
const MAX_GAP_METERS = 2_000

function toRadians(degrees: number): number {
  return (degrees * Math.PI) / 180
}

export function haversineMeters(a: GeoSample, b: GeoSample): number {
  const dLat = toRadians(b.lat - a.lat)
  const dLon = toRadians(b.lon - a.lon)
  const lat1 = toRadians(a.lat)
  const lat2 = toRadians(b.lat)
  const h =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLon / 2) ** 2
  return 2 * EARTH_RADIUS_M * Math.asin(Math.min(1, Math.sqrt(h)))
}

// Seconds between two samples, or null when either timestamp is missing/invalid.
function gapSeconds(a: GeoSample, b: GeoSample): number | null {
  if (!a.recordedAt || !b.recordedAt) return null
  const start = Date.parse(a.recordedAt)
  const end = Date.parse(b.recordedAt)
  if (!Number.isFinite(start) || !Number.isFinite(end)) return null
  return Math.abs(end - start) / 1000
}

// Returns true when the segment between `a` and `b` must NOT be drawn as one
// continuous line. Note: a change of processing-segment id alone is not a
// reason to break — consecutive samples must stay visually connected.
export function shouldBreakSegment(a: GeoSample, b: GeoSample): boolean {
  const meters = haversineMeters(a, b)
  // A jump too large to be real positioning -> always break.
  if (meters > MAX_GAP_METERS) return true
  // With timestamps, break only on an impossible implied speed. Long pauses
  // (small distance, large time) are intentionally NOT broken so the trace
  // stays visually continuous through stops.
  const seconds = gapSeconds(a, b)
  if (seconds != null && seconds > 0 && meters / seconds > MAX_IMPLIED_SPEED_MPS) return true
  return false
}

// Splits items into runs of consecutive samples that can be drawn as one line.
// Singleton runs are kept so callers can decide how to render lone points.
export function splitByGap<T>(items: T[], toSample: (item: T) => GeoSample): T[][] {
  if (items.length === 0) return []
  const runs: T[][] = []
  let current: T[] = [items[0]]
  for (let i = 1; i < items.length; i++) {
    if (shouldBreakSegment(toSample(items[i - 1]), toSample(items[i]))) {
      runs.push(current)
      current = [items[i]]
    } else {
      current.push(items[i])
    }
  }
  runs.push(current)
  return runs
}
