export type WsStatus = 'disconnected' | 'connecting' | 'connected';

export interface TrackingStatus {
  serviceRunning: boolean;
  wsStatus: WsStatus;
  wsLastError: string | null;
  pendingCount: number;
  failedCount: number;
  lastLocationTime: string | null;
  lastSyncTime: string | null;
}

export type PermissionState =
  | 'granted'
  | 'denied'
  | 'never_ask_again'
  | 'unknown';

export interface PermissionsStatus {
  fineLocation: PermissionState;
  backgroundLocation: PermissionState;
  notifications: PermissionState;
}
