import type {
  Map as MapLibreMap,
  GeoJSONSource,
  LngLatBoundsLike,
  MapLayerMouseEvent,
  Popup,
} from "maplibre-gl";
import type { ExportImageOptions, MapProvider, MapProviderOptions, MapProviderState } from "./MapProvider";
import { getRouteBounds } from "./MapProvider";
import { captureMaplibrePng } from "./maplibreExport";
import { recencyLineGradient } from "./recencyGradient";
import { buildActivityRuns, buildRoutePolylines, buildTelemetrySegments } from "./routeSegments";
import {
  briefPointHtml,
  fullPointHtml,
  pointProperties,
  type PointProperties,
} from "../pointDetails";

const SOURCE_ID = "route";
const DEFAULT_CENTER: [number, number] = [37.618, 55.751];

function parseMs(value: unknown): number | null {
  if (typeof value !== "string" || value === "") return null;
  const ms = Date.parse(value);
  return Number.isFinite(ms) ? ms : null;
}

function sameCalendarDay(a: number, b: number): boolean {
  const da = new Date(a);
  const db = new Date(b);
  return da.getFullYear() === db.getFullYear() && da.getMonth() === db.getMonth() && da.getDate() === db.getDate();
}

// Clock with seconds; prefixed with the date when the run crosses days so a
// multi-day span is not mistaken for a short one.
function formatClock(ms: number, withDate: boolean): string {
  const date = new Date(ms);
  if (withDate) {
    return date.toLocaleString(undefined, { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
  }
  return date.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function formatSpan(a: number, b: number): string {
  const sec = Math.max(0, Math.round((b - a) / 1000));
  if (sec < 60) return `${sec}s`;
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  if (m < 60) return s > 0 ? `${m}m ${s}s` : `${m}m`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

// Popup body for an activity run: type, time span (start -> end) and duration.
function activityHtml(props: Record<string, unknown>): string {
  const activity = String(props.activity ?? "unknown").toUpperCase();
  const a = parseMs(props.startAt);
  const b = parseMs(props.endAt);
  // Include the date on each end when the run crosses calendar days.
  const withDate = a != null && b != null && !sameCalendarDay(a, b);
  const range = a != null && b != null
    ? `${formatClock(a, withDate)} → ${formatClock(b, withDate)} · ${formatSpan(a, b)}`
    : "";
  return (
    `<div style="font:600 11px ui-monospace,monospace;letter-spacing:.04em">` +
    `<div style="color:#39d353;text-transform:uppercase">${activity}</div>` +
    (range ? `<div style="margin-top:2px;color:#94a3b8">${range}</div>` : "") +
    `</div>`
  );
}

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

  const highlightedLines = buildRoutePolylines(state.highlightedPath ?? []);
  if (highlightedLines.length > 0) {
    features.push({
      type: "Feature",
      properties: { role: "fast-highlight" },
      geometry: { type: "MultiLineString", coordinates: highlightedLines },
    });
  }

  // One feature per activity run, used for hover (type) + start-to-end highlight.
  for (const run of buildActivityRuns(state.points)) {
    features.push({
      type: "Feature",
      properties: {
        role: "activity",
        runId: run.runId,
        activity: run.activity,
        startAt: run.startAt ?? "",
        endAt: run.endAt ?? "",
        pointCount: run.pointCount,
      },
      geometry: { type: "MultiLineString", coordinates: run.lines },
    });
  }

  for (const p of state.points) {
    features.push({
      type: "Feature",
      properties: { role: "node", ...pointProperties(p) },
      geometry: { type: "Point", coordinates: [p.lon, p.lat] },
    });
  }

  const newest = state.points[state.points.length - 1];
  for (const [point, role] of [
    [state.startPoint ?? state.points[0], "start"],
    // In recency-fade mode the newest sample is the bright "head" instead of a
    // finish gate, so the default finish marker is suppressed.
    [state.fadeByRecency ? state.finishPoint : (state.finishPoint ?? newest), "finish"],
    [state.selectedPoint, "selected"],
    [state.fadeByRecency ? newest : undefined, "head"],
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
  private activityPopup: Popup | null = null;
  private fittedPoints: MapProviderState["points"] | null = null;

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
      // Required so the WebGL buffer survives compositing and can be exported.
      canvasContextAttributes: { preserveDrawingBuffer: true },
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
    const dimmed = (state.highlightedPath?.length ?? 0) > 1;
    if (this.map?.getLayer("route-line")) {
      this.map.setPaintProperty("route-casing", "line-opacity", dimmed ? 0.25 : 0.95);
      this.map.setPaintProperty("route-glow-outer", "line-opacity", dimmed ? 0.04 : 0.1);
      this.map.setPaintProperty("route-glow", "line-opacity", dimmed ? 0.08 : 0.28);
      this.map.setPaintProperty("route-line", "line-opacity", dimmed ? 0.22 : 1);
      this.map.setPaintProperty(
        "route-segments",
        "line-opacity",
        dimmed ? ["*", ["get", "opacity"], 0.18] : ["get", "opacity"],
      );
    }

    if (this.map?.getLayer("route-nodes")) {
      this.map.setLayoutProperty("route-nodes", "visibility", state.showPoints ? "visible" : "none");
    }

    const bounds = getRouteBounds(state);
    if (bounds && this.map && this.fittedPoints !== state.points) {
      this.fittedPoints = state.points;
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
    this.activityPopup?.remove();
    this.activityPopup = null;
    this.map?.remove();
    this.map = null;
    this.fittedPoints = null;
  }

  // Light up a single activity run (runId), or none with -1.
  private setActivityHighlight(runId: number): void {
    this.map?.setFilter("route-activity-highlight", [
      "all",
      ["==", ["get", "role"], "activity"],
      ["==", ["get", "runId"], runId],
    ]);
  }

  exportImage(options: ExportImageOptions): Promise<Blob> {
    if (!this.map) return Promise.reject(new Error("Map not ready"));
    return captureMaplibrePng(this.map, options, getRouteBounds(this.state));
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

    // Hover an activity run -> show its type/time span and light up the whole
    // run from its start to its end.
    map.on("mousemove", "route-activity-hit", (event) => {
      const props = event.features?.[0]?.properties as Record<string, unknown> | undefined;
      if (!props) return;
      map.getCanvas().style.cursor = "crosshair";
      this.setActivityHighlight(Number(props.runId));
      if (!this.activityPopup) {
        this.activityPopup = new PopupCtor({ closeButton: false, closeOnClick: false, offset: 12 });
      }
      this.activityPopup.setLngLat(event.lngLat).setHTML(activityHtml(props)).addTo(map);
    });
    map.on("mouseleave", "route-activity-hit", () => {
      map.getCanvas().style.cursor = "";
      this.setActivityHighlight(-1);
      this.activityPopup?.remove();
      this.activityPopup = null;
    });
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
      layout: { "line-cap": "round", "line-join": "round" },
      paint: fade
        ? { "line-gradient": recencyLineGradient("8,12,18", 0.95), "line-width": 10 }
        : { "line-color": "#080c12", "line-width": 10, "line-opacity": 0.95 },
    });
    this.map.addLayer({
      id: "route-glow-outer",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route"],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: fade
        ? { "line-gradient": recencyLineGradient("57,211,83", 0.1), "line-width": 14, "line-blur": 8 }
        : { "line-color": "#39d353", "line-width": 14, "line-opacity": 0.1, "line-blur": 8 },
    });
    this.map.addLayer({
      id: "route-glow",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route"],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: fade
        ? { "line-gradient": recencyLineGradient("57,211,83", 0.28), "line-width": 7, "line-blur": 4 }
        : { "line-color": "#39d353", "line-width": 7, "line-opacity": 0.28, "line-blur": 4 },
    });
    this.map.addLayer({
      id: "route-line",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "route"],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: {
        // Thinner when zoomed out, thicker when zoomed in.
        "line-width": ["interpolate", ["linear"], ["zoom"], 10, 2, 14, 3.5, 18, 6],
        ...(fade
          ? { "line-gradient": recencyLineGradient("57,211,83") }
          : { "line-color": "#39d353" }),
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
      id: "route-fast-highlight-glow",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "fast-highlight"],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: {
        "line-color": "#f59e0b",
        "line-width": ["interpolate", ["linear"], ["zoom"], 10, 12, 14, 16, 18, 22],
        "line-opacity": 0.32,
        "line-blur": 6,
      },
    });
    this.map.addLayer({
      id: "route-fast-highlight",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "fast-highlight"],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: {
        "line-color": "#fef3c7",
        "line-width": ["interpolate", ["linear"], ["zoom"], 10, 5, 14, 7, 18, 11],
        "line-opacity": 0.98,
      },
    });

    // Bright highlight of the hovered activity run (filtered to one runId).
    this.map.addLayer({
      id: "route-activity-highlight",
      type: "line",
      source: SOURCE_ID,
      filter: ["all", ["==", ["get", "role"], "activity"], ["==", ["get", "runId"], -1]],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: {
        "line-color": "#fef9c3",
        "line-width": ["interpolate", ["linear"], ["zoom"], 10, 5, 14, 7, 18, 11],
        "line-opacity": 0.9,
        "line-blur": 0.4,
      },
    });
    // Wide transparent hit target for hovering an activity run at any zoom.
    this.map.addLayer({
      id: "route-activity-hit",
      type: "line",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "activity"],
      layout: { "line-cap": "round", "line-join": "round" },
      paint: { "line-color": "#000000", "line-opacity": 0, "line-width": 18 },
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

    // Bright "head" on the newest sample of the current selection.
    this.map.addLayer({
      id: "route-head-glow",
      type: "circle",
      source: SOURCE_ID,
      filter: ["==", ["get", "role"], "head"],
      paint: {
        "circle-radius": 16,
        "circle-color": "#39d353",
        "circle-opacity": 0.22,
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
        "circle-color": "#d6ffe0",
        "circle-stroke-width": 3,
        "circle-stroke-color": "#39d353",
      },
    });
  }
}
