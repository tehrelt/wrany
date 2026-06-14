import React, { useCallback, useEffect, useState } from 'react';
import {
  ActivityIndicator,
  Linking,
  PermissionsAndroid,
  Platform,
  ScrollView,
  StatusBar,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from 'react-native';
import {
  DEFAULT_API_URL,
  getApiUrl,
  saveApiUrl,
} from '../storage/settingsStorage';

type TestStatus = 'idle' | 'testing' | 'ok' | 'error';

type PermState = 'granted' | 'denied' | 'n/a';

interface PermDescriptor {
  key: string;
  label: string;
  permission: string;
  // Minimum API level at which the permission applies; below it the OS grants it
  // implicitly, so we report it as not-applicable rather than denied.
  minSdk?: number;
  // Runtime (dangerous) permissions can be requested; install-time ones cannot.
  runtime: boolean;
}

const PERMISSIONS: PermDescriptor[] = [
  {
    key: 'fine',
    label: 'Fine location',
    permission: PermissionsAndroid.PERMISSIONS.ACCESS_FINE_LOCATION,
    runtime: true,
  },
  {
    key: 'coarse',
    label: 'Coarse location',
    permission: PermissionsAndroid.PERMISSIONS.ACCESS_COARSE_LOCATION,
    runtime: true,
  },
  {
    key: 'background',
    label: 'Background location',
    permission: 'android.permission.ACCESS_BACKGROUND_LOCATION',
    minSdk: 29,
    runtime: true,
  },
  {
    key: 'notifications',
    label: 'Notifications',
    permission: 'android.permission.POST_NOTIFICATIONS',
    minSdk: 33,
    runtime: true,
  },
  {
    key: 'activity',
    label: 'Activity recognition',
    permission: 'android.permission.ACTIVITY_RECOGNITION',
    minSdk: 29,
    runtime: true,
  },
  {
    key: 'internet',
    label: 'Internet',
    permission: 'android.permission.INTERNET',
    runtime: false,
  },
  {
    key: 'network_state',
    label: 'Network state',
    permission: 'android.permission.ACCESS_NETWORK_STATE',
    runtime: false,
  },
  {
    key: 'fg_service',
    label: 'Foreground service',
    permission: 'android.permission.FOREGROUND_SERVICE',
    runtime: false,
  },
  {
    key: 'fg_service_location',
    label: 'Foreground service (location)',
    permission: 'android.permission.FOREGROUND_SERVICE_LOCATION',
    minSdk: 34,
    runtime: false,
  },
];

export function SettingsScreen(): React.JSX.Element {
  const [url, setUrl] = useState('');
  const [saved, setSaved] = useState(false);
  const [testStatus, setTestStatus] = useState<TestStatus>('idle');
  const [testMsg, setTestMsg] = useState('');
  const [testedUrl, setTestedUrl] = useState('');
  const [perms, setPerms] = useState<Record<string, PermState>>({});

  const checkPermissions = useCallback(async () => {
    const apiLevel = Number(Platform.Version);
    const next: Record<string, PermState> = {};
    for (const p of PERMISSIONS) {
      if (p.minSdk && apiLevel < p.minSdk) {
        next[p.key] = 'n/a';
        continue;
      }
      try {
        const granted = await PermissionsAndroid.check(
          p.permission as Parameters<typeof PermissionsAndroid.check>[0],
        );
        next[p.key] = granted ? 'granted' : 'denied';
      } catch {
        next[p.key] = 'n/a';
      }
    }
    setPerms(next);
  }, []);

  async function requestPermissions(): Promise<void> {
    const apiLevel = Number(Platform.Version);
    const applicable = (p: PermDescriptor): boolean =>
      p.runtime && (!p.minSdk || apiLevel >= p.minSdk);

    // Background location must be requested on its own, after foreground location.
    const foreground = PERMISSIONS.filter(
      p => applicable(p) && p.key !== 'background',
    ).map(p => p.permission as keyof typeof PermissionsAndroid.PERMISSIONS);
    if (foreground.length > 0) {
      await PermissionsAndroid.requestMultiple(foreground as never);
    }
    const background = PERMISSIONS.find(p => p.key === 'background');
    if (background && applicable(background)) {
      await PermissionsAndroid.request(background.permission as never);
    }
    await checkPermissions();
  }

  useEffect(() => {
    getApiUrl().then(setUrl);
    checkPermissions();
  }, [checkPermissions]);

  async function handleSave(): Promise<void> {
    const trimmed = url.trim();
    if (!trimmed) return;
    await saveApiUrl(trimmed);
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  }

  async function handleTest(): Promise<void> {
    const trimmed = url.trim() || DEFAULT_API_URL;
    const endpoint = `${trimmed}/healthz`;
    setTestStatus('testing');
    setTestMsg('');
    setTestedUrl(endpoint);
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 5000);
    try {
      const res = await fetch(endpoint, { signal: controller.signal });
      clearTimeout(timer);
      if (res.ok) {
        setTestStatus('ok');
        setTestMsg(`HTTP ${res.status} — соединение успешно`);
      } else {
        setTestStatus('error');
        setTestMsg(`HTTP ${res.status}`);
      }
    } catch (e) {
      clearTimeout(timer);
      setTestStatus('error');
      setTestMsg(e instanceof Error ? e.message : 'Ошибка соединения');
    }
  }

  function handleReset(): void {
    setUrl(DEFAULT_API_URL);
    setTestStatus('idle');
    setTestMsg('');
    setTestedUrl('');
  }

  return (
    <ScrollView style={s.scroll} contentContainerStyle={s.container}>
      <Text style={s.title}>Settings</Text>

      <View style={s.card}>
        <Text style={s.label}>API URL</Text>
        <TextInput
          style={s.input}
          value={url}
          onChangeText={v => {
            setUrl(v);
            setTestStatus('idle');
            setTestMsg('');
          }}
          autoCapitalize="none"
          autoCorrect={false}
          keyboardType="url"
          placeholder={DEFAULT_API_URL}
          placeholderTextColor="#9ca3af"
        />
        <Text style={s.hint}>
          Эмулятор: http://10.0.2.2:8080 · Устройство:
          http://&lt;LAN-IP&gt;:8080
        </Text>

        <View style={s.row}>
          <TouchableOpacity
            style={[s.btn, s.btnSecondary]}
            onPress={handleReset}
          >
            <Text style={s.btnSecondaryText}>Сбросить</Text>
          </TouchableOpacity>
          <TouchableOpacity style={s.btn} onPress={handleSave}>
            <Text style={s.btnText}>{saved ? 'Сохранено ✓' : 'Сохранить'}</Text>
          </TouchableOpacity>
        </View>
      </View>

      <View style={s.card}>
        <Text style={s.label}>Тест соединения</Text>
        <Text style={s.hint}>GET {url.trim() || DEFAULT_API_URL}/healthz</Text>

        <TouchableOpacity
          style={[s.btn, s.btnTest, testStatus === 'testing' && s.btnDisabled]}
          onPress={handleTest}
          disabled={testStatus === 'testing'}
        >
          {testStatus === 'testing' ? (
            <ActivityIndicator color="#fff" size="small" />
          ) : (
            <Text style={s.btnText}>Проверить соединение</Text>
          )}
        </TouchableOpacity>

        {testStatus !== 'idle' && testStatus !== 'testing' && (
          <View
            style={[
              s.resultBox,
              testStatus === 'ok' ? s.resultOk : s.resultError,
            ]}
          >
            <Text style={s.resultUrl}>→ {testedUrl}</Text>
            <Text
              style={[
                s.resultText,
                testStatus === 'ok' ? s.resultOkText : s.resultErrorText,
              ]}
            >
              {testMsg}
            </Text>
          </View>
        )}
      </View>

      <View style={s.card}>
        <Text style={s.label}>Permissions</Text>
        {PERMISSIONS.map(p => (
          <PermRow key={p.key} label={p.label} state={perms[p.key] ?? 'n/a'} />
        ))}

        <View style={[s.row, s.permButtons]}>
          <TouchableOpacity
            style={[s.btn, s.btnSecondary]}
            onPress={checkPermissions}
          >
            <Text style={s.btnSecondaryText}>Обновить</Text>
          </TouchableOpacity>
          <TouchableOpacity style={s.btn} onPress={requestPermissions}>
            <Text style={s.btnText}>Запросить</Text>
          </TouchableOpacity>
        </View>
        <TouchableOpacity
          style={[s.btn, s.btnSecondary, s.btnFull]}
          onPress={() => Linking.openSettings()}
        >
          <Text style={s.btnSecondaryText}>Открыть настройки приложения</Text>
        </TouchableOpacity>
      </View>
    </ScrollView>
  );
}

function PermRow({
  label,
  state,
}: {
  label: string;
  state: PermState;
}): React.JSX.Element {
  const color =
    state === 'granted'
      ? '#059669'
      : state === 'denied'
      ? '#dc2626'
      : '#9ca3af';
  return (
    <View style={s.permRow}>
      <Text style={s.permLabel}>{label}</Text>
      <Text style={[s.permValue, { color }]}>{state}</Text>
    </View>
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
  card: {
    backgroundColor: '#fff',
    borderRadius: 10,
    padding: 14,
    marginBottom: 12,
    elevation: 2,
    shadowColor: '#000',
    shadowOpacity: 0.05,
    shadowRadius: 4,
  },
  label: {
    fontSize: 13,
    fontWeight: '600',
    color: '#6b7280',
    textTransform: 'uppercase',
    marginBottom: 8,
  },
  input: {
    borderWidth: 1,
    borderColor: '#d1d5db',
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 10,
    fontSize: 14,
    color: '#111827',
    fontFamily: 'monospace',
    marginBottom: 6,
  },
  hint: {
    fontSize: 11,
    color: '#9ca3af',
    marginBottom: 12,
  },
  row: {
    flexDirection: 'row',
    gap: 8,
  },
  btn: {
    flex: 1,
    backgroundColor: '#2563eb',
    borderRadius: 8,
    paddingVertical: 12,
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: 44,
  },
  btnTest: {
    marginTop: 4,
    marginBottom: 8,
    backgroundColor: '#059669',
  },
  btnSecondary: {
    backgroundColor: '#f3f4f6',
  },
  btnDisabled: {
    backgroundColor: '#6ee7b7',
  },
  btnText: { color: '#fff', fontWeight: '600', fontSize: 14 },
  btnSecondaryText: { color: '#374151', fontWeight: '600', fontSize: 14 },
  resultBox: {
    borderRadius: 8,
    padding: 10,
  },
  resultOk: { backgroundColor: '#d1fae5' },
  resultError: { backgroundColor: '#fee2e2' },
  resultUrl: {
    fontSize: 11,
    color: '#6b7280',
    fontFamily: 'monospace',
    marginBottom: 4,
  },
  resultText: { fontSize: 13 },
  resultOkText: { color: '#065f46' },
  resultErrorText: { color: '#dc2626' },
  permRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: 5,
    borderBottomWidth: 1,
    borderBottomColor: '#f3f4f6',
  },
  permLabel: { fontSize: 14, color: '#374151', flex: 1 },
  permValue: {
    fontSize: 13,
    fontWeight: '600',
    fontFamily: 'monospace',
  },
  permButtons: { marginTop: 12 },
  btnFull: { flex: 0, marginTop: 8 },
});
