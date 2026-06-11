import { getDevices } from '@/api/generated/devices/devices'

export interface Device {
  id: string
  device_id: string
  name: string | null
  platform: string | null
  last_seen_at: string
  created_at: string
}

const api = getDevices()

export async function listDevices(): Promise<Device[]> {
  const env = await api.getV1Devices()
  return (env.data ?? []) as Device[]
}
