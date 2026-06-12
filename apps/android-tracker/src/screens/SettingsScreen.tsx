import React, { useEffect, useState } from 'react';
import {
  ActivityIndicator,
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

export function SettingsScreen(): React.JSX.Element {
  const [url, setUrl] = useState('');
  const [saved, setSaved] = useState(false);
  const [testStatus, setTestStatus] = useState<TestStatus>('idle');
  const [testMsg, setTestMsg] = useState('');
  const [testedUrl, setTestedUrl] = useState('');

  useEffect(() => {
    getApiUrl().then(setUrl);
  }, []);

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
    </ScrollView>
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
});
