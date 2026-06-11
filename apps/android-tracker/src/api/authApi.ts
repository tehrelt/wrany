import { apiFetch } from './httpClient';

export interface TokenPair {
  access_token: string;
  refresh_token: string;
}

export function register(email: string, password: string): Promise<TokenPair> {
  return apiFetch<TokenPair>('/v1/auth/register', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export function login(email: string, password: string): Promise<TokenPair> {
  return apiFetch<TokenPair>('/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
}

export function refreshTokens(refreshToken: string): Promise<TokenPair> {
  // Called directly (no auth header) — the refresh token IS the credential.
  return apiFetch<TokenPair>('/v1/auth/refresh', {
    method: 'POST',
    body: JSON.stringify({ refresh_token: refreshToken }),
  });
}
