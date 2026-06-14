import { apiClient } from '@/api/client'
import { getRoutes } from '@/api/generated/routes/routes'
import { formatDistance, formatDuration } from '@/features/trips/tripsApi'
import type {
  HttpRouteResultResponse as RouteResultResponse,
  HttpTripAttemptItem as TripAttemptItem,
} from '@/api/generated/model'

export { formatDistance, formatDuration }
export type { RouteResultResponse, TripAttemptItem }

export interface Route {
  id: string
  user_id: string
  name: string | null
  status: string
  start_lat: number
  start_lon: number
  end_lat: number
  end_lon: number
  distance_m: number
  trips_count: number
  created_at: string
  updated_at: string
}

export interface RouteTrip {
  trip_id: string
  match_score: number
  matched_at: string
  duration_sec: number
  distance_m: number
  started_at: string
  ended_at: string | null
}

export interface RoutePoint {
  lat: number
  lon: number
}

export interface RoutesResponse {
  items: Route[]
  next_cursor: string | null
}

export interface RouteTripsResponse {
  items: RouteTrip[]
  next_cursor: string | null
}

export async function listRoutes(params?: {
  device_id?: string
  limit?: number
  cursor?: string
}): Promise<RoutesResponse> {
  const res = await apiClient.get<{ data: RoutesResponse }>('/v1/routes', { params })
  return res.data.data
}

export async function getRoute(routeId: string): Promise<Route> {
  const res = await apiClient.get<{ data: Route }>(`/v1/routes/${routeId}`)
  return res.data.data
}

export async function getRouteTrips(
  routeId: string,
  params?: { limit?: number; cursor?: string },
): Promise<RouteTripsResponse> {
  const res = await apiClient.get<{ data: RouteTripsResponse }>(
    `/v1/routes/${routeId}/trips`,
    { params },
  )
  return res.data.data
}

export async function getRoutePoints(routeId: string): Promise<RoutePoint[]> {
  const res = await apiClient.get<{ data: RoutePoint[] }>(`/v1/routes/${routeId}/points`)
  return res.data.data
}

export async function getRouteResults(routeId: string): Promise<RouteResultResponse> {
  const res = await apiClient.get<{ data: RouteResultResponse }>(`/v1/routes/${routeId}/results`)
  return res.data.data
}

export async function deleteRoute(routeId: string): Promise<void> {
  await getRoutes().deleteV1RoutesId(routeId)
}

export interface RouteAttemptsResponse {
  items: TripAttemptItem[]
  next_cursor: string | null
}

export async function getRouteAttempts(
  routeId: string,
  params?: { limit?: number; cursor?: string },
): Promise<RouteAttemptsResponse> {
  const res = await apiClient.get<{ data: RouteAttemptsResponse }>(
    `/v1/routes/${routeId}/attempts`,
    { params },
  )
  return res.data.data
}
