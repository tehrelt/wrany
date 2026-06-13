import type { Map as MapLibreMap, GeoJSONSource, LngLatBoundsLike } from "maplibre-gl";
import type { MapProvider, MapProviderOptions, MapProviderState } from "./MapProvider";
import { getRouteBounds } from "./MapProvider";

const SOURCE_ID = "route";
const DEFAULT_CENTER: [number, number] = [37.618, 55.751];

function toGeoJson(state: MapProviderState): GeoJSON.FeatureCollection {
  const features: GeoJSON.Feature[] = [];

  if (state.points.length >= 2) {
    features.push({
      type: "Feature",
      properties: { role: "route" },
      geometry: {
        type: "LineString",
        coordinates: state.points.map((p) => [p.lon, p.lat]),
      },
    });
  }

  for (const p of state.points) {
    features.push({
      type: "Feature",
      properties: { role: "node" },
      geometry: { type: "Point", coordinates: [p.lon, p.lat] },
    });
  }

  for (const [point, role] of [
    [state.startPoint ?? state.points[0], "start"],
    [state.finishPoint ?? state.points[state.points.length - 1], "finish"],
    [state.selectedPoint, "selected"],
  ] as const) {
    if (point) {
      features.push({
        type: "Feature",
        properties: { role },
        geometry: { type: "Point", coordinates: [point.lon, point.lat] },
      });
    }
  }

  return { type: "FeatureCollection", features };
}

export class OpenFreeMapProvider implements MapProvider {
  readonly type = "maplibre-vector" as const;
  private map: MapLibreMap | null = null;
  private state: MapProviderState = { points: [] };

  async mount(container: HTMLElement, options: MapProviderOptions): Promise<void> {
    this.state = options;
    const { Map } = await import("maplibre-gl");
    if (options.signal.aborted) return;

    this.map = new Map({
      container,
      center: DEFAULT_CENTER,
      zoom: 11,
      style: "https://tiles.openfreemap.org/styles/dark",
    });

    await new Promise<void>((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.map?.off("load", onLoad);
        this.map?.off("error", onInitialError);
        reject(new Error("Map load timeout"));
      }, 12_000);

      const onLoad = () => {
        clearTimeout(timeout);
        this.map?.off("error", onInitialError);
        resolve();
      };
      const onInitialError = (event: { error?: Error }) => {
        clearTimeout(timeout);
        this.map?.off("load", onLoad);
        reject(event.error ?? new Error("Map failed to load"));
      };

      this.map?.once("load", onLoad);
      this.map?.once("error", onInitialError);
    });

    if (options.signal.aborted) {
      this.destroy();
      return;
    }

    this.map.on("error", (event) => {
      options.onError(event.error?.message ?? "Map tile loading failed");
    });

    this.addLayers();
    this.update(options);
  }

  update(state: MapProviderState): void {
    this.state = state;
    const source = this.map?.getSource(SOURCE_ID) as GeoJSONSource | undefined;
    source?.setData(toGeoJson(state));

    const bounds = getRouteBounds(state);
    if (bounds && this.map) {
      this.map.fitBounds(bounds as LngLatBoundsLike, {
        padding: 56,
        duration: 400,
        maxZoom: 17,
      });
    }
  }

  destroy(): void {
    this.map?.remove();
    this.map = null;
  }

  private addLayers(): void {
    if (!this.map) return;

    this.map.addSource(SOURCE_ID, {
      type: "geojson",
      data: toGeoJson(this.state),
    });

    this.map.addLayer({
      id: "route-casing",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route"],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: { "line-color": "#080c12", "line-width": 10, "line-opacity": 0.95 },
    });
    this.map.addLayer({
      id: "route-glow-outer",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route"],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: {
        "line-color": "#39d353",
        "line-width": 14,
        "line-opacity": 0.1,
        "line-blur": 8,
      },
    });
    this.map.addLayer({
      id: "route-glow",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route"],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: {
        "line-color": "#39d353",
        "line-width": 7,
        "line-opacity": 0.28,
        "line-blur": 4,
      },
    });
    this.map.addLayer({
      id: "route-line",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route"],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: { "line-color": "#39d353", "line-width": 3.5 },
    });

    this.map.addLayer({
      id: "route-nodes",
      type: "circle",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "node"],
      paint: {
        "circle-radius": 3,
        "circle-color": "#39d353",
        "circle-opacity": 0.85,
        "circle-stroke-width": 1.5,
        "circle-stroke-color": "#0a0e14",
      },
    });

    for (const [role, color, radius] of [
      ["start", "#39d353", 7],
      ["finish", "#e2e8f0", 7],
      ["selected", "#d88a08", 8],
    ] as const) {
      this.map.addLayer({
        id: `route-${role}-halo`,
        type: "circle",
        source: SOURCE_ID,
        filter: ["==", ["get", "role"], role],
        paint: {
          "circle-radius": radius + 7,
          "circle-color": color,
          "circle-opacity": 0.18,
          "circle-blur": 0.5,
        },
      });
      this.map.addLayer({
        id: `route-${role}`,
        type: "circle",
        source: SOURCE_ID,
        filter: ["==", ["get", "role"], role],
        paint: {
          "circle-radius": radius,
          "circle-color": color,
          "circle-stroke-width": 2.5,
          "circle-stroke-color": "#080c12",
        },
      });
    }
  }
}
