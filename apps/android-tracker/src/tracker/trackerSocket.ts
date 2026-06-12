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
  onAck: (
    accepted: string[],
    duplicated: string[],
    rejected: Array<{ event_id: string; reason: string }>,
  ) => void;
  onError: (code: string, message: string) => void;
  onDisconnect?: (code: number, reason: string) => void;
}

const CONNECT_TIMEOUT_MS = 8000;

export class TrackerSocket {
  private ws: WebSocket | null = null;
  private deviceId = '';
  private sessionAccepted = false;
  private connectTimer: ReturnType<typeof setTimeout> | null = null;
  private readonly queue: BatchQueue;
  private readonly callbacks: SocketCallbacks;

  constructor(callbacks: SocketCallbacks) {
    this.callbacks = callbacks;
    this.queue = new BatchQueue({
      onFlush: batch => this.sendBatch(batch),
    });
  }

  connect(token: string, deviceId: string, wsUrl: string): void {
    if (this.ws) return;
    this.deviceId = deviceId;
    this.callbacks.onStatusChange('connecting');

    // React Native Android WebSocket does not support custom headers on upgrade.
    // Pass token as query param — accepted by WSAuthMiddleware (EPIC 06, T02).
    this.ws = new WebSocket(`${wsUrl}?access_token=${token}`);

    this.connectTimer = setTimeout(() => {
      if (this.ws && this.ws.readyState !== WebSocket.OPEN) {
        this.ws.close();
        this.callbacks.onDisconnect?.(1006, 'connect timeout (8s)');
        this.ws = null;
        this.sessionAccepted = false;
        this.queue.stop();
        this.callbacks.onStatusChange('disconnected');
      }
    }, CONNECT_TIMEOUT_MS);

    this.ws.onopen = () => {
      this.clearConnectTimer();
      this.callbacks.onStatusChange('connected');
      this.sendSessionStart();
    };

    this.ws.onmessage = e => this.handleMessage(e.data as string);

    this.ws.onerror = (e: Event) => {
      const msg =
        (e as unknown as { message?: string }).message ?? 'WebSocket error';
      console.error('[WS] onerror:', msg, e);
      this.callbacks.onError('WS_ERROR', msg);
    };

    this.ws.onclose = (e: Event) => {
      this.clearConnectTimer();
      const ev = e as unknown as { code?: number; reason?: string };
      const code = ev.code ?? 1006;
      const reason = ev.reason || closeCodeLabel(code);
      this.callbacks.onDisconnect?.(code, reason);
      this.ws = null;
      this.sessionAccepted = false;
      this.queue.stop();
      this.callbacks.onStatusChange('disconnected');
    };
  }

  disconnect(): void {
    this.clearConnectTimer();
    this.queue.stop();
    this.ws?.close();
  }

  private clearConnectTimer(): void {
    if (this.connectTimer !== null) {
      clearTimeout(this.connectTimer);
      this.connectTimer = null;
    }
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
    this.send({
      type: 'location.batch',
      request_id: `batch_${Date.now()}`,
      payload,
    });
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
        this.callbacks.onAck(
          p.accepted ?? [],
          p.duplicated ?? [],
          p.rejected ?? [],
        );
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

function closeCodeLabel(code: number): string {
  switch (code) {
    case 1000:
      return 'normal closure';
    case 1001:
      return 'server going away';
    case 1006:
      return 'connection refused or network unreachable';
    case 1008:
      return 'policy violation';
    case 1011:
      return 'server error';
    case 4001:
      return 'unauthorized (invalid token)';
    case 4003:
      return 'forbidden';
    default:
      return `close code ${code}`;
  }
}
