import { describe, expect, it } from "vitest";
import type { LayerSpecification } from "maplibre-gl";
import {
  boundsCorners,
  computeCropRect,
  isCasingLayer,
  isRouteLayer,
  selectLayersToHideForTrace,
  selectNonRouteLayerIds,
  type GeoBounds,
} from "./maplibreExport";

function layer(id: string): LayerSpecification {
  return { id, type: "background" } as LayerSpecification;
}

describe("isRouteLayer", () => {
  it("matches every route-prefixed layer", () => {
    for (const id of [
      "route-casing",
      "route-glow",
      "route-glow-outer",
      "route-line",
      "route-segments",
      "route-nodes",
      "route-start",
      "route-start-halo",
      "route-finish",
      "route-selected-halo",
    ]) {
      expect(isRouteLayer(id)).toBe(true);
    }
  });

  it("rejects basemap layers", () => {
    for (const id of ["osm-tiles", "background", "water", "landuse", "road-primary"]) {
      expect(isRouteLayer(id)).toBe(false);
    }
  });
});

describe("selectNonRouteLayerIds", () => {
  it("returns only the basemap layers, preserving order", () => {
    const layers = [
      layer("background"),
      layer("water"),
      layer("route-casing"),
      layer("road-primary"),
      layer("route-line"),
      layer("route-start"),
    ];
    expect(selectNonRouteLayerIds(layers)).toEqual(["background", "water", "road-primary"]);
  });

  it("returns an empty list when every layer is a route layer", () => {
    expect(selectNonRouteLayerIds([layer("route-line"), layer("route-start")])).toEqual([]);
  });
});

describe("isCasingLayer", () => {
  it("matches casing layers only", () => {
    expect(isCasingLayer("route-casing")).toBe(true);
    expect(isCasingLayer("route-line")).toBe(false);
    expect(isCasingLayer("route-glow")).toBe(false);
  });
});

describe("selectLayersToHideForTrace", () => {
  it("hides the basemap and the route casing, keeps the trace and gates", () => {
    const layers = [
      layer("background"),
      layer("water"),
      layer("route-casing"),
      layer("route-glow"),
      layer("route-line"),
      layer("route-start"),
    ];
    expect(selectLayersToHideForTrace(layers)).toEqual(["background", "water", "route-casing"]);
  });
});

describe("boundsCorners", () => {
  it("returns the four corners of the geo bbox", () => {
    const bounds: GeoBounds = [[10, 20], [30, 40]];
    expect(boundsCorners(bounds)).toEqual([
      [10, 20],
      [10, 40],
      [30, 20],
      [30, 40],
    ]);
  });
});

describe("computeCropRect", () => {
  it("falls back to the whole canvas when no corners are given", () => {
    expect(computeCropRect([], 800, 600, 1)).toEqual({ sx: 0, sy: 0, sw: 800, sh: 600 });
  });

  it("frames the route with a clamped safe-zone and applies dpr", () => {
    // 200x100 css bbox -> pad = clamp(0.08*200, 28, 140) = 28 css.
    const corners = [
      { x: 100, y: 100 },
      { x: 300, y: 100 },
      { x: 100, y: 200 },
      { x: 300, y: 200 },
    ];
    const rect = computeCropRect(corners, 1600, 1200, 2);
    // left=(100-28)*2=144, top=(100-28)*2=144, right=(300+28)*2=656, bottom=(200+28)*2=456
    expect(rect).toEqual({ sx: 144, sy: 144, sw: 512, sh: 312 });
  });

  it("clamps the crop to the canvas bounds", () => {
    const corners = [
      { x: 0, y: 0 },
      { x: 50, y: 50 },
    ];
    const rect = computeCropRect(corners, 100, 100, 1);
    expect(rect.sx).toBe(0);
    expect(rect.sy).toBe(0);
    expect(rect.sx + rect.sw).toBeLessThanOrEqual(100);
    expect(rect.sy + rect.sh).toBeLessThanOrEqual(100);
  });
});
