import type { Map as MapLibreMap, GeoJSONSource, LngLatBoundsLike } from "maplibre-gl";
import type { MapProvider, MapProviderOptions, MapProviderState } from "./MapProvider";
import { getRouteBounds } from "./MapProvider";

const SOURCE_ID = "route";
const DEFAULT_CENTER: [number, number] = [37.618, 55.751];

const DARK_STYLE = {
  version: 8 as const,
  glyphs: "https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf",
  sources: {
    openmaptiles: {
      type: "vector" as const,
      url: "https://tiles.openfreemap.org/planet",
    },
  },
  layers: [
    { id: "bg", type: "background", paint: { "background-color": "#0a0e14" } },
    {
      id: "water",
      type: "fill",
      source: "openmaptiles",
      "source-layer": "water",
      paint: { "fill-color": "#0c1828" },
    },
    {
      id: "waterway",
      type: "line",
      source: "openmaptiles",
      "source-layer": "waterway",
      paint: { "line-color": "#0d1c30", "line-width": 1 },
    },
    {
      id: "landcover",
      type: "fill",
      source: "openmaptiles",
      "source-layer": "landcover",
      paint: { "fill-color": "#0d1419", "fill-opacity": 0.7 },
    },
    {
      id: "landuse",
      type: "fill",
      source: "openmaptiles",
      "source-layer": "landuse",
      paint: { "fill-color": "#0e1520", "fill-opacity": 0.5 },
    },
    {
      id: "building",
      type: "fill",
      source: "openmaptiles",
      "source-layer": "building",
      minzoom: 13,
      paint: { "fill-color": "#141c2e", "fill-opacity": 0.9 },
    },
    {
      id: "road-minor",
      type: "line",
      source: "openmaptiles",
      "source-layer": "transportation",
      filter: ["in", ["get", "class"], ["literal", ["track", "path", "minor", "service"]]],
      minzoom: 13,
      paint: {
        "line-color": "#1a2538",
        "line-width": ["interpolate", ["linear"], ["zoom"], 13, 0.5, 18, 2],
      },
    },
    {
      id: "road-secondary",
      type: "line",
      source: "openmaptiles",
      "source-layer": "transportation",
      filter: ["in", ["get", "class"], ["literal", ["secondary", "tertiary"]]],
      minzoom: 10,
      paint: {
        "line-color": "#1e2c40",
        "line-width": ["interpolate", ["linear"], ["zoom"], 10, 0.5, 16, 3],
      },
    },
    {
      id: "road-primary",
      type: "line",
      source: "openmaptiles",
      "source-layer": "transportation",
      filter: ["==", ["get", "class"], "primary"],
      minzoom: 8,
      paint: {
        "line-color": "#2a3d58",
        "line-width": ["interpolate", ["linear"], ["zoom"], 8, 0.5, 16, 4],
      },
    },
    {
      id: "road-motorway",
      type: "line",
      source: "openmaptiles",
      "source-layer": "transportation",
      filter: ["in", ["get", "class"], ["literal", ["motorway", "trunk"]]],
      minzoom: 5,
      paint: {
        "line-color": "#314a6a",
        "line-width": ["interpolate", ["linear"], ["zoom"], 5, 0.5, 16, 5],
      },
    },
    {
      id: "place-city",
      type: "symbol",
      source: "openmaptiles",
      "source-layer": "place",
      filter: ["in", ["get", "class"], ["literal", ["city", "town"]]],
      layout: {
        "text-field": ["get", "name"],
        "text-font": ["Noto Sans Regular"],
        "text-size": ["interpolate", ["linear"], ["zoom"], 5, 9, 12, 13],
        "text-max-width": 8,
        "text-anchor": "center",
      },
      paint: {
        "text-color": "#4a6688",
        "text-halo-color": "#0a0e14",
        "text-halo-width": 1.5,
      },
    },
  ],
};

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

  for (const [point, role] of [
    [state.startPoint, "start"],
    [state.finishPoint, "finish"],
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
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      style: DARK_STYLE as any,
    });

    await new Promise<void>((resolve, reject) => {
      const onLoad = () => {
        this.map?.off("error", onInitialError);
        resolve();
      };
      const onInitialError = (event: { error?: Error }) => {
        this.map?.off("load", onLoad);
        reject(event.error ?? new Error("OpenFreeMap failed to load"));
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
      paint: { "line-color": "#0d1a2e", "line-width": 9, "line-opacity": 0.9 },
    });
    this.map.addLayer({
      id: "route-glow",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route"],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: {
        "line-color": "#39d353",
        "line-width": 8,
        "line-opacity": 0.22,
        "line-blur": 5,
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

    for (const [role, color, radius] of [
      ["start", "#39d353", 7],
      ["finish", "#0d1a2e", 7],
      ["selected", "#d88a08", 8],
    ] as const) {
      this.map.addLayer({
        id: `route-${role}-halo`,
        type: "circle",
        source: SOURCE_ID,
        filter: ["==", ["get", "role"], role],
        paint: {
          "circle-radius": radius + 6,
          "circle-color": color,
          "circle-opacity": 0.2,
          "circle-blur": 0.4,
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
          "circle-stroke-color": "#0a0e14",
        },
      });
    }
  }
}
