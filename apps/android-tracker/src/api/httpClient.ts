import { API_BASE_URL } from '../config/env';
import {
  clearTokens,
  getAccessToken,
  getRefreshToken,
  saveTokens,
} from '../storage/tokenStorage';

interface ApiResponse<T> {
  data: T | null;
  error: string | null;
}

// Thrown when the refresh token itself is expired or invalid.
// Callers (screens) should catch this and navigate to AuthScreen.
export class AuthExpiredError extends Error {
  constructor() {
    super('Session expired — please log in again');
    this.name = 'AuthExpiredError';
  }
}

// Single-flight mutex: if a refresh is already in progress, every concurrent
// 401 waits for the same promise instead of hammering the refresh endpoint.
let refreshInFlight: Promise<string> | null = null;

async function doRefresh(): Promise<string> {
  const rt = await getRefreshToken();
  if (!rt) throw new AuthExpiredError();

  const res = await fetch(`${API_BASE_URL}/v1/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: rt }),
  });

  if (!res.ok) {
    await clearTokens();
    throw new AuthExpiredError();
  }

  const json = (await res.json()) as ApiResponse<{
    access_token: string;
    refresh_token: string;
  }>;
  if (!json.data) {
    await clearTokens();
    throw new AuthExpiredError();
  }

  await saveTokens(json.data.access_token, json.data.refresh_token);
  return json.data.access_token;
}

async function refreshOnce(): Promise<string> {
  if (!refreshInFlight) {
    refreshInFlight = doRefresh().finally(() => {
      refreshInFlight = null;
    });
  }
  return refreshInFlight;
}

export async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
  // When explicitly provided (auth/register/login), used as-is and 401 is NOT retried.
  explicitToken?: string,
): Promise<T> {
  const isAuthEndpoint = explicitToken !== undefined;

  const accessToken = isAuthEndpoint
    ? explicitToken
    : await getAccessToken();

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };
  if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`;

  const url = `${API_BASE_URL}${path}`;
  console.log(`[HTTP] ${options.method ?? 'GET'} ${url}`);

  const res = await fetch(url, { ...options, headers });

  // 401 on a non-auth endpoint → try to refresh and retry once.
  if (res.status === 401 && !isAuthEndpoint) {
    const newToken = await refreshOnce(); // throws AuthExpiredError on failure
    const retryHeaders = { ...headers, Authorization: `Bearer ${newToken}` };
    const retry = await fetch(url, { ...options, headers: retryHeaders });
    const retryJson = (await retry.json()) as ApiResponse<T>;
    if (!retry.ok || retryJson.error) {
      throw new Error(retryJson.error ?? `HTTP ${retry.status}`);
    }
    return retryJson.data as T;
  }

  const json = (await res.json()) as ApiResponse<T>;
  console.log(
    `[HTTP] ${res.status} ${url}`,
    JSON.stringify(json).slice(0, 300),
  );

  if (!res.ok || json.error) {
    throw new Error(json.error ?? `HTTP ${res.status}`);
  }
  return json.data as T;
}
