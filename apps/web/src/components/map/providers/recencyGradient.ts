import type { ExpressionSpecification } from "maplibre-gl";

// Points are ordered oldest -> newest, so line-progress 0 is the oldest sample
// and 1 is the newest. The gradient brightens toward the newest end (the head),
// producing a comet-tail recency fade. Requires the source `lineMetrics: true`.
const FADE_STOPS: readonly [number, number][] = [
  [0, 0.08],
  [0.35, 0.28],
  [0.7, 0.6],
  [1, 1],
];

/**
 * Build a `line-gradient` expression that fades a fixed RGB color from faint
 * (oldest) to fully opaque (newest).
 *
 * @param rgb base color as an "r,g,b" string, e.g. "57,211,83"
 * @param maxAlpha alpha at the newest end (defaults to 1)
 */
export function recencyLineGradient(rgb: string, maxAlpha = 1): ExpressionSpecification {
  const stops = FADE_STOPS.flatMap(([progress, alpha]) => [
    progress,
    `rgba(${rgb},${(alpha * maxAlpha).toFixed(3)})`,
  ]);
  return ["interpolate", ["linear"], ["line-progress"], ...stops] as unknown as ExpressionSpecification;
}
