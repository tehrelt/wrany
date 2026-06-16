import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { FastSegmentsPanel, fastSegmentId } from './FastSegmentsPanel'
import type { FastSegment } from './trackingApi'

const segment: FastSegment = {
  rank: 1,
  device_id: 'device-1',
  started_at: '2026-06-15T12:00:00Z',
  ended_at: '2026-06-15T12:00:30Z',
  duration_sec: 30,
  distance_m: 120,
  avg_speed_mps: 4,
  baseline_speed_mps: 2,
  uplift_percent: 100,
  points: [
    { lat: 55, lon: 37, recorded_at: '2026-06-15T12:00:00Z' },
    { lat: 55.001, lon: 37.001, recorded_at: '2026-06-15T12:00:30Z' },
  ],
}

describe('FastSegmentsPanel', () => {
  it('activates a segment on hover and keyboard focus', async () => {
    const user = userEvent.setup()
    const onActiveChange = vi.fn()
    render(
      <FastSegmentsPanel
        items={[segment]}
        loading={false}
        preset="normal"
        limit={5}
        activeId={null}
        onPresetChange={vi.fn()}
        onLimitChange={vi.fn()}
        onActiveChange={onActiveChange}
      />,
    )

    const item = screen.getByRole('button', { name: /rank 1/i })
    await user.hover(item)
    expect(onActiveChange).toHaveBeenLastCalledWith(fastSegmentId(segment))

    await user.unhover(item)
    expect(onActiveChange).toHaveBeenLastCalledWith(null)

    await user.tab()
    await user.tab()
    await user.tab()
    await user.tab()
    await user.tab()
    await user.tab()
    await user.tab()
    expect(item).toHaveFocus()
    expect(onActiveChange).toHaveBeenLastCalledWith(fastSegmentId(segment))
  })
})
