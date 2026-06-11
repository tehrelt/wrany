import { render, screen } from '@testing-library/react'
import { SummaryCards } from './SummaryCards'
import { TrackingSummary } from '../features/tracking/trackingApi'

const emptySummary: TrackingSummary = {
  points_count: 0,
  first_recorded_at: null,
  last_recorded_at: null,
  duration_sec: 0,
  avg_speed_mps: null,
  max_speed_mps: null,
}

test('renders zero points', () => {
  render(<SummaryCards summary={emptySummary} />)
  expect(screen.getByText('0')).toBeInTheDocument()
})

test('renders point count', () => {
  render(<SummaryCards summary={{ ...emptySummary, points_count: 42 }} />)
  expect(screen.getByText('42')).toBeInTheDocument()
})

test('renders speed values', () => {
  render(<SummaryCards summary={{ ...emptySummary, max_speed_mps: 3.5, avg_speed_mps: 1.2 }} />)
  expect(screen.getByText('3.5 m/s')).toBeInTheDocument()
  expect(screen.getByText('1.2 m/s')).toBeInTheDocument()
})
