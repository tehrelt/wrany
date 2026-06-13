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
      style: "https://tiles.openfreemap.org/styles/positron",
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

    this.applyDarkTheme();

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

  private applyDarkTheme(): void {
    if (!this.map) return;
    const layers = this.map.getStyle()?.layers ?? [];

    for (const layer of layers) {
      const sl = (layer as { "source-layer"?: string })["source-layer"];

      try {
        if (layer.type === "background") {
          this.map.setPaintProperty(layer.id, "background-color", "#0a0e14");
        } else if (layer.type === "fill") {
          if (sl === "water") {
            this.map.setPaintProperty(layer.id, "fill-color", "#0c1828");
            this.map.setPaintProperty(layer.id, "fill-opacity", 1);
          } else if (sl === "building" || sl === "buildings") {
            this.map.setPaintProperty(layer.id, "fill-color", "#141c2e");
            this.map.setPaintProperty(layer.id, "fill-opacity", 0.9);
          } else {
            this.map.setPaintProperty(layer.id, "fill-color", "#0d1419");
            this.map.setPaintProperty(layer.id, "fill-opacity", 0.6);
          }
        } else if (layer.type === "line") {
          if (sl === "waterway") {
            this.map.setPaintProperty(layer.id, "line-color", "#0e1e32");
          } else {
            this.map.setPaintProperty(layer.id, "line-color", "#1e2c40");
          }
        } else if (layer.type === "symbol") {
          if (sl === "poi" || sl === "pois") {
            this.map.setLayoutProperty(layer.id, "visibility", "none");
          } else {
            this.map.setPaintProperty(layer.id, "text-color", "#4a6688");
            this.map.setPaintProperty(layer.id, "text-halo-color", "#0a0e14");
            this.map.setPaintProperty(layer.id, "text-halo-width", 1.5);
          }
        }
      } catch {
        // skip layers where the property doesn't apply
      }
    }
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
