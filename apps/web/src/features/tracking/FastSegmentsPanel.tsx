import { Flame, Gauge, Route, Timer } from 'lucide-react'
import { cn } from '@/lib/utils'
import { SectionHeader } from '@/components/analytics/AnalyticsUi'
import type {
  FastSegment,
  FastSegmentLimit,
  FastSegmentPreset,
} from './trackingApi'

interface Props {
  items: FastSegment[]
  loading: boolean
  preset: FastSegmentPreset
  limit: FastSegmentLimit
  activeId: string | null
  onPresetChange: (value: FastSegmentPreset) => void
  onLimitChange: (value: FastSegmentLimit) => void
  onActiveChange: (value: string | null) => void
}

const presets: { value: FastSegmentPreset; label: string }[] = [
  { value: 'soft', label: 'Soft' },
  { value: 'normal', label: 'Normal' },
  { value: 'strict', label: 'Strict' },
]

const limits: FastSegmentLimit[] = [5, 10, 20]

export function fastSegmentId(segment: FastSegment): string {
  return `${segment.device_id}:${segment.started_at}:${segment.ended_at}`
}

function formatDuration(seconds: number): string {
  const minutes = Math.floor(seconds / 60)
  const remainder = seconds % 60
  return minutes > 0 ? `${minutes}m ${remainder}s` : `${remainder}s`
}

function formatDistance(meters: number): string {
  return meters >= 1000 ? `${(meters / 1000).toFixed(2)} km` : `${Math.round(meters)} m`
}

export function FastSegmentsPanel({
  items,
  loading,
  preset,
  limit,
  activeId,
  onPresetChange,
  onLimitChange,
  onActiveChange,
}: Props) {
  return (
    <section className="race-panel overflow-hidden">
      <div className="border-b px-4 py-3">
        <SectionHeader
          title="Fastest sectors"
          description="Ranked against median window pace"
          action={<Flame className="size-4 text-amber-600" />}
        />
        <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
          <div className="flex border border-border" aria-label="Sensitivity">
            {presets.map(option => (
              <button
                key={option.value}
                type="button"
                aria-pressed={preset === option.value}
                onClick={() => onPresetChange(option.value)}
                className={cn(
                  'cursor-pointer border-r px-2 py-1 font-mono text-[8px] font-bold uppercase tracking-[0.1em] transition-colors last:border-r-0',
                  preset === option.value
                    ? 'bg-primary/12 text-primary'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                )}
              >
                {option.label}
              </button>
            ))}
          </div>
          <div className="flex border border-border" aria-label="Ranking size">
            {limits.map(value => (
              <button
                key={value}
                type="button"
                aria-pressed={limit === value}
                onClick={() => onLimitChange(value)}
                className={cn(
                  'cursor-pointer border-r px-2 py-1 font-mono text-[8px] font-bold tabular-nums transition-colors last:border-r-0',
                  limit === value
                    ? 'bg-primary/12 text-primary'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground',
                )}
              >
                {value}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="max-h-[360px] overflow-y-auto">
        {loading ? (
          <p className="px-4 py-8 text-center font-mono text-[10px] uppercase text-muted-foreground">
            Ranking sectors...
          </p>
        ) : items.length === 0 ? (
          <p className="px-4 py-8 text-center text-xs leading-5 text-muted-foreground">
            No sustained fast sectors found.
          </p>
        ) : (
          <ol>
            {items.map(segment => {
              const id = fastSegmentId(segment)
              const active = activeId === id
              return (
                <li key={id} className="border-b last:border-b-0">
                  <button
                    type="button"
                    onMouseEnter={() => onActiveChange(id)}
                    onMouseLeave={() => onActiveChange(null)}
                    onFocus={() => onActiveChange(id)}
                    onBlur={() => onActiveChange(null)}
                    className={cn(
                      'w-full cursor-pointer px-4 py-3 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary',
                      active ? 'bg-amber-500/10' : 'hover:bg-muted/60',
                    )}
                    aria-label={`Rank ${segment.rank}, ${segment.avg_speed_mps.toFixed(2)} meters per second`}
                  >
                    <div className="flex items-start gap-3">
                      <span className={cn(
                        'grid size-8 shrink-0 place-items-center border font-mono text-sm font-black',
                        active
                          ? 'border-amber-600 bg-amber-500/15 text-amber-700'
                          : 'border-primary/30 bg-primary/8 text-primary',
                      )}>
                        {segment.rank}
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-baseline justify-between gap-2">
                          <p className="font-mono text-base font-black tabular-nums">
                            {segment.avg_speed_mps.toFixed(2)} <span className="text-[9px] text-muted-foreground">M/S</span>
                          </p>
                          <span className="font-mono text-[10px] font-bold text-primary">
                            +{segment.uplift_percent.toFixed(0)}%
                          </span>
                        </div>
                        <div className="mt-2 grid grid-cols-3 gap-2 font-mono text-[9px] text-muted-foreground">
                          <span className="flex items-center gap-1"><Route className="size-3" />{formatDistance(segment.distance_m)}</span>
                          <span className="flex items-center gap-1"><Timer className="size-3" />{formatDuration(segment.duration_sec)}</span>
                          <span className="flex items-center gap-1"><Gauge className="size-3" />{segment.baseline_speed_mps.toFixed(1)}</span>
                        </div>
                      </div>
                    </div>
                  </button>
                </li>
              )
            })}
          </ol>
        )}
      </div>
    </section>
  )
}
