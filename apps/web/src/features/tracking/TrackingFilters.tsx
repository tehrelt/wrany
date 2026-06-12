import { useState } from 'react'
import { ChevronDown, ChevronRight, RefreshCw } from 'lucide-react'
import { subHours, formatISO } from 'date-fns'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Slider } from '@/components/ui/slider'
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

export interface TrackDisplaySettings {
  speedThresholdMps: number
  minStaySec: number
  minMoveSec: number
}

export const defaultTrackSettings: TrackDisplaySettings = {
  speedThresholdMps: 2.0,
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
  onSettingsChange: (s: TrackDisplaySettings) => void
  onRefresh: () => void
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
  const [showAdvanced, setShowAdvanced] = useState(false)

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

      <Separator />

      <button
        type="button"
        className="flex items-center justify-between text-xs text-muted-foreground hover:text-foreground transition-colors"
        onClick={() => setShowAdvanced(v => !v)}
      >
        <span>Display settings</span>
        {showAdvanced
          ? <ChevronDown className="h-3.5 w-3.5" />
          : <ChevronRight className="h-3.5 w-3.5" />}
      </button>

      {showAdvanced && (
        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1.5">
            <div className="flex justify-between items-center">
              <Label className="text-xs">Speed threshold</Label>
              <span className="text-xs text-muted-foreground">
                {(settings.speedThresholdMps * 3.6).toFixed(1)} km/h
              </span>
            </div>
            <Slider
              min={0.5}
              max={10}
              step={0.5}
              value={[settings.speedThresholdMps]}
              onValueChange={([v]) => onSettingsChange({ ...settings, speedThresholdMps: v })}
            />
            <p className="text-[10px] text-muted-foreground">Points below this speed are treated as stationary</p>
          </div>

          <div className="flex flex-col gap-1.5">
            <div className="flex justify-between items-center">
              <Label className="text-xs">Min stay duration</Label>
              <span className="text-xs text-muted-foreground">
                {settings.minStaySec >= 60
                  ? `${Math.floor(settings.minStaySec / 60)} min`
                  : `${settings.minStaySec} s`}
              </span>
            </div>
            <Slider
              min={0}
              max={600}
              step={30}
              value={[settings.minStaySec]}
              onValueChange={([v]) => onSettingsChange({ ...settings, minStaySec: v })}
            />
            <p className="text-[10px] text-muted-foreground">Stationary clusters shorter than this are hidden</p>
          </div>

          <div className="flex flex-col gap-1.5">
            <div className="flex justify-between items-center">
              <Label className="text-xs">Min move duration</Label>
              <span className="text-xs text-muted-foreground">
                {(settings.minMoveSec ?? 30) >= 60
                  ? `${Math.floor((settings.minMoveSec ?? 30) / 60)} min`
                  : `${settings.minMoveSec ?? 30} s`}
              </span>
            </div>
            <Slider
              min={0}
              max={300}
              step={10}
              value={[settings.minMoveSec ?? 30]}
              onValueChange={([v]) => onSettingsChange({ ...settings, minMoveSec: v })}
            />
            <p className="text-[10px] text-muted-foreground">Movement bursts shorter than this are treated as noise</p>
          </div>
        </div>
      )}
    </div>
  )
}
