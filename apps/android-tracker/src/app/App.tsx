import React, { useEffect, useState } from 'react';
import { StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import { SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';
import { getAccessToken } from '../storage/tokenStorage';
import { syncTokensFromNative } from '../features/tracking/tokenSync';
import { AuthScreen } from '../screens/AuthScreen';
import { SettingsScreen } from '../screens/SettingsScreen';
import { TrackingStatusScreen } from '../features/tracking/TrackingStatusScreen';

type Tab = 'background' | 'settings';

export function App(): React.JSX.Element {
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<Tab>('background');

  useEffect(() => {
    // Absorb any token the background service rotated to while the app was closed
    // before deciding whether the user is authenticated.
    syncTokensFromNative()
      .then(getAccessToken)
      .then(t => {
        setToken(t);
        setLoading(false);
      });
  }, []);

  if (loading) return <></>;

  if (!token) {
    return <AuthScreen onAuth={t => setToken(t)} />;
  }

  return (
    <SafeAreaProvider>
      <SafeAreaView style={styles.root}>
        <View style={styles.tabs}>
          <TouchableOpacity
            style={[styles.tab, tab === 'background' && styles.activeTab]}
            onPress={() => setTab('background')}
          >
            <Text
              style={[
                styles.tabText,
                tab === 'background' && styles.activeTabText,
              ]}
            >
              Background
            </Text>
          </TouchableOpacity>
          <TouchableOpacity
            style={[styles.tab, tab === 'settings' && styles.activeTab]}
            onPress={() => setTab('settings')}
          >
            <Text
              style={[
                styles.tabText,
                tab === 'settings' && styles.activeTabText,
              ]}
            >
              Settings
            </Text>
          </TouchableOpacity>
        </View>
        <View style={styles.content}>
          {tab === 'background' && <TrackingStatusScreen />}
          {tab === 'settings' && <SettingsScreen />}
        </View>
      </SafeAreaView>
    </SafeAreaProvider>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#f5f5f5' },
  tabs: {
    flexDirection: 'row',
    backgroundColor: '#fff',
    borderBottomWidth: 1,
    borderBottomColor: '#e0e0e0',
  },
  tab: {
    flex: 1,
    paddingVertical: 12,
    alignItems: 'center',
  },
  activeTab: {
    borderBottomWidth: 2,
    borderBottomColor: '#1565c0',
  },
  tabText: { fontSize: 14, color: '#666' },
  activeTabText: { color: '#1565c0', fontWeight: '600' },
  content: { flex: 1 },
});
