import { describe, expect, it } from 'vitest'
import { haversineMeters, shouldBreakSegment, splitByGap } from './geoGap'

describe('haversineMeters', () => {
  it('measures roughly 111m for 0.001 degree of latitude', () => {
    const meters = haversineMeters({ lat: 55, lon: 37 }, { lat: 55.001, lon: 37 })
    expect(meters).toBeGreaterThan(100)
    expect(meters).toBeLessThan(120)
  })
})

describe('shouldBreakSegment', () => {
  it('keeps nearby samples connected', () => {
    expect(
      shouldBreakSegment(
        { lat: 55.75, lon: 37.6, recordedAt: '2026-01-01T00:00:00Z' },
        { lat: 55.751, lon: 37.601, recordedAt: '2026-01-01T00:00:05Z' },
      ),
    ).toBe(false)
  })

  it('breaks on an impossible implied speed (teleport)', () => {
    expect(
      shouldBreakSegment(
        { lat: 55.75, lon: 37.6, recordedAt: '2026-01-01T00:00:00Z' },
        { lat: 56.5, lon: 37.6, recordedAt: '2026-01-01T00:00:05Z' },
      ),
    ).toBe(true)
  })

  it('stays connected through a long pause in place', () => {
    // A long stop barely moves -> the line must remain continuous through it.
    expect(
      shouldBreakSegment(
        { lat: 55.75, lon: 37.6, recordedAt: '2026-01-01T00:00:00Z' },
        { lat: 55.7501, lon: 37.6001, recordedAt: '2026-01-01T00:20:00Z' },
      ),
    ).toBe(false)
  })

  it('falls back to distance when timestamps are missing', () => {
    expect(shouldBreakSegment({ lat: 55, lon: 37 }, { lat: 55.01, lon: 37 })).toBe(false)
    expect(shouldBreakSegment({ lat: 55, lon: 37 }, { lat: 55.05, lon: 37 })).toBe(true)
  })
})

describe('splitByGap', () => {
  it('keeps a continuous run as one group', () => {
    const runs = splitByGap(
      [
        { lat: 55.75, lon: 37.6 },
        { lat: 55.751, lon: 37.601 },
        { lat: 55.752, lon: 37.602 },
      ],
      s => s,
    )
    expect(runs).toHaveLength(1)
    expect(runs[0]).toHaveLength(3)
  })

  it('splits at a teleport gap', () => {
    const runs = splitByGap(
      [
        { lat: 55.75, lon: 37.6 },
        { lat: 56.5, lon: 37.6 },
      ],
      s => s,
    )
    expect(runs).toHaveLength(2)
  })
})
