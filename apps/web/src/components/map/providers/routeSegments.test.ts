import { describe, expect, it, test } from 'vitest'
import { buildRoutePolylines, buildTelemetrySegments, percentileRank, speedColor } from './routeSegments'

describe('percentileRank', () => {
  it('returns 0 for empty or single-element lists', () => {
    expect(percentileRank([], 5)).toBe(0)
    expect(percentileRank([3], 3)).toBe(0)
  })

  it('maps the slowest to 0 and the fastest to 1', () => {
    const sorted = [1, 2, 3, 4, 5]
    expect(percentileRank(sorted, 1)).toBe(0)
    expect(percentileRank(sorted, 5)).toBe(1)
    expect(percentileRank(sorted, 3)).toBe(0.5)
  })

  it('spreads clustered values by distribution, not magnitude', () => {
    // Mostly-slow data with one fast burst: the median still lands mid-ramp.
    const sorted = [0.01, 0.02, 0.02, 0.03, 0.03, 0.04, 3.5].sort((a, b) => a - b)
    expect(percentileRank(sorted, 0.03)).toBeGreaterThan(0.3)
    expect(percentileRank(sorted, 0.03)).toBeLessThan(0.8)
  })
})

describe('speedColor', () => {
  it('returns distinct ramp endpoints for slow vs fast', () => {
    expect(speedColor(0)).toBe('rgb(37, 99, 235)')
    expect(speedColor(1)).toBe('rgb(239, 68, 68)')
  })

  it('interpolates between stops', () => {
    // Halfway between blue(0) and cyan(0.25) stops.
    expect(speedColor(0.125)).toBe('rgb(22, 141, 224)')
  })
})

describe('buildTelemetrySegments', () => {
  test('fades old segments and colors by speed', () => {
    const segments = buildTelemetrySegments([
      { lat: 55.75, lon: 37.6, recordedAt: '2026-01-01T00:00:00Z', speedMps: 1 },
      { lat: 55.751, lon: 37.601, recordedAt: '2026-01-01T00:01:00Z', speedMps: 5 },
      { lat: 55.752, lon: 37.602, recordedAt: '2026-01-01T00:02:00Z', speedMps: 10 },
    ])

    expect(segments).toHaveLength(2)
    expect(segments[0].opacity).toBeLessThan(segments[1].opacity)
    expect(segments[0].color).not.toBe(segments[1].color)
  })

  it('connects consecutive points across processing segments', () => {
    const segments = buildTelemetrySegments([
      { lat: 55, lon: 37, segmentId: 1 },
      { lat: 55.001, lon: 37.001, segmentId: 1 },
      { lat: 55.01, lon: 37.01, segmentId: 2 },
      { lat: 55.011, lon: 37.011, segmentId: 2 },
    ])

    expect(segments).toHaveLength(3)
    expect(segments[0].from.segmentId).toBe(1)
    expect(segments[1].from.segmentId).toBe(1)
    expect(segments[1].to.segmentId).toBe(2)
  })

  it('drops a segment across a GPS teleport', () => {
    const segments = buildTelemetrySegments([
      { lat: 55.75, lon: 37.6, recordedAt: '2026-01-01T00:00:00Z' },
      { lat: 55.751, lon: 37.601, recordedAt: '2026-01-01T00:00:10Z' },
      // ~80 km away within 10s -> impossible -> not connected.
      { lat: 56.5, lon: 37.6, recordedAt: '2026-01-01T00:00:20Z' },
    ])

    expect(segments).toHaveLength(1)
  })
})

describe('buildRoutePolylines', () => {
  it('splits the route into runs at a teleport', () => {
    const lines = buildRoutePolylines([
      { lat: 55.75, lon: 37.6, recordedAt: '2026-01-01T00:00:00Z' },
      { lat: 55.751, lon: 37.601, recordedAt: '2026-01-01T00:00:10Z' },
      { lat: 56.5, lon: 37.6, recordedAt: '2026-01-01T00:00:20Z' },
      { lat: 56.501, lon: 37.601, recordedAt: '2026-01-01T00:00:30Z' },
    ])

    expect(lines).toHaveLength(2)
    expect(lines[0]).toHaveLength(2)
    expect(lines[1]).toHaveLength(2)
  })

  it('keeps one continuous run when there is no gap', () => {
    const lines = buildRoutePolylines([
      { lat: 55.75, lon: 37.6 },
      { lat: 55.751, lon: 37.601 },
      { lat: 55.752, lon: 37.602 },
    ])

    expect(lines).toHaveLength(1)
    expect(lines[0]).toEqual([
      [37.6, 55.75],
      [37.601, 55.751],
      [37.602, 55.752],
    ])
  })
})
