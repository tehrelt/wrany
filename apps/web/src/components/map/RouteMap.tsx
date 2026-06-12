import { useEffect, useRef, useState } from "react";
import { YANDEX_MAPS_API_KEY } from "@/config/env";
import { OsmMapProvider } from "./providers/OsmMapProvider";
import { YandexMapProvider } from "./providers/YandexMapProvider";
import type {
  MapPoint,
  MapProvider,
  MapProviderState,
  MapProviderType,
  ResolvedMapProviderType,
} from "./providers/MapProvider";

export type { MapPoint, MapProvider, MapProviderType };
export { OsmMapProvider, YandexMapProvider };

export interface RouteMapProps {
  points: MapPoint[];
  selectedPoint?: MapPoint | null;
  startPoint?: MapPoint;
  finishPoint?: MapPoint;
  height?: string | number;
  provider?: MapProviderType;
  onProviderFallback?: (
    from: string,
    to: string,
    reason: string,
  ) => void;
}

function createProvider(type: ResolvedMapProviderType): MapProvider {
  return type === "yandex"
    ? new YandexMapProvider(YANDEX_MAPS_API_KEY)
    : new OsmMapProvider();
}

function getInitialProvider(provider: MapProviderType): ResolvedMapProviderType {
  return provider === "osm" ? "osm" : "yandex";
}

export function RouteMap({
  points,
  selectedPoint,
  startPoint,
  finishPoint,
  height = "100%",
  provider = "auto",
  onProviderFallback,
}: RouteMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const providerRef = useRef<MapProvider | null>(null);
  const fallbackRef = useRef(onProviderFallback);
  const stateRef = useRef<MapProviderState>({
    points,
    selectedPoint,
    startPoint,
    finishPoint,
  });
  const [activeProvider, setActiveProvider] =
    useState<ResolvedMapProviderType>(() => getInitialProvider(provider));
  const [status, setStatus] = useState<"loading" | "ready" | "error">(
    "loading",
  );
  const [debugMessage, setDebugMessage] = useState("");

  fallbackRef.current = onProviderFallback;
  stateRef.current = { points, selectedPoint, startPoint, finishPoint };

  useEffect(() => {
    setActiveProvider(getInitialProvider(provider));
  }, [provider]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    let cancelled = false;
    const abortController = new AbortController();
    const mapProvider = createProvider(activeProvider);
    providerRef.current = mapProvider;
    container.replaceChildren();
    setStatus("loading");
    setDebugMessage("");

    const fallback = (reason: string) => {
      if (cancelled || activeProvider !== "yandex") {
        return;
      }
      console.error(`[RouteMap] ${activeProvider} failed: ${reason}`);
      setStatus("error");
      setDebugMessage(reason);
      fallbackRef.current?.("yandex", "osm", reason);
      setActiveProvider("osm");
    };

    mapProvider
      .mount(container, {
        ...stateRef.current,
        onError: fallback,
        signal: abortController.signal,
      })
      .then(() => {
        if (!cancelled) {
          console.info(`[RouteMap] ${activeProvider} ready`);
          setStatus("ready");
        }
      })
      .catch((error: unknown) => {
        const reason =
          error instanceof Error ? error.message : "Map initialization failed";
        fallback(reason);
      });

    return () => {
      cancelled = true;
      abortController.abort();
      mapProvider.destroy();
      if (providerRef.current === mapProvider) providerRef.current = null;
    };
  }, [activeProvider, provider]);

  useEffect(() => {
    providerRef.current?.update(stateRef.current);
  }, [points, selectedPoint, startPoint, finishPoint]);

  return (
    <div
      className="relative"
      style={{
        width: "100%",
        height,
        minHeight: height === "100%" ? 320 : undefined,
      }}
    >
      <div
        ref={containerRef}
        data-map-provider={activeProvider}
        data-map-status={status}
        style={{ width: "100%", height: "100%" }}
      />
      <div className="absolute left-2 top-2 z-20 rounded bg-background/90 px-2 py-1 text-xs shadow">
        {activeProvider} · {status}
        {debugMessage ? ` · ${debugMessage}` : ""}
      </div>
    </div>
  );
}
