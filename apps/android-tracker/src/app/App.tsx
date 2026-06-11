import React, { useEffect, useState } from 'react';
import { clearTokens, getAccessToken } from '../storage/tokenStorage';
import { AuthScreen } from '../screens/AuthScreen';
import { TrackerScreen } from '../screens/TrackerScreen';

export function App(): React.JSX.Element {
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

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
  return <TrackerScreen token={token} onSessionExpired={handleSessionExpired} />;
}
