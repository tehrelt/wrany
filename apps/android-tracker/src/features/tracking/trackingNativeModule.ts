import { NativeModules } from 'react-native';
import type { TrackingStatus } from './types';

const { TrackingModule } = NativeModules as {
  TrackingModule: {
    enableTracking(
      deviceId: string,
      token: string,
      wsUrl: string,
    ): Promise<void>;
    disableTracking(): Promise<void>;
    getTrackingStatus(): Promise<TrackingStatus>;
    flushNow(): Promise<void>;
    clearFailed(): Promise<void>;
    updateToken(token: string): Promise<void>;
    cleanupOldPoints(): Promise<void>;
  };
};

export const trackingModule = TrackingModule;
