import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { queryClient } from './queryClient'
import { useAuth } from '../features/auth/useAuth'
import { AuthGuard } from '../features/auth/AuthGuard'
import { LoginPage } from '../pages/LoginPage'
import { RegisterPage } from '../pages/RegisterPage'
import { DashboardPage } from '../pages/DashboardPage'

function AppRoutes() {
  const { isAuthenticated, error, loading, login, register, logout } = useAuth()

  return (
    <Routes>
      <Route
        path="/login"
        element={
          isAuthenticated
            ? <Navigate to="/" replace />
            : <LoginPage onLogin={login} error={error} loading={loading} />
        }
      />
      <Route
        path="/register"
        element={
          isAuthenticated
            ? <Navigate to="/" replace />
            : <RegisterPage onRegister={register} error={error} loading={loading} />
        }
      />
      <Route
        path="/"
        element={
          <AuthGuard isAuthenticated={isAuthenticated}>
            <DashboardPage onLogout={logout} />
          </AuthGuard>
        }
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </QueryClientProvider>
  )
}
