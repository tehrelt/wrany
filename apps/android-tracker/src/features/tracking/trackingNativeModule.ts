import { NativeModules } from 'react-native';
import type { TrackingStatus } from './types';

const { TrackingModule } = NativeModules as {
  TrackingModule: {
    enableTracking(
      deviceId: string,
      token: string,
      refreshToken: string,
      wsUrl: string,
      apiUrl: string,
    ): Promise<void>;
    disableTracking(): Promise<void>;
    getTrackingStatus(): Promise<TrackingStatus>;
    reconnectWs(): Promise<void>;
    flushNow(): Promise<void>;
    clearFailed(): Promise<void>;
    updateToken(token: string): Promise<void>;
    updateTokens(accessToken: string, refreshToken: string): Promise<void>;
    getStoredTokens(): Promise<{
      accessToken: string | null;
      refreshToken: string | null;
    }>;
    cleanupOldPoints(): Promise<void>;
  };
};

export const trackingModule = TrackingModule;
