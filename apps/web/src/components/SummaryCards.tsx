import { TrackingSummary } from '../features/tracking/trackingApi'

function fmt(n: number | null, unit: string, decimals = 1): string {
  if (n === null) return '—'
  return `${n.toFixed(decimals)} ${unit}`
}

interface Props {
  summary: TrackingSummary
}

export function SummaryCards({ summary }: Props) {
  const cards = [
    { label: 'Points', value: String(summary.points_count) },
    { label: 'Duration', value: summary.duration_sec ? `${Math.round(summary.duration_sec / 60)} min` : '—' },
    { label: 'Avg speed', value: fmt(summary.avg_speed_mps, 'm/s') },
    { label: 'Max speed', value: fmt(summary.max_speed_mps, 'm/s') },
    { label: 'First point', value: summary.first_recorded_at ? new Date(summary.first_recorded_at).toLocaleTimeString() : '—' },
    { label: 'Last point', value: summary.last_recorded_at ? new Date(summary.last_recorded_at).toLocaleTimeString() : '—' },
  ]

  return (
    <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
      {cards.map(({ label, value }) => (
        <div key={label} style={{ border: '1px solid #ddd', borderRadius: 6, padding: '8px 16px', minWidth: 120 }}>
          <div style={{ fontSize: 11, color: '#888', textTransform: 'uppercase' }}>{label}</div>
          <div style={{ fontSize: 20, fontWeight: 600 }}>{value}</div>
        </div>
      ))}
    </div>
  )
}
