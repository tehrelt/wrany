import { BatchQueue } from '../src/tracker/batchQueue';
import { LocationEvent } from '../src/tracker/types';

function makeEvent(id: string): LocationEvent {
  return {
    event_id: id,
    recorded_at: new Date().toISOString(),
    lat: 55.0,
    lon: 37.0,
    accuracy_m: 10,
  };
}

describe('BatchQueue', () => {
  it('flushes when batch size reached', () => {
    const flushed: LocationEvent[][] = [];
    const q = new BatchQueue({ onFlush: b => flushed.push(b) });

    for (let i = 0; i < 50; i++) q.enqueue(makeEvent(`e${i}`));

    expect(flushed.length).toBe(1);
    expect(flushed[0]).toHaveLength(50);
  });

  it('flushes remaining on stop', () => {
    const flushed: LocationEvent[][] = [];
    const q = new BatchQueue({ onFlush: b => flushed.push(b) });
    q.enqueue(makeEvent('e1'));
    q.enqueue(makeEvent('e2'));
    q.stop();
    expect(flushed[0]).toHaveLength(2);
  });

  it('ack removes accepted and duplicated from pending', () => {
    const q = new BatchQueue({ onFlush: () => {} });
    q.enqueue(makeEvent('e1'));
    q.enqueue(makeEvent('e2'));
    q.enqueue(makeEvent('e3'));
    q.ack(['e1'], ['e2']);
    expect(q.pendingCount).toBe(1);
  });

  it('ack does not remove rejected events', () => {
    const q = new BatchQueue({ onFlush: () => {} });
    q.enqueue(makeEvent('e1'));
    q.enqueue(makeEvent('e2'));
    q.ack([], []);
    expect(q.pendingCount).toBe(2);
  });

  it('keeps pending when no ack received', () => {
    const q = new BatchQueue({ onFlush: () => {} });
    q.enqueue(makeEvent('e1'));
    expect(q.pendingCount).toBe(1);
  });
});
