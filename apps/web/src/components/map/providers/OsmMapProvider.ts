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
import { buildRoutePolylines, buildTelemetrySegments } from "./routeSegments";

const SOURCE_ID = "route";
const DEFAULT_CENTER: [number, number] = [37.618, 55.751];

function toGeoJson(state: MapProviderState): GeoJSON.FeatureCollection {
  const features: GeoJSON.Feature[] = [];

  // Always draw one continuous MultiLineString (split only at GPS gaps) as the
  // base route, so dense per-pair telemetry segments dropping out on zoom out
  // never make the trace look broken.
  const lines = buildRoutePolylines(state.points);
  if (lines.length > 0) {
    features.push({
      type: "Feature",
      properties: { role: "route" },
      geometry: { type: "MultiLineString", coordinates: lines },
    });
  }

  // Speed-colored segments are an overlay on top of the base line.
  if (state.colorByTelemetry) {
    for (const segment of buildTelemetrySegments(state.points)) {
      features.push({
        type: "Feature",
        properties: {
          role: "route-segment",
          color: segment.color,
          opacity: segment.opacity,
        },
        geometry: {
          type: "LineString",
          coordinates: [[segment.from.lon, segment.from.lat], [segment.to.lon, segment.to.lat]],
        },
      });
    }
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
        layers: [{
          id: "osm-tiles",
          type: "raster",
          source: "osm",
          paint: {
            "raster-saturation": -0.9,
            "raster-contrast": 0.12,
            "raster-brightness-min": 0.16,
            "raster-brightness-max": 0.96,
            "raster-opacity": 0.82,
          },
        }],
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
      id: "route-casing",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route"],
      layout: {
        "line-cap": "round",
        "line-join": "round",
      },
      paint: {
        "line-color": "#142033",
        "line-width": 9,
        "line-opacity": 0.88,
      },
    });
    this.map.addLayer({
      id: "route-glow",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route"],
      layout: {
        "line-cap": "round",
        "line-join": "round",
      },
      paint: {
        "line-color": "#4cc43f",
        "line-width": 7,
        "line-opacity": 0.25,
        "line-blur": 4,
      },
    });
    this.map.addLayer({
      id: "route-line",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route"],
      layout: {
        "line-cap": "round",
        "line-join": "round",
      },
      paint: {
        "line-color": "#45b936",
        // Thinner when zoomed out, thicker when zoomed in.
        "line-width": ["interpolate", ["linear"], ["zoom"], 10, 2.5, 14, 4, 18, 6.5],
      },
    });
    this.map.addLayer({
      id: "route-segments",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route-segment"],
      layout: {
        "line-cap": "round",
        "line-join": "round",
      },
      paint: {
        "line-color": ["get", "color"],
        "line-width": 5,
        "line-opacity": ["get", "opacity"],
      },
    });

    for (const [role, color, radius] of [
      ["start", "#45b936", 7],
      ["finish", "#152238", 7],
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
          "circle-opacity": 0.16,
          "circle-blur": 0.35,
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
          "circle-stroke-width": 3,
          "circle-stroke-color": "#ffffff",
        },
      });
    }
  }
}
