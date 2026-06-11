import { useQuery } from '@tanstack/react-query'
import { listDevices, Device } from './devicesApi'

interface Props {
  value: string
  onChange: (deviceId: string) => void
}

export function DeviceSelector({ value, onChange }: Props) {
  const { data: devices = [], isLoading } = useQuery({
    queryKey: ['devices'],
    queryFn: listDevices,
  })

  if (isLoading) return <select disabled><option>Loading…</option></select>

  return (
    <select value={value} onChange={(e) => onChange(e.target.value)}>
      <option value="">All devices</option>
      {devices.map((d: Device) => (
        <option key={d.id} value={d.device_id}>
          {d.name ?? d.device_id} {d.platform ? `(${d.platform})` : ''}
        </option>
      ))}
    </select>
  )
}
