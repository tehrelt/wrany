import type {
  GeoJSONSource,
  LngLatBoundsLike,
  Map as MapLibreMap,
} from "maplibre-gl";
import type {
  ExportImageOptions,
  MapProvider,
  MapProviderOptions,
  MapProviderState,
} from "./MapProvider";
import { getRouteBounds } from "./MapProvider";
import { captureMaplibrePng } from "./maplibreExport";
import { recencyLineGradient } from "./recencyGradient";
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

  for (const p of state.points) {
    features.push({
      type: "Feature",
      properties: { role: "node" },
      geometry: { type: "Point", coordinates: [p.lon, p.lat] },
    });
  }

  const newest = state.points[state.points.length - 1];
  const markers = [
    { role: "start", point: state.startPoint ?? state.points[0] },
    {
      role: "finish",
      // In recency-fade mode the newest sample is the bright "head" instead.
      point: state.fadeByRecency ? state.finishPoint : (state.finishPoint ?? newest),
    },
    { role: "selected", point: state.selectedPoint },
    { role: "head", point: state.fadeByRecency ? newest : undefined },
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
      // Required so the WebGL buffer survives compositing and can be exported.
      canvasContextAttributes: { preserveDrawingBuffer: true },
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

    if (this.map?.getLayer("route-nodes")) {
      this.map.setLayoutProperty("route-nodes", "visibility", state.showPoints ? "visible" : "none");
    }

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

  exportImage(options: ExportImageOptions): Promise<Blob> {
    if (!this.map) return Promise.reject(new Error("Map not ready"));
    return captureMaplibrePng(this.map, options, getRouteBounds(this.state));
  }

  private addLayers(): void {
    if (!this.map) return;

    const fade = this.state.fadeByRecency === true;

    this.map.addSource(SOURCE_ID, {
      type: "geojson",
      // lineMetrics enables the line-progress used by the recency gradient.
      lineMetrics: true,
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
      paint: fade
        ? { "line-gradient": recencyLineGradient("20,32,51", 0.88), "line-width": 9 }
        : { "line-color": "#142033", "line-width": 9, "line-opacity": 0.88 },
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
      paint: fade
        ? { "line-gradient": recencyLineGradient("76,196,63", 0.25), "line-width": 7, "line-blur": 4 }
        : { "line-color": "#4cc43f", "line-width": 7, "line-opacity": 0.25, "line-blur": 4 },
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
        // Thinner when zoomed out, thicker when zoomed in.
        "line-width": ["interpolate", ["linear"], ["zoom"], 10, 2.5, 14, 4, 18, 6.5],
        ...(fade
          ? { "line-gradient": recencyLineGradient("69,185,54") }
          : { "line-color": "#45b936" }),
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

    this.map.addLayer({
      id: "route-nodes",
      type: "circle",
      source: SOURCE_ID,
      // Per-GPS-point dots are toggled via showPoints (hidden by default) and,
      // even when on, only appear when zoomed in (>= 16) so they never scatter
      // across the trace on zoom-out.
      minzoom: 16,
      layout: { visibility: this.state.showPoints ? "visible" : "none" },
      filter: ["==", ["get", "role"], "node"],
      paint: {
        "circle-radius": 3,
        "circle-color": "#45b936",
        "circle-opacity": 0.85,
        "circle-stroke-width": 1.5,
        "circle-stroke-color": "#ffffff",
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

    // Bright "head" on the newest sample of the current selection.
    this.map.addLayer({
      id: "route-head-glow",
      type: "circle",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "head"],
      paint: {
        "circle-radius": 15,
        "circle-color": "#45b936",
        "circle-opacity": 0.2,
        "circle-blur": 0.6,
      },
    });
    this.map.addLayer({
      id: "route-head",
      type: "circle",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "head"],
      paint: {
        "circle-radius": 7,
        "circle-color": "#ffffff",
        "circle-stroke-width": 3,
        "circle-stroke-color": "#45b936",
      },
    });
  }
}
