import { useEffect, useRef, useState } from 'react'
import { Crosshair, Flag, Radio, Route } from 'lucide-react'
import { YANDEX_MAPS_API_KEY } from '@/config/env'
import { OsmMapProvider } from './providers/OsmMapProvider'
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
export { OsmMapProvider, YandexMapProvider, YandexMapV2Provider }

export interface RouteMapProps {
  points: MapPoint[]
  selectedPoint?: MapPoint | null
  startPoint?: MapPoint
  finishPoint?: MapPoint
  height?: string | number
  provider?: MapProviderType
  onProviderFallback?: (from: string, to: string, reason: string) => void
}

function createProvider(type: ResolvedMapProviderType): MapProvider {
  if (type === 'yandex') return new YandexMapProvider(YANDEX_MAPS_API_KEY)
  if (type === 'yandex-v2') return new YandexMapV2Provider(YANDEX_MAPS_API_KEY)
  return new OsmMapProvider()
}

function getInitialProvider(provider: MapProviderType): ResolvedMapProviderType {
  if (provider === 'osm') return 'osm'
  if (provider === 'yandex-v2') return 'yandex-v2'
  return 'yandex'
}

function getFallbackProvider(
  active: ResolvedMapProviderType,
): ResolvedMapProviderType | null {
  if (active === 'yandex') return 'yandex-v2'
  if (active === 'yandex-v2') return 'osm'
  return null
}

export function RouteMap({
  points,
  selectedPoint,
  startPoint,
  finishPoint,
  height = '100%',
  provider = 'auto',
  onProviderFallback,
}: RouteMapProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const providerRef = useRef<MapProvider | null>(null)
  const fallbackRef = useRef(onProviderFallback)
  const stateRef = useRef<MapProviderState>({ points, selectedPoint, startPoint, finishPoint })
  const [activeProvider, setActiveProvider] = useState<ResolvedMapProviderType>(() => getInitialProvider(provider))
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')

  fallbackRef.current = onProviderFallback
  stateRef.current = { points, selectedPoint, startPoint, finishPoint }

  useEffect(() => setActiveProvider(getInitialProvider(provider)), [provider])

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    let cancelled = false
    const abortController = new AbortController()
    const mapProvider = createProvider(activeProvider)
    providerRef.current = mapProvider
    container.replaceChildren()
    setStatus('loading')

    const fallback = (reason: string) => {
      if (cancelled) return
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
      if (!cancelled) setStatus('ready')
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

  useEffect(() => providerRef.current?.update(stateRef.current), [points, selectedPoint, startPoint, finishPoint])

  return (
    <div className="relative overflow-hidden bg-[#e8ecec]" style={{ width: '100%', height, minHeight: height === '100%' ? 320 : undefined }}>
      <div ref={containerRef} data-map-provider={activeProvider} data-map-status={status} className="absolute inset-0 grayscale-[15%]" />
      <div className="pointer-events-none absolute inset-0 z-10 bg-[radial-gradient(circle_at_72%_18%,rgba(69,185,54,0.08),transparent_24rem),linear-gradient(180deg,rgba(255,255,255,0.18),rgba(20,32,51,0.035))]" />
      <div className="pointer-events-none absolute inset-x-0 top-0 z-10 h-20 bg-gradient-to-b from-white/75 via-white/20 to-transparent" />
      <div className="pointer-events-none absolute bottom-0 left-0 z-10 h-28 w-64 bg-gradient-to-tr from-white/70 to-transparent" />

      <div className="absolute left-3 top-3 z-20 flex items-center gap-2 border border-slate-400/70 bg-white/90 px-3 py-2 shadow-sm backdrop-blur">
        <Radio className={status === 'ready' ? 'size-3.5 text-primary' : 'size-3.5 animate-pulse text-amber-600'} />
        <span className="font-mono text-[9px] font-bold uppercase tracking-[0.12em] text-slate-800">
          Map link / {activeProvider} / {status}
        </span>
      </div>

      <div className="absolute bottom-3 left-3 z-20 flex border border-slate-400/70 bg-white/92 shadow-sm backdrop-blur">
        <div className="flex items-center gap-2 border-r border-slate-300 px-3 py-2">
          <span className="size-2.5 rounded-full border-2 border-white bg-[#45b936] shadow-[0_0_0_1px_#45b936]" />
          <span className="font-mono text-[8px] font-bold uppercase text-slate-700">Start gate</span>
        </div>
        <div className="flex items-center gap-2 border-r border-slate-300 px-3 py-2">
          <Flag className="size-3.5 text-[#152238]" />
          <span className="font-mono text-[8px] font-bold uppercase text-slate-700">Finish gate</span>
        </div>
        <div className="flex items-center gap-2 px-3 py-2">
          <Crosshair className="size-3.5 text-amber-600" />
          <span className="font-mono text-[8px] font-bold uppercase text-slate-700">Selected</span>
        </div>
      </div>

      <div className="absolute bottom-3 right-3 z-20 hidden items-center gap-2 border border-slate-400/70 bg-white/92 px-3 py-2 shadow-sm backdrop-blur sm:flex">
        <Route className="size-3.5 text-primary" />
        <span className="font-mono text-[8px] font-bold uppercase tracking-[0.12em] text-slate-700">{points.length} route nodes</span>
      </div>
    </div>
  )
}
