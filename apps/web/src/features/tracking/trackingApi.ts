import { getTracking } from '@/api/generated/tracking/tracking'

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
