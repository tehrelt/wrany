import type {
  MapPoint,
  MapProvider,
  MapProviderOptions,
  MapProviderState,
} from './MapProvider'
import { getRouteBounds } from './MapProvider'
import { mapTokens } from './yandexTelemetryMapStyle'
import { buildRoutePolylines, buildTelemetrySegments } from './routeSegments'

const SCRIPT_ID = 'yandex-maps-api-v2'
const LOAD_TIMEOUT_MS = 10_000
const DEFAULT_CENTER: [number, number] = [55.751, 37.618]

interface YandexGeoObject {
  geometry: { setCoordinates: (coordinates: unknown) => void }
}

interface YandexMap {
  container: { fitToViewport: () => void }
  geoObjects: {
    add: (object: YandexGeoObject) => void
    removeAll: () => void
  }
  setBounds: (
    bounds: [[number, number], [number, number]],
    options: Record<string, unknown>,
  ) => void
  destroy: () => void
}

interface YandexMapsV2 {
  ready: (success: () => void, error?: () => void) => void
  Map: new (
    container: HTMLElement,
    state: Record<string, unknown>,
    options?: Record<string, unknown>,
  ) => YandexMap
  Polyline: new (
    coordinates: [number, number][],
    properties?: Record<string, unknown>,
    options?: Record<string, unknown>,
  ) => YandexGeoObject
  Placemark: new (
    coordinates: [number, number],
    properties?: Record<string, unknown>,
    options?: Record<string, unknown>,
  ) => YandexGeoObject
}

declare global {
  interface Window {
    ymaps?: YandexMapsV2
  }
}

let yandexV2Loader: Promise<YandexMapsV2> | null = null

function loadYandexMapsV2(apiKey: string): Promise<YandexMapsV2> {
  if (window.ymaps) return Promise.resolve(window.ymaps)
  if (yandexV2Loader) return yandexV2Loader

  yandexV2Loader = new Promise<YandexMapsV2>((resolve, reject) => {
    const timeout = window.setTimeout(() => {
      yandexV2Loader = null
      reject(new Error('Yandex Maps v2.1 loading timed out'))
    }, LOAD_TIMEOUT_MS)

    const ready = () => {
      if (!window.ymaps) {
        window.clearTimeout(timeout)
        yandexV2Loader = null
        reject(new Error('Yandex Maps v2.1 is unavailable'))
        return
      }
      window.ymaps.ready(
        () => {
          window.clearTimeout(timeout)
          resolve(window.ymaps as YandexMapsV2)
        },
        () => reject(new Error('Yandex Maps v2.1 initialization failed')),
      )
    }

    const existing = document.getElementById(SCRIPT_ID) as HTMLScriptElement | null
    if (existing) {
      existing.addEventListener('load', ready, { once: true })
      existing.addEventListener('error', () => reject(new Error('Yandex Maps v2.1 script was blocked')), { once: true })
      return
    }

    const script = document.createElement('script')
    script.id = SCRIPT_ID
    script.async = true
    script.src = `https://api-maps.yandex.ru/2.1/?apikey=${encodeURIComponent(apiKey)}&lang=ru_RU`
    script.addEventListener('load', ready, { once: true })
    script.addEventListener('error', () => reject(new Error('Yandex Maps v2.1 script failed to load')), { once: true })
    document.head.append(script)
  })

  return yandexV2Loader
}

function toYandexPoint(point: MapPoint): [number, number] {
  return [point.lat, point.lon]
}

export class YandexMapV2Provider implements MapProvider {
  readonly type = 'yandex-v2' as const
  private map: YandexMap | null = null
  private ymaps: YandexMapsV2 | null = null

  constructor(private readonly apiKey: string) {}

  async mount(container: HTMLElement, options: MapProviderOptions): Promise<void> {
    if (!this.apiKey) throw new Error('Yandex Maps API key is missing')

    this.ymaps = await loadYandexMapsV2(this.apiKey)
    if (options.signal.aborted) return

    this.map = new this.ymaps.Map(
      container,
      { center: DEFAULT_CENTER, zoom: 11, controls: ['zoomControl'] },
      { suppressMapOpenBlock: true },
    )
    this.map.container.fitToViewport()
    this.update(options)
  }

  update(state: MapProviderState): void {
    if (!this.map || !this.ymaps) return

    this.map.geoObjects.removeAll()

    // Continuous base route per run (split only at GPS gaps). v2 wants [lat,lon]
    // order. Always drawn so the trace stays continuous under dense telemetry.
    for (const line of buildRoutePolylines(state.points)) {
      const coordinates = line.map(([lon, lat]) => [lat, lon] as [number, number])
      this.map.geoObjects.add(new this.ymaps.Polyline(
        coordinates,
        {},
        { strokeColor: '#142033', strokeWidth: 11, strokeOpacity: 0.24 },
      ))
      this.map.geoObjects.add(new this.ymaps.Polyline(
        coordinates,
        {},
        { strokeColor: mapTokens.routePrimary, strokeWidth: 5, strokeOpacity: 0.96 },
      ))
    }

    // Speed-colored segments overlay the base line.
    if (state.colorByTelemetry) {
      for (const segment of buildTelemetrySegments(state.points)) {
        const coordinates = [toYandexPoint(segment.from), toYandexPoint(segment.to)]
        this.map.geoObjects.add(new this.ymaps.Polyline(
          coordinates,
          {},
          { strokeColor: segment.color, strokeWidth: 5, strokeOpacity: segment.opacity },
        ))
      }
    }

    const markers = [
      { point: state.startPoint ?? state.points[0], color: mapTokens.start },
      { point: state.finishPoint ?? state.points[state.points.length - 1], color: mapTokens.finish },
      { point: state.selectedPoint, color: mapTokens.selected },
    ]

    for (const marker of markers) {
      if (!marker.point) continue
      this.map.geoObjects.add(new this.ymaps.Placemark(
        toYandexPoint(marker.point),
        {},
        { preset: 'islands#circleDotIcon', iconColor: marker.color },
      ))
    }

    const bounds = getRouteBounds(state)
    if (bounds) {
      this.map.setBounds(
        [[bounds[0][1], bounds[0][0]], [bounds[1][1], bounds[1][0]]],
        { checkZoomRange: true, zoomMargin: 56, duration: 300 },
      )
    }
  }

  destroy(): void {
    this.map?.destroy()
    this.map = null
    this.ymaps = null
  }
}
