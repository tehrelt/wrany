import { LocationEvent } from './types';

export const MAX_BATCH_SIZE = 50;
export const FLUSH_INTERVAL_MS = 10_000;

export interface BatchQueueCallbacks {
  onFlush: (events: LocationEvent[]) => void;
}

export class BatchQueue {
  private pending: LocationEvent[] = [];
  private timer: ReturnType<typeof setInterval> | null = null;
  private readonly callbacks: BatchQueueCallbacks;

  constructor(callbacks: BatchQueueCallbacks) {
    this.callbacks = callbacks;
  }

  start(): void {
    if (this.timer) return;
    this.timer = setInterval(() => this.flush(), FLUSH_INTERVAL_MS);
  }

  stop(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
    this.flush();
  }

  enqueue(event: LocationEvent): void {
    this.pending.push(event);
    if (this.pending.length >= MAX_BATCH_SIZE) {
      this.flush();
    }
  }

  flush(): void {
    if (this.pending.length === 0) return;
    const batch = this.pending.splice(0, MAX_BATCH_SIZE);
    this.callbacks.onFlush(batch);
  }

  ack(accepted: string[], duplicated: string[]): void {
    const done = new Set([...accepted, ...duplicated]);
    this.pending = this.pending.filter(e => !done.has(e.event_id));
  }

  get pendingCount(): number {
    return this.pending.length;
  }
}
