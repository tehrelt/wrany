# EPIC 17: Route PNG Export

## Status

Implemented

## Goal

Add a web button that exports the currently displayed route map window to a PNG
image, in two modes:

- **With background** — full map (basemap tiles + route trace + gates).
- **Without background** — Strava-style transparent PNG with only the route
  trace and gate markers, no basemap.

## Context

The web analytics client renders routes through `RouteMap`, which delegates to
swappable `MapProvider` implementations. Two providers are MapLibre-based
(`maplibre-vector` / OpenFreeMap, `osm`) and two are Yandex-based
(`yandex`, `yandex-v2`). The canonical route view lives in the "Circuit
geometry" panel on `RoutesPage`.

Users want to share a route render as an image, like Strava's activity export
which offers a map snapshot and a transparent route overlay.

## Problem Analysis

- MapLibre renders everything (basemap, route line, glow/casing, gate circles)
  onto a single WebGL canvas. Capturing the canvas requires
  `preserveDrawingBuffer: true`; otherwise `toBlob`/`toDataURL` returns blank.
- The transparent ("no background") variant must keep the exact route styling
  the user sees. Re-implementing styling on a 2D canvas would drift from the
  MapLibre paint. Instead we hide every non-route layer, repaint, and capture —
  the WebGL clear color is transparent, so only route layers remain.
- DOM overlays (status chip, provider switch, legend, speed scale) are not on
  the canvas, so they are naturally excluded from the export — a clean image.
- Yandex providers do not expose a comparable capture path; export is offered
  only when a MapLibre provider is active.

## Best Practice Research

- MapLibre GL official guidance for image export is `preserveDrawingBuffer:true`
  plus capturing the canvas after a render frame.
- Layer-visibility toggling (`setLayoutProperty(id,'visibility','none')`) is the
  documented way to isolate features for a snapshot without rebuilding sources.
- `canvas.toBlob('image/png')` yields a transparent PNG when the GL buffer alpha
  is preserved and no opaque background layer paints.

## Solution Design

1. `MapProvider` gains an optional capability:
   `exportImage(opts: { background: boolean }): Promise<Blob>`.
2. Shared helper `providers/maplibreExport.ts`:
   - `captureMaplibrePng(map, { background })`.
   - For transparent mode: snapshot current layer visibilities, hide all layers
     whose id does not start with `route`, repaint, capture, then restore.
   - Capture uses a `triggerRepaint` + `once('render')` + `toBlob` sequence.
3. Both MapLibre providers (`OpenFreeMapProvider`, `OsmMapProvider`):
   - set `preserveDrawingBuffer: true` on the `Map` constructor;
   - implement `exportImage` by delegating to the shared helper.
4. `RouteMap` overlay gains an export control (two actions: with/without
   background), shown only while a MapLibre provider is active. It calls
   `providerRef.current?.exportImage` and triggers a browser download.
5. Download utility `lib/downloadBlob.ts` creates an object URL, clicks an
   anchor, and revokes the URL.

## Architecture Notes

- No backend changes. Pure web client feature.
- Capability is optional on the `MapProvider` interface; Yandex providers omit
  it and the UI hides the control accordingly.

## Tasks

- [x] Create epic branch and EPIC.md.
- [x] Add `exportImage` to `MapProvider` interface.
- [x] Add `maplibreExport.ts` shared capture helper.
- [x] Enable `preserveDrawingBuffer` (via `canvasContextAttributes`) + implement
      `exportImage` in both MapLibre providers.
- [x] Add `downloadBlob.ts` utility.
- [x] Add export control to `RouteMap` overlay.
- [x] Unit-test the layer-isolation logic.
- [x] Drop the route casing in transparent export (clean Strava-style trace).

## Acceptance Criteria

- A control on the route map exports a PNG with the basemap and the route.
- A second action exports a transparent PNG containing only the route trace and
  gate markers.
- The control is hidden when a Yandex provider is active.
- After a transparent export, the live map is visually unchanged (visibilities
  restored).
- Exported file downloads with a meaningful name.

## Test Plan

- Unit: `selectNonRouteLayerIds` returns every non-`route` layer id and excludes
  route layers; visibility restore map round-trips.
- Manual: export both modes on `maplibre-vector` and `osm`; confirm transparent
  PNG has alpha and map state is restored.

## Documentation Plan

- This EPIC.md (design + log).

## Implementation Log

- Created branch `epic/17-route-png-export` and EPIC.md.
- Added `exportImage` capability, `maplibreExport.ts` helper, `downloadBlob.ts`,
  and the `RouteMap` export control (PNG / No BG).
- Feedback: transparent export showed an ugly dark outline — now drops the
  `*-casing` layer for the No-BG trace.
- Feedback: output should fit the route's width/height with a safe-zone — crop
  the captured canvas to the projected route bbox plus a clamped safe-zone
  (`computeCropRect`).
- Feedback: capture must be independent of the live map zoom — before capture
  the camera is framed to the route bounds (`cameraForBounds` + `jumpTo`,
  north-up), tiles awaited for the background variant, then the camera is
  restored.
- Tests: 10 unit tests cover layer isolation, casing drop, bbox corners, and
  crop-rect math (safe-zone, dpr, clamping). Full web suite green.

## Final Report

_Pending._
