import { useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'
import { defaultTrackSettings, type TrackDisplaySettings } from './TrackingFilters'

// Maps each setting to its short search-param key.
const PARAM_KEYS: Record<keyof TrackDisplaySettings, string> = {
  speedThresholdMps: 'speed',
  minStaySec: 'stay',
  minMoveSec: 'move',
}

function parseNumber(value: string | null, fallback: number): number {
  if (value === null) return fallback
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

/**
 * Persists the PROCESSING SETUP (track display settings) in the URL search
 * params so the configuration survives reloads and can be shared via link.
 * Default values are omitted from the URL to keep it clean.
 */
export function useTrackSettings(): [TrackDisplaySettings, (next: TrackDisplaySettings) => void] {
  const [params, setParams] = useSearchParams()

  const settings: TrackDisplaySettings = {
    speedThresholdMps: parseNumber(params.get(PARAM_KEYS.speedThresholdMps), defaultTrackSettings.speedThresholdMps),
    minStaySec: parseNumber(params.get(PARAM_KEYS.minStaySec), defaultTrackSettings.minStaySec),
    minMoveSec: parseNumber(params.get(PARAM_KEYS.minMoveSec), defaultTrackSettings.minMoveSec),
  }

  const setSettings = useCallback((next: TrackDisplaySettings) => {
    setParams((prev) => {
      const updated = new URLSearchParams(prev)
      for (const field of Object.keys(PARAM_KEYS) as (keyof TrackDisplaySettings)[]) {
        const key = PARAM_KEYS[field]
        if (next[field] === defaultTrackSettings[field]) {
          updated.delete(key)
        } else {
          updated.set(key, String(next[field]))
        }
      }
      return updated
    }, { replace: true })
  }, [setParams])

  return [settings, setSettings]
}
