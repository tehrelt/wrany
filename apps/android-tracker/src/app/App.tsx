import React, { useEffect, useState } from 'react';
import { StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import { SafeAreaProvider, SafeAreaView } from 'react-native-safe-area-context';
import { clearTokens, getAccessToken } from '../storage/tokenStorage';
import { AuthScreen } from '../screens/AuthScreen';
import { TrackerScreen } from '../screens/TrackerScreen';
import { TrackingStatusScreen } from '../features/tracking/TrackingStatusScreen';

type Tab = 'legacy' | 'background';

export function App(): React.JSX.Element {
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [tab, setTab] = useState<Tab>('background');

  useEffect(() => {
    getAccessToken().then(t => {
      setToken(t);
      setLoading(false);
    });
  }, []);

  function handleSessionExpired(): void {
    clearTokens().catch(() => {});
    setToken(null);
  }

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
            style={[styles.tab, tab === 'legacy' && styles.activeTab]}
            onPress={() => setTab('legacy')}
          >
            <Text
              style={[styles.tabText, tab === 'legacy' && styles.activeTabText]}
            >
              Legacy WS
            </Text>
          </TouchableOpacity>
        </View>
        <View style={styles.content}>
          {tab === 'background' ? (
            <TrackingStatusScreen />
          ) : (
            <TrackerScreen onSessionExpired={handleSessionExpired} />
          )}
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
