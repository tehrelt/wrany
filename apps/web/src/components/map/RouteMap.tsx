import { useEffect, useRef, useState } from 'react'
import { Crosshair, Flag, Radio, Route } from 'lucide-react'
import { YANDEX_MAPS_API_KEY } from '@/config/env'
import { OsmMapProvider } from './providers/OsmMapProvider'
import { OpenFreeMapProvider } from './providers/OpenFreeMapProvider'
import { YandexMapProvider } from './providers/YandexMapProvider'
import { YandexMapV2Provider } from './providers/YandexMapV2Provider'
import type {
  MapPoint,
  MapProvider,
  MapProviderState,
  MapProviderType,
  ResolvedMapProviderType,
} from './providers/MapProvider'

export type { MapPoint, MapProvider, MapProviderType }
export { OsmMapProvider, OpenFreeMapProvider, YandexMapProvider, YandexMapV2Provider }

export interface RouteMapProps {
  points: MapPoint[]
  selectedPoint?: MapPoint | null
  startPoint?: MapPoint
  finishPoint?: MapPoint
  colorByTelemetry?: boolean
  height?: string | number
  provider?: MapProviderType
  onProviderFallback?: (from: string, to: string, reason: string) => void
}

function createProvider(type: ResolvedMapProviderType): MapProvider {
  if (type === 'yandex') return new YandexMapProvider(YANDEX_MAPS_API_KEY)
  if (type === 'yandex-v2') return new YandexMapV2Provider(YANDEX_MAPS_API_KEY)
  if (type === 'maplibre-vector') return new OpenFreeMapProvider()
  return new OsmMapProvider()
}

function getInitialProvider(provider: MapProviderType): ResolvedMapProviderType {
  if (provider === 'osm') return 'osm'
  if (provider === 'yandex') return 'yandex'
  if (provider === 'yandex-v2') return 'yandex-v2'
  return 'maplibre-vector'
}

const PROVIDER_OPTIONS: { value: ResolvedMapProviderType; label: string }[] = [
  { value: 'maplibre-vector', label: 'Vector' },
  { value: 'osm', label: 'OSM' },
  { value: 'yandex', label: 'Yandex' },
]

function getFallbackProvider(
  active: ResolvedMapProviderType,
): ResolvedMapProviderType | null {
  if (active === 'maplibre-vector') return 'osm'
  if (active === 'yandex') return 'yandex-v2'
  if (active === 'yandex-v2') return 'osm'
  return null
}

export function RouteMap({
  points,
  selectedPoint,
  startPoint,
  finishPoint,
  colorByTelemetry = false,
  height = '100%',
  provider = 'auto',
  onProviderFallback,
}: RouteMapProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const providerRef = useRef<MapProvider | null>(null)
  const fallbackRef = useRef(onProviderFallback)
  const stateRef = useRef<MapProviderState>({ points, selectedPoint, startPoint, finishPoint, colorByTelemetry })
  const manualSelectionRef = useRef(false)
  const [activeProvider, setActiveProvider] = useState<ResolvedMapProviderType>(() => getInitialProvider(provider))
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')

  fallbackRef.current = onProviderFallback
  stateRef.current = { points, selectedPoint, startPoint, finishPoint, colorByTelemetry }

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

  useEffect(() => providerRef.current?.update(stateRef.current), [points, selectedPoint, startPoint, finishPoint, colorByTelemetry])

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
  const middleSpeed = (minSpeed + maxSpeed) / 2

  return (
    <div
      className="relative overflow-hidden"
      style={{
        width: '100%',
        height,
        minHeight: height === '100%' ? 320 : undefined,
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

      <div className={`absolute right-3 top-3 z-20 flex border ${overlayBorder} ${overlayBg} shadow-sm backdrop-blur`}>
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
              style={{ background: 'linear-gradient(90deg, hsl(210 88% 54%), hsl(105 88% 54%), hsl(0 88% 54%))' }}
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
