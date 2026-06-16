import { useEffect, useRef, useState } from 'react'
import { CircleDot, Crosshair, Download, Flag, ImageOff, Maximize2, Minimize2, Radio, Route } from 'lucide-react'
import { toast } from 'sonner'
import { downloadBlob } from '@/lib/downloadBlob'
import { SPEED_RAMP_CSS } from './providers/routeSegments'
import { OsmMapProvider } from './providers/OsmMapProvider'
import { OpenFreeMapProvider } from './providers/OpenFreeMapProvider'
import type {
  MapPoint,
  MapProvider,
  MapProviderState,
  MapProviderType,
  ResolvedMapProviderType,
} from './providers/MapProvider'

export type { MapPoint, MapProvider, MapProviderType }
export { OsmMapProvider, OpenFreeMapProvider }

export interface RouteMapProps {
  points: MapPoint[]
  highlightedPath?: MapPoint[]
  selectedPoint?: MapPoint | null
  startPoint?: MapPoint
  finishPoint?: MapPoint
  colorByTelemetry?: boolean
  fadeByRecency?: boolean
  height?: string | number
  provider?: MapProviderType
  onProviderFallback?: (from: string, to: string, reason: string) => void
}

function createProvider(type: ResolvedMapProviderType): MapProvider {
  if (type === 'maplibre-vector') return new OpenFreeMapProvider()
  return new OsmMapProvider()
}

function getInitialProvider(provider: MapProviderType): ResolvedMapProviderType {
  if (provider === 'osm') return 'osm'
  return 'maplibre-vector'
}

const PROVIDER_OPTIONS: { value: ResolvedMapProviderType; label: string }[] = [
  { value: 'maplibre-vector', label: 'Vector' },
  { value: 'osm', label: 'OSM' },
]

function getFallbackProvider(
  active: ResolvedMapProviderType,
): ResolvedMapProviderType | null {
  if (active === 'maplibre-vector') return 'osm'
  return null
}

export function RouteMap({
  points,
  highlightedPath,
  selectedPoint,
  startPoint,
  finishPoint,
  colorByTelemetry = false,
  fadeByRecency = false,
  height = '100%',
  provider = 'auto',
  onProviderFallback,
}: RouteMapProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const providerRef = useRef<MapProvider | null>(null)
  const fallbackRef = useRef(onProviderFallback)
  const stateRef = useRef<MapProviderState>({ points, highlightedPath, selectedPoint, startPoint, finishPoint, colorByTelemetry, fadeByRecency, showPoints: false })
  const manualSelectionRef = useRef(false)
  const [activeProvider, setActiveProvider] = useState<ResolvedMapProviderType>(() => getInitialProvider(provider))
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const [exporting, setExporting] = useState(false)
  const [showPoints, setShowPoints] = useState(false)
  const [fullscreen, setFullscreen] = useState(false)

  fallbackRef.current = onProviderFallback
  stateRef.current = { points, highlightedPath, selectedPoint, startPoint, finishPoint, colorByTelemetry, fadeByRecency, showPoints }

  useEffect(() => {
    manualSelectionRef.current = false
    setActiveProvider(getInitialProvider(provider))
  }, [provider])

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    let cancelled = false
    let ready = false
    const abortController = new AbortController()
    const mapProvider = createProvider(activeProvider)
    providerRef.current = mapProvider
    container.replaceChildren()
    setStatus('loading')

    const fallback = (reason: string) => {
      if (cancelled) return
      // Once the provider has loaded, transient tile errors (e.g. while
      // panning after a filter change) must not switch the provider.
      if (ready) return
      if (manualSelectionRef.current) {
        setStatus('error')
        return
      }
      const nextProvider = getFallbackProvider(activeProvider)
      if (!nextProvider) {
        setStatus('error')
        return
      }
      setStatus('error')
      fallbackRef.current?.(activeProvider, nextProvider, reason)
      setActiveProvider(nextProvider)
    }

    mapProvider.mount(container, {
      ...stateRef.current,
      onError: fallback,
      signal: abortController.signal,
    }).then(() => {
      if (cancelled) return
      ready = true
      setStatus('ready')
      // Points may have arrived while the map style was still loading; the
      // update effect's setData is dropped before the source exists. Re-apply
      // the latest state now that layers are ready.
      mapProvider.update(stateRef.current)
    }).catch((error: unknown) => {
      fallback(error instanceof Error ? error.message : 'Map initialization failed')
    })

    return () => {
      cancelled = true
      abortController.abort()
      mapProvider.destroy()
      if (providerRef.current === mapProvider) providerRef.current = null
    }
  }, [activeProvider, provider])

  useEffect(() => providerRef.current?.update(stateRef.current), [points, highlightedPath, selectedPoint, startPoint, finishPoint, colorByTelemetry, fadeByRecency, showPoints])

  // Exit fullscreen on Escape.
  useEffect(() => {
    if (!fullscreen) return
    const onKey = (event: KeyboardEvent) => { if (event.key === 'Escape') setFullscreen(false) }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [fullscreen])

  const canExport = status === 'ready' && typeof providerRef.current?.exportImage === 'function'

  async function handleExport(background: boolean) {
    const exportImage = providerRef.current?.exportImage
    if (!exportImage || exporting) return
    setExporting(true)
    try {
      const blob = await exportImage.call(providerRef.current, { background })
      const suffix = background ? 'map' : 'trace'
      downloadBlob(blob, `route-${suffix}-${Date.now()}.png`)
      toast.success(background ? 'Exported route with map' : 'Exported transparent route')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Export failed')
    } finally {
      setExporting(false)
    }
  }

  const isDark = activeProvider === 'maplibre-vector'
  const overlayBg = isDark ? 'bg-[#0a0e14]/85' : 'bg-white/90'
  const overlayBorder = isDark ? 'border-[#1e2c40]/80' : 'border-slate-400/70'
  const overlayText = isDark ? 'text-[#4a6688]' : 'text-slate-700'
  const overlayTextBright = isDark ? 'text-[#39d353]' : 'text-slate-800'
  const speeds = colorByTelemetry
    ? points.flatMap(point => point.speedMps == null ? [] : [point.speedMps])
    : []
  const minSpeed = speeds.length > 0 ? Math.min(...speeds) : 0
  const maxSpeed = speeds.length > 0 ? Math.max(...speeds) : 0
  // Colors are spread by percentile, so the mid-ramp color marks the median
  // speed (not the arithmetic midpoint).
  const sortedSpeeds = [...speeds].sort((a, b) => a - b)
  const middleSpeed = sortedSpeeds.length > 0 ? sortedSpeeds[Math.floor((sortedSpeeds.length - 1) / 2)] : 0

  return (
    <div
      className={`overflow-hidden ${fullscreen ? 'fixed inset-0 z-[60]' : 'relative'}`}
      style={{
        width: fullscreen ? '100vw' : '100%',
        height: fullscreen ? '100vh' : height,
        minHeight: !fullscreen && height === '100%' ? 320 : undefined,
        background: isDark ? '#0a0e14' : '#e8ecec',
      }}
    >
      <div ref={containerRef} data-map-provider={activeProvider} data-map-status={status} className="absolute inset-0 h-full w-full" />

      {isDark ? (
        <>
          <div className="pointer-events-none absolute inset-0 z-10 bg-[radial-gradient(circle_at_72%_18%,rgba(57,211,83,0.06),transparent_24rem)]" />
          <div className="pointer-events-none absolute inset-x-0 top-0 z-10 h-16 bg-gradient-to-b from-[#0a0e14]/60 to-transparent" />
          <div className="pointer-events-none absolute bottom-0 left-0 z-10 h-24 w-56 bg-gradient-to-tr from-[#0a0e14]/50 to-transparent" />
        </>
      ) : (
        <>
          <div className="pointer-events-none absolute inset-0 z-10 bg-[radial-gradient(circle_at_72%_18%,rgba(69,185,54,0.08),transparent_24rem),linear-gradient(180deg,rgba(255,255,255,0.18),rgba(20,32,51,0.035))]" />
          <div className="pointer-events-none absolute inset-x-0 top-0 z-10 h-20 bg-gradient-to-b from-white/75 via-white/20 to-transparent" />
          <div className="pointer-events-none absolute bottom-0 left-0 z-10 h-28 w-64 bg-gradient-to-tr from-white/70 to-transparent" />
        </>
      )}

      <div className={`absolute left-3 top-3 z-20 flex items-center gap-2 border ${overlayBorder} ${overlayBg} px-3 py-2 shadow-sm backdrop-blur`}>
        <Radio className={status === 'ready' ? `size-3.5 ${isDark ? 'text-[#39d353]' : 'text-primary'}` : 'size-3.5 animate-pulse text-amber-500'} />
        <span className={`font-mono text-[9px] font-bold uppercase tracking-[0.12em] ${overlayTextBright}`}>
          {activeProvider} / {status}
        </span>
      </div>

      <div className="absolute right-3 top-3 z-20 flex flex-col items-end gap-2">
        <div className={`flex border ${overlayBorder} ${overlayBg} shadow-sm backdrop-blur`}>
          {PROVIDER_OPTIONS.map(({ value, label }) => (
            <button
              key={value}
              type="button"
              onClick={() => { manualSelectionRef.current = true; setActiveProvider(value) }}
              className={`px-2.5 py-1.5 font-mono text-[8px] font-bold uppercase tracking-[0.1em] transition-colors ${
                activeProvider === value
                  ? isDark
                    ? 'bg-[#39d353]/15 text-[#39d353]'
                    : 'bg-primary/10 text-primary'
                  : `${overlayText} hover:opacity-80`
              }`}
            >
              {label}
            </button>
          ))}
        </div>

        <div className={`flex border ${overlayBorder} ${overlayBg} shadow-sm backdrop-blur`}>
          <button
            type="button"
            onClick={() => setShowPoints(value => !value)}
            aria-pressed={showPoints}
            title={showPoints ? 'Hide GPS points' : 'Show GPS points'}
            className={`flex items-center gap-1.5 px-2.5 py-1.5 font-mono text-[8px] font-bold uppercase tracking-[0.1em] transition-colors ${
              showPoints
                ? isDark ? 'bg-[#39d353]/15 text-[#39d353]' : 'bg-primary/10 text-primary'
                : `${overlayText} hover:opacity-80`
            }`}
          >
            <CircleDot className="size-3" /> Points
          </button>
          <button
            type="button"
            onClick={() => setFullscreen(value => !value)}
            aria-pressed={fullscreen}
            title={fullscreen ? 'Exit fullscreen (Esc)' : 'Fullscreen map'}
            className={`flex items-center gap-1.5 border-l ${isDark ? 'border-[#1e2c40]/60' : 'border-slate-300'} px-2.5 py-1.5 font-mono text-[8px] font-bold uppercase tracking-[0.1em] transition-colors ${overlayText} hover:opacity-80`}
          >
            {fullscreen ? <Minimize2 className="size-3" /> : <Maximize2 className="size-3" />} Full
          </button>
        </div>

        {canExport ? (
          <div className={`flex border ${overlayBorder} ${overlayBg} shadow-sm backdrop-blur`}>
            <button
              type="button"
              onClick={() => handleExport(true)}
              disabled={exporting}
              title="Export PNG with map background"
              className={`flex items-center gap-1.5 px-2.5 py-1.5 font-mono text-[8px] font-bold uppercase tracking-[0.1em] transition-colors disabled:opacity-50 ${overlayText} hover:opacity-80`}
            >
              <Download className="size-3" /> PNG
            </button>
            <button
              type="button"
              onClick={() => handleExport(false)}
              disabled={exporting}
              title="Export transparent PNG (route only, no background)"
              className={`flex items-center gap-1.5 border-l ${isDark ? 'border-[#1e2c40]/60' : 'border-slate-300'} px-2.5 py-1.5 font-mono text-[8px] font-bold uppercase tracking-[0.1em] transition-colors disabled:opacity-50 ${overlayText} hover:opacity-80`}
            >
              <ImageOff className="size-3" /> No BG
            </button>
          </div>
        ) : null}
      </div>

      <div className={`absolute bottom-3 left-3 z-20 flex border ${overlayBorder} ${overlayBg} shadow-sm backdrop-blur`}>
        <div className={`flex items-center gap-2 border-r ${isDark ? 'border-[#1e2c40]/60' : 'border-slate-300'} px-3 py-2`}>
          <span className={`size-2.5 rounded-full border-2 ${isDark ? 'border-[#0a0e14] bg-[#39d353] shadow-[0_0_0_1px_#39d353]' : 'border-white bg-[#45b936] shadow-[0_0_0_1px_#45b936]'}`} />
          <span className={`font-mono text-[8px] font-bold uppercase ${overlayText}`}>Start gate</span>
        </div>
        <div className={`flex items-center gap-2 border-r ${isDark ? 'border-[#1e2c40]/60' : 'border-slate-300'} px-3 py-2`}>
          <Flag className={`size-3.5 ${isDark ? 'text-[#4a6688]' : 'text-[#152238]'}`} />
          <span className={`font-mono text-[8px] font-bold uppercase ${overlayText}`}>Finish gate</span>
        </div>
        <div className="flex items-center gap-2 px-3 py-2">
          <Crosshair className="size-3.5 text-amber-500" />
          <span className={`font-mono text-[8px] font-bold uppercase ${overlayText}`}>Selected</span>
        </div>
      </div>

      <div className={`absolute bottom-3 right-3 z-20 hidden items-center gap-2 border ${overlayBorder} ${overlayBg} px-3 py-2 shadow-sm backdrop-blur sm:flex`}>
        {colorByTelemetry ? (
          <div className="w-44">
            <div className={`mb-1 flex justify-between font-mono text-[8px] font-bold uppercase tracking-[0.1em] ${overlayText}`}>
              <span>Speed</span>
              <span>m/s</span>
            </div>
            <div
              className="h-2 rounded-full"
              style={{ background: SPEED_RAMP_CSS }}
            />
            <div className={`mt-1 flex justify-between font-mono text-[8px] font-bold tabular-nums ${overlayTextBright}`}>
              <span>{minSpeed.toFixed(1)}</span>
              <span>{middleSpeed.toFixed(1)}</span>
              <span>{maxSpeed.toFixed(1)}</span>
            </div>
          </div>
        ) : (
          <>
            <Route className={`size-3.5 ${isDark ? 'text-[#39d353]' : 'text-primary'}`} />
            <span className={`font-mono text-[8px] font-bold uppercase tracking-[0.12em] ${overlayText}`}>{points.length} route nodes</span>
          </>
        )}
      </div>
    </div>
  )
}
