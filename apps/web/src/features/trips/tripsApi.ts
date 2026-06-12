import { apiClient } from '@/api/client'

export interface Trip {
  id: string
  user_id: string
  device_id: string
  status: 'TRIP_ACTIVE' | 'TRIP_COMPLETED'
  started_at: string
  ended_at: string | null
  start_lat: number
  start_lon: number
  end_lat: number | null
  end_lon: number | null
  distance_m: number
  duration_sec: number
  points_count: number
  created_at: string
  updated_at: string
}

export interface TripPoint {
  event_id: string
  trip_id: string
  recorded_at: string
  lat: number
  lon: number
}

export interface TripsResponse {
  items: Trip[]
  next_cursor: string | null
}

export interface TripPointsResponse {
  items: TripPoint[]
  next_cursor: string | null
}

export async function listTrips(params?: {
  status?: string
  device_id?: string
  limit?: number
  cursor?: string
}): Promise<TripsResponse> {
  const res = await apiClient.get<{ data: TripsResponse }>('/v1/trips', { params })
  return res.data.data
}

export async function getTripPoints(
  tripId: string,
  params?: { limit?: number; cursor?: string },
): Promise<TripPointsResponse> {
  const res = await apiClient.get<{ data: TripPointsResponse }>(
    `/v1/trips/${tripId}/points`,
    { params },
  )
  return res.data.data
}

// Helpers

export function formatDuration(sec: number): string {
  if (sec < 60) return `${sec}s`
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

export function formatDistance(m: number): string {
  if (m < 1000) return `${Math.round(m)} m`
  return `${(m / 1000).toFixed(2)} km`
}
