import { createContext, useContext, useState, useCallback, useEffect } from 'react'
import { LOGOUT_EVENT } from '@/api/client'
import { login as apiLogin, register as apiRegister, type LoginInput, type RegisterInput } from './authApi'

interface AuthContextValue {
  token: string | null
  isAuthenticated: boolean
  loading: boolean
  error: string | null
  login: (input: LoginInput) => Promise<void>
  register: (input: RegisterInput) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('access_token'))
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    const handle = () => setToken(null)
    window.addEventListener(LOGOUT_EVENT, handle)
    return () => window.removeEventListener(LOGOUT_EVENT, handle)
  }, [])

  const saveTokens = useCallback((access: string, refresh: string) => {
    localStorage.setItem('access_token', access)
    localStorage.setItem('refresh_token', refresh)
    setToken(access)
  }, [])

  const login = useCallback(async (input: LoginInput) => {
    setLoading(true)
    setError(null)
    try {
      const pair = await apiLogin(input)
      saveTokens(pair.access_token, pair.refresh_token)
    } catch {
      setError('Invalid email or password')
    } finally {
      setLoading(false)
    }
  }, [saveTokens])

  const register = useCallback(async (input: RegisterInput) => {
    setLoading(true)
    setError(null)
    try {
      const pair = await apiRegister(input)
      saveTokens(pair.access_token, pair.refresh_token)
    } catch {
      setError('Registration failed')
    } finally {
      setLoading(false)
    }
  }, [saveTokens])

  const logout = useCallback(() => {
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
    setToken(null)
  }, [])

  return (
    <AuthContext.Provider value={{ token, isAuthenticated: !!token, loading, error, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be inside AuthProvider')
  return ctx
}
