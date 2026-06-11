import { API_BASE_URL } from '../config/env';

interface ApiResponse<T> {
  data: T | null;
  error: string | null;
}

export async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
  token?: string,
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const url = `${API_BASE_URL}${path}`;
  console.log(`[HTTP] ${options.method ?? 'GET'} ${url}`);

  const res = await fetch(url, { ...options, headers });
  const json = (await res.json()) as ApiResponse<T>;
  console.log(`[HTTP] ${res.status} ${url}`, JSON.stringify(json).slice(0, 300));

  if (!res.ok || json.error) {
    throw new Error(json.error ?? `HTTP ${res.status}`);
  }
  return json.data as T;
}
