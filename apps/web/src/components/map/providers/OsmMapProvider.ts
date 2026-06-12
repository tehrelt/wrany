import type {
  GeoJSONSource,
  LngLatBoundsLike,
  Map as MapLibreMap,
} from "maplibre-gl";
import type {
  MapProvider,
  MapProviderOptions,
  MapProviderState,
} from "./MapProvider";
import { getRouteBounds } from "./MapProvider";

const SOURCE_ID = "route";
const DEFAULT_CENTER: [number, number] = [37.618, 55.751];

function toGeoJson(state: MapProviderState): GeoJSON.FeatureCollection {
  const features: GeoJSON.Feature[] = [];

  if (state.points.length > 1) {
    features.push({
      type: "Feature",
      properties: { role: "route" },
      geometry: {
        type: "LineString",
        coordinates: state.points.map((point) => [point.lon, point.lat]),
      },
    });
  }

  const markers = [
    { role: "start", point: state.startPoint ?? state.points[0] },
    {
      role: "finish",
      point: state.finishPoint ?? state.points[state.points.length - 1],
    },
    { role: "selected", point: state.selectedPoint },
  ];

  for (const marker of markers) {
    if (!marker.point) continue;
    features.push({
      type: "Feature",
      properties: { role: marker.role },
      geometry: {
        type: "Point",
        coordinates: [marker.point.lon, marker.point.lat],
      },
    });
  }

  return { type: "FeatureCollection", features };
}

export class OsmMapProvider implements MapProvider {
  readonly type = "osm" as const;
  private map: MapLibreMap | null = null;
  private state: MapProviderState = { points: [] };

  async mount(
    container: HTMLElement,
    options: MapProviderOptions,
  ): Promise<void> {
    this.state = options;
    const { Map } = await import("maplibre-gl");
    if (options.signal.aborted) return;

    this.map = new Map({
      container,
      center: DEFAULT_CENTER,
      zoom: 11,
      style: {
        version: 8,
        sources: {
          osm: {
            type: "raster",
            tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
            tileSize: 256,
            attribution: "OpenStreetMap contributors",
          },
        },
        layers: [{ id: "osm-tiles", type: "raster", source: "osm" }],
      },
    });

    await new Promise<void>((resolve, reject) => {
      const onLoad = () => {
        this.map?.off("error", onInitialError);
        resolve();
      };
      const onInitialError = (event: { error?: Error }) => {
        this.map?.off("load", onLoad);
        reject(event.error ?? new Error("OSM map failed to load"));
      };
      this.map?.once("load", onLoad);
      this.map?.once("error", onInitialError);
    });
    if (options.signal.aborted) {
      this.destroy();
      return;
    }

    this.map.on("error", (event) => {
      options.onError(event.error?.message ?? "OSM tile loading failed");
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
      id: "route-line",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route"],
      paint: {
        "line-color": "#3b82f6",
        "line-width": 4,
      },
    });

    for (const [role, color, radius] of [
      ["start", "#22c55e", 7],
      ["finish", "#ef4444", 7],
      ["selected", "#f59e0b", 8],
    ] as const) {
      this.map.addLayer({
        id: `route-${role}`,
        type: "circle",
        source: SOURCE_ID,
        filter: ["==", ["get", "role"], role],
        paint: {
          "circle-radius": radius,
          "circle-color": color,
          "circle-stroke-width": 2,
          "circle-stroke-color": "#ffffff",
        },
      });
    }
  }
}
