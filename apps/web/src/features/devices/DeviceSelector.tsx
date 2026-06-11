import { useQuery } from '@tanstack/react-query'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { listDevices } from './devicesApi'

interface Props {
  value: string
  onChange: (id: string) => void
}

export function DeviceSelector({ value, onChange }: Props) {
  const { data: devices, isLoading } = useQuery({
    queryKey: ['devices'],
    queryFn: listDevices,
  })

  if (isLoading) return <Skeleton className="h-9 w-full" />

  return (
    <Select value={value || 'all'} onValueChange={v => onChange(v === 'all' ? '' : v)}>
      <SelectTrigger className="h-9 text-xs">
        <SelectValue placeholder="All devices" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="all">All devices</SelectItem>
        {(devices ?? []).map(d => (
          <SelectItem key={d.device_id} value={d.device_id}>
            {d.name ?? d.device_id}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}