import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { Toaster } from '@/components/ui/sonner'
import { queryClient } from './queryClient'
import { useAuth } from '@/features/auth/useAuth'
import { AuthProvider } from '@/features/auth/AuthContext'
import { AuthGuard } from '@/features/auth/AuthGuard'
import { LoginPage } from '@/pages/LoginPage'
import { DashboardPage } from '@/pages/DashboardPage'
import { TripsPage } from '@/pages/TripsPage'
import { RoutesPage } from '@/pages/RoutesPage'

function AppRoutes() {
  const { isAuthenticated, logout } = useAuth()

  return (
    <Routes>
      <Route
        path="/login"
        element={isAuthenticated ? <Navigate to="/" replace /> : <LoginPage />}
      />
      <Route
        path="/"
        element={
          <AuthGuard isAuthenticated={isAuthenticated}>
            <DashboardPage onLogout={logout} />
          </AuthGuard>
        }
      />
      <Route
        path="/trips"
        element={
          <AuthGuard isAuthenticated={isAuthenticated}>
            <TripsPage onLogout={logout} />
          </AuthGuard>
        }
      />
      <Route
        path="/routes"
        element={
          <AuthGuard isAuthenticated={isAuthenticated}>
            <RoutesPage onLogout={logout} />
          </AuthGuard>
        }
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export function App() {
  return (
    <AuthProvider>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <AppRoutes />
          <Toaster />
        </BrowserRouter>
      </QueryClientProvider>
    </AuthProvider>
  )
}
