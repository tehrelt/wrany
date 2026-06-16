import type { Map as MapLibreMap, LayerSpecification, LngLatLike } from "maplibre-gl";
import type { ExportImageOptions } from "./MapProvider";

// Route layers are the only ones kept when exporting without a background.
// Every layer id our providers add for the trace/gates starts with "route".
const ROUTE_LAYER_PREFIX = "route";

export function isRouteLayer(id: string): boolean {
  return id.startsWith(ROUTE_LAYER_PREFIX);
}

// The casing is a dark stroke drawn under the trace to lift it off the basemap.
// On a transparent export it reads as an ugly outline, so it is dropped.
export function isCasingLayer(id: string): boolean {
  return id.endsWith("-casing");
}

/** Ids of every layer that is NOT part of the route trace (i.e. the basemap). */
export function selectNonRouteLayerIds(layers: readonly LayerSpecification[]): string[] {
  return layers.filter((layer) => !isRouteLayer(layer.id)).map((layer) => layer.id);
}

/**
 * Layers to hide for a transparent ("no background") export: the basemap plus
 * the route casing, leaving a clean Strava-style trace.
 */
export function selectLayersToHideForTrace(layers: readonly LayerSpecification[]): string[] {
  return layers
    .filter((layer) => !isRouteLayer(layer.id) || isCasingLayer(layer.id))
    .map((layer) => layer.id);
}

export type GeoBounds = [[number, number], [number, number]];

export interface PixelPoint {
  x: number;
  y: number;
}

export interface CropRect {
  sx: number;
  sy: number;
  sw: number;
  sh: number;
}

// Safe-zone padding around the route, as a share of its larger pixel dimension,
// clamped so tiny routes still breathe and huge ones don't get a giant frame.
const SAFE_ZONE_RATIO = 0.08;
const SAFE_ZONE_MIN_CSS = 28;
const SAFE_ZONE_MAX_CSS = 140;

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

/** The four [lon,lat] corners of a geo bounding box. */
export function boundsCorners(bounds: GeoBounds): [number, number][] {
  const [[minLon, minLat], [maxLon, maxLat]] = bounds;
  return [
    [minLon, minLat],
    [minLon, maxLat],
    [maxLon, minLat],
    [maxLon, maxLat],
  ];
}

/**
 * Crop rectangle (in device pixels) tightly framing the projected route with a
 * pleasant safe-zone, clamped to the canvas. Pure math so it can be unit-tested.
 *
 * @param corners projected route corners in CSS pixels
 * @param canvasW canvas width in device pixels
 * @param canvasH canvas height in device pixels
 * @param dpr device-pixels-per-CSS-pixel
 */
export function computeCropRect(
  corners: readonly PixelPoint[],
  canvasW: number,
  canvasH: number,
  dpr: number,
): CropRect {
  if (corners.length === 0) {
    return { sx: 0, sy: 0, sw: canvasW, sh: canvasH };
  }

  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const { x, y } of corners) {
    minX = Math.min(minX, x);
    minY = Math.min(minY, y);
    maxX = Math.max(maxX, x);
    maxY = Math.max(maxY, y);
  }

  const widthCss = maxX - minX;
  const heightCss = maxY - minY;
  const padCss = clamp(SAFE_ZONE_RATIO * Math.max(widthCss, heightCss), SAFE_ZONE_MIN_CSS, SAFE_ZONE_MAX_CSS);

  const left = clamp((minX - padCss) * dpr, 0, canvasW);
  const top = clamp((minY - padCss) * dpr, 0, canvasH);
  const right = clamp((maxX + padCss) * dpr, 0, canvasW);
  const bottom = clamp((maxY + padCss) * dpr, 0, canvasH);

  const sx = Math.round(left);
  const sy = Math.round(top);
  const sw = Math.max(1, Math.round(right - left));
  const sh = Math.max(1, Math.round(bottom - top));
  return { sx, sy, sw, sh };
}

// Padding (CSS px) used when framing the route for export, before the crop's
// safe-zone is applied. Small so it does not clip the gate marker circles.
const FIT_PADDING_CSS = 24;
const IDLE_TIMEOUT_MS = 5000;

interface SavedCamera {
  center: [number, number];
  zoom: number;
  bearing: number;
  pitch: number;
}

function canvasToPng(canvas: HTMLCanvasElement): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (blob) resolve(blob);
      else reject(new Error("Canvas export failed (toBlob returned null)"));
    }, "image/png");
  });
}

function cropToPng(source: HTMLCanvasElement, rect: CropRect): Promise<Blob> {
  const cropped = document.createElement("canvas");
  cropped.width = rect.sw;
  cropped.height = rect.sh;
  const ctx = cropped.getContext("2d");
  if (!ctx) return Promise.reject(new Error("2D context unavailable for crop"));
  ctx.drawImage(source, rect.sx, rect.sy, rect.sw, rect.sh, 0, 0, rect.sw, rect.sh);
  return canvasToPng(cropped);
}

function saveCamera(map: MapLibreMap): SavedCamera {
  const center = map.getCenter();
  return { center: [center.lng, center.lat], zoom: map.getZoom(), bearing: map.getBearing(), pitch: map.getPitch() };
}

// Frame the route independently of the user's current zoom/pan, so the export
// is deterministic. North-up, no pitch — keeps the crop projection exact.
function frameToRoute(map: MapLibreMap, bounds: GeoBounds): void {
  const camera = map.cameraForBounds(bounds as [[number, number], [number, number]], {
    padding: FIT_PADDING_CSS,
    bearing: 0,
    pitch: 0,
  });
  if (camera) map.jumpTo(camera);
}

// Wait until tiles for the framed view have loaded, so the basemap is complete
// in the capture. Bounded by a timeout to avoid hanging on slow tiles.
function waitIdle(map: MapLibreMap): Promise<void> {
  return new Promise((resolve) => {
    if (map.loaded() && map.areTilesLoaded()) {
      resolve();
      return;
    }
    const onIdle = () => {
      clearTimeout(timer);
      resolve();
    };
    const timer = setTimeout(() => {
      map.off("idle", onIdle);
      resolve();
    }, IDLE_TIMEOUT_MS);
    map.once("idle", onIdle);
  });
}

// Grab the freshly composited WebGL buffer on the next render frame and crop it
// to the route bounds. preserveDrawingBuffer must be enabled on the map.
function captureFrame(map: MapLibreMap, bounds: GeoBounds | null): Promise<Blob> {
  return new Promise((resolve, reject) => {
    map.once("render", () => {
      const canvas = map.getCanvas();
      if (!bounds) {
        void canvasToPng(canvas).then(resolve, reject);
        return;
      }
      const corners = boundsCorners(bounds).map((corner) => map.project(corner as LngLatLike));
      const dpr = canvas.clientWidth > 0 ? canvas.width / canvas.clientWidth : 1;
      const rect = computeCropRect(corners, canvas.width, canvas.height, dpr);
      void cropToPng(canvas, rect).then(resolve, reject);
    });
    map.triggerRepaint();
  });
}

/**
 * Render the route to a PNG, framed to the route bounds independently of the
 * live map zoom/pan, then cropped to those bounds plus a safe-zone. With
 * background: the full map. Without background: hide every basemap layer and the
 * casing, capturing only the route trace on a transparent buffer. The map's
 * camera and layer visibilities are restored afterwards.
 */
export async function captureMaplibrePng(
  map: MapLibreMap,
  { background }: ExportImageOptions,
  bounds: GeoBounds | null,
): Promise<Blob> {
  const camera = saveCamera(map);
  const layers = map.getStyle().layers ?? [];
  const previous = new Map<string, "visible" | "none">();

  if (!background) {
    for (const id of selectLayersToHideForTrace(layers)) {
      const visibility = (map.getLayoutProperty(id, "visibility") as "visible" | "none" | undefined) ?? "visible";
      previous.set(id, visibility);
      map.setLayoutProperty(id, "visibility", "none");
    }
  }

  try {
    if (bounds) frameToRoute(map, bounds);
    // Only the basemap needs tiles; the transparent trace is already loaded.
    if (background) await waitIdle(map);
    return await captureFrame(map, bounds);
  } finally {
    for (const [id, visibility] of previous) {
      map.setLayoutProperty(id, "visibility", visibility);
    }
    map.jumpTo(camera);
  }
}
