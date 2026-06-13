import type {
  Map as MapLibreMap,
  GeoJSONSource,
  LngLatBoundsLike,
  MapLayerMouseEvent,
  Popup,
} from "maplibre-gl";
import type { MapProvider, MapProviderOptions, MapProviderState } from "./MapProvider";
import { getRouteBounds } from "./MapProvider";
import { buildRoutePolylines, buildTelemetrySegments } from "./routeSegments";
import {
  briefPointHtml,
  fullPointHtml,
  pointProperties,
  type PointProperties,
} from "../pointDetails";

const SOURCE_ID = "route";
const DEFAULT_CENTER: [number, number] = [37.618, 55.751];

function toGeoJson(state: MapProviderState): GeoJSON.FeatureCollection {
  const features: GeoJSON.Feature[] = [];

  // Always draw one continuous MultiLineString (split only at GPS gaps) as the
  // base route. With thousands of points 1-2 m apart the per-pair telemetry
  // segments become sub-pixel and drop out on zoom out; this single geometry
  // renders reliably and keeps the trace visually continuous.
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
      properties: { role: "node", ...pointProperties(p) },
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
  private PopupCtor: typeof Popup | null = null;
  private hoverPopup: Popup | null = null;
  private detailPopup: Popup | null = null;

  async mount(container: HTMLElement, options: MapProviderOptions): Promise<void> {
    this.state = options;
    const { Map, Popup } = await import("maplibre-gl");
    this.PopupCtor = Popup;
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
    this.attachInteractions();
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
    this.hoverPopup?.remove();
    this.hoverPopup = null;
    this.detailPopup?.remove();
    this.detailPopup = null;
    this.map?.remove();
    this.map = null;
  }

  // Hover a GPS node -> brief tooltip (time + speed); click -> full details
  // popup. Nodes only exist at zoom >= 16, so this is active when zoomed in.
  private attachInteractions(): void {
    const map = this.map;
    const PopupCtor = this.PopupCtor;
    if (!map || !PopupCtor) return;

    const propsOf = (event: MapLayerMouseEvent): { coords: [number, number]; props: PointProperties } | null => {
      const feature = event.features?.[0];
      if (!feature || feature.geometry.type !== "Point") return null;
      const [lon, lat] = feature.geometry.coordinates;
      return { coords: [lon, lat] as [number, number], props: (feature.properties ?? {}) as unknown as PointProperties };
    };

    map.on("mouseenter", "route-nodes", () => {
      map.getCanvas().style.cursor = "pointer";
    });
    map.on("mouseleave", "route-nodes", () => {
      map.getCanvas().style.cursor = "";
      this.hoverPopup?.remove();
      this.hoverPopup = null;
    });
    map.on("mousemove", "route-nodes", (event) => {
      const hit = propsOf(event);
      if (!hit) return;
      if (!this.hoverPopup) {
        this.hoverPopup = new PopupCtor({ closeButton: false, closeOnClick: false, offset: 12 });
      }
      this.hoverPopup.setLngLat(hit.coords).setHTML(briefPointHtml(hit.props)).addTo(map);
    });
    map.on("click", "route-nodes", (event) => {
      const hit = propsOf(event);
      if (!hit) return;
      // Replace any previous detail popup; brief hover popup steps aside.
      this.hoverPopup?.remove();
      this.hoverPopup = null;
      this.detailPopup?.remove();
      this.detailPopup = new PopupCtor({ closeButton: true, closeOnClick: false, offset: 14 })
        .setLngLat(hit.coords)
        .setHTML(fullPointHtml(hit.props))
        .addTo(map);
    });
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
      paint: {
        "line-color": "#39d353",
        // Thinner when zoomed out, thicker when zoomed in.
        "line-width": ["interpolate", ["linear"], ["zoom"], 10, 2, 14, 3.5, 18, 6],
      },
    });
    this.map.addLayer({
      id: "route-segments",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route-segment"],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: {
        "line-color": ["get", "color"],
        "line-width": ["interpolate", ["linear"], ["zoom"], 10, 3, 14, 4.5, 18, 7],
        "line-opacity": ["get", "opacity"],
      },
    });

    this.map.addLayer({
      id: "route-nodes",
      type: "circle",
      source: SOURCE_ID,
      // Per-GPS-point dots are a debug overlay. Hidden on zoom out where they
      // would scatter and make the route look broken; visible only when zoomed
      // in (>= 16), where individual samples are useful.
      minzoom: 16,
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
