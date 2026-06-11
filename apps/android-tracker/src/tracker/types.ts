// WebSocket protocol types — must match EPIC 4 spec exactly.

export interface WsMessage<T = unknown> {
  type: string;
  request_id?: string;
  payload?: T;
}

// Client → Server

export interface SessionStartPayload {
  device_id: string;
  app_version: string;
  platform: 'android';
}

export interface LocationEvent {
  event_id: string;
  recorded_at: string; // RFC3339
  lat: number;
  lon: number;
  accuracy_m: number;
  speed_mps?: number;
  bearing_deg?: number;
  activity_type?: 'walking' | 'running' | 'bicycle' | 'vehicle' | 'stationary' | 'unknown';
  activity_confidence?: number;
  battery_level?: number;
}

export interface LocationBatchPayload {
  device_id: string;
  events: LocationEvent[];
}

// Server → Client

export interface SessionAcceptedPayload {
  session_id: string;
  server_time: string;
  config: {
    max_batch_size: number;
    recommended_flush_interval_sec: number;
  };
}

export interface RejectedEvent {
  event_id: string;
  reason: string;
}

export interface LocationBatchAckPayload {
  accepted: string[];
  duplicated: string[];
  rejected: RejectedEvent[];
}

export interface ErrorPayload {
  code: string;
  message: string;
}
