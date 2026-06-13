import type { ReactNode } from 'react'
import {
  AlertTriangle,
  Check,
  ChevronRight,
  CircleDashed,
  Inbox,
  LoaderCircle,
  MapPinned,
  Trophy,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

export function PageHeader({
  eyebrow,
  title,
  description,
  actions,
}: {
  eyebrow?: string
  title: string
  description: string
  actions?: ReactNode
}) {
  return (
    <header className="speed-lines relative border-b px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
      <div className="flex flex-col gap-5 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <div className="mb-3 flex items-center gap-3">
            <span className="h-px w-8 bg-primary" />
            <p className="font-mono text-[10px] font-bold uppercase tracking-[0.24em] text-primary">
              {eyebrow ?? 'Race intelligence'}
            </p>
          </div>
          <h2 className="max-w-4xl text-3xl font-black uppercase italic leading-none tracking-[-0.06em] sm:text-4xl lg:text-5xl">
            {title}
          </h2>
          <p className="mt-3 max-w-2xl text-sm leading-6 text-muted-foreground">{description}</p>
        </div>
        {actions ? <div className="flex shrink-0 gap-2">{actions}</div> : null}
      </div>
      <span className="absolute right-6 top-3 hidden font-mono text-[9px] uppercase tracking-[0.2em] text-muted-foreground/35 lg:block">
        WR // BEST LAPS FOR REAL LIFE
      </span>
    </header>
  )
}

export function SectionHeader({
  title,
  description,
  action,
}: {
  title: string
  description?: string
  action?: ReactNode
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div>
        <h3 className="font-mono text-[11px] font-bold uppercase tracking-[0.16em] text-foreground">{title}</h3>
        {description ? <p className="mt-1 text-xs text-muted-foreground">{description}</p> : null}
      </div>
      {action}
    </div>
  )
}

export function MetricCard({
  label,
  value,
  detail,
  icon,
  accent = 'primary',
}: {
  label: string
  value: string
  detail?: string
  icon: ReactNode
  accent?: 'primary' | 'cyan' | 'amber'
}) {
  return (
    <div className="race-panel cut-corner group min-h-32 p-4 transition-colors hover:border-primary/60">
      <div className="flex items-start justify-between gap-3">
        <p className="font-mono text-[10px] font-bold uppercase tracking-[0.14em] text-muted-foreground">{label}</p>
        <span className={cn(
          'grid size-7 place-items-center border',
          accent === 'primary' && 'border-primary/30 bg-primary/10 text-primary',
          accent === 'cyan' && 'border-cyan-700/25 bg-cyan-600/10 text-cyan-700',
          accent === 'amber' && 'border-amber-700/25 bg-amber-500/10 text-amber-700',
        )}>
          {icon}
        </span>
      </div>
      <p className="mono-data mt-5 font-mono text-2xl font-bold leading-none tracking-[-0.05em]">{value}</p>
      {detail ? <p className="mt-2 text-[11px] text-muted-foreground">{detail}</p> : null}
    </div>
  )
}

export function StatusBadge({
  children,
  tone = 'neutral',
}: {
  children: ReactNode
  tone?: 'success' | 'warning' | 'neutral' | 'info'
}) {
  return (
    <span className={cn(
      'inline-flex items-center gap-2 border px-2.5 py-1 font-mono text-[9px] font-bold uppercase tracking-[0.12em]',
      tone === 'success' && 'border-primary/35 bg-primary/10 text-primary',
      tone === 'warning' && 'border-amber-700/30 bg-amber-500/10 text-amber-700',
      tone === 'info' && 'border-cyan-700/30 bg-cyan-600/10 text-cyan-700',
      tone === 'neutral' && 'border-border bg-muted/50 text-muted-foreground',
    )}>
      <span className="signal-light size-1.5 rounded-full bg-current" aria-hidden="true" />
      {children}
    </span>
  )
}

export function ComparisonDelta({ seconds, isBest }: { seconds: number; isBest?: boolean }) {
  if (isBest || seconds === 0) {
    return (
      <span className="inline-flex items-center gap-1.5 font-mono text-xs font-bold uppercase text-primary">
        <Trophy className="size-3.5" />
        Personal best
      </span>
    )
  }
  const absolute = Math.abs(Math.round(seconds))
  const minutes = Math.floor(absolute / 60)
  const remainder = absolute % 60
  return (
    <span className={cn('mono-data font-mono text-sm font-bold', seconds > 0 ? 'text-amber-700' : 'text-primary')}>
      {seconds > 0 ? '+' : '-'}{String(minutes).padStart(2, '0')}:{String(remainder).padStart(2, '0')}
    </span>
  )
}

export function EmptyState({ title, description, compact = false }: { title: string; description: string; compact?: boolean }) {
  return (
    <div className={cn('race-grid grid place-items-center px-5 text-center', compact ? 'py-8' : 'min-h-72 py-12')}>
      <div className="max-w-sm">
        <span className="mx-auto grid size-12 place-items-center border border-primary/25 bg-primary/5 text-primary">
          <MapPinned className="size-5" />
        </span>
        <h3 className="mt-4 text-sm font-bold uppercase tracking-[0.08em]">{title}</h3>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">{description}</p>
      </div>
    </div>
  )
}

export function ErrorState({ title, description, onRetry }: { title: string; description: string; onRetry?: () => void }) {
  return (
    <div role="alert" className="m-4 flex items-start gap-3 border border-destructive/35 bg-destructive/8 p-4">
      <AlertTriangle className="mt-0.5 size-5 shrink-0 text-destructive" />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-bold uppercase">{title}</p>
        <p className="mt-1 text-sm text-muted-foreground">{description}</p>
      </div>
      {onRetry ? <Button variant="outline" size="sm" onClick={onRetry}>Retry</Button> : null}
    </div>
  )
}

export function LoadingSkeleton({ rows = 4 }: { rows?: number }) {
  return (
    <div className="space-y-2" aria-busy="true" aria-label="Loading content">
      {Array.from({ length: rows }).map((_, index) => <Skeleton key={index} className="h-20" />)}
    </div>
  )
}

export function RouteTypeBadge({ isLoop }: { isLoop: boolean }) {
  return (
    <span className="inline-flex items-center gap-1.5 font-mono text-[10px] font-bold uppercase text-muted-foreground">
      {isLoop ? <CircleDashed className="size-3.5" /> : <ChevronRight className="size-3.5" />}
      {isLoop ? 'Circuit' : 'Sprint'}
    </span>
  )
}

export function ActivityTypeBadge({ value }: { value?: string | null }) {
  return <StatusBadge tone={value ? 'info' : 'neutral'}>{value?.trim() || 'Unclassified'}</StatusBadge>
}

export function InlineStatus({ state, children }: { state: 'ready' | 'loading' | 'empty'; children: ReactNode }) {
  const Icon = state === 'ready' ? Check : state === 'loading' ? LoaderCircle : Inbox
  return (
    <span className="inline-flex items-center gap-1.5 font-mono text-[10px] uppercase text-muted-foreground">
      <Icon className={cn('size-3.5', state === 'loading' && 'animate-spin')} />
      {children}
    </span>
  )
}
