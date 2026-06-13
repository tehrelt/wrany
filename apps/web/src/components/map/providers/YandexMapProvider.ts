import type {
  MapPoint,
  MapProvider,
  MapProviderOptions,
  MapProviderState,
} from './MapProvider'
import { getRouteBounds } from './MapProvider'
import { mapTokens, yandexTelemetryMapStyle } from './yandexTelemetryMapStyle'
import { buildRoutePolylines, buildTelemetrySegments } from './routeSegments'

const SCRIPT_ID = 'yandex-maps-api-v3'
const LOAD_TIMEOUT_MS = 10_000
const DEFAULT_LOCATION = { center: [37.618, 55.751], zoom: 11 }

interface YandexEntity {
  update?: (properties: Record<string, unknown>) => void
}

interface YandexMap {
  addChild: (entity: YandexEntity) => YandexMap
  removeChild: (entity: YandexEntity) => YandexMap
  update: (properties: Record<string, unknown>) => void
  destroy: () => void
}

interface YandexMapsV3 {
  ready: Promise<void>
  YMap: new (container: HTMLElement, properties: Record<string, unknown>) => YandexMap
  YMapDefaultSchemeLayer: new (properties: Record<string, unknown>) => YandexEntity
  YMapDefaultFeaturesLayer: new (properties?: Record<string, unknown>) => YandexEntity
  YMapFeature: new (properties: Record<string, unknown>) => YandexEntity
  YMapMarker: new (properties: Record<string, unknown>, element: HTMLElement) => YandexEntity
}

declare global {
  interface Window {
    ymaps3?: YandexMapsV3
  }
}

let yandexLoader: Promise<YandexMapsV3> | null = null

function loadYandexMaps(apiKey: string): Promise<YandexMapsV3> {
  if (window.ymaps3) return window.ymaps3.ready.then(() => window.ymaps3 as YandexMapsV3)
  if (yandexLoader) return yandexLoader

  yandexLoader = new Promise<YandexMapsV3>((resolve, reject) => {
    const timeout = window.setTimeout(() => {
      yandexLoader = null
      reject(new Error('Yandex Maps v3 loading timed out'))
    }, LOAD_TIMEOUT_MS)

    const finish = () => {
      if (!window.ymaps3) {
        window.clearTimeout(timeout)
        yandexLoader = null
        reject(new Error('Yandex Maps v3 is unavailable'))
        return
      }
      window.ymaps3.ready.then(() => {
        window.clearTimeout(timeout)
        resolve(window.ymaps3 as YandexMapsV3)
      }).catch(reject)
    }

    const existing = document.getElementById(SCRIPT_ID) as HTMLScriptElement | null
    if (existing) {
      existing.addEventListener('load', finish, { once: true })
      existing.addEventListener('error', () => reject(new Error('Yandex Maps v3 script was blocked')), { once: true })
      return
    }

    const script = document.createElement('script')
    script.id = SCRIPT_ID
    script.async = true
    script.src = `https://api-maps.yandex.ru/v3/?apikey=${encodeURIComponent(apiKey)}&lang=ru_RU`
    script.addEventListener('load', finish, { once: true })
    script.addEventListener('error', () => reject(new Error('Yandex Maps v3 script failed to load')), { once: true })
    document.head.append(script)
  })

  return yandexLoader
}

function markerElement(role: 'start' | 'finish' | 'selected'): HTMLElement {
  const element = document.createElement('div')
  element.className = `telemetry-map-marker telemetry-map-marker--${role}`
  element.setAttribute('aria-label', `${role} route point`)
  return element
}

function lineCoordinates(points: MapPoint[]): number[][] {
  return points.map(point => [point.lon, point.lat])
}

export class YandexMapProvider implements MapProvider {
  readonly type = 'yandex' as const
  private map: YandexMap | null = null
  private ymaps: YandexMapsV3 | null = null
  private entities: YandexEntity[] = []

  constructor(private readonly apiKey: string) {}

  async mount(container: HTMLElement, options: MapProviderOptions): Promise<void> {
    if (!this.apiKey) throw new Error('Yandex Maps API key is missing')

    this.ymaps = await loadYandexMaps(this.apiKey)
    if (options.signal.aborted) return

    this.map = new this.ymaps.YMap(container, { location: DEFAULT_LOCATION })
    this.map.addChild(new this.ymaps.YMapDefaultSchemeLayer({
      customization: yandexTelemetryMapStyle,
    }))
    this.map.addChild(new this.ymaps.YMapDefaultFeaturesLayer())
    this.update(options)

    await new Promise<void>(resolve => window.requestAnimationFrame(() => resolve()))
    if (!container.firstElementChild) {
      this.destroy()
      throw new Error('Yandex Maps v3 rendered no map content')
    }
  }

  update(state: MapProviderState): void {
    if (!this.map || !this.ymaps) return

    for (const entity of this.entities) this.map.removeChild(entity)
    this.entities = []

    // Continuous base route per run (split only at GPS gaps). Always drawn so
    // the trace stays continuous even where dense telemetry segments overlap.
    for (const coordinates of buildRoutePolylines(state.points)) {
      const route = new this.ymaps.YMapFeature({
        geometry: { type: 'LineString', coordinates },
        style: {
          stroke: [
            { color: mapTokens.routeGlow, width: 12 },
            { color: mapTokens.routePrimary, width: 5 },
          ],
        },
      })
      this.map.addChild(route)
      this.entities.push(route)
    }

    // Speed-colored segments overlay the base line.
    if (state.colorByTelemetry) {
      for (const segment of buildTelemetrySegments(state.points)) {
        const route = new this.ymaps.YMapFeature({
          geometry: {
            type: 'LineString',
            coordinates: lineCoordinates([segment.from, segment.to]),
          },
          style: {
            stroke: [{ color: segment.color, width: 5, opacity: segment.opacity }],
          },
        })
        this.map.addChild(route)
        this.entities.push(route)
      }
    }

    const markers = [
      { role: 'start' as const, point: state.startPoint ?? state.points[0] },
      { role: 'finish' as const, point: state.finishPoint ?? state.points[state.points.length - 1] },
      { role: 'selected' as const, point: state.selectedPoint },
    ]

    for (const marker of markers) {
      if (!marker.point) continue
      const entity = new this.ymaps.YMapMarker(
        { coordinates: [marker.point.lon, marker.point.lat] },
        markerElement(marker.role),
      )
      this.map.addChild(entity)
      this.entities.push(entity)
    }

    const bounds = getRouteBounds(state)
    if (bounds) {
      this.map.update({
        location: {
          bounds,
          easing: 'ease-in-out',
          duration: 350,
        },
      })
    }
  }

  destroy(): void {
    this.map?.destroy()
    this.map = null
    this.ymaps = null
    this.entities = []
  }
}
