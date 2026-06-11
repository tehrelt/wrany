import { apiClient } from '../../api/client'

export interface Device {
  id: string
  device_id: string
  name: string | null
  platform: string | null
  last_seen_at: string
  created_at: string
}

export async function listDevices(): Promise<Device[]> {
  const res = await apiClient.get<{ data: Device[] }>('/v1/devices')
  return res.data.data ?? []
}
