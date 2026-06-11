import { TrackingPoint } from '../features/tracking/trackingApi'

const MAX_VISIBLE = 500

interface Props {
  points: TrackingPoint[]
}

export function PointsTable({ points }: Props) {
  if (points.length === 0) return null

  const visible = points.slice(0, MAX_VISIBLE)
  const truncated = points.length > MAX_VISIBLE

  return (
    <div style={{ overflowX: 'auto' }}>
      {truncated && (
        <div style={{ color: '#888', marginBottom: 4 }}>
          Showing first {MAX_VISIBLE} of {points.length} points.
        </div>
      )}
      <table style={{ borderCollapse: 'collapse', width: '100%', fontSize: 13 }}>
        <thead>
          <tr>
            {['Time', 'Device', 'Lat', 'Lon', 'Accuracy (m)', 'Speed (m/s)', 'Activity'].map((h) => (
              <th key={h} style={{ textAlign: 'left', padding: '4px 8px', borderBottom: '1px solid #ccc', whiteSpace: 'nowrap' }}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {visible.map((p) => (
            <tr key={p.event_id}>
              <td style={{ padding: '3px 8px', whiteSpace: 'nowrap' }}>
                {new Date(p.recorded_at).toLocaleTimeString()}
              </td>
              <td style={{ padding: '3px 8px', fontFamily: 'monospace', fontSize: 11 }}>
                {p.device_id.slice(0, 8)}…
              </td>
              <td style={{ padding: '3px 8px' }}>{p.lat.toFixed(6)}</td>
              <td style={{ padding: '3px 8px' }}>{p.lon.toFixed(6)}</td>
              <td style={{ padding: '3px 8px' }}>{p.accuracy_m.toFixed(1)}</td>
              <td style={{ padding: '3px 8px' }}>{p.speed_mps !== null ? p.speed_mps.toFixed(2) : '—'}</td>
              <td style={{ padding: '3px 8px' }}>{p.activity_type}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
