import type { MapPoint } from './providers/MapProvider'

// Plain, serializable properties attached to each GPS node feature so the map
// can show details on hover (brief) and click (full).
export interface PointProperties {
  recordedAt: string | null
  speedMps: number | null
  accuracyM: number | null
  activityType: string | null
  kind: string | null
  eventId: string | null
  segmentId: number | null
}

export function pointProperties(p: MapPoint): PointProperties {
  return {
    recordedAt: p.recordedAt ?? null,
    speedMps: p.speedMps ?? null,
    accuracyM: p.accuracyM ?? null,
    activityType: p.activityType ?? null,
    kind: p.kind ?? null,
    eventId: p.eventId ?? null,
    segmentId: p.segmentId ?? null,
  }
}

const ESCAPES: Record<string, string> = {
  '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
}

function esc(value: unknown): string {
  return String(value).replace(/[&<>"']/g, ch => ESCAPES[ch])
}

function fmtTime(iso: string | null): string {
  if (!iso) return '—'
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleTimeString()
}

function fmtSpeed(value: number | null): string {
  return value == null ? '—' : `${value.toFixed(1)} m/s`
}

function fmtAccuracy(value: number | null): string {
  return value == null ? '—' : `±${Math.round(value)} m`
}

// Brief tooltip shown on hover: time + speed only.
export function briefPointHtml(p: PointProperties): string {
  return (
    `<div style="font:600 11px/1.3 ui-monospace,monospace;white-space:nowrap">` +
    `${esc(fmtTime(p.recordedAt))} · ${esc(fmtSpeed(p.speedMps))}</div>`
  )
}

// Full details shown on click.
export function fullPointHtml(p: PointProperties): string {
  const rows: [string, string][] = [
    ['Time', fmtTime(p.recordedAt)],
    ['Speed', fmtSpeed(p.speedMps)],
    ['Accuracy', fmtAccuracy(p.accuracyM)],
    ['Kind', p.kind ?? '—'],
    ['Activity', p.activityType ?? '—'],
    ['Segment', p.segmentId == null ? '—' : String(p.segmentId)],
    ['Event', p.eventId ?? '—'],
  ]
  const body = rows
    .map(([label, value]) =>
      `<div style="display:flex;gap:16px;justify-content:space-between">` +
      `<span style="color:#64748b">${esc(label)}</span>` +
      `<b style="font-variant-numeric:tabular-nums">${esc(value)}</b></div>`,
    )
    .join('')
  return `<div style="font:11px/1.5 ui-monospace,monospace;min-width:180px">${body}</div>`
}
