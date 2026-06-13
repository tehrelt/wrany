import { describe, expect, it, test } from 'vitest'
import { buildRoutePolylines, buildTelemetrySegments } from './routeSegments'

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
