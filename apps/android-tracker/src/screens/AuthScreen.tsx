import React, { useState } from 'react';
import {
  ActivityIndicator,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from 'react-native';
import { login, register } from '../api/authApi';
import { saveTokens } from '../storage/tokenStorage';

interface Props {
  onAuth: (token: string) => void;
}

export function AuthScreen({ onAuth }: Props): React.JSX.Element {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handle(action: 'register' | 'login'): Promise<void> {
    setError(null);
    setLoading(true);
    try {
      const pair = await (action === 'register'
        ? register(email, password)
        : login(email, password));
      await saveTokens(pair.access_token, pair.refresh_token);
      onAuth(pair.access_token);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  }

  return (
    <View style={s.container}>
      <Text style={s.title}>WR any% Tracker</Text>
      <TextInput
        style={s.input}
        placeholder="email"
        autoCapitalize="none"
        keyboardType="email-address"
        value={email}
        onChangeText={setEmail}
      />
      <TextInput
        style={s.input}
        placeholder="password"
        secureTextEntry
        value={password}
        onChangeText={setPassword}
      />
      {error && <Text style={s.error}>{error}</Text>}
      {loading ? (
        <ActivityIndicator style={s.btn} />
      ) : (
        <>
          <TouchableOpacity style={s.btn} onPress={() => handle('register')}>
            <Text style={s.btnText}>Register</Text>
          </TouchableOpacity>
          <TouchableOpacity style={[s.btn, s.btnSecondary]} onPress={() => handle('login')}>
            <Text style={s.btnText}>Login</Text>
          </TouchableOpacity>
        </>
      )}
    </View>
  );
}

const s = StyleSheet.create({
  container: { flex: 1, justifyContent: 'center', padding: 24, backgroundColor: '#fff' },
  title: { fontSize: 22, fontWeight: '700', marginBottom: 24, textAlign: 'center' },
  input: {
    borderWidth: 1, borderColor: '#ccc', borderRadius: 8,
    padding: 12, marginBottom: 12, fontSize: 16,
  },
  btn: {
    backgroundColor: '#2563eb', borderRadius: 8,
    padding: 14, alignItems: 'center', marginBottom: 10,
  },
  btnSecondary: { backgroundColor: '#6b7280' },
  btnText: { color: '#fff', fontWeight: '600', fontSize: 16 },
  error: { color: '#dc2626', marginBottom: 10, textAlign: 'center' },
});
