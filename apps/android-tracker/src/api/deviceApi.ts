import { apiFetch } from './httpClient';

export interface Device {
  id: string;
  device_id: string;
  name: string | null;
  platform: string | null;
  created_at: string;
}

export function registerDevice(deviceId: string): Promise<Device> {
  return apiFetch<Device>('/v1/devices/register', {
    method: 'POST',
    body: JSON.stringify({ device_id: deviceId, platform: 'android' }),
  });
}
