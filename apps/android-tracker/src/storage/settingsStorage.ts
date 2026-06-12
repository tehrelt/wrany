import AsyncStorage from '@react-native-async-storage/async-storage';

const API_URL_KEY = '@wrany/api_url';

export const DEFAULT_API_URL = 'http://192.168.31.78:8080';

export async function getApiUrl(): Promise<string> {
  const stored = await AsyncStorage.getItem(API_URL_KEY);
  return stored ?? DEFAULT_API_URL;
}

export async function saveApiUrl(url: string): Promise<void> {
  await AsyncStorage.setItem(API_URL_KEY, url.replace(/\/$/, ''));
}

export function apiUrlToWsUrl(apiUrl: string): string {
  return apiUrl.replace(/^http/, 'ws') + '/v1/ws/tracker';
}
