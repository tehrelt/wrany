import { WS_URL } from '../config/env';
import { BatchQueue } from './batchQueue';
import {
  LocationBatchAckPayload,
  LocationBatchPayload,
  LocationEvent,
  SessionAcceptedPayload,
  SessionStartPayload,
  WsMessage,
} from './types';

export type SocketStatus =
  | 'disconnected'
  | 'connecting'
  | 'connected'
  | 'session_accepted';

export interface SocketCallbacks {
  onStatusChange: (s: SocketStatus) => void;
  onAck: (accepted: string[], duplicated: string[], rejected: Array<{event_id: string; reason: string}>) => void;
  onError: (code: string, message: string) => void;
}

export class TrackerSocket {
  private ws: WebSocket | null = null;
  private deviceId = '';
  private sessionAccepted = false;
  private readonly queue: BatchQueue;
  private readonly callbacks: SocketCallbacks;

  constructor(callbacks: SocketCallbacks) {
    this.callbacks = callbacks;
    this.queue = new BatchQueue({
      onFlush: batch => this.sendBatch(batch),
    });
  }

  connect(token: string, deviceId: string): void {
    if (this.ws) return;
    this.deviceId = deviceId;
    this.callbacks.onStatusChange('connecting');

    // React Native Android WebSocket does not support custom headers on upgrade.
    // Pass token as query param — accepted by WSAuthMiddleware (EPIC 06, T02).
    this.ws = new WebSocket(`${WS_URL}?access_token=${token}`);

    this.ws.onopen = () => {
      this.callbacks.onStatusChange('connected');
      this.sendSessionStart();
    };

    this.ws.onmessage = e => this.handleMessage(e.data as string);

    this.ws.onerror = () => {
      this.callbacks.onError('WS_ERROR', 'WebSocket error');
    };

    this.ws.onclose = () => {
      this.ws = null;
      this.sessionAccepted = false;
      this.queue.stop();
      this.callbacks.onStatusChange('disconnected');
    };
  }

  disconnect(): void {
    this.queue.stop();
    this.ws?.close();
  }

  enqueue(event: LocationEvent): void {
    if (!this.sessionAccepted) return;
    this.queue.enqueue(event);
  }

  private sendSessionStart(): void {
    const payload: SessionStartPayload = {
      device_id: this.deviceId,
      app_version: '0.1.0',
      platform: 'android',
    };
    this.send({ type: 'session.start', request_id: 'req_session', payload });
  }

  private sendBatch(events: LocationEvent[]): void {
    if (!this.ws || !this.sessionAccepted || events.length === 0) return;
    const payload: LocationBatchPayload = { device_id: this.deviceId, events };
    this.send({ type: 'location.batch', request_id: `batch_${Date.now()}`, payload });
  }

  private handleMessage(raw: string): void {
    let msg: WsMessage;
    try {
      msg = JSON.parse(raw) as WsMessage;
    } catch {
      return;
    }

    switch (msg.type) {
      case 'session.accepted': {
        const p = msg.payload as SessionAcceptedPayload;
        this.sessionAccepted = true;
        this.callbacks.onStatusChange('session_accepted');
        this.queue.start();
        // Respect server-recommended flush interval if available
        void p;
        break;
      }
      case 'location.batch.ack': {
        const p = msg.payload as LocationBatchAckPayload;
        this.queue.ack(p.accepted ?? [], p.duplicated ?? []);
        this.callbacks.onAck(p.accepted ?? [], p.duplicated ?? [], p.rejected ?? []);
        break;
      }
      case 'error': {
        const p = msg.payload as { code: string; message: string };
        this.callbacks.onError(p.code, p.message);
        break;
      }
      case 'ping':
        this.send({ type: 'pong', request_id: msg.request_id });
        break;
    }
  }

  private send(msg: WsMessage): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  get pendingCount(): number {
    return this.queue.pendingCount;
  }
}
