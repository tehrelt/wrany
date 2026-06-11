jest.mock('@react-native-async-storage/async-storage', () => ({
  __esModule: true,
  default: { getItem: jest.fn(), setItem: jest.fn() },
}));

import AsyncStorage from '@react-native-async-storage/async-storage';
import { getOrCreateDeviceId } from '../src/tracker/deviceId';

const getItem = AsyncStorage.getItem as jest.Mock;
const setItem = AsyncStorage.setItem as jest.Mock;

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

// Note: `cached` module var persists across tests. Tests are written to
// reflect real call-sequence behavior rather than resetting module state.

describe('deviceId', () => {
  it('generates a UUID v4, persists it, and returns the same value in the same process', async () => {
    getItem.mockResolvedValue(null);
    setItem.mockResolvedValue(undefined);

    const id1 = await getOrCreateDeviceId();
    expect(id1).toMatch(UUID_RE);
    expect(setItem).toHaveBeenCalledWith('@wrany/device_id', id1);

    // Second call returns identical value (module-level cache)
    const id2 = await getOrCreateDeviceId();
    expect(id2).toBe(id1);
    // setItem called only once — cache hit on second call
    expect(setItem).toHaveBeenCalledTimes(1);
  });
});
