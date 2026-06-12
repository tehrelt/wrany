import type {
  MapPoint,
  MapProvider,
  MapProviderOptions,
  MapProviderState,
} from "./MapProvider";
import { getRouteBounds } from "./MapProvider";

const SCRIPT_ID = "yandex-maps-api";
const LOAD_TIMEOUT_MS = 10_000;
const DEFAULT_CENTER: [number, number] = [55.751, 37.618];

interface YandexGeoObject {
  geometry: { setCoordinates: (coordinates: unknown) => void };
  options: { set: (options: Record<string, unknown>) => void };
}

interface YandexMap {
  container: { fitToViewport: () => void };
  geoObjects: {
    add: (object: YandexGeoObject) => void;
    removeAll: () => void;
  };
  setBounds: (
    bounds: [[number, number], [number, number]],
    options: Record<string, unknown>,
  ) => void;
  destroy: () => void;
}

interface YandexMapsApi {
  ready: (success: () => void, error?: (error: Error) => void) => void;
  Map: new (
    container: HTMLElement,
    state: Record<string, unknown>,
    options?: Record<string, unknown>,
  ) => YandexMap;
  Polyline: new (
    coordinates: [number, number][],
    properties?: Record<string, unknown>,
    options?: Record<string, unknown>,
  ) => YandexGeoObject;
  Placemark: new (
    coordinates: [number, number],
    properties?: Record<string, unknown>,
    options?: Record<string, unknown>,
  ) => YandexGeoObject;
}

declare global {
  interface Window {
    ymaps?: YandexMapsApi;
  }
}

let yandexLoader: Promise<YandexMapsApi> | null = null;

function loadYandexMaps(apiKey: string): Promise<YandexMapsApi> {
  if (window.ymaps) return Promise.resolve(window.ymaps);
  if (yandexLoader) return yandexLoader;

  yandexLoader = new Promise<YandexMapsApi>((resolve, reject) => {
    const timeout = window.setTimeout(() => {
      reject(new Error("Yandex Maps loading timed out"));
    }, LOAD_TIMEOUT_MS);
    const fail = (reason: string) => {
      window.clearTimeout(timeout);
      yandexLoader = null;
      reject(new Error(reason));
    };
    const ready = () => {
      if (!window.ymaps) {
        fail("Yandex Maps API is unavailable");
        return;
      }
      window.ymaps.ready(
        () => {
          window.clearTimeout(timeout);
          resolve(window.ymaps as YandexMapsApi);
        },
        () => fail("Yandex Maps initialization failed"),
      );
    };

    const existing = document.getElementById(SCRIPT_ID) as
      | HTMLScriptElement
      | null;
    if (existing) {
      existing.addEventListener("load", ready, { once: true });
      existing.addEventListener(
        "error",
        () => fail("Yandex Maps script was blocked"),
        { once: true },
      );
      return;
    }

    const script = document.createElement("script");
    script.id = SCRIPT_ID;
    script.async = true;
    script.src = `https://api-maps.yandex.ru/2.1/?apikey=${encodeURIComponent(apiKey)}&lang=ru_RU`;
    script.addEventListener("load", ready, { once: true });
    script.addEventListener(
      "error",
      () => fail("Yandex Maps script failed to load"),
      { once: true },
    );
    document.head.append(script);
  });

  return yandexLoader;
}

function toYandexPoint(point: MapPoint): [number, number] {
  return [point.lat, point.lon];
}

export class YandexMapProvider implements MapProvider {
  readonly type = "yandex" as const;
  private map: YandexMap | null = null;
  private ymaps: YandexMapsApi | null = null;
  private onWindowError: ((event: Event) => void) | null = null;
  private onUnhandledRejection:
    | ((event: PromiseRejectionEvent) => void)
    | null = null;

  constructor(private readonly apiKey: string) {}

  async mount(
    container: HTMLElement,
    options: MapProviderOptions,
  ): Promise<void> {
    if (!this.apiKey) throw new Error("Yandex Maps API key is missing");

    this.ymaps = await loadYandexMaps(this.apiKey);
    if (options.signal.aborted) return;
    this.onWindowError = (event) => {
      const errorEvent = event as ErrorEvent;
      const target = event.target as HTMLImageElement | HTMLScriptElement | null;
      const source =
        errorEvent.filename ||
        errorEvent.message ||
        target?.src ||
        "";
      if (/yandex|ymaps|api-maps/i.test(source)) {
        options.onError(errorEvent.message || "Yandex Maps resource error");
      }
    };
    this.onUnhandledRejection = (event) => {
      const reason =
        event.reason instanceof Error
          ? event.reason.message
          : String(event.reason ?? "");
      if (/yandex|ymaps|api-maps|quota/i.test(reason)) {
        options.onError(reason || "Yandex Maps request failed");
      }
    };
    window.addEventListener("error", this.onWindowError, true);
    window.addEventListener(
      "unhandledrejection",
      this.onUnhandledRejection,
    );

    try {
      this.map = new this.ymaps.Map(
        container,
        { center: DEFAULT_CENTER, zoom: 11, controls: ["zoomControl"] },
        { suppressMapOpenBlock: true },
      );
      if (options.signal.aborted) {
        this.destroy();
        return;
      }
      this.map.container.fitToViewport();
      this.update(options);
    } catch (error) {
      this.destroy();
      throw error;
    }
  }

  update(state: MapProviderState): void {
    if (!this.map || !this.ymaps) return;

    this.map.geoObjects.removeAll();
    if (state.points.length > 1) {
      this.map.geoObjects.add(
        new this.ymaps.Polyline(
          state.points.map(toYandexPoint),
          {},
          { strokeColor: "#3b82f6", strokeWidth: 4, strokeOpacity: 0.9 },
        ),
      );
    }

    const markers = [
      {
        point: state.startPoint ?? state.points[0],
        color: "#22c55e",
      },
      {
        point: state.finishPoint ?? state.points[state.points.length - 1],
        color: "#ef4444",
      },
      { point: state.selectedPoint, color: "#f59e0b" },
    ];
    for (const marker of markers) {
      if (!marker.point) continue;
      this.map.geoObjects.add(
        new this.ymaps.Placemark(
          toYandexPoint(marker.point),
          {},
          {
            preset: "islands#circleDotIcon",
            iconColor: marker.color,
          },
        ),
      );
    }

    const bounds = getRouteBounds(state);
    if (bounds) {
      this.map.setBounds(
        [
          [bounds[0][1], bounds[0][0]],
          [bounds[1][1], bounds[1][0]],
        ],
        { checkZoomRange: true, zoomMargin: 56, duration: 300 },
      );
    }
  }

  destroy(): void {
    if (this.onWindowError) {
      window.removeEventListener("error", this.onWindowError, true);
      this.onWindowError = null;
    }
    if (this.onUnhandledRejection) {
      window.removeEventListener(
        "unhandledrejection",
        this.onUnhandledRejection,
      );
      this.onUnhandledRejection = null;
    }
    this.map?.destroy();
    this.map = null;
    this.ymaps = null;
  }
}
