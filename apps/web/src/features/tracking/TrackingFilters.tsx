import { useState } from 'react'
import { ChevronDown, RefreshCw, SlidersHorizontal } from 'lucide-react'
import { formatISO, subHours } from 'date-fns'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'
import { DateTimePicker } from '@/components/ui/date-time-picker'
import { DeviceSelector } from '@/features/devices/DeviceSelector'

export function defaultFrom(): string {
  return formatISO(subHours(new Date(), 24))
}

export function defaultTo(): string {
  return formatISO(new Date())
}

export interface TrackDisplaySettings {
  speedThresholdMps: number
  minStaySec: number
  minMoveSec: number
}

export const defaultTrackSettings: TrackDisplaySettings = {
  speedThresholdMps: 1,
  minStaySec: 60,
  minMoveSec: 30,
}

interface Props {
  deviceId: string
  from: string
  to: string
  loading?: boolean
  settings: TrackDisplaySettings
  onDeviceChange: (id: string) => void
  onFromChange: (iso: string) => void
  onToChange: (iso: string) => void
  onSettingsChange: (settings: TrackDisplaySettings) => void
  onRefresh: () => void
}

function FilterLabel({ children, value }: { children: React.ReactNode; value?: string }) {
  return (
    <div className="mb-2 flex items-center justify-between">
      <Label className="font-mono text-[9px] font-bold uppercase tracking-[0.14em] text-muted-foreground">{children}</Label>
      {value ? <span className="mono-data font-mono text-[9px] text-primary">{value}</span> : null}
    </div>
  )
}

export function TrackingFilters({
  deviceId,
  from,
  to,
  loading,
  settings,
  onDeviceChange,
  onFromChange,
  onToChange,
  onSettingsChange,
  onRefresh,
}: Props) {
  const [advanced, setAdvanced] = useState(false)

  return (
    <div className="space-y-5">
      <div>
        <FilterLabel>Transponder</FilterLabel>
        <DeviceSelector value={deviceId} onChange={onDeviceChange} />
      </div>
      <div className="grid grid-cols-2 gap-2 lg:grid-cols-1">
        <DateTimePicker value={from} onChange={onFromChange} label="Window start" />
        <DateTimePicker value={to} onChange={onToChange} label="Window end" />
      </div>
      <Button onClick={onRefresh} disabled={loading} className="cut-corner h-11 w-full gap-2 font-mono text-[10px] font-bold uppercase tracking-[0.14em]">
        <RefreshCw className={loading ? 'size-3.5 animate-spin' : 'size-3.5'} />
        Update telemetry
      </Button>
      <div className="border-t pt-4">
        <button type="button" onClick={() => setAdvanced(value => !value)} className="flex w-full cursor-pointer items-center justify-between py-1 text-left font-mono text-[9px] font-bold uppercase tracking-[0.14em] text-muted-foreground hover:text-foreground">
          <span className="flex items-center gap-2"><SlidersHorizontal className="size-3.5" /> Processing setup</span>
          <ChevronDown className={advanced ? 'size-3.5 rotate-180 transition-transform' : 'size-3.5 transition-transform'} />
        </button>
        {advanced ? (
          <div className="mt-5 space-y-6">
            <div>
              <FilterLabel value={`${settings.speedThresholdMps.toFixed(1)} M/S`}>Movement gate</FilterLabel>
              <Slider min={0.5} max={10} step={0.5} value={[settings.speedThresholdMps]} onValueChange={([value]) => onSettingsChange({ ...settings, speedThresholdMps: value })} />
            </div>
            <div>
              <FilterLabel value={`${settings.minStaySec} SEC`}>Minimum stop</FilterLabel>
              <Slider min={0} max={600} step={30} value={[settings.minStaySec]} onValueChange={([value]) => onSettingsChange({ ...settings, minStaySec: value })} />
            </div>
            <div>
              <FilterLabel value={`${settings.minMoveSec} SEC`}>Minimum movement</FilterLabel>
              <Slider min={0} max={300} step={10} value={[settings.minMoveSec]} onValueChange={([value]) => onSettingsChange({ ...settings, minMoveSec: value })} />
            </div>
          </div>
        ) : null}
      </div>
    </div>
  )
}
