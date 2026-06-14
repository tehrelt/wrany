export type WsStatus = 'disconnected' | 'connecting' | 'connected';

export interface TrackingStatus {
  serviceRunning: boolean;
  wsStatus: WsStatus;
  wsLastError: string | null;
  // True when the service gave up reconnecting because the refresh token was
  // rejected. JS must push fresh credentials and reconnect after re-login.
  authExpired: boolean;
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
