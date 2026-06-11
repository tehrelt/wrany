import { PermissionsAndroid, Platform } from 'react-native';
import Geolocation from 'react-native-geolocation-service';
import { LocationEvent } from './types';

export async function requestLocationPermission(): Promise<boolean> {
  if (Platform.OS !== 'android') return true;
  const result = await PermissionsAndroid.request(
    PermissionsAndroid.PERMISSIONS.ACCESS_FINE_LOCATION,
    {
      title: 'Location Permission',
      message: 'WR any% needs location access to record your route.',
      buttonNeutral: 'Ask Later',
      buttonNegative: 'Deny',
      buttonPositive: 'Allow',
    },
  );
  return result === PermissionsAndroid.RESULTS.GRANTED;
}

function toRFC3339(ts: number): string {
  return new Date(ts).toISOString();
}

function makeEventId(ts: number): string {
  return `evt_${ts}_${Math.random().toString(36).slice(2, 8)}`;
}

export function startWatcher(
  deviceId: string,
  onEvent: (e: LocationEvent) => void,
  onError: (msg: string) => void,
): number {
  return Geolocation.watchPosition(
    pos => {
      const { latitude, longitude, accuracy, speed, heading } = pos.coords;
      const event: LocationEvent = {
        event_id: makeEventId(pos.timestamp),
        recorded_at: toRFC3339(pos.timestamp),
        lat: latitude,
        lon: longitude,
        accuracy_m: accuracy ?? 0,
        speed_mps: speed != null && speed >= 0 ? speed : undefined,
        bearing_deg: heading != null && heading >= 0 ? heading : undefined,
        activity_type: 'unknown',
      };
      onEvent(event);
    },
    err => onError(`GPS error ${err.code}: ${err.message}`),
    {
      enableHighAccuracy: true,
      distanceFilter: 5,
      interval: 3000,
      fastestInterval: 1000,
    },
  );
}

export function stopWatcher(watchId: number): void {
  Geolocation.clearWatch(watchId);
}

export function makeSyntheticEvent(deviceId: string): LocationEvent {
  const ts = Date.now();
  return {
    event_id: makeEventId(ts),
    recorded_at: toRFC3339(ts),
    lat: 55.751244,
    lon: 37.618423,
    accuracy_m: 10,
    activity_type: 'unknown',
  };
}
