import { getTracking } from '@/api/generated/tracking/tracking'
import { apiClient } from '@/api/client'

export interface TrackingPoint {
  event_id: string
  device_id: string
  recorded_at: string
  lat: number
  lon: number
  accuracy_m: number
  speed_mps: number | null
  bearing_deg: number | null
  activity_type: string
}

export interface TrackingSummary {
  points_count: number
  first_recorded_at: string | null
  last_recorded_at: string | null
  duration_sec: number
  avg_speed_mps: number | null
  max_speed_mps: number | null
}

export interface PointsFilter {
  device_id?: string
  from: string
  to: string
  limit?: number
  cursor?: string
}

export interface PointsResponse {
  items: TrackingPoint[]
  next_cursor: string | null
}

const api = getTracking()

export async function getPoints(filter: PointsFilter): Promise<PointsResponse> {
  const env = await api.getV1TrackingPoints({
    device_id: filter.device_id,
    from: filter.from,
    to: filter.to,
    limit: filter.limit,
    cursor: filter.cursor,
  })
  const data = env.data
  return {
    items: (data?.items ?? []) as TrackingPoint[],
    next_cursor: data?.next_cursor ?? null,
  }
}

export async function getSummary(filter: Omit<PointsFilter, 'limit' | 'cursor'>): Promise<TrackingSummary> {
  const env = await api.getV1TrackingSummary({
    device_id: filter.device_id,
    from: filter.from,
    to: filter.to,
  })
  return env.data as TrackingSummary
}

export async function deletePoint(eventId: string): Promise<void> {
  await api.deleteV1TrackingPointsEventId(eventId)
}

export interface TrackSegment {
  kind: 'move' | 'stay'
  segment_id: number
  event_id: string
  recorded_at: string
  period_end: string
  lat: number
  lon: number
  speed_mps: number | null
  accuracy_m: number | null
  stay_duration_sec: number
  merged_count: number
  activity_type: string
}

export interface TrackFilter {
  device_id?: string
  from: string
  to: string
  speed_threshold_mps?: number
  min_stay_sec?: number
  min_move_sec?: number
}

export async function getTrack(filter: TrackFilter): Promise<TrackSegment[]> {
  const params = new URLSearchParams({ from: filter.from, to: filter.to })
  if (filter.device_id) params.set('device_id', filter.device_id)
  if (filter.speed_threshold_mps != null) params.set('speed_threshold_mps', String(filter.speed_threshold_mps))
  if (filter.min_stay_sec != null) params.set('min_stay_sec', String(filter.min_stay_sec))
  if (filter.min_move_sec != null) params.set('min_move_sec', String(filter.min_move_sec))
  const res = await apiClient.get<{ data: { items: TrackSegment[] }; error: string | null }>(
    `/v1/tracking/track?${params}`,
  )
  return res.data.data.items ?? []
}

export type FastSegmentPreset = 'soft' | 'normal' | 'strict'
export type FastSegmentLimit = 5 | 10 | 20

export interface FastSegmentPoint {
  lat: number
  lon: number
  recorded_at: string
}

export interface FastSegment {
  rank: number
  device_id: string
  started_at: string
  ended_at: string
  duration_sec: number
  distance_m: number
  avg_speed_mps: number
  baseline_speed_mps: number
  uplift_percent: number
  points: FastSegmentPoint[]
}

export interface FastSegmentsFilter {
  device_id?: string
  from: string
  to: string
  preset: FastSegmentPreset
  limit: FastSegmentLimit
}

export async function getFastSegments(filter: FastSegmentsFilter): Promise<FastSegment[]> {
  const params = new URLSearchParams({
    from: filter.from,
    to: filter.to,
    preset: filter.preset,
    limit: String(filter.limit),
  })
  if (filter.device_id) params.set('device_id', filter.device_id)
  const res = await apiClient.get<{ data: { items: FastSegment[] }; error: string | null }>(
    `/v1/tracking/fast-segments?${params}`,
  )
  return res.data.data.items ?? []
}
