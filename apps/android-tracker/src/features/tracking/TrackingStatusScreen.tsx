import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Alert,
  Button,
  Linking,
  PermissionsAndroid,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';
import { getValidToken } from '../../api/httpClient';
import { getOrCreateDeviceId } from '../../tracker/deviceId';
import {
  clearTokens,
  getAccessToken,
  getRefreshToken,
} from '../../storage/tokenStorage';
import { apiUrlToWsUrl, getApiUrl } from '../../storage/settingsStorage';
import { syncTokensFromNative } from './tokenSync';
import { trackingModule } from './trackingNativeModule';
import type {
  PermissionsStatus,
  PermissionState,
  TrackingStatus,
} from './types';

const POLL_INTERVAL_MS = 3000;

const INITIAL_STATUS: TrackingStatus = {
  serviceRunning: false,
  wsStatus: 'disconnected',
  wsLastError: null,
  authExpired: false,
  pendingCount: 0,
  failedCount: 0,
  lastLocationTime: null,
  lastSyncTime: null,
};

const INITIAL_PERMS: PermissionsStatus = {
  fineLocation: 'unknown',
  backgroundLocation: 'unknown',
  notifications: 'unknown',
};

interface Props {
  // Called when stored credentials are gone/invalid and the user must re-auth.
  // The parent (App) clears its token state and renders AuthScreen.
  onLoggedOut: () => void;
}

export function TrackingStatusScreen({
  onLoggedOut,
}: Props): React.JSX.Element {
  const [status, setStatus] = useState<TrackingStatus>(INITIAL_STATUS);
  const [perms, setPerms] = useState<PermissionsStatus>(INITIAL_PERMS);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const refreshStatus = useCallback(async () => {
    try {
      const s = await trackingModule.getTrackingStatus();
      setStatus(s);
    } catch {
      // silently ignore poll errors
    }
  }, []);

  const checkPermissions = useCallback(async () => {
    const fine = await PermissionsAndroid.check(
      PermissionsAndroid.PERMISSIONS.ACCESS_FINE_LOCATION,
    );
    const bg = await PermissionsAndroid.check(
      'android.permission.ACCESS_BACKGROUND_LOCATION' as any,
    );
    let notifications: PermissionState = 'granted';
    if (Platform.Version >= 33) {
      const notif = await PermissionsAndroid.check(
        'android.permission.POST_NOTIFICATIONS' as any,
      );
      notifications = notif ? 'granted' : 'denied';
    }
    setPerms({
      fineLocation: fine ? 'granted' : 'denied',
      backgroundLocation: bg ? 'granted' : 'denied',
      notifications,
    });
  }, []);

  useEffect(() => {
    checkPermissions();
    refreshStatus();
    pollRef.current = setInterval(refreshStatus, POLL_INTERVAL_MS);
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [checkPermissions, refreshStatus]);

  useEffect(() => {
    // Absorb a background-rotated token pair, then (re)connect if ready.
    syncTokensFromNative().then(autoConnectIfReady);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function requestFineLocation(): Promise<boolean> {
    const result = await PermissionsAndroid.request(
      PermissionsAndroid.PERMISSIONS.ACCESS_FINE_LOCATION,
      {
        title: 'Location permission',
        message: 'WR any% needs location access to track your routes.',
        buttonPositive: 'Allow',
        buttonNegative: 'Deny',
      },
    );
    return result === PermissionsAndroid.RESULTS.GRANTED;
  }

  async function requestNotifications(): Promise<void> {
    if (Platform.Version >= 33) {
      await PermissionsAndroid.request(
        'android.permission.POST_NOTIFICATIONS' as any,
      );
    }
  }

  // Activity Recognition powers the low-power motion wake-up. Optional: if the
  // user denies it, tracking still works via sparse-GPS fallback.
  async function requestActivityRecognition(): Promise<void> {
    if (Platform.Version >= 29) {
      await PermissionsAndroid.request(
        'android.permission.ACTIVITY_RECOGNITION' as any,
      );
    }
  }

  function openBackgroundLocationSettings(): void {
    Alert.alert(
      'Background location needed',
      'To track routes when the app is in background, open Settings and select "Allow all the time" for location.',
      [
        { text: 'Cancel', style: 'cancel' },
        { text: 'Open Settings', onPress: () => Linking.openSettings() },
      ],
    );
  }

  async function autoConnectIfReady(): Promise<void> {
    try {
      const s = await trackingModule.getTrackingStatus();
      if (s.serviceRunning) {
        // Service alive but it gave up after the refresh token was rejected
        // (e.g. user re-logged in). Hand it fresh credentials and reconnect.
        if (s.authExpired) await refreshNativeCredentials();
        return;
      }
      // Cheap pre-check: skip entirely if not logged in.
      if (!(await getAccessToken())) return;
      const fineOk = await PermissionsAndroid.check(
        PermissionsAndroid.PERMISSIONS.ACCESS_FINE_LOCATION,
      );
      if (!fineOk) return;
      await startNativeTracking();
      await refreshStatus();
    } catch {
      // silent — user can enable manually
    }
  }

  // Gathers fresh credentials and hands them to the native foreground service.
  // The service refreshes the access token itself on a 401, so it also needs
  // the refresh token and API base URL.
  async function startNativeTracking(): Promise<void> {
    const [token, refreshToken, apiUrl, deviceId] = await Promise.all([
      getValidToken(),
      getRefreshToken(),
      getApiUrl(),
      getOrCreateDeviceId(),
    ]);
    await trackingModule.enableTracking(
      deviceId,
      token,
      refreshToken ?? '',
      apiUrlToWsUrl(apiUrl),
      apiUrl,
    );
  }

  // Pushes a fresh token pair into the running service and forces a reconnect.
  // Used after re-login when the service had marked itself auth-expired.
  async function refreshNativeCredentials(): Promise<void> {
    try {
      const [token, refreshToken] = await Promise.all([
        getValidToken(),
        getRefreshToken(),
      ]);
      await trackingModule.updateTokens(token, refreshToken ?? '');
      await trackingModule.reconnectWs();
    } catch {
      // refresh failed — user still needs to re-authenticate
    }
  }

  async function handleEnable(): Promise<void> {
    setError(null);
    setBusy(true);
    try {
      const fineOk = await requestFineLocation();
      if (!fineOk) {
        setError('Location permission denied. Cannot start tracking.');
        return;
      }
      await requestNotifications();
      await requestActivityRecognition();
      await checkPermissions();

      // getValidToken refreshes a missing/expired access token using the
      // stored refresh token; it only throws AuthExpiredError when refresh
      // itself is dead. A raw getAccessToken() check here would falsely report
      // "Not authenticated" whenever the access token expired but refresh works.
      try {
        await getValidToken();
      } catch {
        // Credentials are gone or unrecoverable — wipe the stale pair and
        // bounce the user to the auth screen instead of stranding them here.
        await clearTokens();
        onLoggedOut();
        return;
      }
      await startNativeTracking();
      await refreshStatus();
    } catch (e: any) {
      setError(e?.message ?? 'Failed to start tracking');
    } finally {
      setBusy(false);
    }
  }

  async function handleDisable(): Promise<void> {
    setBusy(true);
    try {
      await trackingModule.disableTracking();
      await refreshStatus();
    } catch (e: any) {
      setError(e?.message ?? 'Failed to stop tracking');
    } finally {
      setBusy(false);
    }
  }

  async function handleReconnectWs(): Promise<void> {
    setError(null);
    try {
      await trackingModule.reconnectWs();
      setTimeout(refreshStatus, 500);
    } catch (e: any) {
      setError(e?.message ?? 'Failed to reconnect');
    }
  }

  async function handleFlush(): Promise<void> {
    await trackingModule.flushNow();
  }

  async function handleClearFailed(): Promise<void> {
    await trackingModule.clearFailed();
    await refreshStatus();
  }

  async function handleRetryFailed(): Promise<void> {
    setError(null);
    try {
      const count = await trackingModule.retryFailed();
      Alert.alert('Retry failed points', `Requeued ${count} point(s).`);
      await refreshStatus();
    } catch (e: any) {
      setError(e?.message ?? 'Failed to retry points');
    }
  }

  const bgDenied = perms.backgroundLocation === 'denied';

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.title}>Background Tracking</Text>

      {/* Permissions */}
      <Section title="Permissions">
        <Row label="Fine location" value={perms.fineLocation} />
        <Row label="Background location" value={perms.backgroundLocation} />
        <Row label="Notifications" value={perms.notifications} />
        {bgDenied && (
          <View style={styles.warningBox}>
            <Text style={styles.warningText}>
              Background location not granted — tracking stops when app is
              closed.
            </Text>
            <Button
              title="Open Settings"
              onPress={openBackgroundLocationSettings}
            />
          </View>
        )}
      </Section>

      {/* Service status */}
      <Section title="Service">
        <Row label="Running" value={status.serviceRunning ? 'yes' : 'no'} />
        <Row label="WS connection" value={status.wsStatus} />
        {status.wsLastError && status.wsStatus !== 'connected' && (
          <Row label="WS error" value={status.wsLastError} />
        )}
        <Row label="Pending points" value={String(status.pendingCount)} />
        <Row label="Failed points" value={String(status.failedCount)} />
        <Row
          label="Last location"
          value={
            status.lastLocationTime ? formatTime(status.lastLocationTime) : '—'
          }
        />
        <Row
          label="Last sync"
          value={status.lastSyncTime ? formatTime(status.lastSyncTime) : '—'}
        />
      </Section>

      {/* Controls */}
      <Section title="Controls">
        <View style={styles.buttonRow}>
          <Button
            title={
              status.serviceRunning ? 'Disable tracking' : 'Enable tracking'
            }
            onPress={status.serviceRunning ? handleDisable : handleEnable}
            disabled={busy}
          />
        </View>
        <View style={styles.buttonRow}>
          <Button
            title="Reconnect WS"
            onPress={handleReconnectWs}
            disabled={!status.serviceRunning || status.wsStatus === 'connected'}
          />
        </View>
        <View style={styles.buttonRow}>
          <Button
            title="Flush now"
            onPress={handleFlush}
            disabled={!status.serviceRunning}
          />
        </View>
        <View style={styles.buttonRow}>
          <Button
            title="Retry failed points"
            onPress={handleRetryFailed}
            disabled={status.failedCount === 0}
          />
        </View>
        <View style={styles.buttonRow}>
          <Button
            title="Clear failed points"
            onPress={handleClearFailed}
            disabled={status.failedCount === 0}
          />
        </View>
      </Section>

      {error && (
        <View style={styles.errorBox}>
          <Text style={styles.errorText}>{error}</Text>
        </View>
      )}

      <Text style={styles.note}>
        Tracking is automatic — the backend detects trips, routes, and records
        from GPS points.
      </Text>
    </ScrollView>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}): React.JSX.Element {
  return (
    <View style={styles.section}>
      <Text style={styles.sectionTitle}>{title}</Text>
      {children}
    </View>
  );
}

function Row({
  label,
  value,
}: {
  label: string;
  value: string;
}): React.JSX.Element {
  const isGood =
    value === 'yes' || value === 'granted' || value === 'connected';
  const isBad =
    value === 'no' ||
    value === 'denied' ||
    value === 'never_ask_again' ||
    value === 'disconnected';
  return (
    <View style={styles.row}>
      <Text style={styles.rowLabel}>{label}</Text>
      <Text
        style={[styles.rowValue, isGood && styles.good, isBad && styles.bad]}
      >
        {value}
      </Text>
    </View>
  );
}

function formatTime(iso: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString();
  } catch {
    return iso;
  }
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#f5f5f5' },
  content: { padding: 16, paddingBottom: 40 },
  title: { fontSize: 20, fontWeight: 'bold', marginBottom: 16, color: '#111' },
  section: {
    backgroundColor: '#fff',
    borderRadius: 8,
    padding: 12,
    marginBottom: 12,
    borderWidth: 1,
    borderColor: '#e0e0e0',
  },
  sectionTitle: {
    fontSize: 13,
    fontWeight: '600',
    color: '#666',
    marginBottom: 8,
    textTransform: 'uppercase',
  },
  row: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: 4,
  },
  rowLabel: { fontSize: 14, color: '#333' },
  rowValue: { fontSize: 14, color: '#333', fontWeight: '500' },
  good: { color: '#2e7d32' },
  bad: { color: '#c62828' },
  buttonRow: { marginVertical: 4 },
  warningBox: {
    marginTop: 8,
    padding: 8,
    backgroundColor: '#fff3e0',
    borderRadius: 6,
  },
  warningText: { fontSize: 13, color: '#e65100', marginBottom: 6 },
  errorBox: {
    marginTop: 8,
    padding: 10,
    backgroundColor: '#ffebee',
    borderRadius: 6,
  },
  errorText: { color: '#b71c1c', fontSize: 13 },
  note: { marginTop: 16, fontSize: 12, color: '#888', textAlign: 'center' },
});
