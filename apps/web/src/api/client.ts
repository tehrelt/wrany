import axios, { type AxiosRequestConfig, type AxiosResponse } from 'axios'
import { toast } from 'sonner'
import { API_BASE_URL } from '../config/env'

export const LOGOUT_EVENT = 'auth:logout'

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: { 'Content-Type': 'application/json' },
})

apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('access_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

let refreshInFlight: Promise<string> | null = null

async function doRefresh(): Promise<string> {
  const rt = localStorage.getItem('refresh_token')
  if (!rt) throw new Error('no refresh token')

  const res = await axios.post<{ data: { access_token: string; refresh_token: string } }>(
    `${API_BASE_URL}/v1/auth/refresh`,
    { refresh_token: rt },
  )

  const { access_token, refresh_token } = res.data.data
  localStorage.setItem('access_token', access_token)
  localStorage.setItem('refresh_token', refresh_token)
  return access_token
}

function extractMessage(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const msg = (error.response?.data as { error?: string } | undefined)?.error
    if (msg) return msg
    if (error.response?.status) return `Ошибка ${error.response.status}`
    if (error.message) return error.message
  }
  return 'Неизвестная ошибка'
}

apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    const original = error.config as AxiosRequestConfig & { _retried?: boolean }

    const is401 = error.response?.status === 401
    const isRefreshEndpoint = original.url?.includes('/v1/auth/refresh')
    const isLoginEndpoint = original.url?.includes('/v1/auth/login')

    if (is401 && !isRefreshEndpoint && !original._retried) {
      original._retried = true

      try {
        if (!refreshInFlight) {
          refreshInFlight = doRefresh().finally(() => { refreshInFlight = null })
        }
        const newToken = await refreshInFlight
        original.headers = { ...original.headers, Authorization: `Bearer ${newToken}` }
        return apiClient(original)
      } catch {
        localStorage.removeItem('access_token')
        localStorage.removeItem('refresh_token')
        window.dispatchEvent(new Event(LOGOUT_EVENT))
        return Promise.reject(error)
      }
    }

    // Show toast for all errors except login (handled inline) and silent 401 after logout.
    const silent = isLoginEndpoint || (is401 && isRefreshEndpoint)
    if (!silent) {
      toast.error(extractMessage(error))
    }

    return Promise.reject(error)
  },
)

// Orval mutator: called by generated API functions.
// Returns response.data (the full envelope from the backend).
export const customRequest = <T>(config: AxiosRequestConfig): Promise<T> =>
  apiClient(config).then((res: AxiosResponse<T>) => res.data)
