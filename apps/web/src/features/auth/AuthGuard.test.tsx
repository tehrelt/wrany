import { render, screen } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { AuthGuard } from './AuthGuard'

test('renders children when authenticated', () => {
  render(
    <MemoryRouter initialEntries={['/']}>
      <AuthGuard isAuthenticated={true}>
        <div>Dashboard</div>
      </AuthGuard>
    </MemoryRouter>
  )
  expect(screen.getByText('Dashboard')).toBeInTheDocument()
})

test('redirects to /login when not authenticated', () => {
  render(
    <MemoryRouter initialEntries={['/']}>
      <AuthGuard isAuthenticated={false}>
        <div>Dashboard</div>
      </AuthGuard>
      <Routes>
        <Route path="/login" element={<div>Login page</div>} />
      </Routes>
    </MemoryRouter>
  )
  expect(screen.getByText('Login page')).toBeInTheDocument()
  expect(screen.queryByText('Dashboard')).not.toBeInTheDocument()
})
