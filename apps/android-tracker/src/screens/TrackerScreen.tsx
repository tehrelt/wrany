import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  ScrollView,
  StatusBar,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import { AuthExpiredError, getValidToken } from '../api/httpClient';
import { registerDevice } from '../api/deviceApi';
import { getOrCreateDeviceId } from '../tracker/deviceId';
import {
  makeSyntheticEvent,
  requestLocationPermission,
  startWatcher,
  stopWatcher,
} from '../tracker/locationService';
import {
  SocketCallbacks,
  SocketStatus,
  TrackerSocket,
} from '../tracker/trackerSocket';

interface Props {
  onSessionExpired: () => void;
}

interface Counters {
  accepted: number;
  duplicated: number;
  rejected: number;
}

export function TrackerScreen({ onSessionExpired }: Props): React.JSX.Element {
  const [deviceId, setDeviceId] = useState('');
  const [deviceRegistered, setDeviceRegistered] = useState(false);
  const [wsStatus, setWsStatus] = useState<SocketStatus>('disconnected');
  const [gpsPermission, setGpsPermission] = useState<
    'unknown' | 'granted' | 'denied'
  >('unknown');
  const [tracking, setTracking] = useState(false);
  const [counters, setCounters] = useState<Counters>({
    accepted: 0,
    duplicated: 0,
    rejected: 0,
  });
  const [pending, setPending] = useState(0);
  const [lastError, setLastError] = useState<string | null>(null);
  const [lastLocation, setLastLocation] = useState<string | null>(null);

  const socketRef = useRef<TrackerSocket | null>(null);
  const watchIdRef = useRef<number | null>(null);
  const pendingRef = useRef(0);

  useEffect(() => {
    getOrCreateDeviceId().then(setDeviceId);
    return () => {
      socketRef.current?.disconnect();
      if (watchIdRef.current != null) stopWatcher(watchIdRef.current);
    };
  }, []);

  const syncPending = useCallback(() => {
    const n = socketRef.current?.pendingCount ?? 0;
    pendingRef.current = n;
    setPending(n);
  }, []);

  const callbacks: SocketCallbacks = {
    onStatusChange: s => setWsStatus(s),
    onAck: (accepted, duplicated, rejected) => {
      setCounters(c => ({
        accepted: c.accepted + accepted.length,
        duplicated: c.duplicated + duplicated.length,
        rejected: c.rejected + rejected.length,
      }));
      syncPending();
    },
    onError: (code, msg) => setLastError(`${code}: ${msg}`),
  };

  async function handleRegisterDevice(): Promise<void> {
    setLastError(null);
    try {
      await registerDevice(deviceId);
      setDeviceRegistered(true);
    } catch (e) {
      if (e instanceof AuthExpiredError) {
        onSessionExpired();
        return;
      }
      setLastError(
        e instanceof Error ? e.message : 'Device registration failed',
      );
    }
  }

  async function handleConnect(): Promise<void> {
    let currentToken: string;
    try {
      // getValidToken proactively refreshes if the stored token expires within 60s.
      currentToken = await getValidToken();
    } catch {
      onSessionExpired();
      return;
    }
    if (!socketRef.current) {
      socketRef.current = new TrackerSocket(callbacks);
    }
    socketRef.current.connect(currentToken, deviceId);
  }

  function handleDisconnect(): void {
    socketRef.current?.disconnect();
    socketRef.current = null;
  }

  async function handleStartTracking(): Promise<void> {
    setLastError(null);
    const granted = await requestLocationPermission();
    if (!granted) {
      setGpsPermission('denied');
      setLastError('LOCATION_PERMISSION_DENIED: grant location permission');
      return;
    }
    setGpsPermission('granted');
    setTracking(true);
    watchIdRef.current = startWatcher(
      deviceId,
      event => {
        setLastLocation(
          `${event.lat.toFixed(5)}, ${event.lon.toFixed(
            5,
          )} ±${event.accuracy_m.toFixed(0)}m`,
        );
        socketRef.current?.enqueue(event);
        syncPending();
      },
      msg => setLastError(msg),
    );
  }

  function handleStopTracking(): void {
    setTracking(false);
    if (watchIdRef.current != null) {
      stopWatcher(watchIdRef.current);
      watchIdRef.current = null;
    }
  }

  function handleSendTestPoint(): void {
    if (!socketRef.current) {
      setLastError('Connect WebSocket first');
      return;
    }
    const event = makeSyntheticEvent(deviceId);
    socketRef.current.enqueue(event);
    syncPending();
  }

  return (
    <ScrollView style={s.scroll} contentContainerStyle={s.container}>
      <Text style={s.title}>WR any% Debug</Text>

      <Section label="Status">
        <Row label="Device ID" value={deviceId || '…'} mono />
        <Row
          label="Device registered"
          value={deviceRegistered ? 'yes' : 'no'}
        />
        <Row label="WS status" value={wsStatus} />
        <Row label="GPS permission" value={gpsPermission} />
        {lastLocation && (
          <Row label="Last location" value={lastLocation} mono />
        )}
      </Section>

      <Section label="Counters">
        <Row label="Pending" value={String(pending)} />
        <Row label="Accepted" value={String(counters.accepted)} />
        <Row label="Duplicated" value={String(counters.duplicated)} />
        <Row label="Rejected" value={String(counters.rejected)} />
      </Section>

      {lastError && (
        <View style={s.errorBox}>
          <Text style={s.errorText}>{lastError}</Text>
        </View>
      )}

      <Section label="Actions">
        <Btn
          label="Register Device"
          onPress={handleRegisterDevice}
          disabled={!deviceId || deviceRegistered}
        />
        <Btn
          label={wsStatus === 'disconnected' ? 'Connect WS' : 'Disconnect WS'}
          onPress={
            wsStatus === 'disconnected' ? handleConnect : handleDisconnect
          }
          disabled={!deviceId}
        />
        <Btn
          label={tracking ? 'Stop Tracking' : 'Start Tracking'}
          onPress={tracking ? handleStopTracking : handleStartTracking}
          disabled={wsStatus !== 'session_accepted'}
          accent={tracking}
        />
        <Btn
          label="Send Test Point"
          onPress={handleSendTestPoint}
          disabled={wsStatus !== 'session_accepted'}
        />
      </Section>
    </ScrollView>
  );
}

function Section({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <View style={s.section}>
      <Text style={s.sectionLabel}>{label}</Text>
      {children}
    </View>
  );
}

function Row({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <View style={s.row}>
      <Text style={s.rowLabel}>{label}</Text>
      <Text style={[s.rowValue, mono && s.mono]} numberOfLines={1}>
        {value}
      </Text>
    </View>
  );
}

function Btn({
  label,
  onPress,
  disabled,
  accent,
}: {
  label: string;
  onPress: () => void;
  disabled?: boolean;
  accent?: boolean;
}) {
  return (
    <TouchableOpacity
      style={[s.btn, disabled && s.btnDisabled, accent && s.btnAccent]}
      onPress={onPress}
      disabled={disabled}
    >
      <Text style={s.btnText}>{label}</Text>
    </TouchableOpacity>
  );
}

const s = StyleSheet.create({
  scroll: { flex: 1, backgroundColor: '#f9fafb' },
  container: {
    padding: 16,
    paddingTop: (StatusBar.currentHeight ?? 24) + 8,
    paddingBottom: 40,
  },
  title: {
    fontSize: 20,
    fontWeight: '700',
    marginBottom: 16,
    textAlign: 'center',
  },
  section: {
    backgroundColor: '#fff',
    borderRadius: 10,
    padding: 12,
    marginBottom: 12,
    shadowColor: '#000',
    shadowOpacity: 0.05,
    shadowRadius: 4,
    elevation: 2,
  },
  sectionLabel: {
    fontSize: 13,
    fontWeight: '600',
    color: '#6b7280',
    marginBottom: 8,
    textTransform: 'uppercase',
  },
  row: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: 4,
  },
  rowLabel: { color: '#374151', flex: 1 },
  rowValue: {
    color: '#111827',
    fontWeight: '500',
    flex: 2,
    textAlign: 'right',
  },
  mono: { fontFamily: 'monospace', fontSize: 12 },
  errorBox: {
    backgroundColor: '#fee2e2',
    borderRadius: 8,
    padding: 10,
    marginBottom: 12,
  },
  errorText: { color: '#dc2626', fontSize: 13 },
  btn: {
    backgroundColor: '#2563eb',
    borderRadius: 8,
    padding: 12,
    alignItems: 'center',
    marginBottom: 8,
  },
  btnDisabled: { backgroundColor: '#d1d5db' },
  btnAccent: { backgroundColor: '#dc2626' },
  btnText: { color: '#fff', fontWeight: '600' },
});
