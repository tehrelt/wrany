import { render, screen } from '@testing-library/react'
import { SummaryCards } from './SummaryCards'
import type { TrackingSummary } from '@/features/tracking/trackingApi'

const empty: TrackingSummary = {
  points_count: 0,
  first_recorded_at: null,
  last_recorded_at: null,
  duration_sec: 0,
  avg_speed_mps: null,
  max_speed_mps: null,
}

test('renders zero points', () => {
  render(<SummaryCards summary={empty} />)
  expect(screen.getByText('0')).toBeInTheDocument()
})

test('renders point count', () => {
  render(<SummaryCards summary={{ ...empty, points_count: 42 }} />)
  expect(screen.getByText('42')).toBeInTheDocument()
})

test('renders speed values', () => {
  render(<SummaryCards summary={{ ...empty, max_speed_mps: 3.5, avg_speed_mps: 1.2 }} />)
  expect(screen.getByText('3.5 m/s')).toBeInTheDocument()
  expect(screen.getByText('1.2 m/s')).toBeInTheDocument()
})

test('shows skeleton when loading', () => {
  const { container } = render(<SummaryCards loading />)
  expect(container.querySelectorAll('.animate-pulse').length).toBeGreaterThan(0)
})
