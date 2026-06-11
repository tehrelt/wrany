import { apiClient } from '../../api/client'

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

export async function getPoints(filter: PointsFilter): Promise<PointsResponse> {
  const params = new URLSearchParams()
  if (filter.device_id) params.set('device_id', filter.device_id)
  params.set('from', filter.from)
  params.set('to', filter.to)
  if (filter.limit) params.set('limit', String(filter.limit))
  if (filter.cursor) params.set('cursor', filter.cursor)

  const res = await apiClient.get<{ data: PointsResponse }>(`/v1/tracking/points?${params}`)
  return res.data.data
}

export async function getSummary(filter: Omit<PointsFilter, 'limit' | 'cursor'>): Promise<TrackingSummary> {
  const params = new URLSearchParams()
  if (filter.device_id) params.set('device_id', filter.device_id)
  params.set('from', filter.from)
  params.set('to', filter.to)

  const res = await apiClient.get<{ data: TrackingSummary }>(`/v1/tracking/summary?${params}`)
  return res.data.data
}
