import { getRefreshToken, saveTokens } from '../../storage/tokenStorage';
import { trackingModule } from './trackingNativeModule';

// Refresh tokens rotate on the server: every refresh revokes the previous one.
// The native foreground service refreshes on its own (background WS 401 handling),
// so JS and native each hold a token pair that the other can invalidate. These
// helpers keep the two stores reconciled to avoid wrongly logging the user out.

// Absorb a token pair the service may have rotated to while the app was closed.
export async function syncTokensFromNative(): Promise<void> {
  try {
    const nt = await trackingModule.getStoredTokens();
    if (!nt.refreshToken || !nt.accessToken) return;
    if (nt.refreshToken !== (await getRefreshToken())) {
      await saveTokens(nt.accessToken, nt.refreshToken);
    }
  } catch {
    // native module unavailable or no stored tokens — nothing to reconcile
  }
}

// Push a freshly-issued JS token pair into native prefs after a JS-side refresh,
// so the service does not later refresh with a revoked token.
export async function syncTokensToNative(
  accessToken: string,
  refreshToken: string,
): Promise<void> {
  try {
    await trackingModule.updateTokens(accessToken, refreshToken);
  } catch {
    // native module unavailable — ignore
  }
}
