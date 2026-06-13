import { describe, expect, it } from 'vitest'
import type { TrackSegment } from '@/features/tracking/trackingApi'
import { trackToGeoJSON } from './trackingGeoJson'

function move(segmentId: number, eventId: string, lat: number): TrackSegment {
  return {
    kind: 'move',
    segment_id: segmentId,
    event_id: eventId,
    recorded_at: '2026-06-13T12:00:00Z',
    period_end: '2026-06-13T12:00:00Z',
    lat,
    lon: 37,
    speed_mps: 1,
    accuracy_m: 5,
    stay_duration_sec: 0,
    merged_count: 1,
  }
}

describe('trackToGeoJSON', () => {
  it('creates one continuous line', () => {
    const { moveGeo } = trackToGeoJSON([
      move(1, 'a', 55),
      move(1, 'b', 55.001),
      move(2, 'c', 55.01),
      move(2, 'd', 55.011),
    ])

    const lines = moveGeo.features.filter(feature => feature.geometry.type === 'LineString')
    expect(lines).toHaveLength(1)
    expect(lines[0].geometry).toMatchObject({
      type: 'LineString',
      coordinates: [
        [37, 55],
        [37, 55.001],
        [37, 55.01],
        [37, 55.011],
      ],
    })
  })
})
