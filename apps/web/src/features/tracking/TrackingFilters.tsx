import { RefreshCw } from 'lucide-react'
import { subHours, formatISO } from 'date-fns'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { DeviceSelector } from '@/features/devices/DeviceSelector'

function toDatetimeLocal(iso: string): string {
  return iso.slice(0, 16)
}

function fromDatetimeLocal(local: string): string {
  return local ? new Date(local).toISOString() : ''
}

export function defaultFrom(): string {
  return formatISO(subHours(new Date(), 24))
}

export function defaultTo(): string {
  return formatISO(new Date())
}

interface Props {
  deviceId: string
  from: string
  to: string
  loading?: boolean
  onDeviceChange: (id: string) => void
  onFromChange: (iso: string) => void
  onToChange: (iso: string) => void
  onRefresh: () => void
}

export function TrackingFilters({
  deviceId,
  from,
  to,
  loading,
  onDeviceChange,
  onFromChange,
  onToChange,
  onRefresh,
}: Props) {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">Device</Label>
        <DeviceSelector value={deviceId} onChange={onDeviceChange} />
      </div>

      <Separator />

      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">From</Label>
        <Input
          type="datetime-local"
          value={toDatetimeLocal(from)}
          onChange={e => onFromChange(fromDatetimeLocal(e.target.value))}
          className="text-xs h-8"
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">To</Label>
        <Input
          type="datetime-local"
          value={toDatetimeLocal(to)}
          onChange={e => onToChange(fromDatetimeLocal(e.target.value))}
          className="text-xs h-8"
        />
      </div>

      <Button
        size="sm"
        onClick={onRefresh}
        disabled={loading}
        className="w-full gap-1.5"
      >
        <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
        Refresh
      </Button>
    </div>
  )
}
