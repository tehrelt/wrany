import {
  getAccessToken,
  getRefreshToken,
  saveTokens,
} from '../../storage/tokenStorage';
import { getTokenExp } from '../../api/jwt';
import { trackingModule } from './trackingNativeModule';

// Refresh tokens rotate on the server: every refresh revokes the previous one.
// The native foreground service refreshes on its own (background WS 401 handling),
// so JS and native each hold a token pair that the other can invalidate. These
// helpers keep the two stores reconciled to avoid wrongly logging the user out.

// Reconcile JS and native token stores, keeping whichever pair is NEWER.
//
// "Differs" is not enough to decide a winner: right after a fresh login the JS
// pair is the newest, but the service may still hold a stale pair from a previous
// session. Blindly adopting native here would clobber the fresh login tokens with
// an expired/revoked pair and log the user straight back out. So compare access
// token expiry: adopt native only when it is genuinely newer (service refreshed
// in the background); otherwise push the newer JS pair down into the service.
export async function syncTokensFromNative(): Promise<void> {
  try {
    const nt = await trackingModule.getStoredTokens();
    if (!nt.refreshToken || !nt.accessToken) return;

    const [jsAccess, jsRefresh] = await Promise.all([
      getAccessToken(),
      getRefreshToken(),
    ]);

    // No JS credentials at all — adopt whatever the service has.
    if (!jsAccess || !jsRefresh) {
      await saveTokens(nt.accessToken, nt.refreshToken);
      return;
    }

    if (nt.refreshToken === jsRefresh) return; // already in sync

    if (getTokenExp(nt.accessToken) > getTokenExp(jsAccess)) {
      // Service holds a newer pair (rotated while the app was closed).
      await saveTokens(nt.accessToken, nt.refreshToken);
    } else {
      // JS holds the newer pair (e.g. a fresh login) — push it to the service.
      await trackingModule.updateTokens(jsAccess, jsRefresh);
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
