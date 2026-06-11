import { useState, useCallback, useEffect } from 'react'
import { login as apiLogin, register as apiRegister, LoginInput, RegisterInput } from './authApi'
import { LOGOUT_EVENT } from '../../api/client'

export function useAuth() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('access_token'))
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  // Listen for forced logout emitted by the axios interceptor when refresh fails.
  useEffect(() => {
    function handleLogout() {
      setToken(null)
    }
    window.addEventListener(LOGOUT_EVENT, handleLogout)
    return () => window.removeEventListener(LOGOUT_EVENT, handleLogout)
  }, [])

  const saveTokens = useCallback((accessToken: string, refreshToken: string) => {
    localStorage.setItem('access_token', accessToken)
    localStorage.setItem('refresh_token', refreshToken)
    setToken(accessToken)
  }, [])

  const doLogin = useCallback(async (input: LoginInput) => {
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

  const doRegister = useCallback(async (input: RegisterInput) => {
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

  return { token, isAuthenticated: !!token, error, loading, login: doLogin, register: doRegister, logout }
}
