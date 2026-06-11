jest.mock('@react-native-async-storage/async-storage', () => ({
  __esModule: true,
  default: { getItem: jest.fn().mockResolvedValue(null), setItem: jest.fn() },
}));

jest.mock('react-native-geolocation-service', () => ({
  requestAuthorization: jest.fn(),
  watchPosition: jest.fn(() => 0),
  clearWatch: jest.fn(),
}));

import React from 'react';
import ReactTestRenderer from 'react-test-renderer';
import App from '../App';

test('renders without crashing', async () => {
  await ReactTestRenderer.act(() => {
    ReactTestRenderer.create(<App />);
  });
});
