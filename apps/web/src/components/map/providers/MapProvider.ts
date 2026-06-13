export type MapProviderType = "yandex" | "yandex-v2" | "osm" | "maplibre-vector" | "auto";
export type ResolvedMapProviderType = Exclude<MapProviderType, "auto">;

export interface MapPoint {
  lat: number;
  lon: number;
  recordedAt?: string;
}

export interface MapProviderState {
  points: MapPoint[];
  selectedPoint?: MapPoint | null;
  startPoint?: MapPoint;
  finishPoint?: MapPoint;
}

export interface MapProviderOptions extends MapProviderState {
  onError: (reason: string) => void;
  signal: AbortSignal;
}

export interface MapProvider {
  readonly type: ResolvedMapProviderType;
  mount(container: HTMLElement, options: MapProviderOptions): Promise<void>;
  update(state: MapProviderState): void;
  destroy(): void;
}

export function getRouteBounds(
  state: MapProviderState,
): [[number, number], [number, number]] | null {
  const points = [
    ...state.points,
    ...(state.startPoint ? [state.startPoint] : []),
    ...(state.finishPoint ? [state.finishPoint] : []),
    ...(state.selectedPoint ? [state.selectedPoint] : []),
  ];

  if (points.length === 0) return null;

  let minLat = points[0].lat;
  let maxLat = points[0].lat;
  let minLon = points[0].lon;
  let maxLon = points[0].lon;

  for (const point of points) {
    minLat = Math.min(minLat, point.lat);
    maxLat = Math.max(maxLat, point.lat);
    minLon = Math.min(minLon, point.lon);
    maxLon = Math.max(maxLon, point.lon);
  }

  if (minLat === maxLat) {
    minLat -= 0.001;
    maxLat += 0.001;
  }
  if (minLon === maxLon) {
    minLon -= 0.001;
    maxLon += 0.001;
  }

  return [
    [minLon, minLat],
    [maxLon, maxLat],
  ];
}
